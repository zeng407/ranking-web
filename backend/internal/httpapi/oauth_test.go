package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/auth"
)

// fakeOAuth stands in for auth.OAuthService. The flow's own rules are covered in
// internal/auth; what matters here is redirects, cookies and what does NOT appear in a
// URL.
type fakeOAuth struct {
	authorizationURL string
	completed        auth.CompletedFlow

	startErr    error
	completeErr error

	startCalls    int
	completeCalls int
	lastReturnTo  string
	lastConnectID int64
	lastState     string
	lastCode      string
	lastClientIP  string
}

func newFakeOAuth() *fakeOAuth {
	return &fakeOAuth{
		authorizationURL: "https://accounts.google.com/o/oauth2/v2/auth?client_id=test&state=abc",
		completed: auth.CompletedFlow{
			Grant: auth.Grant{
				Access: auth.AccessToken{
					Token: "header.claims.signature", TokenType: "Bearer", ExpiresIn: 300,
					ExpiresAt: time.Now().Add(5 * time.Minute),
				},
				Refresh: auth.IssuedRefresh{
					Token: "opaque-refresh-token", CSRFToken: "the-csrf-token",
					FamilyID: "family-1", ExpiresAt: time.Now().Add(720 * time.Hour),
				},
				UserID: 42,
			},
			ReturnTo: "http://localhost:4173/",
			UserID:   42,
		},
	}
}

func (service *fakeOAuth) Start(
	_ context.Context, returnTo string, connectUserID int64,
) (auth.StartedFlow, error) {
	service.startCalls++
	service.lastReturnTo, service.lastConnectID = returnTo, connectUserID
	if service.startErr != nil {
		return auth.StartedFlow{}, service.startErr
	}
	return auth.StartedFlow{AuthorizationURL: service.authorizationURL, State: "abc"}, nil
}

func (service *fakeOAuth) Complete(
	_ context.Context, state, code string, client auth.ClientInfo,
) (auth.CompletedFlow, error) {
	service.completeCalls++
	service.lastState, service.lastCode = state, code
	service.lastClientIP = client.IP
	if service.completeErr != nil {
		return auth.CompletedFlow{}, service.completeErr
	}
	return service.completed, nil
}

func oauthTestHandler(service OAuthService) http.Handler {
	return New(Options{
		Environment:    "test",
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:4173"},
		OAuthService:   service,
		// A verifier has to be present or requireAuth answers 503 before it ever looks
		// at the token, and the connect tests would be asserting the wrong refusal.
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42"}},
	})
}

func TestOAuthStartRedirectsToTheProvider(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/start?return_to=http://localhost:4173/profile", nil)

	oauthTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body = %s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); location != service.authorizationURL {
		t.Errorf("Location = %q, want the provider's url", location)
	}
	// A cached 302 would send a later visitor to a state that has since been consumed.
	if cacheControl := response.Header().Get("Cache-Control"); !strings.Contains(cacheControl, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cacheControl)
	}
	if service.lastReturnTo != "http://localhost:4173/profile" {
		t.Errorf("return_to was not passed through: %q", service.lastReturnTo)
	}
	// Zero means "this is a login, not a link".
	if service.lastConnectID != 0 {
		t.Errorf("connect user id = %d on a plain login", service.lastConnectID)
	}
}

func TestOAuthEndpointsAnswer503WhenUnconfigured(t *testing.T) {
	handler := New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, path := range []string{
		"/api/v1/auth/oauth/google/start",
		"/api/v1/auth/oauth/google/callback?state=abc&code=xyz",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", path, response.Code)
		}
	}
}

// THE CALLBACK MUST NOT PUT A TOKEN IN THE URL. A URL reaches the browser's history,
// the Referer header of the next request, and any proxy log in between. The session
// travels as cookies on this same response instead.
func TestTheCallbackPutsNoTokensInTheRedirect(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?state=abc&code=xyz", nil))

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body = %s", response.Code, response.Body.String())
	}

	location := response.Header().Get("Location")
	for _, secret := range []string{
		service.completed.Grant.Access.Token,
		service.completed.Grant.Refresh.Token,
		service.completed.Grant.Refresh.CSRFToken,
	} {
		if strings.Contains(location, secret) {
			t.Errorf("the redirect url contains %q: %s", secret, location)
		}
	}
	// And not in the body either, which for a redirect is a courtesy page.
	if body := response.Body.String(); strings.Contains(body, service.completed.Grant.Refresh.Token) {
		t.Errorf("the response body contains the refresh token: %s", body)
	}
}

func TestASuccessfulCallbackSetsTheSessionCookiesAndReports(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?state=abc&code=xyz", nil)
	request.RemoteAddr = "203.0.113.7:41234"

	oauthTestHandler(service).ServeHTTP(response, request)

	refresh := cookieByName(response, "2pick_refresh")
	if refresh == nil || refresh.Value != service.completed.Grant.Refresh.Token {
		t.Fatalf("the refresh cookie was not set: %+v", refresh)
	}
	if !refresh.HttpOnly {
		t.Error("the refresh cookie must be httpOnly")
	}
	csrf := cookieByName(response, "2pick_csrf")
	if csrf == nil || csrf.Value != service.completed.Grant.Refresh.CSRFToken {
		t.Fatalf("the csrf cookie was not set: %+v", csrf)
	}
	if csrf.HttpOnly {
		t.Error("the csrf cookie must be readable by script")
	}

	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("the redirect url does not parse: %v", err)
	}
	if got := location.Query().Get("auth"); got != "signed-in" {
		t.Errorf("auth = %q, want signed-in", got)
	}
	if service.lastState != "abc" || service.lastCode != "xyz" {
		t.Errorf("state/code were not passed through: %q %q", service.lastState, service.lastCode)
	}
	if service.lastClientIP != "203.0.113.7" {
		t.Errorf("client ip = %q, want the port stripped", service.lastClientIP)
	}
}

// A first sign-in is reported differently, because a brand new account has no nickname
// and the SPA sends those users somewhere else.
func TestANewAccountIsReportedAsRegistered(t *testing.T) {
	service := newFakeOAuth()
	service.completed.Created = true
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?state=abc&code=xyz", nil))

	location, _ := url.Parse(response.Header().Get("Location"))
	if got := location.Query().Get("auth"); got != "registered" {
		t.Errorf("auth = %q, want registered", got)
	}
}

// A link must not hand out a session. The caller was already signed in when the flow
// started, and setting cookies here would let a link escalate into a login.
func TestALinkCallbackSetsNoCookies(t *testing.T) {
	service := newFakeOAuth()
	service.completed = auth.CompletedFlow{Linked: true, ReturnTo: "http://localhost:4173/profile", UserID: 7}
	response := httptest.NewRecorder()

	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?state=abc&code=xyz", nil))

	if cookie := cookieByName(response, "2pick_refresh"); cookie != nil {
		t.Errorf("a link set a session cookie: %+v", cookie)
	}
	location, _ := url.Parse(response.Header().Get("Location"))
	if got := location.Query().Get("auth"); got != "linked" {
		t.Errorf("auth = %q, want linked", got)
	}
	if location.Path != "/profile" {
		t.Errorf("returned to %q, want the target the flow remembered", location.Path)
	}
}

// Every failure is a redirect, not a JSON error: this endpoint is reached by a top-level
// navigation, so the person on the other side is looking at a page.
func TestEveryCallbackFailureRedirectsWithAReason(t *testing.T) {
	cases := map[error]string{
		auth.ErrOAuthEmailTaken:       "email-taken",
		auth.ErrOAuthEmailUnverified:  "email-unverified",
		auth.ErrOAuthAlreadyLinked:    "already-linked",
		auth.ErrOAuthStateInvalid:     "expired",
		errors.New("something broke"): "failed",
	}

	for failure, wantReason := range cases {
		service := newFakeOAuth()
		service.completeErr = failure
		response := httptest.NewRecorder()
		oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
			"/api/v1/auth/oauth/google/callback?state=abc&code=xyz", nil))

		if response.Code != http.StatusFound {
			t.Errorf("%v: status = %d, want 302", failure, response.Code)
			continue
		}
		location, err := url.Parse(response.Header().Get("Location"))
		if err != nil {
			t.Errorf("%v: the redirect does not parse: %v", failure, err)
			continue
		}
		if got := location.Query().Get("auth"); got != "failed" {
			t.Errorf("%v: auth = %q, want failed", failure, got)
		}
		if got := location.Query().Get("reason"); got != wantReason {
			t.Errorf("%v: reason = %q, want %q", failure, got, wantReason)
		}
		// A failed flow must not leave a half-set session behind.
		if cookie := cookieByName(response, "2pick_refresh"); cookie == nil || cookie.MaxAge >= 0 {
			t.Errorf("%v: the refresh cookie was not cleared: %+v", failure, cookie)
		}
	}
}

// The consent screen being dismissed arrives as ?error=access_denied with no code. There
// is nothing to exchange, so the provider must not be called at all.
func TestADeclinedConsentScreenNeverReachesTheService(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?error=access_denied&state=abc", nil))

	if service.completeCalls != 0 {
		t.Errorf("the service was called %d times for a declined consent", service.completeCalls)
	}
	location, _ := url.Parse(response.Header().Get("Location"))
	if got := location.Query().Get("reason"); got != "declined" {
		t.Errorf("reason = %q, want declined", got)
	}
}

// A failure has no validated per-flow target to return to — the state that held it may
// be exactly what could not be read — so it falls back to the configured origin rather
// than to this API's own root, which is a 404 with nothing to show.
func TestAFailureFallsBackToTheConfiguredOrigin(t *testing.T) {
	service := newFakeOAuth()
	service.completeErr = auth.ErrOAuthStateInvalid
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/callback?state=stale&code=xyz", nil))

	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("the redirect does not parse: %v", err)
	}
	if location.Host != "localhost:4173" {
		t.Errorf("failed back to %q, want the allowed origin", location.String())
	}
}

// A start that fails is a JSON 500, not a redirect: nothing has happened yet and the
// caller is the SPA, which asked for this with fetch or a navigation it controls.
func TestAFailedStartIsAnError(t *testing.T) {
	service := newFakeOAuth()
	service.startErr = errors.New("redis is down")
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/start", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "redis") {
		t.Errorf("the internal error leaked into the response: %s", response.Body.String())
	}
}

// connect answers with a URL rather than redirecting, because it needs an Authorization
// header and a browser cannot put one on a navigation. Without a token it must refuse.
func TestConnectRequiresAuthentication(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/auth/oauth/google/connect", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if service.startCalls != 0 {
		t.Error("an unauthenticated request started a link flow")
	}
}

// With a token, connect returns the URL for the SPA to navigate to — and remembers WHO
// is linking, because the callback that follows carries no credentials at all.
func TestConnectReturnsAURLAndRemembersTheUser(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/auth/oauth/google/connect?return_to=http://localhost:4173/profile", nil)
	request.Header.Set("Authorization", "Bearer a.valid.token")

	oauthTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	// A URL in the body, not a Location header: the SPA navigates itself.
	if response.Header().Get("Location") != "" {
		t.Error("connect answered with a redirect; the SPA cannot attach a token to one")
	}
	if !strings.Contains(response.Body.String(), "accounts.google.com") {
		t.Errorf("the body carries no authorization url: %s", response.Body.String())
	}
	// staticTokenVerifier reports subject "42".
	if service.lastConnectID != 42 {
		t.Errorf("connect user id = %d, want 42 from the token's subject", service.lastConnectID)
	}
	if service.lastReturnTo != "http://localhost:4173/profile" {
		t.Errorf("return_to = %q", service.lastReturnTo)
	}
}

// GET must not reach connect: it is the one endpoint here that is not a navigation, and
// allowing the navigation form back would reintroduce the credential problem it exists
// to avoid.
func TestConnectRejectsGet(t *testing.T) {
	service := newFakeOAuth()
	response := httptest.NewRecorder()
	oauthTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oauth/google/connect", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", response.Code)
	}
}

func TestOAuthReturnAllowlistEntriesEndWithASlash(t *testing.T) {
	// Without the trailing slash, "http://localhost:4173" is a prefix of
	// "http://localhost:4173.evil.test" and the allowlist would authorise it.
	allowlist := OAuthReturnAllowlist([]string{"http://localhost:4173", "http://localhost:5173/"})
	want := []string{"http://localhost:4173/", "http://localhost:5173/"}
	if len(allowlist) != len(want) {
		t.Fatalf("allowlist = %v, want %v", allowlist, want)
	}
	for index, entry := range want {
		if allowlist[index] != entry {
			t.Errorf("entry %d = %q, want %q", index, allowlist[index], entry)
		}
	}
}

// A wildcard CORS entry must not become a wildcard redirect target: those are different
// permissions, and one of them is an open redirect.
func TestAWildcardOriginIsNotAReturnTarget(t *testing.T) {
	if allowlist := OAuthReturnAllowlist([]string{"*"}); len(allowlist) != 0 {
		t.Errorf("allowlist = %v, want it to drop the wildcard", allowlist)
	}
	if allowlist := OAuthReturnAllowlist([]string{"", "   "}); len(allowlist) != 0 {
		t.Errorf("allowlist = %v, want it to drop empty entries", allowlist)
	}
}

func TestAppendQueryKeepsAnExistingQuery(t *testing.T) {
	got := appendQuery("http://localhost:4173/profile?tab=links", map[string]string{"auth": "linked"})
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
	if parsed.Query().Get("tab") != "links" {
		t.Errorf("the existing query was lost: %s", got)
	}
	if parsed.Query().Get("auth") != "linked" {
		t.Errorf("the new parameter is missing: %s", got)
	}
}
