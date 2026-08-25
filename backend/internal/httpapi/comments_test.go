package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/comments"
)

type fakeComments struct {
	page        int
	viewer      comments.Viewer
	created     comments.CreateInput
	reportedID  int64
	reportInput comments.ReportInput
	deletedID   int64
	deleted     comments.Viewer
	err         error
}

func (repository *fakeComments) List(_ context.Context, _ string, page, _ int, viewer comments.Viewer) (comments.Page, error) {
	repository.page = page
	repository.viewer = viewer
	return comments.Page{
		Items: []comments.Comment{{ID: 7, Nickname: "Tester", Content: "hello", Champions: []string{"Winner"}}},
		Page:  page, PerPage: 10, Total: 11, TotalPages: 2,
		Profile: comments.Profile{Nickname: "Tester", IsAuthenticated: viewer.UserID != nil, Champions: []string{"Winner"}},
	}, repository.err
}

func (repository *fakeComments) Create(_ context.Context, _ string, input comments.CreateInput) (comments.Comment, error) {
	repository.created = input
	return comments.Comment{ID: 8, Nickname: "Tester", Content: input.Content}, repository.err
}

func (repository *fakeComments) Report(_ context.Context, _ string, commentID int64, input comments.ReportInput) error {
	repository.reportedID = commentID
	repository.reportInput = input
	return repository.err
}

func (repository *fakeComments) Delete(_ context.Context, _ string, commentID int64, viewer comments.Viewer) error {
	repository.deletedID = commentID
	repository.deleted = viewer
	return repository.err
}

func commentsTestHandler(repository comments.Repository) http.Handler {
	return New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{
			Subject: "42", ExpiresAt: time.Now().Add(time.Minute),
		}},
		Comments: repository,
	})
}

func TestCommentsEndpointRestoresListProfileAndPrivateCacheContract(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-1/comments?page=2", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.page != 2 || repository.viewer.UserID == nil || *repository.viewer.UserID != 42 {
		t.Fatalf("page = %d, viewer = %#v", repository.page, repository.viewer)
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Cloudflare-CDN-Cache-Control") != "no-store" {
		t.Fatalf("cache headers = %q / %q", response.Header().Get("Cache-Control"), response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
	if !strings.Contains(response.Body.String(), `"champions":["Winner"]`) || !strings.Contains(response.Body.String(), `"total_pages":2`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestCreateCommentValidatesAndForwardsOriginalAnonymousFeatures(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{
		"content":"new comment","anonymous":true,"anonymous_id":"browser-id"
	}`))
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.created.Content != "new comment" || !repository.created.Anonymous || repository.created.AnonymousID != "browser-id" {
		t.Fatalf("input = %#v", repository.created)
	}
	if repository.created.Viewer.UserID == nil || *repository.created.Viewer.UserID != 42 {
		t.Fatalf("viewer = %#v", repository.created.Viewer)
	}

	invalid := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(invalid, httptest.NewRequest(
		http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"   ","anonymous_id":"browser-id"}`),
	))
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
}

func TestReportCommentAcceptsPresetOrCustomReason(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts/post-1/comments/7/report", strings.NewReader(`{"reason":"Harassment","anonymous_id":"browser-id"}`))
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.reportedID != 7 || repository.reportInput.Reason != "Harassment" {
		t.Fatalf("comment = %d, input = %#v", repository.reportedID, repository.reportInput)
	}
}

// hashOf is what the API stores for a delete key: the hash, never the key.
func hashOf(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func TestCreateCommentIssuesADeleteKeyOnceAndSendsItsHashToTheRepository(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"mine"}`))
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var issued *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == commentKeyCookieName {
			issued = cookie
		}
	}
	if issued == nil {
		t.Fatalf("cookies = %#v", response.Result().Cookies())
	}
	if !issued.HttpOnly || issued.Path != commentKeyCookiePath || issued.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %#v", issued)
	}
	if repository.created.Viewer.DeleteHash != hashOf(issued.Value) {
		t.Fatalf("hash = %q, key = %q", repository.created.Viewer.DeleteHash, issued.Value)
	}
	if strings.Contains(response.Body.String(), issued.Value) {
		t.Fatalf("the key was echoed into the body: %s", response.Body.String())
	}

	// A browser that already holds a key keeps it: a second one would strand every
	// comment posted with the first.
	second := httptest.NewRequest(http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"also mine"}`))
	second.AddCookie(&http.Cookie{Name: commentKeyCookieName, Value: issued.Value})
	secondResponse := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(secondResponse, second)
	for _, cookie := range secondResponse.Result().Cookies() {
		if cookie.Name == commentKeyCookieName {
			t.Fatalf("a second key was issued: %#v", cookie)
		}
	}
	if repository.created.Viewer.DeleteHash != hashOf(issued.Value) {
		t.Fatalf("hash = %q", repository.created.Viewer.DeleteHash)
	}
}

func TestListCommentsCarriesTheDeleteKeyHashSoOwnershipCanBeAnswered(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-1/comments", nil)
	request.AddCookie(&http.Cookie{Name: commentKeyCookieName, Value: "guest-key"})
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if repository.viewer.DeleteHash != hashOf("guest-key") {
		t.Fatalf("viewer = %#v", repository.viewer)
	}
	// Reading is not posting: a reader with no key is not given one.
	bare := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(bare, httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-1/comments", nil))
	if len(bare.Result().Cookies()) != 0 {
		t.Fatalf("cookies = %#v", bare.Result().Cookies())
	}
}

func TestCreateCommentForwardsTheReplyTargetAndRejectsAnImpossibleOne(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"a reply","parent_id":7}`))
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if repository.created.ParentID == nil || *repository.created.ParentID != 7 {
		t.Fatalf("input = %#v", repository.created)
	}

	negative := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(negative, httptest.NewRequest(
		http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"a reply","parent_id":0}`),
	))
	if negative.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", negative.Code, negative.Body.String())
	}

	repository.err = comments.ErrInvalidParent
	tooDeep := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(tooDeep, httptest.NewRequest(
		http.MethodPost, "/api/v1/posts/post-1/comments", strings.NewReader(`{"content":"a reply","parent_id":7}`),
	))
	if tooDeep.Code != http.StatusUnprocessableEntity || !strings.Contains(tooDeep.Body.String(), "invalid_parent") {
		t.Fatalf("status = %d, body = %s", tooDeep.Code, tooDeep.Body.String())
	}
}

func TestDeleteCommentAnswersNoContentAndHidesWhoseCommentItWas(t *testing.T) {
	repository := &fakeComments{}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/post-1/comments/7", nil)
	request.AddCookie(&http.Cookie{Name: commentKeyCookieName, Value: "guest-key"})
	response := httptest.NewRecorder()

	commentsTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.deletedID != 7 || repository.deleted.DeleteHash != hashOf("guest-key") {
		t.Fatalf("comment = %d, viewer = %#v", repository.deletedID, repository.deleted)
	}
	// Deleting never mints a key. A browser that has none owns nothing, and issuing one
	// here would be issuing a claim on comments it did not write.
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == commentKeyCookieName {
			t.Fatalf("a key was issued on delete: %#v", cookie)
		}
	}

	repository.err = comments.ErrNotFound
	denied := httptest.NewRecorder()
	commentsTestHandler(repository).ServeHTTP(denied, httptest.NewRequest(http.MethodDelete, "/api/v1/posts/post-1/comments/7", nil))
	if denied.Code != http.StatusNotFound || strings.Contains(denied.Body.String(), "forbidden") {
		t.Fatalf("status = %d, body = %s", denied.Code, denied.Body.String())
	}
}
