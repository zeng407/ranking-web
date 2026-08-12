package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"2pick.app/backend/internal/authoring"
)

// The post editor's endpoints, from Api\MyPostController and the read and edit half of
// Api\ElementController.
//
// Every one is scoped to the caller's own posts inside the SQL rather than by a check
// before it, so there is no object id to authorize here: a serial that is not the
// caller's simply does not resolve.

// AuthoringService is the slice of authoring.Service this layer uses.
type AuthoringService interface {
	Posts(ctx context.Context, userID int64, page int) ([]authoring.Post, int, error)
	Post(ctx context.Context, userID int64, serial string) (authoring.Post, error)
	CreatePost(ctx context.Context, userID int64, draft authoring.PostDraft) (string, error)
	UpdatePost(ctx context.Context, userID int64, serial string, draft authoring.PostDraft) (authoring.Post, error)
	DeletePost(ctx context.Context, userID int64, serial, accountPassword string) error
	Elements(ctx context.Context, userID int64, serial string, query authoring.ElementQuery) (authoring.ElementPage, error)
	EditElement(ctx context.Context, userID int64, elementID int64, edit authoring.ElementEdit) (authoring.Element, error)
	DeleteElement(ctx context.Context, userID int64, elementID int64) error
}

const maxEditorRequestBytes = 16 << 10

type postResponse struct {
	Serial      string `json:"serial"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// AccessPolicy is private, public or password.
	AccessPolicy string `json:"access_policy"`
	// HasPassword says whether one is set, never what it is. The editor needs it to
	// decide between "set a password" and "change it"; the Blade page read the column
	// directly, which a client cannot do.
	HasPassword       bool     `json:"has_password"`
	Tags              []string `json:"tags"`
	PlayCount         int      `json:"play_count"`
	ThisWeekPlayCount int      `json:"this_week_play_count"`
	LastWeekPlayCount int      `json:"last_week_play_count"`
	CreatedAt         string   `json:"created_at,omitempty"`
}

type postListResponse struct {
	Posts []postResponse `json:"posts"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	// PerPage is fixed, but serving it saves the client from hard-coding the number it
	// needs to compute how many pages there are.
	PerPage int `json:"per_page"`
}

type postDraftRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	AccessPolicy string   `json:"access_policy"`
	Password     string   `json:"password"`
	Tags         []string `json:"tags"`
}

type deletePostRequest struct {
	Password string `json:"password"`
}

type elementResponse struct {
	ID          int64  `json:"id"`
	SourceURL   string `json:"source_url"`
	ThumbURL    string `json:"thumb_url"`
	MediumURL   string `json:"mediumthumb_url"`
	LowURL      string `json:"lowthumb_url"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	VideoSource string `json:"video_source,omitempty"`
	VideoID     string `json:"video_id,omitempty"`
	Duration    *int   `json:"video_duration_second"`
	StartSecond *int   `json:"video_start_second"`
	EndSecond   *int   `json:"video_end_second"`
	CreatedAt   string `json:"created_at,omitempty"`
	Rank        *struct {
		Rank         int     `json:"rank"`
		WinRate      float64 `json:"win_rate"`
		FinalWinRate float64 `json:"final_win_rate"`
	} `json:"rank"`
}

type elementListResponse struct {
	Elements []elementResponse `json:"elements"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PerPage  int               `json:"per_page"`
}

// elementEditRequest uses pointers so "not sent" and "sent as empty" stay different,
// which is what the original's `sometimes` rule meant.
type elementEditRequest struct {
	Title       *string `json:"title"`
	StartSecond *int    `json:"video_start_second"`
	EndSecond   *int    `json:"video_end_second"`
}

// accountPosts is the collection: list or create.
func (a *api) accountPosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.listAccountPosts(w, r)
	case http.MethodPost:
		a.createAccountPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// accountPost is one post: read, edit or delete.
func (a *api) accountPost(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.showAccountPost(w, r)
	case http.MethodPut:
		a.updateAccountPost(w, r)
	case http.MethodDelete:
		a.deleteAccountPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// accountElement is one element: edit or delete.
func (a *api) accountElement(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		a.updateAccountElement(w, r)
	case http.MethodDelete:
		a.deleteAccountElement(w, r)
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *api) listAccountPosts(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	posts, total, err := a.authoring.Posts(r.Context(), userID, page)
	if err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	if page < 1 {
		page = 1
	}

	response := postListResponse{
		Posts: make([]postResponse, 0, len(posts)),
		Total: total, Page: page, PerPage: authoring.PostsPerPage,
	}
	for _, post := range posts {
		response.Posts = append(response.Posts, renderPost(post))
	}
	writePrivateJSON(w, r, http.StatusOK, response)
}

func (a *api) showAccountPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	post, err := a.authoring.Post(r.Context(), userID, r.PathValue("serial"))
	if err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusOK, renderPost(post))
}

func (a *api) createAccountPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	var request postDraftRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	serial, err := a.authoring.CreatePost(r.Context(), userID, request.draft())
	if err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	// 201 with the serial: the client navigates to the editor for it next, and there is
	// nothing else about a post that has no elements yet worth serialising.
	w.Header().Set("Location", "/api/v1/account/posts/"+serial)
	writePrivateJSON(w, r, http.StatusCreated, map[string]string{"serial": serial})
}

func (a *api) updateAccountPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	var request postDraftRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	post, err := a.authoring.UpdatePost(r.Context(), userID, r.PathValue("serial"), request.draft())
	if err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusOK, renderPost(post))
}

// deleteAccountPost takes the account password in a body, not the query string: a query
// string reaches the access log and the browser's history.
func (a *api) deleteAccountPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	// An account with no password sends no body at all, so an empty one is not an error.
	// Decoding is attempted whenever there is anything to read rather than only when a
	// Content-Length says so: a chunked request carries -1 there, and skipping the decode
	// would silently drop a password the caller did send.
	var request deletePostRequest
	if r.ContentLength != 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
		if err := decodeJSON(w, r, &request); err != nil && !errors.Is(err, io.EOF) &&
			!strings.Contains(err.Error(), "EOF") {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}

	if err := a.authoring.DeletePost(
		r.Context(), userID, r.PathValue("serial"), request.Password); err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listPostElements(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))
	elements, err := a.authoring.Elements(r.Context(), userID, r.PathValue("serial"),
		authoring.ElementQuery{
			TitleLike: query.Get("title"),
			SortBy:    query.Get("sort_by"),
			// Newest first unless the caller says otherwise, matching the original's
			// ['id' => 'desc'].
			Descending: query.Get("sort_dir") != "asc",
			Page:       page,
			PerPage:    perPage,
		})
	if err != nil {
		a.writeAuthoringError(w, r, err)
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

func (a *api) updateAccountElement(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}
	elementID, ok := editorElementID(w, r)
	if !ok {
		return
	}

	var request elementEditRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	element, err := a.authoring.EditElement(r.Context(), userID, elementID, authoring.ElementEdit{
		Title:       request.Title,
		StartSecond: request.StartSecond,
		EndSecond:   request.EndSecond,
	})
	if err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusOK, renderElement(element))
}

func (a *api) deleteAccountElement(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.editorCaller(w, r)
	if !ok {
		return
	}
	elementID, ok := editorElementID(w, r)
	if !ok {
		return
	}

	if err := a.authoring.DeleteElement(r.Context(), userID, elementID); err != nil {
		a.writeAuthoringError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (request postDraftRequest) draft() authoring.PostDraft {
	return authoring.PostDraft{
		Title:        request.Title,
		Description:  request.Description,
		AccessPolicy: request.AccessPolicy,
		Password:     request.Password,
		Tags:         request.Tags,
	}
}

func renderPost(post authoring.Post) postResponse {
	response := postResponse{
		Serial: post.Serial, Title: post.Title, Description: post.Description,
		AccessPolicy: post.AccessPolicy, HasPassword: post.HasPassword,
		Tags:              post.Tags,
		PlayCount:         post.PlayCount,
		ThisWeekPlayCount: post.ThisWeekPlayCount,
		LastWeekPlayCount: post.LastWeekPlayCount,
	}
	if response.Tags == nil {
		response.Tags = []string{}
	}
	if !post.CreatedAt.IsZero() {
		response.CreatedAt = post.CreatedAt.Format(time.RFC3339)
	}
	return response
}

func renderElement(element authoring.Element) elementResponse {
	response := elementResponse{
		ID: element.ID, SourceURL: element.SourceURL, ThumbURL: element.ThumbURL,
		MediumURL: element.MediumURL, LowURL: element.LowURL, Title: element.Title,
		Type: element.Type, VideoSource: element.VideoSource, VideoID: element.VideoID,
		Duration: element.DurationSecs, StartSecond: element.StartSecond,
		EndSecond: element.EndSecond,
	}
	if !element.CreatedAt.IsZero() {
		response.CreatedAt = element.CreatedAt.Format(time.RFC3339)
	}
	if element.Rank != nil {
		response.Rank = &struct {
			Rank         int     `json:"rank"`
			WinRate      float64 `json:"win_rate"`
			FinalWinRate float64 `json:"final_win_rate"`
		}{element.Rank.Rank, element.Rank.WinRate, element.Rank.FinalWinRate}
	}
	return response
}

// editorCaller resolves the account, and refuses when the editor is not configured.
func (a *api) editorCaller(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if a.authoring == nil {
		writeError(w, r, http.StatusServiceUnavailable, "editor_not_configured",
			"the post editor is not configured on this server")
		return 0, false
	}
	return a.callerUserID(w, r)
}

func editorElementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	elementID, err := strconv.ParseInt(r.PathValue("elementID"), 10, 64)
	if err != nil || elementID <= 0 {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return 0, false
	}
	return elementID, true
}

// writeAuthoringError renders the editor's failures.
//
// A post or element that is not the caller's answers 404, never 403: a 403 would confirm
// that the serial exists and belongs to someone, which is more than a stranger should be
// able to learn by guessing.
func (a *api) writeAuthoringError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *authoring.ErrInvalid
	if errors.As(err, &invalid) {
		writeFieldErrors(w, r, invalid.Fields)
		return
	}
	if errors.Is(err, authoring.ErrPostNotFound) || errors.Is(err, authoring.ErrElementNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	a.logger.Error("authoring_request_failed", "error", err)
	writeError(w, r, http.StatusInternalServerError, "internal_error",
		"the request could not be completed")
}
