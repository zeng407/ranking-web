package httpapi

import (
	"context"
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
