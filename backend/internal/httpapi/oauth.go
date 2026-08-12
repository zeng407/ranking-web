package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"2pick.app/backend/internal/auth"
)

// OAuthService is the slice of auth.OAuthService this layer uses.
type OAuthService interface {
	Start(ctx context.Context, returnTo string, connectUserID int64) (auth.StartedFlow, error)
	Complete(ctx context.Context, state, code string, client auth.ClientInfo) (auth.CompletedFlow, error)
}

// oauthResultQuery names the parameters the SPA reads off the return URL.
//
// Only an outcome and, on failure, a reason. NO TOKENS: a URL ends up in the browser's
// history, in the Referer header of the next request, and in any logging proxy in
// between. The session arrives as cookies on this same response, and the SPA turns
// them into an access token by calling /api/v1/auth/refresh.
const (
	oauthResultQuery = "auth"
	oauthReasonQuery = "reason"
)

func (a *api) requireOAuthService(w http.ResponseWriter, r *http.Request) bool {
	if a.oauthService == nil {
		writeError(w, r, http.StatusServiceUnavailable, "oauth_unavailable",
			"this server has no identity provider configured")
		return false
	}
	return true
}

// startGoogleOAuth sends the browser to Google.
//
// A 302 rather than JSON with a URL. The SPA reaches this by setting
// window.location, so a redirect is one hop instead of two, and it keeps the flow
// working without script — which matters because the callback cannot use script at
// all.
func (a *api) startGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if !a.requireOAuthService(w, r) {
		return
	}

	flow, err := a.oauthService.Start(r.Context(), r.URL.Query().Get("return_to"), 0)
	if err != nil {
		a.logger.Error("oauth_start_failed", "provider", "google", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error",
			"the sign-in could not be started")
		return
	}

	// No-store on the redirect itself: a cached 302 would send a later visitor to a
	// stale state that has since been consumed.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, flow.AuthorizationURL, http.StatusFound)
}

// connectGoogleAuthorizationURL is the connect flow's response body.
type connectGoogleAuthorizationURL struct {
	AuthorizationURL string `json:"authorization_url"`
}

// connectGoogleOAuth starts linking a provider account to the caller's existing one.
//
// THIS ONE ANSWERS WITH JSON WHILE start REDIRECTS, and the asymmetry is forced rather
// than a matter of taste. Linking has to know who is asking, which means an
// Authorization header — and a browser cannot put a header on a top-level navigation.
// So the SPA calls this with fetch, reads the URL out of the body, and navigates
// itself. The user id is remembered in the flow state, because the callback that
// follows arrives as a cross-site navigation from Google with no credentials at all.
func (a *api) connectGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if !a.requireOAuthService(w, r) {
		return
	}

	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "not_authenticated", "sign in first")
		return
	}
	userID, err := auth.SubjectToUserID(identity.Subject)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "not_authenticated", "sign in first")
		return
	}

	flow, err := a.oauthService.Start(r.Context(), r.URL.Query().Get("return_to"), userID)
	if err != nil {
		a.logger.Error("oauth_connect_start_failed", "provider", "google", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error",
			"the link could not be started")
		return
	}

	writePrivateJSON(w, r, http.StatusOK, connectGoogleAuthorizationURL{
		AuthorizationURL: flow.AuthorizationURL,
	})
}

// googleOAuthCallback is where Google sends the browser back.
//
// EVERY OUTCOME IS A REDIRECT, including the failures. This endpoint is reached by a
// top-level navigation, so the person on the other side is looking at a page, not at a
// JSON body; answering 401 here would show them a raw error document. The result is
// carried in a query parameter for the SPA to read and turn into a message.
func (a *api) googleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !a.requireOAuthService(w, r) {
		return
	}

	query := r.URL.Query()

	// Google's own refusal — the consent screen was dismissed, most often. Nothing
	// was exchanged, so there is nothing to clean up.
	if providerError := query.Get("error"); providerError != "" {
		a.logger.Info("oauth_declined", "provider", "google", "error", providerError)
		a.redirectOAuthFailure(w, r, "declined")
		return
	}

	completed, err := a.oauthService.Complete(r.Context(),
		query.Get("state"), query.Get("code"), a.clientInfo(r))
	if err != nil {
		a.redirectOAuthFailure(w, r, oauthFailureReason(err))
		if !isExpectedOAuthFailure(err) {
			a.logger.Error("oauth_callback_failed", "provider", "google", "error", err)
		}
		return
	}

	// A link does not change who is signed in, so it must not issue a session — the
	// caller already had one when the flow started.
	if completed.Linked {
		a.redirectOAuthSuccess(w, r, completed.ReturnTo, "linked")
		return
	}

	// The cookies land on this response; the SPA calls /api/v1/auth/refresh to turn
	// them into an access token. See oauthResultQuery for why the token is not in the
	// URL.
	a.setSessionCookies(w, r, completed.Grant)
	result := "signed-in"
	if completed.Created {
		result = "registered"
	}
	a.redirectOAuthSuccess(w, r, completed.ReturnTo, result)
}

func (a *api) redirectOAuthSuccess(w http.ResponseWriter, r *http.Request, returnTo, result string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, appendQuery(returnTo, map[string]string{oauthResultQuery: result}),
		http.StatusFound)
}

// redirectOAuthFailure sends the browser back with a reason and no session.
//
// The cookies are cleared on the way out. A failed flow must not leave a half-set
// session behind, and clearing is also what recovers a client whose refresh cookie the
// server has since revoked.
func (a *api) redirectOAuthFailure(w http.ResponseWriter, r *http.Request, reason string) {
	a.clearSessionCookies(w, r)
	w.Header().Set("Cache-Control", "no-store")
	target := a.oauthFailureReturnTo
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, appendQuery(target, map[string]string{
		oauthResultQuery: "failed",
		oauthReasonQuery: reason,
	}), http.StatusFound)
}

// oauthFailureReason maps a failure to a token the SPA can turn into a message.
//
// Unlike the password login, these are distinguishable. There is nothing to enumerate:
// whoever got here already proved to Google that they control the address, and
// "this address already has an account" is the only thing that tells them what to do
// next. Anything unrecognised collapses to a generic reason.
func oauthFailureReason(err error) string {
	switch {
	case errors.Is(err, auth.ErrOAuthEmailTaken):
		return "email-taken"
	case errors.Is(err, auth.ErrOAuthEmailUnverified):
		return "email-unverified"
	case errors.Is(err, auth.ErrOAuthAlreadyLinked):
		return "already-linked"
	case errors.Is(err, auth.ErrOAuthStateInvalid):
		// Overwhelmingly a stale browser tab or a back-button re-submission, not an
		// attack. "Start again" is the right advice either way.
		return "expired"
	default:
		return "failed"
	}
}

// isExpectedOAuthFailure separates the outcomes that are part of normal use from the
// ones that mean something is broken. Logging a dismissed consent screen at error
// level would bury the exchanges that actually failed.
func isExpectedOAuthFailure(err error) bool {
	return errors.Is(err, auth.ErrOAuthEmailTaken) ||
		errors.Is(err, auth.ErrOAuthEmailUnverified) ||
		errors.Is(err, auth.ErrOAuthAlreadyLinked) ||
		errors.Is(err, auth.ErrOAuthStateInvalid)
}

// appendQuery adds parameters to a URL that may already have some.
//
// Parsed rather than concatenated: the return target comes from configuration and can
// legitimately carry a query already, and "?a=1?b=2" is not a URL.
func appendQuery(target string, parameters map[string]string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		// An unparseable target is a configuration error. Falling back to the raw
		// string keeps the redirect useful for a human reading the address bar.
		return target
	}
	query := parsed.Query()
	for key, value := range parameters {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// firstAllowedOrigin is where a failed callback goes when nothing else is configured.
// The SPA's origin in every deployment so far, and in any case a page that can render
// the reason — unlike this API's own root, which is a 404.
func firstAllowedOrigin(origins []string) string {
	for _, origin := range origins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" && trimmed != "*" {
			return strings.TrimSuffix(trimmed, "/") + "/"
		}
	}
	return "/"
}

// OAuthDefaultReturnTo is where a flow that named no target ends up.
func OAuthDefaultReturnTo(origins []string) string {
	return firstAllowedOrigin(origins)
}

// OAuthReturnAllowlist derives the allowed return targets from the CORS allowlist.
//
// The same origins the browser is allowed to call this API from are the ones it may be
// sent back to. Deriving it means there is one list to keep correct instead of two
// that can disagree — and a disagreement here is an open redirect on a URL users
// follow immediately after signing in.
func OAuthReturnAllowlist(origins []string) []string {
	allowlist := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(origin), "/"))
		if origin == "" || origin == "*" {
			// A wildcard CORS entry must not become a wildcard redirect target.
			continue
		}
		allowlist = append(allowlist, origin+"/")
	}
	return allowlist
}
