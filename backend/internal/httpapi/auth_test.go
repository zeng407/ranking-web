package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/auth"
)

// fakeAuth stands in for auth.Service so the transport behaviour can be tested on
// its own. The service's own rules are covered in internal/auth; what matters here
// is cookies, headers and status codes.
type fakeAuth struct {
	grant auth.Grant

	loginErr    error
	registerErr error
	refreshErr  error
	csrfErr     error
	logoutErr   error

	// The account settings half. account is what Account and ChangeName answer with;
	// accountErr overrides it.
	account       auth.Account
	accountErr    error
	avatarURL     string
	avatarErr     error
	passwordErr   error
	initErr       error
	nameCalls     int
	avatarCalls   int
	passwordCalls int
	initCalls     int
	lastNewName   string
	lastCurrent   string
	lastNew       string
	lastAvatar    []byte
	lastAvatarKey string

	loginCalls    int
	registerCalls int
	refreshCalls  int
	logoutCalls   int
	lastEmail     string
	lastName      string
	lastPassword  string
	lastRefresh   string
	lastCSRF      string
	lastClientIP  string
	lastUserAgent string
}

func newFakeAuth() *fakeAuth {
	return &fakeAuth{grant: auth.Grant{
		Access: auth.AccessToken{
			Token: "header.claims.signature", TokenType: "Bearer", ExpiresIn: 300,
			ExpiresAt: time.Now().Add(5 * time.Minute),
		},
		Refresh: auth.IssuedRefresh{
			Token: "opaque-refresh-token", CSRFToken: "the-csrf-token",
			FamilyID: "family-1", ExpiresAt: time.Now().Add(720 * time.Hour),
		},
		UserID: 42,
		Roles:  []string{"admin"},
	}}
}

func (service *fakeAuth) Login(_ context.Context, email, password string, client auth.ClientInfo) (auth.Grant, error) {
	service.loginCalls++
	service.lastEmail, service.lastPassword = email, password
	service.lastClientIP, service.lastUserAgent = client.IP, client.UserAgent
	if service.loginErr != nil {
		return auth.Grant{}, service.loginErr
	}
	return service.grant, nil
}

func (service *fakeAuth) Register(
	_ context.Context, registration auth.Registration, client auth.ClientInfo,
) (auth.Grant, error) {
	service.registerCalls++
	service.lastEmail, service.lastPassword = registration.Email, registration.Password
	service.lastName = registration.Name
	service.lastClientIP, service.lastUserAgent = client.IP, client.UserAgent
	if service.registerErr != nil {
		return auth.Grant{}, service.registerErr
	}
	return service.grant, nil
}

func (service *fakeAuth) Refresh(_ context.Context, refreshToken, csrfToken string, _ auth.ClientInfo) (auth.Grant, error) {
	service.refreshCalls++
	service.lastRefresh, service.lastCSRF = refreshToken, csrfToken
	if service.refreshErr != nil {
		return auth.Grant{}, service.refreshErr
	}
	return service.grant, nil
}

func (service *fakeAuth) Logout(_ context.Context, refreshToken string) error {
	service.logoutCalls++
	service.lastRefresh = refreshToken
	return service.logoutErr
}

func (service *fakeAuth) VerifyCSRF(_ context.Context, refreshToken, csrfToken string) error {
	service.lastRefresh, service.lastCSRF = refreshToken, csrfToken
	return service.csrfErr
}

func authTestHandler(service AuthService) http.Handler {
	return New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService: service,
	})
}

func loginPost(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func cookieByName(response *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// THE ASYMMETRY THAT MAKES THE CSRF DEFENCE WORK. The refresh cookie must be
// unreadable by script; the CSRF cookie must be readable, because the client has to
// copy it into a header. Getting either backwards silently removes the protection:
// httpOnly on the CSRF cookie breaks the client, and httpOnly missing on the refresh
// cookie hands the session to any XSS.
func TestLoginSetsAnHTTPOnlyRefreshCookieAndAReadableCSRFCookie(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		loginPost(`{"email":"player@example.test","password":"secret"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	refresh := cookieByName(response, "2pick_refresh")
	if refresh == nil {
		t.Fatal("no refresh cookie was set")
	}
	if !refresh.HttpOnly {
		t.Error("the refresh cookie must be httpOnly; without it XSS can read the session")
	}
	if refresh.Value != service.grant.Refresh.Token {
		t.Errorf("refresh cookie value = %q", refresh.Value)
	}
	// Scoped to the auth endpoints, so it is not attached to every other API call.
	if refresh.Path != "/api/v1/auth" {
		t.Errorf("refresh cookie path = %q, want /api/v1/auth", refresh.Path)
	}
	if refresh.SameSite != http.SameSiteStrictMode {
		t.Errorf("refresh cookie SameSite = %v, want Strict", refresh.SameSite)
	}

	csrf := cookieByName(response, "2pick_csrf")
	if csrf == nil {
		t.Fatal("no csrf cookie was set")
	}
	if csrf.HttpOnly {
		t.Error("the csrf cookie must NOT be httpOnly: the client has to read it to echo it back")
	}
	if csrf.Path != "/" {
		t.Errorf("csrf cookie path = %q, want /", csrf.Path)
	}
}

// The refresh token must never reach the response body, where script could read it
// and defeat the httpOnly cookie entirely.
func TestLoginDoesNotLeakTheRefreshTokenInTheBody(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		loginPost(`{"email":"player@example.test","password":"secret"}`))

	body := response.Body.String()
	if strings.Contains(body, service.grant.Refresh.Token) {
		t.Fatalf("the response body contains the refresh token: %s", body)
	}
	// The access token and the CSRF value do belong there.
	for _, expected := range []string{service.grant.Access.Token, service.grant.Refresh.CSRFToken} {
		if !strings.Contains(body, expected) {
			t.Errorf("the body is missing %q: %s", expected, body)
		}
	}
}

func TestLoginPassesTheCredentialsAndClientInfoThrough(t *testing.T) {
	service := newFakeAuth()
	request := loginPost(`{"email":" Player@Example.test ","password":"secret"}`)
	request.RemoteAddr = "203.0.113.7:54321"
	request.Header.Set("User-Agent", "probe/1.0")

	authTestHandler(service).ServeHTTP(httptest.NewRecorder(), request)

	// Trimming and case folding are the service's job, so the handler must not do it.
	if service.lastEmail != " Player@Example.test " {
		t.Errorf("email = %q; the handler should pass it through unchanged", service.lastEmail)
	}
	if service.lastPassword != "secret" {
		t.Errorf("password = %q", service.lastPassword)
	}
	// The port is stripped: it is not part of the client's identity.
	if service.lastClientIP != "203.0.113.7" {
		t.Errorf("client ip = %q, want 203.0.113.7", service.lastClientIP)
	}
	if service.lastUserAgent != "probe/1.0" {
		t.Errorf("user agent = %q", service.lastUserAgent)
	}
}

// Login, refresh and CSRF failures must be indistinguishable to the caller. Anything
// more specific tells an attacker probing addresses or tokens what they hit.
func TestAuthFailuresAllAnswer401(t *testing.T) {
	cases := map[string]error{
		"bad credentials": auth.ErrInvalidCredentials,
		"unknown token":   auth.ErrRefreshTokenInvalid,
		"replayed token":  auth.ErrRefreshTokenReused,
		"csrf mismatch":   auth.ErrCSRFMismatch,
	}
	for name, failure := range cases {
		service := newFakeAuth()
		service.loginErr = failure
		response := httptest.NewRecorder()
		authTestHandler(service).ServeHTTP(response,
			loginPost(`{"email":"player@example.test","password":"secret"}`))

		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, response.Code)
		}
		// And no cookie is handed out on a failure.
		if cookieByName(response, "2pick_refresh") != nil && cookieByName(response, "2pick_refresh").Value != "" {
			t.Errorf("%s: a refresh cookie was set on a failed login", name)
		}
	}
}

// An unexpected failure is a 500, not a 401: telling a user their password is wrong
// during a database outage is both untrue and hides the outage.
func TestAnUnexpectedAuthFailureIs500(t *testing.T) {
	service := newFakeAuth()
	service.loginErr = errors.New("connection refused")
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		loginPost(`{"email":"player@example.test","password":"secret"}`))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	// The underlying message must not reach the client.
	if strings.Contains(response.Body.String(), "connection refused") {
		t.Errorf("the response leaks the internal error: %s", response.Body.String())
	}
}

func refreshPost(cookieValue, csrfHeader string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	if cookieValue != "" {
		request.AddCookie(&http.Cookie{Name: "2pick_refresh", Value: cookieValue})
	}
	if csrfHeader != "" {
		request.Header.Set("X-CSRF-Token", csrfHeader)
	}
	return request
}

func TestRefreshForwardsTheCookieAndHeader(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, refreshPost("cookie-token", "echoed-csrf"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.lastRefresh != "cookie-token" {
		t.Errorf("refresh token = %q, want the cookie value", service.lastRefresh)
	}
	// The header, not a body field or a query parameter: a value the browser would
	// attach automatically could not serve as a CSRF token.
	if service.lastCSRF != "echoed-csrf" {
		t.Errorf("csrf = %q, want the X-CSRF-Token header", service.lastCSRF)
	}
}

// No cookie is "not logged in", and it must not reach the service at all.
func TestRefreshWithoutACookieIs401AndNeverCallsTheService(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, refreshPost("", "echoed-csrf"))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if service.refreshCalls != 0 {
		t.Errorf("the service was called %d times for a request with no cookie", service.refreshCalls)
	}
}

// A failed refresh clears the cookies. Leaving one the server will keep rejecting
// puts the client in a loop it cannot escape.
func TestAFailedRefreshClearsTheSessionCookies(t *testing.T) {
	service := newFakeAuth()
	service.refreshErr = auth.ErrRefreshTokenReused
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, refreshPost("cookie-token", "echoed-csrf"))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	for _, name := range []string{"2pick_refresh", "2pick_csrf"} {
		cookie := cookieByName(response, name)
		if cookie == nil {
			t.Fatalf("%s was not cleared", name)
		}
		if cookie.Value != "" || cookie.MaxAge >= 0 {
			t.Errorf("%s = %q with MaxAge %d; want an expiring empty value", name, cookie.Value, cookie.MaxAge)
		}
	}
}

// ---------- logout ----------

func TestLogoutRequiresCSRFAndThenRevokes(t *testing.T) {
	service := newFakeAuth()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "2pick_refresh", Value: "cookie-token"})
	request.Header.Set("X-CSRF-Token", "echoed-csrf")

	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if service.logoutCalls != 1 {
		t.Errorf("logout called %d times, want 1", service.logoutCalls)
	}
}

// A forged logout is a small denial of service, and free to prevent.
func TestLogoutWithoutCSRFDoesNotRevoke(t *testing.T) {
	service := newFakeAuth()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "2pick_refresh", Value: "cookie-token"})

	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if service.logoutCalls != 0 {
		t.Errorf("the session was revoked without a csrf token")
	}
}

// Logging out with no cookie, or one the server has forgotten, still has to clear the
// client's state — otherwise a stale cookie can never be shed.
func TestLogoutWithoutACookieStillClearsAndSucceeds(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if cookieByName(response, "2pick_refresh") == nil {
		t.Error("the refresh cookie was not cleared")
	}
	if service.logoutCalls != 0 {
		t.Error("the service should not be called with no cookie")
	}
}

// A revocation failure must not stop the client from clearing its cookies.
func TestLogoutSucceedsEvenWhenRevocationFails(t *testing.T) {
	service := newFakeAuth()
	service.logoutErr = errors.New("connection refused")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "2pick_refresh", Value: "cookie-token"})
	request.Header.Set("X-CSRF-Token", "echoed-csrf")

	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if cookieByName(response, "2pick_refresh") == nil {
		t.Error("the refresh cookie was not cleared")
	}
}

// ---------- configuration ----------

// Without a configured service the endpoints answer 503 and the rest of the API is
// untouched. That is the state before authentication moved off Laravel, so it has to
// keep working.
func TestAuthEndpointsAnswer503WhenUnconfigured(t *testing.T) {
	handler := New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", path, response.Code)
		}
	}
}

// Secure must be off on a local http origin — the browser would drop the cookie
// otherwise — and on for anything else, or the session travels in clear.
func TestCookieSecureFlagFollowsTheOrigin(t *testing.T) {
	cases := map[string]struct {
		host       string
		forwarded  string
		wantSecure bool
	}{
		"localhost http":     {"localhost:8080", "", false},
		"loopback http":      {"127.0.0.1:8080", "", false},
		"public host":        {"api.2pick.app", "", true},
		"behind a tls proxy": {"localhost:8080", "https", true},
	}
	for name, test := range cases {
		service := newFakeAuth()
		request := loginPost(`{"email":"a@b.test","password":"secret"}`)
		request.Host = test.host
		if test.forwarded != "" {
			request.Header.Set("X-Forwarded-Proto", test.forwarded)
		}

		response := httptest.NewRecorder()
		authTestHandler(service).ServeHTTP(response, request)

		cookie := cookieByName(response, "2pick_refresh")
		if cookie == nil {
			t.Fatalf("%s: no refresh cookie", name)
		}
		if cookie.Secure != test.wantSecure {
			t.Errorf("%s: Secure = %v, want %v", name, cookie.Secure, test.wantSecure)
		}
	}
}

func TestLoginRejectsAMalformedBody(t *testing.T) {
	service := newFakeAuth()
	for name, body := range map[string]string{
		"broken json":   `{`,
		"not an object": `"string"`,
	} {
		response := httptest.NewRecorder()
		authTestHandler(service).ServeHTTP(response, loginPost(body))
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, response.Code)
		}
	}
	if service.loginCalls != 0 {
		t.Errorf("the service was called %d times for malformed bodies", service.loginCalls)
	}
}

// The response must not be cacheable: it carries a token and a per-session CSRF
// value, and a shared cache holding either would hand one user's session to another.
func TestGrantResponsesAreNotCacheable(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		loginPost(`{"email":"a@b.test","password":"secret"}`))

	control := response.Header().Get("Cache-Control")
	if !strings.Contains(control, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", control)
	}
}

func TestGrantBodyShape(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	authTestHandler(service).ServeHTTP(response,
		loginPost(`{"email":"a@b.test","password":"secret"}`))

	var envelope struct {
		Data loginResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v (body %s)", err, response.Body.String())
	}
	if envelope.Data.UserID != "42" {
		t.Errorf("user_id = %q, want \"42\"", envelope.Data.UserID)
	}
	if envelope.Data.ExpiresIn != 300 || envelope.Data.TokenType != "Bearer" {
		t.Errorf("token metadata = %+v", envelope.Data)
	}
	if len(envelope.Data.Roles) != 1 || envelope.Data.Roles[0] != "admin" {
		t.Errorf("roles = %v", envelope.Data.Roles)
	}
}

func (service *fakeAuth) Account(_ context.Context, _ int64) (auth.Account, error) {
	if service.accountErr != nil {
		return auth.Account{}, service.accountErr
	}
	return service.account, nil
}

func (service *fakeAuth) ChangeName(_ context.Context, _ int64, name string) (auth.Account, error) {
	service.nameCalls++
	service.lastNewName = name
	if service.accountErr != nil {
		return auth.Account{}, service.accountErr
	}
	changed := service.account
	changed.Name = name
	return changed, nil
}

func (service *fakeAuth) UploadAvatar(
	_ context.Context, _ int64, image []byte, keyName func(string) string,
) (string, error) {
	service.avatarCalls++
	service.lastAvatar = image
	// Called with a fixed extension only to record that the handler passed a key
	// builder through rather than inventing the key itself.
	service.lastAvatarKey = keyName("png")
	if service.avatarErr != nil {
		return "", service.avatarErr
	}
	return service.avatarURL, nil
}

func (service *fakeAuth) ChangePassword(
	_ context.Context, _ int64, currentPassword, newPassword string, client auth.ClientInfo,
) (auth.Grant, error) {
	service.passwordCalls++
	service.lastCurrent, service.lastNew = currentPassword, newPassword
	service.lastClientIP, service.lastUserAgent = client.IP, client.UserAgent
	if service.passwordErr != nil {
		return auth.Grant{}, service.passwordErr
	}
	return service.grant, nil
}

func (service *fakeAuth) SetInitialPassword(
	_ context.Context, _ int64, newPassword string, client auth.ClientInfo,
) (auth.Grant, error) {
	service.initCalls++
	service.lastNew = newPassword
	service.lastClientIP, service.lastUserAgent = client.IP, client.UserAgent
	if service.initErr != nil {
		return auth.Grant{}, service.initErr
	}
	return service.grant, nil
}

// NameChangeAllowedAt mirrors the real rule closely enough for the handler test: a
// stamp means "one day later", a zero stamp means no limit.
func (service *fakeAuth) NameChangeAllowedAt(account auth.Account) time.Time {
	if account.NameChangedAt.IsZero() {
		return time.Time{}
	}
	return account.NameChangedAt.AddDate(0, 0, 1)
}
