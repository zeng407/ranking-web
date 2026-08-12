package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/postaccess"
)

// postAccessHeader carries the proof that the caller knows a post's door code.
//
// Its value is one or more "serial:token" pairs, comma-separated, and the header may also
// be repeated. A caller can be part-way through games on more than one protected post, and
// the browser has no way to know which of them a given request concerns.
const postAccessHeader = "X-Post-Access"

// maxPostAccessTokens caps how many pairs one request may present.
//
// Each accepted pair becomes a placeholder in an IN list on every post query, so an
// uncapped header is a way to make the server do arbitrary work — and a genuine visitor
// has a handful at most.
const maxPostAccessTokens = 10

// PostAccessService is the door-code check. Optional: without it, protected posts stay
// invisible to this API, which is how it behaved before this existed.
type PostAccessService interface {
	Grant(ctx context.Context, serial, password string) (string, time.Time, error)
	CallerFor(userID int64, serialsToTokens map[string]string) postaccess.Caller
	Reissue(serial string) (string, time.Time)
}

type postAccessRequest struct {
	Password string `json:"password"`
}

type postAccessResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	ExpiresIn int    `json:"expires_in"`
}

/*
grantPostAccess exchanges a post's password for a token.

Laravel took the plaintext password in the Authorization header
(GameController::access). That header is the one most likely to be recorded by a proxy,
an access log or an error reporter, so here the password travels in the request body
instead. The response is the same shape of thing either way: proof, good for thirty
minutes.
*/
func (a *api) grantPostAccess(w http.ResponseWriter, r *http.Request) {
	if a.postAccess == nil {
		writeError(w, r, http.StatusServiceUnavailable, "post_access_not_configured",
			"post access is not configured")
		return
	}
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_post_serial",
			"post serial is required and must contain at most 255 characters")
		return
	}

	var request postAccessRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	token, expiresAt, err := a.postAccess.Grant(r.Context(), serial, request.Password)
	switch {
	case errors.Is(err, postaccess.ErrPostNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "post not found")
		return
	case errors.Is(err, postaccess.ErrWrongPassword):
		// 403, as Laravel answered. Not 401: the caller is not being asked to
		// authenticate as anyone, and a WWW-Authenticate challenge would make browsers
		// pop their own credential dialog.
		writeError(w, r, http.StatusForbidden, "wrong_password", "the password is incorrect")
		return
	case errors.Is(err, postaccess.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		writeError(w, r, http.StatusTooManyRequests, "too_many_attempts",
			"too many attempts, try again in a minute")
		return
	case err != nil:
		a.logger.Error("post_access_grant_failed", "serial", serial, "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to check the password")
		return
	}

	writePrivateJSON(w, r, http.StatusOK, postAccessResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		ExpiresIn: int(time.Until(expiresAt).Seconds()),
	})
}

/*
callerFor builds the postaccess.Caller for one request.

Every gameplay and rank query takes one of these. A request with no bearer token and no
access header produces the zero value, which is exactly "the public may see public posts"
— so a path that forgets to thread it through fails closed rather than open.
*/
func (a *api) callerFor(r *http.Request) postaccess.Caller {
	var userID int64
	if identity, ok := auth.IdentityFromContext(r.Context()); ok {
		// A subject that is not a user id leaves the caller anonymous rather than
		// failing the request: the query then shows them what the public may see.
		if parsed, err := auth.SubjectToUserID(identity.Subject); err == nil {
			userID = parsed
		}
	}
	if a.postAccess == nil {
		return postaccess.Caller{UserID: userID}
	}
	return a.postAccess.CallerFor(userID, presentedPostTokens(r))
}

// presentedPostTokens reads the serial:token pairs off the request.
//
// Malformed pairs are skipped rather than refused: the header is set by the client from
// whatever it has in storage, and one stale entry must not stop a request that has nothing
// to do with that post.
func presentedPostTokens(r *http.Request) map[string]string {
	pairs := make(map[string]string)
	for _, value := range r.Header.Values(postAccessHeader) {
		for _, entry := range strings.Split(value, ",") {
			serial, token, found := strings.Cut(strings.TrimSpace(entry), ":")
			if !found {
				continue
			}
			serial, token = strings.TrimSpace(serial), strings.TrimSpace(token)
			if serial == "" || token == "" || utf8.RuneCountInString(serial) > 255 {
				continue
			}
			if _, exists := pairs[serial]; !exists && len(pairs) >= maxPostAccessTokens {
				return pairs
			}
			pairs[serial] = token
		}
	}
	return pairs
}

/*
writeScopedJSON writes a body that a protected post may have contributed to.

Two things have to happen together here, which is why they share a function. The response
carries a refreshed access token, and it must not be cached publicly when the caller saw it
only because of who they are — a shared cache would then serve a protected post's ranks to
the next anonymous visitor with the same URL. An anonymous caller with no tokens saw only
public data, so that response stays cacheable, which is nearly all of them.
*/
func (a *api) writeScopedJSON(
	w http.ResponseWriter, r *http.Request, caller postaccess.Caller, serial string, payload any,
) {
	a.refreshPostAccess(w, caller, serial)
	if caller.UserID != 0 || len(caller.UnlockedSerials) > 0 {
		writePrivateJSON(w, r, http.StatusOK, payload)
		return
	}
	writePublicJSON(w, r, payload)
}

// refreshPostAccess hands back a fresh token for a protected post the caller is using.
//
// This is AccessTokenService::extendPostAccessToken, which pushed the session entry's
// expiry forward on every use so that a visitor part-way through a long game was not
// locked out mid-play. Statelessly, that means putting a new token on the response and
// letting the client replace what it holds.
func (a *api) refreshPostAccess(w http.ResponseWriter, caller postaccess.Caller, serial string) {
	if a.postAccess == nil || !caller.Unlocked(serial) {
		return
	}
	token, _ := a.postAccess.Reissue(serial)
	w.Header().Set(postAccessHeader, serial+":"+token)
}
