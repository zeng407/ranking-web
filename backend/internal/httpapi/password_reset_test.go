package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"2pick.app/backend/internal/auth"
)

// The transport half of forgot / reset. The rules — who gets a mail, what a spent link
// answers — are in internal/auth; what matters here is status codes, cookies and the fact
// that a successful request says nothing about the address.

func passwordPost(path, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// payload is the response without meta, which holds the per-request id.
func payload(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	delete(body, "meta")
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(encoded)
}

func fieldErrors(t *testing.T, response *httptest.ResponseRecorder) map[string][]string {
	t.Helper()
	var body struct {
		Data struct {
			Errors map[string][]string `json:"errors"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "validation_failed" {
		t.Errorf("error.code = %q, want validation_failed", body.Error.Code)
	}
	return body.Data.Errors
}

func TestForgotPasswordPassesTheAddressAndLocaleThrough(t *testing.T) {
	service := newFakeAuth()
	request := passwordPost("/api/v1/auth/password/forgot",
		`{"email":" Player@Example.test ","locale":"ja"}`)
	request.RemoteAddr = "203.0.113.7:54321"
	response := httptest.NewRecorder()

	authTestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.forgotCalls != 1 {
		t.Fatalf("RequestPasswordReset was called %d times, want 1", service.forgotCalls)
	}
	// Trimming and case folding belong to the service, as with login.
	if service.lastEmail != " Player@Example.test " {
		t.Errorf("email = %q; the handler should pass it through unchanged", service.lastEmail)
	}
	if service.lastLocale != "ja" {
		t.Errorf("locale = %q, want ja", service.lastLocale)
	}
	// The per-source cap is keyed on this, so a handler that dropped it would disable it.
	if service.lastClientIP != "203.0.113.7" {
		t.Errorf("client ip = %q, want 203.0.113.7", service.lastClientIP)
	}
	if !strings.Contains(response.Body.String(), `"status":"sent"`) {
		t.Errorf("body = %s", response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	// Asking for a reset is not a login: no session may come out of it.
	if cookieByName(response, "2pick_refresh") != nil {
		t.Error("a refresh cookie was set by a forgot-password request")
	}
}

// The answer for an address with no account is byte-for-byte the answer for one with an
// account — the service reports success for both. This asserts the handler adds nothing
// that could tell them apart.
func TestForgotPasswordAnswers200WhateverTheServiceFound(t *testing.T) {
	registered := httptest.NewRecorder()
	authTestHandler(newFakeAuth()).ServeHTTP(registered,
		passwordPost("/api/v1/auth/password/forgot", `{"email":"player@example.test"}`))

	unknown := httptest.NewRecorder()
	authTestHandler(newFakeAuth()).ServeHTTP(unknown,
		passwordPost("/api/v1/auth/password/forgot", `{"email":"nobody@example.test"}`))

	if registered.Code != unknown.Code {
		t.Errorf("statuses differ: %d and %d", registered.Code, unknown.Code)
	}
	// Everything but meta.request_id, which is per-request by design and carries nothing
	// about the address.
	if got, want := payload(t, unknown), payload(t, registered); got != want {
		t.Errorf("bodies differ:\n%s\n%s", got, want)
	}
}

func TestForgotPasswordReportsAMalformedAddressAs422(t *testing.T) {
	service := newFakeAuth()
	service.forgotErr = &auth.ErrAccountInvalid{
		Fields: auth.FieldErrors{"email": {auth.CodeInvalidEmail}},
	}
	response := httptest.NewRecorder()

	authTestHandler(service).ServeHTTP(response,
		passwordPost("/api/v1/auth/password/forgot", `{"email":"not-an-address"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if got := fieldErrors(t, response)["email"]; len(got) != 1 || got[0] != auth.CodeInvalidEmail {
		t.Errorf("errors.email = %v, want [%s]", got, auth.CodeInvalidEmail)
	}
}

// A server with no mail transport configured cannot ever satisfy this request, so 503 —
// a 500 would have the client retrying something that will never work.
func TestForgotPasswordAnswers503WhenMailIsNotConfigured(t *testing.T) {
	service := newFakeAuth()
	service.forgotErr = fmt.Errorf("%w: password reset", auth.ErrNotConfigured)
	response := httptest.NewRecorder()

	authTestHandler(service).ServeHTTP(response,
		passwordPost("/api/v1/auth/password/forgot", `{"email":"player@example.test"}`))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "account_not_configured") {
		t.Errorf("body = %s", response.Body.String())
	}
}

// A finished reset is a login: the same cookies come out, or the user lands on a page that
// still thinks they are a guest.
func TestResetPasswordSignsTheAccountIn(t *testing.T) {
	service := newFakeAuth()
	response := httptest.NewRecorder()

	authTestHandler(service).ServeHTTP(response, passwordPost("/api/v1/auth/password/reset",
		`{"token":"the-mailed-token","new_password":"a-brand-new-password"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.resetCalls != 1 {
		t.Fatalf("ResetPassword was called %d times, want 1", service.resetCalls)
	}
	if service.lastResetToken != "the-mailed-token" || service.lastNew != "a-brand-new-password" {
		t.Errorf("token = %q, password = %q", service.lastResetToken, service.lastNew)
	}

	refresh := cookieByName(response, "2pick_refresh")
	if refresh == nil {
		t.Fatal("no refresh cookie on the response")
	}
	if !refresh.HttpOnly {
		t.Error("the refresh cookie must be httpOnly")
	}
	if refresh.Value != service.grant.Refresh.Token {
		t.Errorf("refresh cookie = %q, want the newly issued token", refresh.Value)
	}
	if cookieByName(response, "2pick_csrf") == nil {
		t.Error("no csrf cookie on the response")
	}
	if strings.Contains(response.Body.String(), service.grant.Refresh.Token) {
		t.Errorf("the response body contains the refresh token: %s", response.Body.String())
	}
}

func TestResetPasswordReportsASpentLinkOnTheTokenField(t *testing.T) {
	service := newFakeAuth()
	service.resetErr = &auth.ErrAccountInvalid{
		Fields: auth.FieldErrors{"token": {auth.CodeInvalid}},
	}
	response := httptest.NewRecorder()

	authTestHandler(service).ServeHTTP(response, passwordPost("/api/v1/auth/password/reset",
		`{"token":"a-spent-token","new_password":"a-brand-new-password"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if got := fieldErrors(t, response)["token"]; len(got) != 1 || got[0] != auth.CodeInvalid {
		t.Errorf("errors.token = %v, want [%s]", got, auth.CodeInvalid)
	}
	if cookieByName(response, "2pick_refresh") != nil {
		t.Error("a refresh cookie was set by a failed reset")
	}
}

// Both endpoints change state, so a link or a prefetch must not be able to reach them.
func TestPasswordResetEndpointsRefuseGET(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/password/forgot",
		"/api/v1/auth/password/reset",
	} {
		service := newFakeAuth()
		response := httptest.NewRecorder()
		authTestHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))

		if response.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want 405", path, response.Code)
		}
		if service.forgotCalls != 0 || service.resetCalls != 0 {
			t.Errorf("GET %s reached the service", path)
		}
	}
}
