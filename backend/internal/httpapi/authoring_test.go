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

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/authoring"
)

// The editor's transport: which verb reaches which operation, that a serial belonging to
// someone else answers 404 rather than 403, and that nothing here is cacheable.

type fakeAuthoring struct {
	post     authoring.Post
	posts    []authoring.Post
	total    int
	elements authoring.ElementPage
	element  authoring.Element
	serial   string
	err      error

	createCalls    int
	updateCalls    int
	deleteCalls    int
	editCalls      int
	deleteElements []int64
	lastDraft      authoring.PostDraft
	lastPassword   string
	lastQuery      authoring.ElementQuery
	lastEdit       authoring.ElementEdit
	lastPage       int
}

func newFakeAuthoring() *fakeAuthoring {
	return &fakeAuthoring{
		post: authoring.Post{
			Serial: "abcdefgh", Title: "a title", Description: "a description",
			AccessPolicy: authoring.PolicyPublic, Tags: []string{"cats"},
			PlayCount: 500, ThisWeekPlayCount: 20, LastWeekPlayCount: 30,
		},
		serial: "newserial",
	}
}

func (service *fakeAuthoring) Posts(_ context.Context, _ int64, page int) ([]authoring.Post, int, error) {
	service.lastPage = page
	if service.err != nil {
		return nil, 0, service.err
	}
	return service.posts, service.total, nil
}

func (service *fakeAuthoring) Post(_ context.Context, _ int64, _ string) (authoring.Post, error) {
	if service.err != nil {
		return authoring.Post{}, service.err
	}
	return service.post, nil
}

func (service *fakeAuthoring) CreatePost(
	_ context.Context, _ int64, draft authoring.PostDraft,
) (string, error) {
	service.createCalls++
	service.lastDraft = draft
	if service.err != nil {
		return "", service.err
	}
	return service.serial, nil
}

func (service *fakeAuthoring) UpdatePost(
	_ context.Context, _ int64, _ string, draft authoring.PostDraft,
) (authoring.Post, error) {
	service.updateCalls++
	service.lastDraft = draft
	if service.err != nil {
		return authoring.Post{}, service.err
	}
	return service.post, nil
}

func (service *fakeAuthoring) DeletePost(_ context.Context, _ int64, _, accountPassword string) error {
	service.deleteCalls++
	service.lastPassword = accountPassword
	return service.err
}

func (service *fakeAuthoring) Elements(
	_ context.Context, _ int64, _ string, query authoring.ElementQuery,
) (authoring.ElementPage, error) {
	service.lastQuery = query
	if service.err != nil {
		return authoring.ElementPage{}, service.err
	}
	return service.elements, nil
}

func (service *fakeAuthoring) EditElement(
	_ context.Context, _ int64, _ int64, edit authoring.ElementEdit,
) (authoring.Element, error) {
	service.editCalls++
	service.lastEdit = edit
	if service.err != nil {
		return authoring.Element{}, service.err
	}
	return service.element, nil
}

func (service *fakeAuthoring) DeleteElement(_ context.Context, _ int64, elementID int64) error {
	service.deleteElements = append(service.deleteElements, elementID)
	return service.err
}

func editorHandler(service AuthoringService) http.Handler {
	return New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Authoring:    service,
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{}}},
	})
}

func editorRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func decodeEditorData(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s: %v", response.Body.String(), err)
	}
	return envelope.Data
}

func TestListingPostsServesThemWithTheirTotal(t *testing.T) {
	service := newFakeAuthoring()
	service.posts = []authoring.Post{service.post}
	service.total = 7
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response, editorRequest(http.MethodGet, "/api/v1/account/posts?page=2", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	data := decodeEditorData(t, response)
	if data["total"] != float64(7) {
		t.Errorf("total = %v, want 7", data["total"])
	}
	if data["page"] != float64(2) {
		t.Errorf("page = %v, want 2", data["page"])
	}
	if service.lastPage != 2 {
		t.Errorf("the service was asked for page %d, want 2", service.lastPage)
	}
	posts, _ := data["posts"].([]any)
	if len(posts) != 1 {
		t.Fatalf("posts = %v, want one", data["posts"])
	}
	first, _ := posts[0].(map[string]any)
	if first["serial"] != "abcdefgh" || first["play_count"] != float64(500) {
		t.Errorf("post = %v", first)
	}
}

// A post list carries titles and play counts for one account. Anything that cached it
// would serve one author's posts to the next visitor.
func TestTheEditorsResponsesAreNotCacheable(t *testing.T) {
	for _, path := range []string{
		"/api/v1/account/posts",
		"/api/v1/account/posts/abcdefgh",
		"/api/v1/account/posts/abcdefgh/elements",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			editorHandler(newFakeAuthoring()).ServeHTTP(response, editorRequest(http.MethodGet, path, ""))

			if control := response.Header().Get("Cache-Control"); !strings.Contains(control, "no-store") {
				t.Errorf("Cache-Control = %q, want it to forbid storing", control)
			}
		})
	}
}

func TestCreatingAPostAnswers201WithTheSerial(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response, editorRequest(http.MethodPost, "/api/v1/account/posts",
		`{"title":"t","description":"d","access_policy":"password","password":"door","tags":["cats"]}`))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
	}
	if data := decodeEditorData(t, response); data["serial"] != "newserial" {
		t.Errorf("serial = %v", data["serial"])
	}
	if got := service.lastDraft; got.Title != "t" || got.AccessPolicy != "password" || got.Password != "door" {
		t.Errorf("draft = %+v", got)
	}
	if location := response.Header().Get("Location"); location != "/api/v1/account/posts/newserial" {
		t.Errorf("Location = %q", location)
	}
}

// Tags absent and tags empty are different requests: one leaves them alone, the other
// clears them. A []string that decodes to nil versus an empty slice is what carries that.
func TestOmittingTagsAndSendingAnEmptyListAreDifferent(t *testing.T) {
	service := newFakeAuthoring()

	editorHandler(service).ServeHTTP(httptest.NewRecorder(),
		editorRequest(http.MethodPut, "/api/v1/account/posts/abcdefgh",
			`{"title":"t","description":"d","access_policy":"public"}`))
	if service.lastDraft.Tags != nil {
		t.Errorf("tags = %v, want nil when the field is absent", service.lastDraft.Tags)
	}

	editorHandler(service).ServeHTTP(httptest.NewRecorder(),
		editorRequest(http.MethodPut, "/api/v1/account/posts/abcdefgh",
			`{"title":"t","description":"d","access_policy":"public","tags":[]}`))
	if service.lastDraft.Tags == nil || len(service.lastDraft.Tags) != 0 {
		t.Errorf("tags = %v, want an empty list when one is sent", service.lastDraft.Tags)
	}
}

// The password reaches the server in a body, never a query string: a query string lands
// in the access log and in the browser's history.
func TestDeletingAPostTakesTheAccountPasswordInTheBody(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response,
		editorRequest(http.MethodDelete, "/api/v1/account/posts/abcdefgh",
			`{"password":"the-account-password"}`))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if service.lastPassword != "the-account-password" {
		t.Errorf("password = %q", service.lastPassword)
	}
}

// A body sent with no Content-Length — what a chunked request looks like. Skipping the
// decode on anything but a positive length would drop the password silently, and the
// service would then refuse a delete the caller had authorised correctly.
func TestDeletingAPostReadsAChunkedBody(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/account/posts/abcdefgh",
		strings.NewReader(`{"password":"the-account-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	request.ContentLength = -1
	editorHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if service.lastPassword != "the-account-password" {
		t.Errorf("password = %q; a chunked body was not read", service.lastPassword)
	}
}

// An account with no password sends no body at all, which must not be a parse error.
func TestDeletingAPostWithNoBodyIsAccepted(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/account/posts/abcdefgh", nil)
	request.Header.Set("Authorization", "Bearer token")
	editorHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if service.deleteCalls != 1 {
		t.Errorf("the service saw %d deletions, want 1", service.deleteCalls)
	}
}

// 404, NEVER 403. A 403 would confirm that the serial exists and belongs to someone,
// which is more than a stranger should learn by guessing eight characters.
func TestSomeoneElsesPostIsNotFoundRatherThanForbidden(t *testing.T) {
	// The body differs per verb because the decoders are strict: sending a draft to
	// DELETE is a 400 for the right reason, and would hide the 404 this is testing.
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/account/posts/abcdefgh", ""},
		{http.MethodPut, "/api/v1/account/posts/abcdefgh", `{"title":"t","description":"d","access_policy":"public"}`},
		{http.MethodDelete, "/api/v1/account/posts/abcdefgh", `{"password":"p"}`},
		{http.MethodGet, "/api/v1/account/posts/abcdefgh/elements", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			service := newFakeAuthoring()
			service.err = authoring.ErrPostNotFound
			response := httptest.NewRecorder()

			editorHandler(service).ServeHTTP(response,
				editorRequest(testCase.method, testCase.path, testCase.body))

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestSomeoneElsesElementIsNotFound(t *testing.T) {
	service := newFakeAuthoring()
	service.err = authoring.ErrElementNotFound
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response,
		editorRequest(http.MethodDelete, "/api/v1/account/elements/5", ""))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestARefusedDraftIs422WithFieldCodes(t *testing.T) {
	service := newFakeAuthoring()
	service.err = &authoring.ErrInvalid{
		Fields: authoring.FieldErrors{"title": []string{authoring.CodeTooLong}},
	}
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response, editorRequest(http.MethodPost, "/api/v1/account/posts",
		`{"title":"t","description":"d","access_policy":"public"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			Errors map[string][]string `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := envelope.Data.Errors["title"]; len(got) != 1 || got[0] != authoring.CodeTooLong {
		t.Errorf("errors.title = %v", got)
	}
}

func TestListingElementsPassesTheQueryThrough(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response, editorRequest(http.MethodGet,
		"/api/v1/account/posts/abcdefgh/elements?page=3&per_page=25&title=cat&sort_by=title&sort_dir=asc", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	got := service.lastQuery
	if got.Page != 3 || got.PerPage != 25 {
		t.Errorf("page = %d, per page = %d; want 3 and 25 — they are separate parameters",
			got.Page, got.PerPage)
	}
	if got.TitleLike != "cat" || got.SortBy != "title" || got.Descending {
		t.Errorf("query = %+v", got)
	}
}

// Newest first is what the original sorted by, so a request that says nothing gets that.
func TestElementsDefaultToNewestFirst(t *testing.T) {
	service := newFakeAuthoring()

	editorHandler(service).ServeHTTP(httptest.NewRecorder(),
		editorRequest(http.MethodGet, "/api/v1/account/posts/abcdefgh/elements", ""))

	if !service.lastQuery.Descending {
		t.Error("the default sort is ascending; the original sorted newest first")
	}
}

func TestEditingAnElementSendsOnlyTheFieldsPresent(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response,
		editorRequest(http.MethodPut, "/api/v1/account/elements/5", `{"title":"renamed"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastEdit.Title == nil || *service.lastEdit.Title != "renamed" {
		t.Errorf("title = %v", service.lastEdit.Title)
	}
	// The trim was not in the body, so it must not be written — that is what separates
	// "leave it" from "set it to zero".
	if service.lastEdit.StartSecond != nil || service.lastEdit.EndSecond != nil {
		t.Errorf("trim = %v, %v; want neither sent", service.lastEdit.StartSecond, service.lastEdit.EndSecond)
	}
}

func TestASentZeroTrimIsDistinctFromAnAbsentOne(t *testing.T) {
	service := newFakeAuthoring()

	editorHandler(service).ServeHTTP(httptest.NewRecorder(),
		editorRequest(http.MethodPut, "/api/v1/account/elements/5", `{"video_start_second":0}`))

	if service.lastEdit.StartSecond == nil {
		t.Fatal("a start of zero was read as absent")
	}
	if *service.lastEdit.StartSecond != 0 {
		t.Errorf("start = %d, want 0", *service.lastEdit.StartSecond)
	}
}

func TestDeletingAnElementAnswers204(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response,
		editorRequest(http.MethodDelete, "/api/v1/account/elements/5", ""))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if len(service.deleteElements) != 1 || service.deleteElements[0] != 5 {
		t.Errorf("deleted %v, want [5]", service.deleteElements)
	}
}

func TestANonNumericElementIDIsNotFound(t *testing.T) {
	service := newFakeAuthoring()
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response,
		editorRequest(http.MethodDelete, "/api/v1/account/elements/not-a-number", ""))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if len(service.deleteElements) != 0 {
		t.Error("a request with no valid id reached the service")
	}
}

func TestTheEditorMethodsAnswer405WithAllow(t *testing.T) {
	cases := []struct {
		method    string
		path      string
		wantAllow string
	}{
		{http.MethodPut, "/api/v1/account/posts", "GET, HEAD, POST"},
		{http.MethodPost, "/api/v1/account/posts/abcdefgh", "GET, HEAD, PUT, DELETE"},
		{http.MethodGet, "/api/v1/account/elements/5", "PUT, DELETE"},
	}
	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			editorHandler(newFakeAuthoring()).ServeHTTP(response,
				editorRequest(testCase.method, testCase.path, "{}"))

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", response.Code)
			}
			if allow := response.Header().Get("Allow"); allow != testCase.wantAllow {
				t.Errorf("Allow = %q, want %q", allow, testCase.wantAllow)
			}
		})
	}
}

func TestTheEditorRequiresABearerToken(t *testing.T) {
	handler := editorHandler(newFakeAuthoring())
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/account/posts"},
		{http.MethodPost, "/api/v1/account/posts"},
		{http.MethodGet, "/api/v1/account/posts/abcdefgh"},
		{http.MethodPut, "/api/v1/account/posts/abcdefgh"},
		{http.MethodDelete, "/api/v1/account/posts/abcdefgh"},
		{http.MethodGet, "/api/v1/account/posts/abcdefgh/elements"},
		{http.MethodPut, "/api/v1/account/elements/5"},
		{http.MethodDelete, "/api/v1/account/elements/5"},
	}
	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", response.Code)
			}
		})
	}
}

// A process built without the editor answers 503 rather than panicking on a nil service.
func TestTheEditorEndpointsAnswer503WhenItIsNotConfigured(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42"}},
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, editorRequest(http.MethodGet, "/api/v1/account/posts", ""))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
}

// The hash never leaves the server; the boolean the editor draws its form from does.
func TestAPostsResponseCarriesWhetherItHasAPasswordButNotTheHash(t *testing.T) {
	service := newFakeAuthoring()
	service.post.AccessPolicy = authoring.PolicyPassword
	service.post.HasPassword = true
	response := httptest.NewRecorder()

	editorHandler(service).ServeHTTP(response, editorRequest(http.MethodGet, "/api/v1/account/posts/abcdefgh", ""))

	data := decodeEditorData(t, response)
	if data["has_password"] != true {
		t.Errorf("has_password = %v, want true", data["has_password"])
	}
	if _, present := data["password"]; present {
		t.Error("the response carries a password field")
	}
	if strings.Contains(response.Body.String(), "hash") {
		t.Errorf("the response mentions a hash: %s", response.Body.String())
	}
}
