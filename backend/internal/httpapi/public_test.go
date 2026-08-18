package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"2pick.app/backend/internal/postaccess"
	"2pick.app/backend/internal/publiccontent"
)

type fakePublicContent struct {
	postsQuery publiccontent.PostsQuery
	ranksPage  int
	ranksLimit int
	ranksGroup publiccontent.RankGroup
	rankSerial string
	rankID     int64
	rankRanges []string
	caller     postaccess.Caller
	err        error
}

func (repository *fakePublicContent) Tags(context.Context, string, int) ([]publiccontent.Tag, error) {
	return []publiccontent.Tag{}, repository.err
}

func (repository *fakePublicContent) HotTags(context.Context, int) (map[string]int64, error) {
	return map[string]int64{}, repository.err
}

func (repository *fakePublicContent) CarouselItems(context.Context) ([]publiccontent.CarouselItem, error) {
	return []publiccontent.CarouselItem{}, repository.err
}

func (repository *fakePublicContent) Posts(_ context.Context, query publiccontent.PostsQuery) (publiccontent.PostsPage, error) {
	repository.postsQuery = query
	return publiccontent.PostsPage{Items: []publiccontent.Post{}, Page: query.Page, PerPage: query.PerPage}, repository.err
}

func (repository *fakePublicContent) Champions(context.Context, int) ([]publiccontent.Champion, error) {
	return []publiccontent.Champion{}, repository.err
}

func (repository *fakePublicContent) Ranks(_ context.Context, serial string, group publiccontent.RankGroup, page, perPage int, caller postaccess.Caller) (publiccontent.RanksPage, error) {
	repository.rankSerial = serial
	repository.caller = caller
	repository.ranksGroup = group
	repository.ranksPage = page
	repository.ranksLimit = perPage
	return publiccontent.RanksPage{Items: []publiccontent.RankReport{}, Page: page, PerPage: perPage}, repository.err
}

func (repository *fakePublicContent) SearchRanks(_ context.Context, _, _ string, _ int, caller postaccess.Caller) ([]publiccontent.RankReport, error) {
	repository.caller = caller
	return []publiccontent.RankReport{}, repository.err
}

func (repository *fakePublicContent) Rank(_ context.Context, serial string, elementID int64, ranges []string, caller postaccess.Caller) (publiccontent.RankDetails, error) {
	repository.rankSerial = serial
	repository.caller = caller
	repository.rankID = elementID
	repository.rankRanges = ranges
	return publiccontent.RankDetails{History: map[string][]publiccontent.RankHistory{}}, repository.err
}

func publicTestHandler(repository publiccontent.Repository) http.Handler {
	return New(Options{
		Environment:   "test",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		PublicContent: repository,
	})
}

func TestPostsEndpointDefaultsAndCacheContract(t *testing.T) {
	repository := &fakePublicContent{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	response := httptest.NewRecorder()

	publicTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.postsQuery.Sort != "hot" || repository.postsQuery.Range != "week" || repository.postsQuery.Page != 1 || repository.postsQuery.PerPage != 15 {
		t.Fatalf("query = %#v", repository.postsQuery)
	}
	if response.Header().Get("Cache-Control") != publicBrowserCache {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") != publicEdgeCache {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q", response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
}

func TestPublicGetEndpointSupportsHeadForCacheInspection(t *testing.T) {
	request := httptest.NewRequest(http.MethodHead, "/api/v1/posts", nil)
	response := httptest.NewRecorder()

	publicTestHandler(&fakePublicContent{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") != publicEdgeCache {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q", response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
}

func TestPostsEndpointRejectsInvalidPagination(t *testing.T) {
	repository := &fakePublicContent{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts?per_page=16", nil)
	response := httptest.NewRecorder()

	publicTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestRankEndpointAcceptsBothArrayQueryForms(t *testing.T) {
	repository := &fakePublicContent{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rank?post_serial=abc&element_id=42&time=week&time%5B%5D=month&time=week", nil)
	response := httptest.NewRecorder()

	publicTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.rankSerial != "abc" || repository.rankID != 42 {
		t.Fatalf("serial = %q, elementID = %d", repository.rankSerial, repository.rankID)
	}
	if len(repository.rankRanges) != 2 || repository.rankRanges[0] != "week" || repository.rankRanges[1] != "month" {
		t.Fatalf("ranges = %#v", repository.rankRanges)
	}
}

func TestRanksEndpointUsesPaginationAndPublicCache(t *testing.T) {
	repository := &fakePublicContent{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ranks?post_serial=abc&page=2&per_page=24", nil)
	response := httptest.NewRecorder()

	publicTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.rankSerial != "abc" || repository.ranksGroup != publiccontent.RankGroupCumulative || repository.ranksPage != 2 || repository.ranksLimit != 24 {
		t.Fatalf("serial = %q, group = %q, page = %d, perPage = %d", repository.rankSerial, repository.ranksGroup, repository.ranksPage, repository.ranksLimit)
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") != publicEdgeCache {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q", response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
}

func TestRanksEndpointSelectsRecentThousandGroup(t *testing.T) {
	repository := &fakePublicContent{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ranks?post_serial=abc&group=recent_1000", nil)
	response := httptest.NewRecorder()

	publicTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.ranksGroup != publiccontent.RankGroupRecent1000 {
		t.Fatalf("group = %q", repository.ranksGroup)
	}
}

func TestRanksEndpointRejectsInvalidQuery(t *testing.T) {
	for _, target := range []string{
		"/api/v1/ranks",
		"/api/v1/ranks?post_serial=abc&page=0",
		"/api/v1/ranks?post_serial=abc&per_page=51",
		"/api/v1/ranks?post_serial=abc&group=weekly",
	} {
		response := httptest.NewRecorder()
		publicTestHandler(&fakePublicContent{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("target = %q, status = %d, body = %s", target, response.Code, response.Body.String())
		}
	}
}

func TestPublicEndpointMapsMissingAndUnavailableRepositories(t *testing.T) {
	t.Run("database not configured", func(t *testing.T) {
		response := httptest.NewRecorder()
		publicTestHandler(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("public resource missing", func(t *testing.T) {
		response := httptest.NewRecorder()
		publicTestHandler(&fakePublicContent{err: publiccontent.ErrNotFound}).ServeHTTP(
			response,
			httptest.NewRequest(http.MethodGet, "/api/v1/rank/search?post_serial=missing&keyword=test", nil),
		)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})

	t.Run("query failure", func(t *testing.T) {
		response := httptest.NewRecorder()
		publicTestHandler(&fakePublicContent{err: errors.New("database offline")}).ServeHTTP(
			response,
			httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil),
		)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})
}

/*
The ranking of an adult post answers 401, not 404.

404 is what every other refused post read returns, and the browser reads it as "this post
wants a door code" — it would put a password box in front of a post that has no password.
The body must also stay out of the shared caches, since the answer depends on the caller.
*/
func TestRanksEndpointAsksAnAdultPostVisitorToSignIn(t *testing.T) {
	response := httptest.NewRecorder()
	publicTestHandler(&fakePublicContent{err: postaccess.ErrSignInRequired}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/ranks?post_serial=abc", nil),
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") != "" {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want none",
			response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
}
