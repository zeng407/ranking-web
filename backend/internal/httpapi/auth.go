package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"2pick.app/backend/internal/auth"
)

// AuthService is the slice of auth.Service this layer uses.
//
// An interface rather than the concrete type so the transport behaviour — cookie
// attributes, CSRF enforcement, the deliberately uniform error responses — can be
// tested without a database behind it. The four methods are the whole surface.
type AuthService interface {
	Login(ctx context.Context, email, password string, client auth.ClientInfo) (auth.Grant, error)
	Register(ctx context.Context, registration auth.Registration, client auth.ClientInfo) (auth.Grant, error)
	Refresh(ctx context.Context, refreshToken, csrfToken string, client auth.ClientInfo) (auth.Grant, error)
	Logout(ctx context.Context, refreshToken string) error
	VerifyCSRF(ctx context.Context, refreshToken, csrfToken string) error
	// Forgot password and reset. RequestPasswordReset returns no session on purpose:
	// asking for a mail proves nothing about who is asking.
	RequestPasswordReset(ctx context.Context, email, locale string, client auth.ClientInfo) error
	ResetPassword(ctx context.Context, token, newPassword string,
		client auth.ClientInfo) (auth.Grant, error)
	// The account settings, from Profile\ProfileController. Part of this interface
	// rather than a second one because two of them re-issue a session, which only the
	// thing that issues sessions can do.
	Account(ctx context.Context, userID int64) (auth.Account, error)
	ChangeName(ctx context.Context, userID int64, name string) (auth.Account, error)
	UploadAvatar(ctx context.Context, userID int64, image []byte,
		keyName func(extension string) string) (string, error)
	ChangePassword(ctx context.Context, userID int64,
		currentPassword, newPassword string, client auth.ClientInfo) (auth.Grant, error)
	SetInitialPassword(ctx context.Context, userID int64,
		newPassword string, client auth.ClientInfo) (auth.Grant, error)
	NameChangeAllowedAt(account auth.Account) time.Time
}

// Cookie and header names for the session.
const (
	// refreshCookieName holds the opaque refresh token. httpOnly, so script running
	// in the page — including injected script — cannot read it.
	refreshCookieName = "2pick_refresh"
	// csrfCookieName holds the value the client must echo in csrfHeaderName. It is
	// deliberately readable by script: that asymmetry is the whole defence. A
	// cross-site request carries the refresh cookie automatically but cannot read
	// this one to copy it into a header.
	csrfCookieName = "2pick_csrf"
	csrfHeaderName = "X-CSRF-Token"
	// refreshCookiePath scopes the refresh cookie to the endpoints that use it, so it
	// is not attached to every API call it has no business on.
	refreshCookiePath = "/api/v1/auth"
)

// maxAuthRequestBytes bounds a login body. Credentials are small; anything larger is
// someone probing.
const maxAuthRequestBytes = 4 << 10

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse deliberately carries the access token in the body and the refresh
// token only in a cookie. The access token is short-lived and has to reach the
// Authorization header, which means script must be able to read it; the refresh
// token has to survive a page reload, which means it must not be.
type loginResponse struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	CSRFToken   string   `json:"csrf_token"`
	UserID      string   `json:"user_id"`
	Roles       []string `json:"roles"`
}

func (a *api) requireAuthService(w http.ResponseWriter, r *http.Request) bool {
	if a.authService == nil {
		writeError(w, r, http.StatusServiceUnavailable, "auth_unavailable",
			"authentication is not configured on this server")
		return false
	}
	return true
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	var request loginRequest
	// Credentials are small; anything larger is someone probing.
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	grant, err := a.authService.Login(r.Context(), request.Email, request.Password, a.clientInfo(r))
	if err != nil {
		a.writeAuthError(w, r, err)
		return
	}

	a.writeGrant(w, r, grant)
}

type registerRequest struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

// register creates a password account and signs it in.
//
// Laravel's RegisterController needed no mail: the User model does not implement
// MustVerifyEmail, so registration was only ever a validator plus an insert plus a
// login. That is why this could move while password reset could not.
func (a *api) register(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	var request registerRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	grant, err := a.authService.Register(r.Context(), auth.Registration{
		Name:                 request.Name,
		Email:                request.Email,
		Password:             request.Password,
		PasswordConfirmation: request.PasswordConfirmation,
	}, a.clientInfo(r))

	var invalid *auth.ErrRegistrationInvalid
	if errors.As(err, &invalid) {
		writeFieldErrors(w, r, invalid.Fields)
		return
	}
	if err != nil {
		a.writeAuthError(w, r, err)
		return
	}

	a.writeGrant(w, r, grant)
}

// writeFieldErrors answers 422 with per-field reasons.
//
// The reasons are in `data` while the envelope's `error` carries the summary, because
// the form has to attach each message to its own input — a single message string cannot
// say which field it belongs to. The values are machine codes, not sentences: the SPA
// translates into three languages and this process has no catalogue.
// The parameter is the underlying map type rather than auth.FieldErrors, so the editor's
// own named type — authoring.FieldErrors, the same map underneath — passes without a
// conversion and without a second copy of this function.
func writeFieldErrors(w http.ResponseWriter, r *http.Request, fields map[string][]string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, http.StatusUnprocessableEntity, envelope{
		Data:  map[string]any{"errors": fields},
		Error: &apiErr{Code: "validation_failed", Message: "the submitted values are not valid"},
	})
}

func (a *api) refreshSession(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		// No cookie at all is simply "not logged in", not an error worth detail.
		a.clearSessionCookies(w, r)
		writeError(w, r, http.StatusUnauthorized, "not_authenticated", "no session")
		return
	}

	grant, err := a.authService.Refresh(r.Context(), cookie.Value, r.Header.Get(csrfHeaderName), a.clientInfo(r))
	if err != nil {
		// Any refresh failure ends the session on the client too. Leaving a cookie
		// that the server will keep rejecting produces a client stuck in a retry loop.
		a.clearSessionCookies(w, r)
		a.writeAuthError(w, r, err)
		return
	}

	a.writeGrant(w, r, grant)
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	cookie, err := r.Cookie(refreshCookieName)
	if err == nil && cookie.Value != "" {
		// CSRF still applies: a forged logout is a denial of service, minor but free
		// to prevent.
		if csrfErr := a.verifyLogoutCSRF(r, cookie.Value); csrfErr != nil {
			a.writeAuthError(w, r, csrfErr)
			return
		}
		if err := a.authService.Logout(r.Context(), cookie.Value); err != nil {
			a.logger.Error("logout_failed", "error", err)
		}
	}

	// Cleared unconditionally. Logging out has to work even when the token is already
	// unknown to the server, or a client can never escape a broken session.
	a.clearSessionCookies(w, r)
	writeJSON(w, r, http.StatusNoContent, envelope{})
}

// verifyLogoutCSRF checks the echoed token against the session the cookie names.
func (a *api) verifyLogoutCSRF(r *http.Request, refreshToken string) error {
	presented := r.Header.Get(csrfHeaderName)
	if presented == "" {
		return auth.ErrCSRFMismatch
	}
	// The service exposes no read-only lookup, so the comparison happens there: a
	// mismatched token fails the same way a mismatched refresh does.
	return a.authService.VerifyCSRF(r.Context(), refreshToken, presented)
}

func (a *api) writeGrant(w http.ResponseWriter, r *http.Request, grant auth.Grant) {
	a.setSessionCookies(w, r, grant)

	roles := grant.Roles
	if roles == nil {
		roles = []string{}
	}
	writePrivateJSON(w, r, http.StatusOK, loginResponse{
		AccessToken: grant.Access.Token,
		TokenType:   grant.Access.TokenType,
		ExpiresIn:   grant.Access.ExpiresIn,
		CSRFToken:   grant.Refresh.CSRFToken,
		UserID:      strconv.FormatInt(grant.UserID, 10),
		Roles:       roles,
	})
}

// setSessionCookies puts the session on the response without writing a body.
//
// Split out from writeGrant for the OAuth callback, which ends in a redirect: the
// cookies are the whole payload there, because a token in a redirect URL would end up
// in the browser's history and in the next request's Referer.
func (a *api) setSessionCookies(w http.ResponseWriter, r *http.Request, grant auth.Grant) {
	secure := a.cookiesAreSecure(r)

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    grant.Refresh.Token,
		Path:     refreshCookiePath,
		Expires:  grant.Refresh.ExpiresAt,
		MaxAge:   int(time.Until(grant.Refresh.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		// Strict rather than Lax. The refresh endpoint is only ever called by the
		// SPA's own script, never by a top-level navigation, so Strict costs nothing
		// and removes the cases Lax still allows.
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:  csrfCookieName,
		Value: grant.Refresh.CSRFToken,
		// Scoped to the whole site: the SPA reads it from any page to build the
		// header.
		Path:    "/",
		Expires: grant.Refresh.ExpiresAt,
		MaxAge:  int(time.Until(grant.Refresh.ExpiresAt).Seconds()),
		// NOT httpOnly, by design. See csrfCookieName.
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *api) clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	secure := a.cookiesAreSecure(r)
	for _, cookie := range []*http.Cookie{
		{Name: refreshCookieName, Path: refreshCookiePath, HttpOnly: true},
		{Name: csrfCookieName, Path: "/", HttpOnly: false},
	} {
		cookie.Value = ""
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(0, 0)
		cookie.Secure = secure
		cookie.SameSite = http.SameSiteStrictMode
		http.SetCookie(w, cookie)
	}
}

// cookiesAreSecure decides the Secure attribute.
//
// Set whenever the request did not arrive over plain http on localhost. Marking a
// cookie Secure on a local http origin would make the browser drop it and break
// development, but defaulting to insecure in production would put the session on the
// wire in clear.
func (a *api) cookiesAreSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	return !(host == "localhost" || host == "127.0.0.1" || host == "::1")
}

// clientInfo is what the audit columns and the reset limiter see. It reads the
// forwarded address rather than RemoteAddr: behind the frontend's nginx every
// request arrives from the same container, so a session row recording RemoteAddr
// records the proxy, and a per-source limit keyed on it limits the whole site as
// one source. See clientIP for why the value can never authorise anything.
func (a *api) clientInfo(r *http.Request) auth.ClientInfo {
	return auth.ClientInfo{IP: clientIP(r), UserAgent: r.Header.Get("User-Agent")}
}

// writeAuthError maps every failure to a deliberately vague response.
//
// Login, refresh and CSRF failures all answer 401 with the same code. Telling the
// caller which one it was is exactly the information an attacker probing tokens or
// addresses is looking for; the detail goes to the log instead.
func (a *api) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "the credentials are not valid")
	case errors.Is(err, auth.ErrRefreshTokenReused):
		// Logged loudly by the service; the client learns nothing extra.
		writeError(w, r, http.StatusUnauthorized, "session_expired", "the session is no longer valid")
	case errors.Is(err, auth.ErrRefreshTokenInvalid), errors.Is(err, auth.ErrCSRFMismatch):
		writeError(w, r, http.StatusUnauthorized, "session_expired", "the session is no longer valid")
	default:
		a.logger.Error("auth_request_failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "the request could not be completed")
	}
}
