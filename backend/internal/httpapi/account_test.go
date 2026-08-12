package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2pick.app/backend/internal/auth"
)

// The account settings transport. What the service decides is tested in internal/auth;
// this is about the shape on the wire — which verb reaches which operation, that a
// password change replaces the session cookies, and that none of it is cacheable.

func accountTestHandler(service AuthService) http.Handler {
	return New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:  service,
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{}}},
	})
}

func accountRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func decodeAccount(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %s: %v", response.Body.String(), err)
	}
	return envelope.Data
}

func TestGetAccountProfileServesTheSettingsFields(t *testing.T) {
	service := newFakeAuth()
	service.account = auth.Account{
		Name:         "the holder",
		Email:        "holder@example.test",
		AvatarURL:    "https://file.2pick.app/avatars/a.png",
		HasPassword:  true,
		GoogleLinked: true,
	}
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodGet, "/api/v1/account/profile", ""))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	data := decodeAccount(t, response)
	for field, want := range map[string]any{
		"name":          "the holder",
		"email":         "holder@example.test",
		"avatar_url":    "https://file.2pick.app/avatars/a.png",
		"has_password":  true,
		"google_linked": true,
	} {
		if data[field] != want {
			t.Errorf("%s = %v, want %v", field, data[field], want)
		}
	}
	// No limit in force, so the field is absent rather than null: a client reads its
	// presence as "cannot change the name yet".
	if _, present := data["name_change_allowed_at"]; present {
		t.Errorf("name_change_allowed_at is present for an account that may change its name now")
	}
}

// The address is on this response and nowhere else. Anything that caches it — a shared
// proxy, Cloudflare — would serve one account's address to the next visitor.
func TestTheAccountProfileIsNotCacheable(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodGet, "/api/v1/account/profile", ""))

	if control := response.Header().Get("Cache-Control"); !strings.Contains(control, "no-store") {
		t.Errorf("Cache-Control = %q, want it to forbid storing", control)
	}
}

func TestTheAccountProfileReportsWhenTheNameMayChangeAgain(t *testing.T) {
	service := newFakeAuth()
	service.account = auth.Account{Name: "renamed", NameChangedAt: time.Now()}
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodGet, "/api/v1/account/profile", ""))

	allowedAt, ok := decodeAccount(t, response)["name_change_allowed_at"].(string)
	if !ok {
		t.Fatalf("name_change_allowed_at missing from %s", response.Body.String())
	}
	when, err := time.Parse(time.RFC3339, allowedAt)
	if err != nil {
		t.Fatalf("name_change_allowed_at = %q, which is not RFC 3339: %v", allowedAt, err)
	}
	if !when.After(time.Now()) {
		t.Errorf("name_change_allowed_at = %v, want a moment in the future", when)
	}
}

func TestPutAccountProfileRenames(t *testing.T) {
	service := newFakeAuth()
	service.account = auth.Account{Name: "before", Email: "holder@example.test"}
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		accountRequest(http.MethodPut, "/api/v1/account/profile", `{"name":"after"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.nameCalls != 1 {
		t.Fatalf("ChangeName was called %d times, want 1", service.nameCalls)
	}
	if service.lastNewName != "after" {
		t.Errorf("name = %q, want %q", service.lastNewName, "after")
	}
	// The response is the account as it now stands, so the form does not have to read
	// it back to redraw.
	if data := decodeAccount(t, response); data["name"] != "after" {
		t.Errorf("name in the response = %v, want %q", data["name"], "after")
	}
}

// The rate limit and the length rules arrive as per-field codes, the same shape
// registration uses, so one renderer covers every form.
func TestARefusedRenameIs422WithAFieldCode(t *testing.T) {
	service := newFakeAuth()
	service.accountErr = &auth.ErrAccountInvalid{
		Fields: auth.FieldErrors{"name": []string{auth.CodeNameChangeTooSoon}},
	}
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		accountRequest(http.MethodPut, "/api/v1/account/profile", `{"name":"after"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			Errors map[string][]string `json:"errors"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := envelope.Data.Errors["name"]; len(got) != 1 || got[0] != auth.CodeNameChangeTooSoon {
		t.Errorf("errors.name = %v, want [%s]", got, auth.CodeNameChangeTooSoon)
	}
	if envelope.Error.Code != "validation_failed" {
		t.Errorf("error.code = %q, want %q", envelope.Error.Code, "validation_failed")
	}
}

func TestPutAccountPasswordChangesItAndReplacesTheSession(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodPut,
		"/api/v1/account/password",
		`{"current_password":"the-old-password","new_password":"the-new-password"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.passwordCalls != 1 {
		t.Fatalf("ChangePassword was called %d times, want 1", service.passwordCalls)
	}
	if service.lastCurrent != "the-old-password" || service.lastNew != "the-new-password" {
		t.Errorf("passwords = %q then %q", service.lastCurrent, service.lastNew)
	}

	// THE COOKIES HAVE TO BE REPLACED. The change ends every session the account holds,
	// the caller's included, so a response that did not carry a new refresh cookie would
	// sign the user out of the page they just used.
	if cookie := cookieByName(response, "2pick_refresh"); cookie == nil {
		t.Error("no refresh cookie on the response")
	} else if cookie.Value != service.grant.Refresh.Token {
		t.Errorf("refresh cookie = %q, want the newly issued token", cookie.Value)
	}
	if cookie := cookieByName(response, "2pick_csrf"); cookie == nil {
		t.Error("no CSRF cookie on the response")
	}
}

func TestPostAccountPasswordSetsTheFirstOne(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodPost,
		"/api/v1/account/password", `{"new_password":"the-new-password"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.initCalls != 1 || service.passwordCalls != 0 {
		t.Errorf("init calls = %d, change calls = %d; POST must reach the initial-password path",
			service.initCalls, service.passwordCalls)
	}
	if service.lastNew != "the-new-password" {
		t.Errorf("new password = %q", service.lastNew)
	}
}

// The two operations differ in what they prove, so reaching the wrong one is a security
// difference and not a routing detail: POST does not require the current password.
func TestTheTwoPasswordVerbsDoNotOverlap(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response, accountRequest(http.MethodPut,
		"/api/v1/account/password",
		`{"current_password":"the-old-password","new_password":"the-new-password"}`))

	if service.initCalls != 0 {
		t.Errorf("PUT reached SetInitialPassword, which does not ask for the current password")
	}
}

func TestUnsupportedMethodsOnTheAccountEndpointsAnswer405WithAllow(t *testing.T) {
	cases := []struct {
		method    string
		path      string
		wantAllow string
	}{
		{http.MethodDelete, "/api/v1/account/profile", "GET, HEAD, PUT"},
		{http.MethodPost, "/api/v1/account/profile", "GET, HEAD, PUT"},
		{http.MethodGet, "/api/v1/account/password", "POST, PUT"},
		{http.MethodDelete, "/api/v1/account/password", "POST, PUT"},
	}
	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			accountTestHandler(newFakeAuth()).ServeHTTP(response,
				accountRequest(testCase.method, testCase.path, ""))

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", response.Code)
			}
			if allow := response.Header().Get("Allow"); allow != testCase.wantAllow {
				t.Errorf("Allow = %q, want %q", allow, testCase.wantAllow)
			}
		})
	}
}

// Every one of these writes to a row chosen by the token's subject, so no token means no
// row to write.
func TestTheAccountEndpointsRequireABearerToken(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:  newFakeAuth(),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42"}},
	})
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/account/profile"},
		{http.MethodPut, "/api/v1/account/profile"},
		{http.MethodPost, "/api/v1/account/avatar"},
		{http.MethodPut, "/api/v1/account/password"},
		{http.MethodPost, "/api/v1/account/password"},
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

// A token that verifies but names something other than a user id must not be coerced to
// a row: every one of these endpoints writes to whatever account that number resolves to.
func TestATokenWhoseSubjectIsNotAUserIDIsRefused(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:  newFakeAuth(),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "service-account"}},
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, accountRequest(http.MethodPut,
		"/api/v1/account/profile", `{"name":"after"}`))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", response.Code, response.Body.String())
	}
}

func avatarUpload(t *testing.T, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create the form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write the form file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close the form: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/account/avatar", body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func TestPostAccountAvatarStoresTheFileAndAnswersWithItsURL(t *testing.T) {
	service := newFakeAuth()
	service.avatarURL = "https://file.2pick.app/avatars/new.png"
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		avatarUpload(t, "avatar", "me.png", []byte("\x89PNG\r\n\x1a\nbody")))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.avatarCalls != 1 {
		t.Fatalf("UploadAvatar was called %d times, want 1", service.avatarCalls)
	}
	if string(service.lastAvatar) != "\x89PNG\r\n\x1a\nbody" {
		t.Errorf("the bytes reaching the service were %q", service.lastAvatar)
	}
	// The key comes from the layout the bucket already uses, and the handler passes the
	// builder through rather than naming the object itself.
	if !strings.HasPrefix(service.lastAvatarKey, "avatars/") ||
		!strings.HasSuffix(service.lastAvatarKey, ".png") {
		t.Errorf("key = %q, want avatars/{name}.png", service.lastAvatarKey)
	}
	if data := decodeAccount(t, response); data["avatar_url"] != service.avatarURL {
		t.Errorf("avatar_url = %v, want %q", data["avatar_url"], service.avatarURL)
	}
}

func TestPostAccountAvatarWithoutAFilePartIs400(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		avatarUpload(t, "not-avatar", "me.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
	if service.avatarCalls != 0 {
		t.Error("a request with no avatar part reached the service")
	}
}

// The size limit is enforced twice: on the declared part size and on the bytes actually
// read. A part that lies about its size still has to be caught, which is what the
// LimitReader's extra byte is for.
func TestPostAccountAvatarRefusesAnOversizedFile(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()
	oversized := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, auth.MaxAvatarBytes)...)

	accountTestHandler(service).ServeHTTP(response, avatarUpload(t, "avatar", "big.png", oversized))

	// 422 specifically: the part declares its size, so it is refused as a field error
	// before a single byte is read, not as a malformed request.
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if service.avatarCalls != 0 {
		t.Errorf("an oversized upload reached the service, which would store %d bytes",
			len(service.lastAvatar))
	}
}

// A refused image comes back as a field error, so the settings form can put the message
// next to the file input.
func TestARefusedAvatarIs422WithAFieldCode(t *testing.T) {
	service := newFakeAuth()
	service.avatarErr = &auth.ErrAccountInvalid{
		Fields: auth.FieldErrors{"avatar": []string{auth.CodeUnsupportedImage}},
	}
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		avatarUpload(t, "avatar", "shell.php", []byte("<?php ?>")))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
}

// An api started without the object-store variables cannot accept an avatar. That is a
// configuration answer, not a fault: 500 would have a client retrying forever.
func TestAnUnconfiguredOperationIs503(t *testing.T) {
	service := newFakeAuth()
	service.avatarErr = fmt.Errorf("%w: avatar uploads", auth.ErrNotConfigured)
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		avatarUpload(t, "avatar", "me.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
}

// A deleted account with a token still in flight: the token verified, so what is wrong
// is the session, not the request.
func TestAMissingAccountIs401(t *testing.T) {
	service := newFakeAuth()
	service.accountErr = auth.ErrUserNotFound
	response := httptest.NewRecorder()

	accountTestHandler(service).ServeHTTP(response,
		accountRequest(http.MethodGet, "/api/v1/account/profile", ""))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", response.Code, response.Body.String())
	}
}
