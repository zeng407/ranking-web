package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"2pick.app/backend/internal/admin"
	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/authoring"
)

// The moderation back office, from routes/admin.php and routes/admin-api.php.
//
// THIS IS THE ONLY PLACE THE ADMIN ROLE IS CHECKED, AND EVERY ROUTE BELOW MUST GO THROUGH
// requireAdmin. The service underneath acts on rows that belong to other people and does
// not re-check the caller — see the internal/admin package comment — so a route registered
// with requireAuth alone would hand any signed-in account the whole back office.
//
// The pages themselves are the SPA's. What Laravel served as Blade views under /admin is
// served here as JSON plus, when a bundle is configured, the permission-gated static files
// in assets.go.

// AdminService is the slice of admin.Service this layer uses.
type AdminService interface {
	Posts(ctx context.Context, page int) (admin.PostPage, error)
	Post(ctx context.Context, serial string) (authoring.Post, error)
	UpdatePost(ctx context.Context, serial string, edit admin.PostEdit) (authoring.Post, error)
	DeletePost(ctx context.Context, serial string) error
	Elements(ctx context.Context, serial string, query authoring.ElementQuery) (authoring.ElementPage, error)
	EditElement(ctx context.Context, elementID int64, edit authoring.ElementEdit) (authoring.Element, error)
	DeleteElement(ctx context.Context, elementID int64) error

	Users(ctx context.Context, keyword string, page int) (admin.UserPage, error)
	BanUser(ctx context.Context, userID int64) error
	UnbanUser(ctx context.Context, userID int64) error

	CarouselItems(ctx context.Context) ([]admin.CarouselItem, error)
	CreateCarouselItem(ctx context.Context, draft admin.CarouselDraft) (admin.CarouselItem, error)
	UpdateCarouselItem(ctx context.Context, itemID int64, edit admin.CarouselEdit) (admin.CarouselItem, error)
	DeleteCarouselItem(ctx context.Context, itemID int64) error
	ReorderCarouselItems(ctx context.Context, positions []admin.CarouselPosition) error

	Announcement(ctx context.Context) (admin.Announcement, bool, error)
	PublishAnnouncement(ctx context.Context, draft admin.AnnouncementDraft) (admin.Announcement, error)
}

type adminPostResponse struct {
	Serial       string `json:"serial"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	AccessPolicy string `json:"access_policy"`
	Censored     bool   `json:"is_censored"`
	PlayCount    int    `json:"play_count"`
	// The owner, which is the column an author's own listing has no reason to carry: the
	// moderation list is the only view where a post's account matters.
	Owner     adminOwnerResponse `json:"owner"`
	CreatedAt string             `json:"created_at,omitempty"`
}

type adminOwnerResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type adminPostListResponse struct {
	Posts   []adminPostResponse `json:"posts"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"per_page"`
}

// adminPostEditRequest is the author's draft plus the one field an author may never set.
type adminPostEditRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	AccessPolicy string   `json:"access_policy"`
	Password     string   `json:"password"`
	Tags         []string `json:"tags"`
	// Censored is a pointer so leaving it out means "do not touch it" rather than
	// "uncensor", which is what the original's `sometimes` rule meant.
	Censored *bool `json:"is_censored"`
}

type adminUserResponse struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	AvatarURL string   `json:"avatar_url"`
	Roles     []string `json:"roles"`
	PostCount int      `json:"post_count"`
	CreatedAt string   `json:"created_at,omitempty"`
}

type adminUserListResponse struct {
	Users   []adminUserResponse `json:"users"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"per_page"`
}

type adminCarouselResponse struct {
	ID          int64  `json:"id"`
	Position    int    `json:"position"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	VideoURL    string `json:"video_url"`
	VideoSource string `json:"video_source,omitempty"`
	VideoID     string `json:"video_id,omitempty"`
	StartSecond *int   `json:"video_start_second"`
	EndSecond   *int   `json:"video_end_second"`
	Active      bool   `json:"is_active"`
}

type adminCarouselCreateRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	VideoURL    string  `json:"video_url"`
	StartSecond *int    `json:"video_start_second"`
	EndSecond   *int    `json:"video_end_second"`
	Active      *bool   `json:"is_active"`
}

type adminCarouselEditRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	StartSecond *int    `json:"video_start_second"`
	EndSecond   *int    `json:"video_end_second"`
	Active      *bool   `json:"is_active"`
}

type adminCarouselReorderRequest struct {
	Items []struct {
		ID       int64 `json:"id"`
		Position int   `json:"position"`
	} `json:"items"`
}

type adminAnnouncementResponse struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	ImageURL    string `json:"image_url"`
	CreatedAt   string `json:"created_at"`
	KeepMinutes int    `json:"keep_minutes"`
}

type adminAnnouncementRequest struct {
	Content  string `json:"content"`
	ImageURL string `json:"image_url"`
	// Minutes of 0 means the service's default.
	Minutes int `json:"keep_minutes"`
}

// requireAdmin is the authorization for everything in this file.
//
// It wraps requireAuth, so the token is verified first, and then insists on the admin role
// carried in it. The roles are re-read from the pivot on every token refresh, so a role
// removed — or a ban added — takes effect within the access token's five minutes rather
// than at the end of Laravel's one-hour role cache.
//
// The answer to a signed-in non-admin is 403 rather than 404: the caller is a known
// account being told it may not do this, and hiding the existence of a back office from
// somebody who can read the SPA's routes buys nothing.
func (a *api) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return a.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}
		if !hasRole(identity.Roles, admin.AdminRoleSlug) {
			writeError(w, r, http.StatusForbidden, "forbidden", "this account is not a moderator")
			return
		}
		next(w, r)
	})
}

func hasRole(roles []string, slug string) bool {
	for _, role := range roles {
		if role == slug {
			return true
		}
	}
	return false
}

// adminPosts is the collection: list.
func (a *api) adminPosts(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	posts, err := a.admin.Posts(r.Context(), page)
	if err != nil {
		a.writeAdminError(w, r, err)
		return
	}

	response := adminPostListResponse{
		Posts: make([]adminPostResponse, 0, len(posts.Posts)),
		Total: posts.Total, Page: posts.Page, PerPage: posts.PerPage,
	}
	for _, post := range posts.Posts {
		response.Posts = append(response.Posts, adminPostResponse{
			Serial: post.Serial, Title: post.Title, Description: post.Description,
			AccessPolicy: post.AccessPolicy, Censored: post.Censored,
			PlayCount: post.PlayCount,
			Owner: adminOwnerResponse{
				ID: post.OwnerID, Name: post.OwnerName, Email: post.OwnerEmail,
			},
			CreatedAt: post.CreatedAt,
		})
	}
	writePrivateJSON(w, r, http.StatusOK, response)
}

// adminPost is one post: read, edit or delete.
func (a *api) adminPost(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}
	serial := r.PathValue("serial")

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		post, err := a.admin.Post(r.Context(), serial)
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusOK, renderPost(post))

	case http.MethodPut:
		var request adminPostEditRequest
		if err := a.decodeAdminJSON(w, r, &request); err != nil {
			return
		}
		post, err := a.admin.UpdatePost(r.Context(), serial, admin.PostEdit{
			Draft: authoring.PostDraft{
				Title:        request.Title,
				Description:  request.Description,
				AccessPolicy: request.AccessPolicy,
				Password:     request.Password,
				Tags:         request.Tags,
			},
			Censored: request.Censored,
		})
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusOK, renderPost(post))

	case http.MethodDelete:
		// No body and no password: see admin.Service.DeletePost.
		if err := a.admin.DeletePost(r.Context(), serial); err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		a.writeNoContent(w)

	default:
		w.Header().Set("Allow", "GET, HEAD, PUT, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *api) adminPostElements(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))
	elements, err := a.admin.Elements(r.Context(), r.PathValue("serial"), authoring.ElementQuery{
		TitleLike:  query.Get("title"),
		SortBy:     query.Get("sort_by"),
		Descending: query.Get("sort_dir") != "asc",
		Page:       page,
		PerPage:    perPage,
	})
	if err != nil {
		a.writeAdminError(w, r, err)
		return
	}

	response := elementListResponse{
		Elements: make([]elementResponse, 0, len(elements.Elements)),
		Total:    elements.Total, Page: elements.Page, PerPage: elements.PerPage,
	}
	for _, element := range elements.Elements {
		response.Elements = append(response.Elements, renderElement(element))
	}
	writePrivateJSON(w, r, http.StatusOK, response)
}

// adminElement is one element: edit or delete.
func (a *api) adminElement(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}
	elementID, ok := editorElementID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPut:
		var request elementEditRequest
		if err := a.decodeAdminJSON(w, r, &request); err != nil {
			return
		}
		element, err := a.admin.EditElement(r.Context(), elementID, authoring.ElementEdit{
			Title:       request.Title,
			StartSecond: request.StartSecond,
			EndSecond:   request.EndSecond,
		})
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusOK, renderElement(element))

	case http.MethodDelete:
		if err := a.admin.DeleteElement(r.Context(), elementID); err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		a.writeNoContent(w)

	default:
		w.Header().Set("Allow", "PUT, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// adminUsers lists accounts, optionally filtered.
//
// One endpoint replaces the original's index and search views: an empty `q` is the full
// list rather than the empty page the search view returned.
func (a *api) adminUsers(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	users, err := a.admin.Users(r.Context(), query.Get("q"), page)
	if err != nil {
		a.writeAdminError(w, r, err)
		return
	}

	response := adminUserListResponse{
		Users: make([]adminUserResponse, 0, len(users.Users)),
		Total: users.Total, Page: users.Page, PerPage: users.PerPage,
	}
	for _, user := range users.Users {
		roles := user.Roles
		if roles == nil {
			roles = []string{}
		}
		response.Users = append(response.Users, adminUserResponse{
			ID: user.ID, Name: user.Name, Email: user.Email, AvatarURL: user.AvatarURL,
			Roles: roles, PostCount: user.PostCount, CreatedAt: user.CreatedAt,
		})
	}
	writePrivateJSON(w, r, http.StatusOK, response)
}

func (a *api) banUser(w http.ResponseWriter, r *http.Request) {
	a.changeBan(w, r, true)
}

func (a *api) unbanUser(w http.ResponseWriter, r *http.Request) {
	a.changeBan(w, r, false)
}

func (a *api) changeBan(w http.ResponseWriter, r *http.Request, banned bool) {
	if !a.requireAdminService(w, r) {
		return
	}
	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	if banned {
		err = a.admin.BanUser(r.Context(), userID)
	} else {
		err = a.admin.UnbanUser(r.Context(), userID)
	}
	if err != nil {
		a.writeAdminError(w, r, err)
		return
	}
	a.writeNoContent(w)
}

// adminCarouselItems is the collection: list or create.
func (a *api) adminCarouselItems(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		items, err := a.admin.CarouselItems(r.Context())
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		response := make([]adminCarouselResponse, 0, len(items))
		for _, item := range items {
			response = append(response, renderCarouselItem(item))
		}
		writePrivateJSON(w, r, http.StatusOK, map[string]any{"items": response})

	case http.MethodPost:
		var request adminCarouselCreateRequest
		if err := a.decodeAdminJSON(w, r, &request); err != nil {
			return
		}
		item, err := a.admin.CreateCarouselItem(r.Context(), admin.CarouselDraft{
			Type:        request.Type,
			Title:       request.Title,
			Description: request.Description,
			ImageURL:    request.ImageURL,
			VideoURL:    request.VideoURL,
			StartSecond: request.StartSecond,
			EndSecond:   request.EndSecond,
			Active:      request.Active,
		})
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusCreated, renderCarouselItem(item))

	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// adminCarouselItem is one slide: edit or delete.
func (a *api) adminCarouselItem(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil || itemID <= 0 {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var request adminCarouselEditRequest
		if err := a.decodeAdminJSON(w, r, &request); err != nil {
			return
		}
		item, err := a.admin.UpdateCarouselItem(r.Context(), itemID, admin.CarouselEdit{
			Title:       request.Title,
			Description: request.Description,
			StartSecond: request.StartSecond,
			EndSecond:   request.EndSecond,
			Active:      request.Active,
		})
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusOK, renderCarouselItem(item))

	case http.MethodDelete:
		if err := a.admin.DeleteCarouselItem(r.Context(), itemID); err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		a.writeNoContent(w)

	default:
		w.Header().Set("Allow", "PUT, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// reorderCarouselItems writes the whole order in one request.
//
// The original took one item per request, so dragging a slide fired a burst of calls and a
// failure part-way left an order nobody chose. Here the list is one body and one
// transaction.
func (a *api) reorderCarouselItems(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	var request adminCarouselReorderRequest
	if err := a.decodeAdminJSON(w, r, &request); err != nil {
		return
	}

	positions := make([]admin.CarouselPosition, 0, len(request.Items))
	for _, item := range request.Items {
		positions = append(positions, admin.CarouselPosition{ID: item.ID, Position: item.Position})
	}
	if err := a.admin.ReorderCarouselItems(r.Context(), positions); err != nil {
		a.writeAdminError(w, r, err)
		return
	}

	items, err := a.admin.CarouselItems(r.Context())
	if err != nil {
		a.writeAdminError(w, r, err)
		return
	}
	response := make([]adminCarouselResponse, 0, len(items))
	for _, item := range items {
		response = append(response, renderCarouselItem(item))
	}
	writePrivateJSON(w, r, http.StatusOK, map[string]any{"items": response})
}

// adminAnnouncement reads or replaces the site-wide announcement.
func (a *api) adminAnnouncement(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdminService(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		announcement, found, err := a.admin.Announcement(r.Context())
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		if !found {
			// Absent is the normal state, so it is data rather than a 404: the form draws
			// empty and the moderator writes a new one.
			writePrivateJSON(w, r, http.StatusOK, map[string]any{"announcement": nil})
			return
		}
		writePrivateJSON(w, r, http.StatusOK, map[string]any{
			"announcement": renderAnnouncement(announcement),
		})

	case http.MethodPut:
		var request adminAnnouncementRequest
		if err := a.decodeAdminJSON(w, r, &request); err != nil {
			return
		}
		announcement, err := a.admin.PublishAnnouncement(r.Context(), admin.AnnouncementDraft{
			Content:  request.Content,
			ImageURL: request.ImageURL,
			Minutes:  request.Minutes,
		})
		if err != nil {
			a.writeAdminError(w, r, err)
			return
		}
		writePrivateJSON(w, r, http.StatusOK, map[string]any{
			"announcement": renderAnnouncement(announcement),
		})

	default:
		w.Header().Set("Allow", "GET, HEAD, PUT")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func renderCarouselItem(item admin.CarouselItem) adminCarouselResponse {
	return adminCarouselResponse{
		ID: item.ID, Position: item.Position, Type: item.Type,
		Title: item.Title, Description: item.Description,
		ImageURL: item.ImageURL, VideoURL: item.VideoURL,
		VideoSource: item.VideoSource, VideoID: item.VideoID,
		StartSecond: item.StartSecond, EndSecond: item.EndSecond,
		Active: item.Active,
	}
}

func renderAnnouncement(announcement admin.Announcement) adminAnnouncementResponse {
	return adminAnnouncementResponse{
		ID: announcement.ID, Content: announcement.Content, ImageURL: announcement.ImageURL,
		CreatedAt: announcement.CreatedAt, KeepMinutes: announcement.KeepMinutes,
	}
}

// requireAdminService refuses when this process was started without the back office.
func (a *api) requireAdminService(w http.ResponseWriter, r *http.Request) bool {
	if a.admin == nil {
		writeError(w, r, http.StatusServiceUnavailable, "admin_not_configured",
			"the moderation back office is not configured on this server")
		return false
	}
	return true
}

// decodeAdminJSON reads a body and writes the 400 itself, so each handler's error path is
// one line. The size cap is decodeJSON's own 64 KiB, which a carousel reorder — one small
// object per slide — stays far inside.
func (a *api) decodeAdminJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if err := decodeJSON(w, r, destination); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return err
	}
	return nil
}

func (a *api) writeNoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// writeAdminError renders the back office's failures.
//
// Unlike the editor's, a 404 here means the row does not exist rather than "not yours":
// the caller is already known to be a moderator, so there is nothing left to conceal.
func (a *api) writeAdminError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *authoring.ErrInvalid
	switch {
	case errors.As(err, &invalid):
		writeFieldErrors(w, r, invalid.Fields)
	case errors.Is(err, admin.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, admin.ErrCannotBanAdministrator):
		writeError(w, r, http.StatusConflict, "cannot_ban_administrator",
			"an account with the administrator role cannot be banned")
	case errors.Is(err, admin.ErrAnnouncementsUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "announcements_not_configured",
			"the announcement store is not configured on this server")
	default:
		a.logger.Error("admin_request_failed",
			"request_id", requestIDFromContext(r.Context()), "path", r.URL.Path, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error",
			"the request could not be completed")
	}
}
