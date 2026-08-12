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
	"time"

	"2pick.app/backend/internal/gameplay"
	"2pick.app/backend/internal/postaccess"
)

/*
The door-code endpoint and the header that carries its proof.

The rule these enforce is small and the cost of getting it wrong is not: a protected post
whose ranks answer to anyone, or a token that opens a post it was never issued for. So the
tests here work through the real Signer rather than a fake — a stub that says "yes" would
pass whatever the handler did with the serial.
*/

func postAccessTestService(t *testing.T, posts map[string]postaccess.Post) *postaccess.Service {
	t.Helper()
	signer, err := postaccess.NewSigner([]byte("a-deployment-secret"))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}
	service, err := postaccess.NewService(postaccess.ServiceOptions{
		Store: stubPostStore(posts), Signer: signer,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

type stubPostStore map[string]postaccess.Post

func (store stubPostStore) Post(_ context.Context, serial string) (postaccess.Post, error) {
	post, found := store[serial]
	if !found {
		return postaccess.Post{}, postaccess.ErrPostNotFound
	}
	return post, nil
}

func lockedPosts() map[string]postaccess.Post {
	return map[string]postaccess.Post{
		"post-1": {ID: 1, Serial: "post-1", OwnerID: 7,
			Policy: postaccess.PolicyPassword, PasswordDigest: postaccess.HashPassword("door-code")},
	}
}

func postAccessHandler(t *testing.T, repository gameplay.Repository) (http.Handler, *postaccess.Service) {
	t.Helper()
	service := postAccessTestService(t, lockedPosts())
	return New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Gameplay:    repository,
		PostAccess:  service,
	}), service
}

func grant(t *testing.T, handler http.Handler, serial, password string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+serial+"/access",
		strings.NewReader(`{"password":`+quote(password)+`}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func quote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestTheRightDoorCodeReturnsAToken(t *testing.T) {
	handler, service := postAccessHandler(t, &fakeGameplay{})

	response := grant(t, handler, "post-1", "door-code")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Data postAccessResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := service.Verify("post-1", body.Data.Token); err != nil {
		t.Fatalf("the token handed out does not verify: %v", err)
	}
	if body.Data.ExpiresIn < 1500 || body.Data.ExpiresIn > 1800 {
		t.Errorf("expires_in = %d, want about %d", body.Data.ExpiresIn, int(postaccess.TTL.Seconds()))
	}
	if _, err := time.Parse(time.RFC3339, body.Data.ExpiresAt); err != nil {
		t.Errorf("expires_at = %q is not RFC3339", body.Data.ExpiresAt)
	}
}

// The password must never come back out, in the token or anywhere else on the response.
func TestTheDoorCodeIsNotEchoedBack(t *testing.T) {
	handler, _ := postAccessHandler(t, &fakeGameplay{})

	response := grant(t, handler, "post-1", "door-code")

	if strings.Contains(response.Body.String(), "door-code") {
		t.Errorf("the password appears in the response: %s", response.Body.String())
	}
}

func TestTheWrongDoorCodeIsForbidden(t *testing.T) {
	handler, _ := postAccessHandler(t, &fakeGameplay{})

	response := grant(t, handler, "post-1", "not-the-code")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	// No challenge header: a WWW-Authenticate would make the browser pop its own
	// credential dialog for a password that is not an account's.
	if response.Header().Get("WWW-Authenticate") != "" {
		t.Errorf("WWW-Authenticate = %q, want none", response.Header().Get("WWW-Authenticate"))
	}
}

func TestAnUnknownPostIsNotFound(t *testing.T) {
	handler, _ := postAccessHandler(t, &fakeGameplay{})

	if response := grant(t, handler, "nosuchpost", "door-code"); response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestTheAccessEndpointIs503WithoutTheService(t *testing.T) {
	handler := New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Gameplay:    &fakeGameplay{},
	})

	response := grant(t, handler, "post-1", "door-code")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

/*
THE PROOF HAS TO REACH THE QUERY.

The handler could check the token itself and then call a repository that still filters on
public — which would pass any test that only looked at the status code. These assert on
what the repository was actually told.
*/
func TestAnUnlockedSerialReachesTheGameplayRepository(t *testing.T) {
	repository := &fakeGameplay{}
	handler, service := postAccessHandler(t, repository)
	token, _ := service.Reissue("post-1")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil)
	request.Header.Set(postAccessHeader, "post-1:"+token)
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if !repository.caller.Unlocked("post-1") {
		t.Fatalf("caller = %#v, want post-1 unlocked", repository.caller)
	}
}

func TestAForgedTokenLeavesTheCallerWithNothing(t *testing.T) {
	repository := &fakeGameplay{}
	handler, _ := postAccessHandler(t, repository)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil)
	request.Header.Set(postAccessHeader, "post-1:9999999999.bm90LWEtc2lnbmF0dXJl")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if len(repository.caller.UnlockedSerials) != 0 {
		t.Fatalf("caller = %#v, want nothing unlocked", repository.caller)
	}
}

// A token minted for one post presented on another must not carry over. The serial is
// signed, so this is really a check that the handler passes the header's serial through
// rather than the path's.
func TestATokenForAnotherPostDoesNotUnlockThisOne(t *testing.T) {
	repository := &fakeGameplay{}
	handler, service := postAccessHandler(t, repository)
	token, _ := service.Reissue("post-1")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-2", nil)
	request.Header.Set(postAccessHeader, "post-2:"+token)
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if repository.caller.Unlocked("post-2") {
		t.Fatalf("caller = %#v, want post-2 locked", repository.caller)
	}
}

// A request with no header at all is the common case, and must produce the zero caller —
// which is the public view, not an error.
func TestNoHeaderMeansThePublicView(t *testing.T) {
	repository := &fakeGameplay{}
	handler, _ := postAccessHandler(t, repository)

	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil))

	if repository.caller.UserID != 0 || len(repository.caller.UnlockedSerials) != 0 {
		t.Fatalf("caller = %#v, want the zero value", repository.caller)
	}
}

// A response that only that caller could have seen must not be publicly cacheable: a
// shared cache would hand a protected post's page to the next anonymous visitor.
func TestAnUnlockedResponseIsNotPubliclyCacheable(t *testing.T) {
	handler, service := postAccessHandler(t, &fakeGameplay{})
	token, _ := service.Reissue("post-1")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil)
	request.Header.Set(postAccessHeader, "post-1:"+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

// AccessTokenService::extendPostAccessToken pushed the expiry forward on every use, so a
// visitor part-way through a long game was not locked out. Here that is a fresh token on
// the response.
func TestAUsedTokenComesBackRefreshed(t *testing.T) {
	handler, service := postAccessHandler(t, &fakeGameplay{})
	token, _ := service.Reissue("post-1")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil)
	request.Header.Set(postAccessHeader, "post-1:"+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	serial, refreshed, found := strings.Cut(response.Header().Get(postAccessHeader), ":")
	if !found || serial != "post-1" {
		t.Fatalf("%s = %q", postAccessHeader, response.Header().Get(postAccessHeader))
	}
	if err := service.Verify("post-1", refreshed); err != nil {
		t.Fatalf("the refreshed token does not verify: %v", err)
	}
}

// A caller who proved nothing gets no header back, so the SPA has nothing to store for a
// post it never unlocked.
func TestAPublicRequestGetsNoTokenBack(t *testing.T) {
	handler, _ := postAccessHandler(t, &fakeGameplay{})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil))

	if got := response.Header().Get(postAccessHeader); got != "" {
		t.Fatalf("%s = %q, want none", postAccessHeader, got)
	}
}

func TestTheHeaderIsExposedToTheBrowser(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil)
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()

	New(Options{
		Environment:    "test",
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Gameplay:       &fakeGameplay{},
		AllowedOrigins: []string{"http://localhost:4173"},
	}).ServeHTTP(response, request)

	if !strings.Contains(response.Header().Get("Access-Control-Expose-Headers"), postAccessHeader) {
		t.Fatalf("Access-Control-Expose-Headers = %q", response.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestTheHeaderIsAllowedOnAPreflight(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/posts/post-1/access", nil)
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()

	New(Options{
		Environment:    "test",
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:4173"},
	}).ServeHTTP(response, request)

	if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), postAccessHeader) {
		t.Fatalf("Access-Control-Allow-Headers = %q", response.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestPresentedPostTokensReadsEveryShapeTheClientMaySend(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Add(postAccessHeader, "post-1:aaa, post-2:bbb")
	request.Header.Add(postAccessHeader, "post-3:ccc")

	pairs := presentedPostTokens(request)

	if len(pairs) != 3 || pairs["post-1"] != "aaa" || pairs["post-2"] != "bbb" || pairs["post-3"] != "ccc" {
		t.Fatalf("pairs = %#v", pairs)
	}
}

// A token contains a dot, so the separator must be the first colon and not the last.
func TestPresentedPostTokensKeepsTheWholeToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(postAccessHeader, "post-1:1234567890.c2lnbmF0dXJl")

	if got := presentedPostTokens(request)["post-1"]; got != "1234567890.c2lnbmF0dXJl" {
		t.Fatalf("token = %q", got)
	}
}

// One unusable entry must not discard the rest: the client sends whatever it has stored,
// and a stale entry has nothing to do with the post being asked for.
func TestPresentedPostTokensSkipsRubbishWithoutDroppingTheRest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(postAccessHeader, "no-colon, :empty-serial, post-2:, post-3:ccc")

	pairs := presentedPostTokens(request)

	if len(pairs) != 1 || pairs["post-3"] != "ccc" {
		t.Fatalf("pairs = %#v", pairs)
	}
}

// Each accepted pair becomes a placeholder in an IN list on every post query, so the
// header is capped rather than trusted for length.
func TestPresentedPostTokensIsCapped(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for index := 0; index < maxPostAccessTokens*3; index++ {
		request.Header.Add(postAccessHeader, "post-"+string(rune('a'+index%26))+string(rune('a'+index/26))+":token")
	}

	if got := len(presentedPostTokens(request)); got > maxPostAccessTokens {
		t.Fatalf("accepted %d pairs, want at most %d", got, maxPostAccessTokens)
	}
}
