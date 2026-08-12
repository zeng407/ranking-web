package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/admin"
	"2pick.app/backend/internal/auth"
)

// assetHarness is a handler serving a small bundle out of a temporary directory, with the
// clock under the test's control so the pass's hour can be crossed without waiting.
type assetHarness struct {
	handler   http.Handler
	directory string
	now       time.Time
}

func newAssetHarness(t *testing.T, roles ...string) *assetHarness {
	t.Helper()
	harness := &assetHarness{
		directory: t.TempDir(),
		now:       time.Unix(1_700_000_000, 0).UTC(),
	}
	harness.write(t, "index.html", "<title>back office</title>")
	harness.write(t, "assets/app.js", "console.log(1)")
	if roles == nil {
		roles = []string{"user", admin.AdminRoleSlug}
	}
	harness.handler = New(Options{
		Environment:   "test",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Admin:         newFakeAdmin(),
		AdminAssetDir: harness.directory,
		AdminAssetKey: AdminAssetKey([]byte("a seed that is not the real one")),
		AuthVerifier:  staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: roles}},
		Now:           func() time.Time { return harness.now },
	})
	return harness
}

func (harness *assetHarness) write(t *testing.T, name, body string) {
	t.Helper()
	full := filepath.Join(harness.directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", full, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", full, err)
	}
}

// grant runs the authenticated grant endpoint and returns the pass it minted.
func (harness *assetHarness) grant(t *testing.T) *http.Cookie {
	t.Helper()
	response := httptest.NewRecorder()
	harness.handler.ServeHTTP(response,
		adminRequest(http.MethodPost, "/api/v1/admin/assets/grant", ""))

	if response.Code != http.StatusNoContent {
		t.Fatalf("grant status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	cookie := adminAssetCookieFrom(response)
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("the grant set no %s cookie: %v", adminAssetCookieName, response.Result().Cookies())
	}
	return cookie
}

func adminAssetCookieFrom(response *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == adminAssetCookieName {
			return cookie
		}
	}
	return nil
}

func (harness *assetHarness) get(path string, pass *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if pass != nil {
		request.AddCookie(pass)
	}
	response := httptest.NewRecorder()
	harness.handler.ServeHTTP(response, request)
	return response
}

// The pass is a cookie because the bundle is loaded by navigations and <script src>, which
// carry no Authorization header. It is httpOnly, scoped to the bundle's own path, and Lax
// so the top-level navigation from the public shell still carries it.
func TestTheGrantSetsAnHTTPOnlyPassScopedToTheBundle(t *testing.T) {
	harness := newAssetHarness(t)

	pass := harness.grant(t)

	if !pass.HttpOnly {
		t.Error("the pass is readable by script")
	}
	if pass.Path != adminAssetPrefix {
		t.Errorf("path = %q, want %q", pass.Path, adminAssetPrefix)
	}
	if pass.SameSite != http.SameSiteLaxMode {
		t.Errorf("same site = %v, want Lax", pass.SameSite)
	}
	if pass.MaxAge != int(adminAssetTTL.Seconds()) {
		t.Errorf("max age = %d, want %d", pass.MaxAge, int(adminAssetTTL.Seconds()))
	}
}

// The whole point of the directory living off the public origin: without a pass the files
// are not readable, and the answer says which of the two problems it is.
func TestTheBundleIsNotReadableWithoutAPass(t *testing.T) {
	harness := newAssetHarness(t)

	for _, path := range []string{"/admin/", "/admin/index.html", "/admin/assets/app.js"} {
		response := harness.get(path, nil)

		if response.Code != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want 403; body = %s", path, response.Code, response.Body.String())
		}
		if code := adminErrorCode(t, response); code != "admin_assets_forbidden" {
			t.Errorf("%s: code = %q, want admin_assets_forbidden", path, code)
		}
		if strings.Contains(response.Body.String(), "<title>") ||
			strings.Contains(response.Body.String(), "console.log") {
			t.Errorf("%s: the refusal leaked the file", path)
		}
	}
}

// A signed-in account without the role never gets a pass, so it never reads the bundle
// either — the gate on the files and the gate on the API are the same role.
func TestAnAccountWithoutTheRoleGetsNoPass(t *testing.T) {
	harness := newAssetHarness(t, "user")
	response := httptest.NewRecorder()

	harness.handler.ServeHTTP(response,
		adminRequest(http.MethodPost, "/api/v1/admin/assets/grant", ""))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
	if cookie := adminAssetCookieFrom(response); cookie != nil {
		t.Errorf("a pass was set anyway: %v", cookie)
	}
}

func TestAValidPassReadsTheBundle(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)

	response := harness.get("/admin/assets/app.js", pass)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() != "console.log(1)" {
		t.Errorf("body = %q, want the file", response.Body.String())
	}
	if control := response.Header().Get("Cache-Control"); !strings.Contains(control, "no-store") {
		t.Errorf("Cache-Control = %q; a shared cache could serve this without a pass", control)
	}
	if frame := response.Header().Get("X-Frame-Options"); frame != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", frame)
	}
}

// The back office's deep links are its own routes, so an unknown path inside the bundle is
// the shell rather than a 404 — but never a directory listing.
func TestUnknownPathsInsideTheBundleFallBackToTheShell(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)

	for _, path := range []string{"/admin/", "/admin/posts/abcdefgh", "/admin/assets/"} {
		response := harness.get(path, pass)

		if response.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", path, response.Code, response.Body.String())
		}
		if response.Body.String() != "<title>back office</title>" {
			t.Errorf("%s: body = %q, want index.html", path, response.Body.String())
		}
	}
}

// The bundle directory is a boundary, not a prefix: a path that climbs out of it must not
// read a file the process happens to be able to open.
func TestARequestCannotClimbOutOfTheBundleDirectory(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)
	secret := filepath.Join(filepath.Dir(harness.directory), "secret.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	for _, path := range []string{
		"/admin/../secret.txt",
		"/admin/..%2Fsecret.txt",
		"/admin/assets/../../secret.txt",
		"/admin/../../../../etc/passwd",
	} {
		response := harness.get(path, pass)

		if strings.Contains(response.Body.String(), "PRIVATE KEY") ||
			strings.Contains(response.Body.String(), "root:") {
			t.Fatalf("%s: served a file outside the bundle: %s", path, response.Body.String())
		}
	}
}

// The signature is what makes the cookie a pass rather than a claim. Editing any part of it
// — the user id, the expiry, the digest — must fail closed.
func TestATamperedPassIsRefused(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)
	parts := strings.Split(pass.Value, ".")
	if len(parts) != 3 {
		t.Fatalf("pass = %q, want three dot-separated parts", pass.Value)
	}

	forged := map[string]string{
		"a later expiry":       parts[0] + "." + "9999999999" + "." + parts[2],
		"another account":      "43." + parts[1] + "." + parts[2],
		"a changed digest":     parts[0] + "." + parts[1] + ".00" + parts[2][2:],
		"no signature at all":  parts[0] + "." + parts[1],
		"an empty user id":     "." + parts[1] + "." + parts[2],
		"a non-numeric expiry": parts[0] + ".soon." + parts[2],
	}
	for name, value := range forged {
		t.Run(name, func(t *testing.T) {
			response := harness.get("/admin/index.html",
				&http.Cookie{Name: adminAssetCookieName, Value: value})

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
			}
		})
	}
}

// A pass signed with another process's key is not a pass here: the key is derived from the
// token signing seed, so rotating that seed invalidates every outstanding pass.
func TestAPassSignedWithAnotherKeyIsRefused(t *testing.T) {
	other := newAssetHarness(t)
	other.handler = New(Options{
		Environment:   "test",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		AdminAssetDir: other.directory,
		AdminAssetKey: AdminAssetKey([]byte("a different seed")),
		AuthVerifier:  staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{admin.AdminRoleSlug}}},
		Now:           func() time.Time { return other.now },
	})
	foreign := other.grant(t)

	harness := newAssetHarness(t)
	response := harness.get("/admin/index.html", foreign)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

func TestAnExpiredPassIsRefused(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)

	harness.now = harness.now.Add(adminAssetTTL + time.Second)
	response := harness.get("/admin/index.html", pass)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

// An open back office tab keeps working past the hour, because a pass that is still valid
// is renewed on the way through. A pass that has expired is not, so the renewal cannot
// keep a revoked moderator inside.
func TestAPassIsRenewedWhileItIsStillValid(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)

	// Before the halfway mark nothing is rewritten: a cookie per request would be noise.
	fresh := harness.get("/admin/index.html", pass)
	if cookie := adminAssetCookieFrom(fresh); cookie != nil {
		t.Errorf("a fresh pass was rewritten: %v", cookie)
	}

	harness.now = harness.now.Add(adminAssetTTL/2 + time.Minute)
	renewed := harness.get("/admin/index.html", pass)
	if renewed.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", renewed.Code, renewed.Body.String())
	}
	cookie := adminAssetCookieFrom(renewed)
	if cookie == nil || cookie.Value == pass.Value {
		t.Fatalf("the pass was not renewed: %v", cookie)
	}

	// The renewed pass carries the original account, not a new one.
	if !strings.HasPrefix(cookie.Value, "42.") {
		t.Errorf("renewed pass = %q, want it to stay account 42", cookie.Value)
	}
	harness.now = harness.now.Add(adminAssetTTL / 2)
	if response := harness.get("/admin/index.html", cookie); response.Code != http.StatusOK {
		t.Fatalf("the renewed pass was refused: %d %s", response.Code, response.Body.String())
	}
}

func TestRevokingClearsThePass(t *testing.T) {
	harness := newAssetHarness(t)
	harness.grant(t)
	response := httptest.NewRecorder()

	harness.handler.ServeHTTP(response,
		adminRequest(http.MethodPost, "/api/v1/admin/assets/revoke", ""))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	cookie := adminAssetCookieFrom(response)
	if cookie == nil {
		t.Fatal("revoke set no cookie, so the browser keeps the pass")
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("cookie = %q with max age %d, want it cleared", cookie.Value, cookie.MaxAge)
	}
	if cookie.Path != adminAssetPrefix {
		t.Errorf("path = %q, want %q so it clears the pass that was set", cookie.Path, adminAssetPrefix)
	}
}

// A deployment with no bundle has no such resource, and hands out no passes for one.
func TestWithoutABundleTheAssetRoutesAre404(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{admin.AdminRoleSlug}}},
	})

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/admin/index.html", nil),
		adminRequest(http.MethodPost, "/api/v1/admin/assets/grant", ""),
		adminRequest(http.MethodPost, "/api/v1/admin/assets/revoke", ""),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404; body = %s",
				request.Method, request.URL.Path, response.Code, response.Body.String())
		}
	}
}

// A bundle mounted without a signing key cannot verify anything, so it refuses everyone
// rather than serving the files unsigned. New() logs the misconfiguration.
func TestABundleWithoutAKeyRefusesEveryone(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("shell"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := New(Options{
		Environment:   "test",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		AdminAssetDir: directory,
		AuthVerifier:  staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{admin.AdminRoleSlug}}},
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/index.html", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() == "shell" {
		t.Fatal("the bundle was served without a key to verify a pass with")
	}

	grant := httptest.NewRecorder()
	handler.ServeHTTP(grant, adminRequest(http.MethodPost, "/api/v1/admin/assets/grant", ""))
	if grant.Code != http.StatusNotFound {
		t.Errorf("grant status = %d, want 404", grant.Code)
	}
}

// The seed is the only input, and it must not be recoverable or reused: the label makes
// this key a different one from any other use of the same seed.
func TestTheAssetKeyIsDerivedFromTheSeedAndDomainSeparated(t *testing.T) {
	seed := []byte("thirty-two bytes of seed material")

	key := AdminAssetKey(seed)
	if len(key) != 32 {
		t.Fatalf("len(key) = %d, want 32", len(key))
	}
	if string(key) == string(seed) {
		t.Error("the key is the seed itself")
	}
	if string(AdminAssetKey(seed)) != string(key) {
		t.Error("the derivation is not stable, so passes stop verifying after a restart")
	}
	if string(AdminAssetKey([]byte("another seed"))) == string(key) {
		t.Error("two seeds derive the same key")
	}
	if AdminAssetKey(nil) != nil {
		t.Error("an empty seed produced a key, which would sign passes with a known value")
	}
}

func TestTheBundleRefusesAWriteMethod(t *testing.T) {
	harness := newAssetHarness(t)
	pass := harness.grant(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/index.html", nil)
	request.AddCookie(pass)
	response := httptest.NewRecorder()

	harness.handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", response.Code, response.Body.String())
	}
	if allow := response.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow = %q, want GET, HEAD", allow)
	}
}
