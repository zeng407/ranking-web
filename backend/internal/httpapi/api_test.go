package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/auth"
)

type staticTokenVerifier struct {
	identity auth.Identity
	err      error
}

func (v staticTokenVerifier) Verify(string) (auth.Identity, error) {
	return v.identity, v.err
}

func testHandler(ready bool) http.Handler {
	return New(Options{
		ServiceName:    "ranking-api",
		Version:        "test-version",
		Commit:         "test-commit",
		Environment:    "test",
		AllowedOrigins: []string{"https://2pick.app"},
		Ready:          func() bool { return ready },
		Now:            func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) },
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func TestSystemInfoContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	request.Header.Set(requestIDHeader, "request-123")
	response := httptest.NewRecorder()

	testHandler(true).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "public, max-age=0" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") == "" {
		t.Fatal("missing Cloudflare-CDN-Cache-Control")
	}
	if response.Header().Get(requestIDHeader) != "request-123" {
		t.Fatalf("X-Request-ID = %q", response.Header().Get(requestIDHeader))
	}

	var payload struct {
		Data map[string]string `json:"data"`
		Meta meta              `json:"meta"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data["service"] != "ranking-api" || payload.Data["version"] != "test-version" {
		t.Fatalf("unexpected data: %#v", payload.Data)
	}
	if payload.Meta.RequestID != "request-123" {
		t.Fatalf("request id = %q", payload.Meta.RequestID)
	}
}

func TestReadinessFailureIsPrivate(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	testHandler(false).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestCORSAllowsConfiguredCredentialOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/system/info", nil)
	request.Header.Set("Origin", "https://2pick.app")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	testHandler(true).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://2pick.app" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentialed CORS was not enabled")
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatal("Authorization was not allowed by CORS")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	request.Host = "api.internal"
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()

	testHandler(true).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unknown origin must not receive CORS headers")
	}
}

func TestMethodAndNotFoundResponsesUseJSON(t *testing.T) {
	for _, testCase := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodPost, path: "/health/live", status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/missing", status: http.StatusNotFound},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		response := httptest.NewRecorder()
		testHandler(true).ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Fatalf("%s %s status = %d", testCase.method, testCase.path, response.Code)
		}
		if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
		}
	}
}

func TestAuthenticatedIdentityEndpoint(t *testing.T) {
	expiresAt := time.Date(2026, 8, 1, 0, 5, 0, 0, time.UTC)
	handler := New(Options{
		Environment:    "test",
		AllowedOrigins: []string{"https://2pick.app"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{
			Subject: "42", Roles: []string{"admin"}, ExpiresAt: expiresAt,
		}},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data["subject"] != "42" {
		t.Fatalf("data = %#v", payload.Data)
	}
}

func TestAuthenticatedIdentityEndpointRejectsMissingToken(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", response.Header().Get("WWW-Authenticate"))
	}
}

func TestAuthenticatedIdentityEndpointIsUnavailableWithoutBridgeKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer ignored")
	response := httptest.NewRecorder()

	testHandler(true).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

// Every deployment puts a proxy in front of this api, so RemoteAddr is the proxy and
// carries nothing about the visitor. A per-source limit reading it would count the
// whole site as one source; an audit column reading it would record the proxy on
// every row.
func TestClientIPReadsTheForwardedAddressRatherThanTheProxy(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "172.18.0.9:41234" // the frontend container
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 172.18.0.9")

	if got := clientIP(request); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, want the left-most forwarded address", got)
	}
}

func TestClientIPFallsBackToThePeerWithoutAForwardedHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "203.0.113.7:41234"

	if got := clientIP(request); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q", got)
	}
}

// The header is arbitrary client-supplied text and the columns it lands in are
// VARCHAR(45), so anything that is not an address has to be dropped rather than
// stored as a fragment — and a spoofed header must not hide the peer either.
func TestClientIPRejectsAForwardedValueThatIsNotAnAddress(t *testing.T) {
	for _, forwarded := range []string{
		"not-an-address",
		"", // header present but empty
		strings.Repeat("x", 200),
		"<script>alert(1)</script>",
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "203.0.113.7:41234"
		request.Header.Set("X-Forwarded-For", forwarded)

		if got := clientIP(request); got != "203.0.113.7" {
			t.Fatalf("clientIP with %q = %q, want the peer", forwarded, got)
		}
	}
}

func TestClientIPReportsNothingWhenThereIsNoAddressAtAll(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "@" // a unix socket peer, as in a test or a local pipe

	// Empty rather than "@": the limiter skips an unknown source and the audit column
	// stays NULL, which is honest. Storing "@" would key a rate limit on a string every
	// such request shares.
	if got := clientIP(request); got != "" {
		t.Fatalf("clientIP = %q, want empty", got)
	}
}
