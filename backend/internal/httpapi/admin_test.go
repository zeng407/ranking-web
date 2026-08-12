package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"2pick.app/backend/internal/admin"
	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/authoring"
)

type fakeAdmin struct {
	posts       admin.PostPage
	post        authoring.Post
	users       admin.UserPage
	items       []admin.CarouselItem
	item        admin.CarouselItem
	announcemnt admin.Announcement
	found       bool
	err         error

	lastPage      int
	lastKeyword   string
	lastSerial    string
	lastEdit      admin.PostEdit
	lastElementID int64
	lastUserID    int64
	lastDraft     admin.CarouselDraft
	lastPositions []admin.CarouselPosition
	lastMinutes   int
	bans          int
	unbans        int
	deletes       int
}

func newFakeAdmin() *fakeAdmin {
	return &fakeAdmin{
		posts: admin.PostPage{
			Posts: []admin.Post{{
				Serial: "abcdefgh", Title: "a post", AccessPolicy: "public",
				Censored: true, PlayCount: 500,
				OwnerID: 42, OwnerName: "ada", OwnerEmail: "ada@example.com",
				CreatedAt: "2026-08-12T00:00:00Z",
			}},
			Total: 7, Page: 2, PerPage: admin.PostsPerPage,
		},
		post: authoring.Post{Serial: "abcdefgh", Title: "a post"},
		users: admin.UserPage{
			Users:   []admin.User{{ID: 3, Name: "ada", Email: "ada@example.com", PostCount: 2}},
			Total:   1,
			Page:    1,
			PerPage: admin.UsersPerPage,
		},
		items: []admin.CarouselItem{{
			ID: 5, Position: 1, Type: admin.CarouselTypeVideo, Title: "slide",
			ImageURL: "https://img.example/1.jpg", VideoURL: "https://youtu.be/abc",
			Active: true,
		}},
		item: admin.CarouselItem{ID: 5, Position: 1, Type: admin.CarouselTypeVideo, Title: "slide"},
	}
}

func (fake *fakeAdmin) Posts(_ context.Context, page int) (admin.PostPage, error) {
	fake.lastPage = page
	return fake.posts, fake.err
}

func (fake *fakeAdmin) Post(_ context.Context, serial string) (authoring.Post, error) {
	fake.lastSerial = serial
	return fake.post, fake.err
}

func (fake *fakeAdmin) UpdatePost(
	_ context.Context, serial string, edit admin.PostEdit,
) (authoring.Post, error) {
	fake.lastSerial, fake.lastEdit = serial, edit
	return fake.post, fake.err
}

func (fake *fakeAdmin) DeletePost(_ context.Context, serial string) error {
	fake.lastSerial = serial
	fake.deletes++
	return fake.err
}

func (fake *fakeAdmin) Elements(
	_ context.Context, serial string, _ authoring.ElementQuery,
) (authoring.ElementPage, error) {
	fake.lastSerial = serial
	return authoring.ElementPage{Page: 1, PerPage: 20}, fake.err
}

func (fake *fakeAdmin) EditElement(
	_ context.Context, elementID int64, _ authoring.ElementEdit,
) (authoring.Element, error) {
	fake.lastElementID = elementID
	return authoring.Element{ID: elementID}, fake.err
}

func (fake *fakeAdmin) DeleteElement(_ context.Context, elementID int64) error {
	fake.lastElementID = elementID
	return fake.err
}

func (fake *fakeAdmin) Users(_ context.Context, keyword string, page int) (admin.UserPage, error) {
	fake.lastKeyword, fake.lastPage = keyword, page
	return fake.users, fake.err
}

func (fake *fakeAdmin) BanUser(_ context.Context, userID int64) error {
	fake.lastUserID = userID
	fake.bans++
	return fake.err
}

func (fake *fakeAdmin) UnbanUser(_ context.Context, userID int64) error {
	fake.lastUserID = userID
	fake.unbans++
	return fake.err
}

func (fake *fakeAdmin) CarouselItems(context.Context) ([]admin.CarouselItem, error) {
	return fake.items, fake.err
}

func (fake *fakeAdmin) CreateCarouselItem(
	_ context.Context, draft admin.CarouselDraft,
) (admin.CarouselItem, error) {
	fake.lastDraft = draft
	return fake.item, fake.err
}

func (fake *fakeAdmin) UpdateCarouselItem(
	_ context.Context, itemID int64, _ admin.CarouselEdit,
) (admin.CarouselItem, error) {
	fake.item.ID = itemID
	return fake.item, fake.err
}

func (fake *fakeAdmin) DeleteCarouselItem(_ context.Context, itemID int64) error {
	fake.item.ID = itemID
	fake.deletes++
	return fake.err
}

func (fake *fakeAdmin) ReorderCarouselItems(_ context.Context, positions []admin.CarouselPosition) error {
	fake.lastPositions = positions
	return fake.err
}

func (fake *fakeAdmin) Announcement(context.Context) (admin.Announcement, bool, error) {
	return fake.announcemnt, fake.found, fake.err
}

func (fake *fakeAdmin) PublishAnnouncement(
	_ context.Context, draft admin.AnnouncementDraft,
) (admin.Announcement, error) {
	fake.lastMinutes = draft.Minutes
	fake.announcemnt = admin.Announcement{
		ID: "generated", Content: draft.Content, ImageURL: draft.ImageURL, KeepMinutes: 60,
	}
	return fake.announcemnt, fake.err
}

func adminHandler(service AdminService, roles ...string) http.Handler {
	return New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Admin:        service,
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: roles}},
	})
}

func moderatorHandler(service AdminService) http.Handler {
	return adminHandler(service, "user", admin.AdminRoleSlug)
}

func adminRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func decodeAdminData(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s: %v", response.Body.String(), err)
	}
	return envelope.Data
}

func adminErrorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s: %v", response.Body.String(), err)
	}
	return envelope.Error.Code
}

// Every route in the back office is behind the role, and the service underneath checks
// nothing: a route wired without requireAdmin would be a route that acts across owners for
// anybody. This walks the whole set.
func TestEveryAdminRouteRefusesAnAccountWithoutTheRole(t *testing.T) {
	requests := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/posts", ""},
		{http.MethodGet, "/api/v1/admin/posts/abcdefgh", ""},
		{http.MethodPut, "/api/v1/admin/posts/abcdefgh", `{"title":"t"}`},
		{http.MethodDelete, "/api/v1/admin/posts/abcdefgh", ""},
		{http.MethodGet, "/api/v1/admin/posts/abcdefgh/elements", ""},
		{http.MethodPut, "/api/v1/admin/elements/1", `{"title":"t"}`},
		{http.MethodDelete, "/api/v1/admin/elements/1", ""},
		{http.MethodGet, "/api/v1/admin/users", ""},
		{http.MethodPut, "/api/v1/admin/users/3/ban", ""},
		{http.MethodPut, "/api/v1/admin/users/3/unban", ""},
		{http.MethodGet, "/api/v1/admin/carousel-items", ""},
		{http.MethodPost, "/api/v1/admin/carousel-items", `{"type":"video","video_url":"u"}`},
		{http.MethodPut, "/api/v1/admin/carousel-items/reorder", `{"items":[]}`},
		{http.MethodPut, "/api/v1/admin/carousel-items/5", `{"title":"t"}`},
		{http.MethodDelete, "/api/v1/admin/carousel-items/5", ""},
		{http.MethodGet, "/api/v1/admin/announcement", ""},
		{http.MethodPut, "/api/v1/admin/announcement", `{"content":"c"}`},
		{http.MethodPost, "/api/v1/admin/assets/grant", ""},
		{http.MethodPost, "/api/v1/admin/assets/revoke", ""},
	}
	for _, testCase := range requests {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			service := newFakeAdmin()
			response := httptest.NewRecorder()

			adminHandler(service, "user").ServeHTTP(response,
				adminRequest(testCase.method, testCase.path, testCase.body))

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
			}
			if code := adminErrorCode(t, response); code != "forbidden" {
				t.Errorf("code = %q, want forbidden", code)
			}
		})
	}
}

// Without a token the answer is 401, not 403: nothing has been decided about the account
// yet.
func TestTheAdminRoutesRefuseAnAnonymousRequest(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/posts", nil)

	moderatorHandler(newFakeAdmin()).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", response.Code, response.Body.String())
	}
}

// A process started without the back office answers 503 on these routes alone.
func TestTheAdminRoutesAnswer503WithoutTheService(t *testing.T) {
	response := httptest.NewRecorder()

	adminHandler(nil, admin.AdminRoleSlug).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/posts", ""))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	if code := adminErrorCode(t, response); code != "admin_not_configured" {
		t.Errorf("code = %q, want admin_not_configured", code)
	}
}

func TestListingPostsCarriesTheOwnerAndTheCensorshipFlag(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/posts?page=2", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastPage != 2 {
		t.Errorf("the service was asked for page %d, want 2", service.lastPage)
	}
	data := decodeAdminData(t, response)
	if data["total"] != float64(7) || data["per_page"] != float64(admin.PostsPerPage) {
		t.Errorf("total/per page = %v/%v", data["total"], data["per_page"])
	}
	posts, _ := data["posts"].([]any)
	if len(posts) != 1 {
		t.Fatalf("posts = %v, want one", data["posts"])
	}
	first, _ := posts[0].(map[string]any)
	if first["is_censored"] != true {
		t.Errorf("is_censored = %v, want true", first["is_censored"])
	}
	owner, _ := first["owner"].(map[string]any)
	if owner["id"] != float64(42) || owner["email"] != "ada@example.com" {
		t.Errorf("owner = %v", owner)
	}
}

// A back office list holds every account's address and every post's owner. Anything that
// cached it would serve one moderator's page to the next visitor.
func TestTheAdminResponsesAreNotCacheable(t *testing.T) {
	for _, path := range []string{
		"/api/v1/admin/posts",
		"/api/v1/admin/posts/abcdefgh",
		"/api/v1/admin/users",
		"/api/v1/admin/carousel-items",
		"/api/v1/admin/announcement",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			moderatorHandler(newFakeAdmin()).ServeHTTP(response,
				adminRequest(http.MethodGet, path, ""))

			if control := response.Header().Get("Cache-Control"); !strings.Contains(control, "no-store") {
				t.Errorf("Cache-Control = %q, want it to forbid storing", control)
			}
		})
	}
}

// The flag is a pointer on the wire so that a client fixing a title does not have to know
// its current value to avoid clearing it.
func TestEditingAPostOnlySendsTheCensorshipFlagWhenItIsInTheBody(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut,
		"/api/v1/admin/posts/abcdefgh", `{"title":"t","access_policy":"public"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastEdit.Censored != nil {
		t.Errorf("censored = %v, want it absent", *service.lastEdit.Censored)
	}
	if service.lastEdit.Draft.Title != "t" {
		t.Errorf("draft = %+v", service.lastEdit.Draft)
	}

	response = httptest.NewRecorder()
	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut,
		"/api/v1/admin/posts/abcdefgh", `{"title":"t","is_censored":false}`))

	if service.lastEdit.Censored == nil || *service.lastEdit.Censored {
		t.Errorf("censored = %v, want false", service.lastEdit.Censored)
	}
}

// A moderator's delete carries no password, unlike the author's own.
func TestDeletingAPostTakesNoBodyAndAnswers204(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodDelete, "/api/v1/admin/posts/abcdefgh", ""))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if service.deletes != 1 || service.lastSerial != "abcdefgh" {
		t.Errorf("deletes = %d, serial = %q", service.deletes, service.lastSerial)
	}
}

func TestAMissingRowIs404(t *testing.T) {
	service := newFakeAdmin()
	service.err = admin.ErrNotFound
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/posts/abcdefgh", ""))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
	}
}

func TestARefusedFormIs422WithFieldCodes(t *testing.T) {
	service := newFakeAdmin()
	service.err = &authoring.ErrInvalid{
		Fields: authoring.FieldErrors{"video_url": {admin.CodeUnresolvable}},
	}
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPost,
		"/api/v1/admin/carousel-items", `{"type":"video","video_url":"https://example.com/x"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), admin.CodeUnresolvable) {
		t.Errorf("body = %s, want the field code", response.Body.String())
	}
}

// Banning a moderator is refused with its own code, so the UI can say why rather than
// showing a generic failure.
func TestBanningAnAdministratorIs409(t *testing.T) {
	service := newFakeAdmin()
	service.err = admin.ErrCannotBanAdministrator
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodPut, "/api/v1/admin/users/3/ban", ""))

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", response.Code, response.Body.String())
	}
	if code := adminErrorCode(t, response); code != "cannot_ban_administrator" {
		t.Errorf("code = %q, want cannot_ban_administrator", code)
	}
}

func TestBanningAndUnbanningAnswer204(t *testing.T) {
	service := newFakeAdmin()

	response := httptest.NewRecorder()
	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodPut, "/api/v1/admin/users/3/ban", ""))
	if response.Code != http.StatusNoContent {
		t.Fatalf("ban status = %d, want 204; body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodPut, "/api/v1/admin/users/3/unban", ""))
	if response.Code != http.StatusNoContent {
		t.Fatalf("unban status = %d, want 204; body = %s", response.Code, response.Body.String())
	}

	if service.bans != 1 || service.unbans != 1 || service.lastUserID != 3 {
		t.Errorf("bans = %d, unbans = %d, user = %d", service.bans, service.unbans, service.lastUserID)
	}
}

// A path parameter that is not a positive id is a 404 rather than a request the service
// sees: id 0 or -1 exists nowhere.
func TestANonNumericOrNonPositiveIDIs404(t *testing.T) {
	for _, path := range []string{
		"/api/v1/admin/users/0/ban",
		"/api/v1/admin/users/-1/ban",
		"/api/v1/admin/users/ada/ban",
	} {
		t.Run(path, func(t *testing.T) {
			service := newFakeAdmin()
			response := httptest.NewRecorder()

			moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut, path, ""))

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
			}
			if service.bans != 0 {
				t.Errorf("bans = %d, want none", service.bans)
			}
		})
	}
}

// One request per drag, not one per slide: the reorder takes the whole list and answers
// with the order it produced, so the client does not have to guess.
func TestAReorderTakesTheWholeListAndAnswersWithIt(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut,
		"/api/v1/admin/carousel-items/reorder", `{"items":[{"id":5,"position":1},{"id":6,"position":2}]}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(service.lastPositions) != 2 || service.lastPositions[1].ID != 6 {
		t.Errorf("positions = %+v", service.lastPositions)
	}
	items, _ := decodeAdminData(t, response)["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items = %v, want the refreshed carousel", items)
	}
}

// /carousel-items/reorder must not be read as /carousel-items/{itemID}.
func TestTheReorderRouteIsNotTheItemRoute(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut,
		"/api/v1/admin/carousel-items/reorder", `{"items":[{"id":5,"position":1}]}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastPositions == nil {
		t.Error("the reorder was handled as an item edit")
	}
}

func TestCreatingASlideAnswers201(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPost,
		"/api/v1/admin/carousel-items",
		`{"type":"video","video_url":"https://youtu.be/abc","video_start_second":5,"is_active":false}`))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
	}
	if service.lastDraft.VideoURL != "https://youtu.be/abc" {
		t.Errorf("draft = %+v", service.lastDraft)
	}
	if service.lastDraft.StartSecond == nil || *service.lastDraft.StartSecond != 5 {
		t.Errorf("start second = %v, want 5", service.lastDraft.StartSecond)
	}
	if service.lastDraft.Active == nil || *service.lastDraft.Active {
		t.Errorf("active = %v, want an explicit false", service.lastDraft.Active)
	}
}

// No announcement is the normal state, so it is data rather than a 404: the form draws
// empty.
func TestAnAbsentAnnouncementIsNullData(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/announcement", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	data := decodeAdminData(t, response)
	if value, present := data["announcement"]; !present || value != nil {
		t.Errorf("announcement = %v (present %v), want a present null", value, present)
	}
}

func TestPublishingAnAnnouncementAnswersWithTheStoredOne(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response, adminRequest(http.MethodPut,
		"/api/v1/admin/announcement", `{"content":"維護公告","keep_minutes":30}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastMinutes != 30 {
		t.Errorf("minutes = %d, want 30", service.lastMinutes)
	}
	announcement, _ := decodeAdminData(t, response)["announcement"].(map[string]any)
	if announcement["id"] != "generated" || announcement["content"] != "維護公告" {
		t.Errorf("announcement = %v", announcement)
	}
}

// Without a shared cache the announcement endpoints say so, and the rest of the back
// office keeps working.
func TestAnnouncementsWithoutAStoreAre503(t *testing.T) {
	service := newFakeAdmin()
	service.err = admin.ErrAnnouncementsUnavailable
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/announcement", ""))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	if code := adminErrorCode(t, response); code != "announcements_not_configured" {
		t.Errorf("code = %q, want announcements_not_configured", code)
	}
}

func TestAdminRoutesRefuseTheWrongMethod(t *testing.T) {
	requests := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/posts/abcdefgh"},
		{http.MethodPost, "/api/v1/admin/elements/1"},
		{http.MethodDelete, "/api/v1/admin/announcement"},
		{http.MethodPost, "/api/v1/admin/carousel-items/5"},
		{http.MethodDelete, "/api/v1/admin/users"},
	}
	for _, testCase := range requests {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			moderatorHandler(newFakeAdmin()).ServeHTTP(response,
				adminRequest(testCase.method, testCase.path, `{}`))

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405; body = %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Allow") == "" {
				t.Error("Allow header is empty")
			}
		})
	}
}

func TestSearchingUsersPassesTheKeywordThrough(t *testing.T) {
	service := newFakeAdmin()
	response := httptest.NewRecorder()

	moderatorHandler(service).ServeHTTP(response,
		adminRequest(http.MethodGet, "/api/v1/admin/users?q=ada&page=3", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastKeyword != "ada" || service.lastPage != 3 {
		t.Errorf("keyword = %q, page = %d", service.lastKeyword, service.lastPage)
	}
	users, _ := decodeAdminData(t, response)["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("users = %v, want one", users)
	}
	first, _ := users[0].(map[string]any)
	if roles, ok := first["roles"].([]any); !ok || roles == nil {
		t.Errorf("roles = %v, want an array rather than null", first["roles"])
	}
}
