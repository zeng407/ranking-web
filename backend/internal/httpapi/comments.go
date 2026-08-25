package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"2pick.app/backend/internal/auth"
	commentdomain "2pick.app/backend/internal/comments"
)

// The delete key: how a signed-out commenter is recognised as the author of their own
// comment later on.
const (
	// commentKeyCookieName holds an opaque random token. httpOnly, because nothing in
	// the page needs to read it — the client learns which comments it may delete from
	// can_delete on each comment, not from the cookie.
	commentKeyCookieName = "2pick_comment_key"
	// commentKeyCookiePath scopes the key to the endpoints that use it. Every comment
	// route lives under this prefix, and no other request has any business carrying it.
	commentKeyCookiePath = "/api/v1/posts"
	// commentKeyCookieMaxAge is a year. The key is the only claim a signed-out
	// commenter has on what they wrote, so it should outlive the visit that created it
	// by as much as a browser will allow.
	commentKeyCookieMaxAge = 365 * 24 * 60 * 60
)

func (a *api) commentsCollection(w http.ResponseWriter, r *http.Request) {
	if a.comments == nil {
		writeError(w, r, http.StatusServiceUnavailable, "comments_not_configured", "comments are not configured")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.listComments(w, r)
	case http.MethodPost:
		a.createComment(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *api) listComments(w http.ResponseWriter, r *http.Request) {
	page, ok := positiveQueryInt(r.URL.Query().Get("page"), 1, 1, 1_000_000)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "page must be a positive integer")
		return
	}
	viewer, ok := commentViewer(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "invalid_identity", "the authenticated identity is invalid")
		return
	}
	viewer.AnonymousID = normalizedAnonymousID(r.URL.Query().Get("anonymous_id"))
	viewer.DeleteHash = commentKeyHash(r)
	result, err := a.comments.List(r.Context(), r.PathValue("serial"), page, 10, viewer)
	if err != nil {
		a.writeCommentError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusOK, result)
}

func (a *api) createComment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Content     string `json:"content"`
		Anonymous   bool   `json:"anonymous"`
		AnonymousID string `json:"anonymous_id"`
		ParentID    *int64 `json:"parent_id"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	request.Content = strings.TrimSpace(request.Content)
	if request.Content == "" || utf8.RuneCountInString(request.Content) > commentdomain.MaxContentLength {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_comment", "content is required and must contain at most 200 characters")
		return
	}
	if utf8.RuneCountInString(request.AnonymousID) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_anonymous_id", "anonymous_id must contain at most 255 characters")
		return
	}
	viewer, ok := commentViewer(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "invalid_identity", "the authenticated identity is invalid")
		return
	}
	if request.ParentID != nil && *request.ParentID <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_parent", "parent_id must be positive")
		return
	}
	viewer.AnonymousID = normalizedAnonymousID(request.AnonymousID)
	viewer.DeleteHash = a.ensureCommentKey(w, r)
	created, err := a.comments.Create(r.Context(), r.PathValue("serial"), commentdomain.CreateInput{
		Content: request.Content, Anonymous: request.Anonymous && viewer.UserID != nil,
		AnonymousID: viewer.AnonymousID, IP: clientIP(r), ParentID: request.ParentID, Viewer: viewer,
	})
	if err != nil {
		a.writeCommentError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusCreated, created)
}

func (a *api) deleteComment(w http.ResponseWriter, r *http.Request) {
	if a.comments == nil {
		writeError(w, r, http.StatusServiceUnavailable, "comments_not_configured", "comments are not configured")
		return
	}
	commentID, ok := positiveInt64(r.PathValue("commentID"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_comment", "comment id must be positive")
		return
	}
	viewer, ok := commentViewer(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "invalid_identity", "the authenticated identity is invalid")
		return
	}
	// Read, never minted: a browser that carries no key owns nothing, and handing one
	// out here would only hand out a claim on comments it did not write.
	viewer.DeleteHash = commentKeyHash(r)
	if err := a.comments.Delete(r.Context(), r.PathValue("serial"), commentID, viewer); err != nil {
		a.writeCommentError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) reportComment(w http.ResponseWriter, r *http.Request) {
	if a.comments == nil {
		writeError(w, r, http.StatusServiceUnavailable, "comments_not_configured", "comments are not configured")
		return
	}
	commentID, ok := positiveInt64(r.PathValue("commentID"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_comment", "comment id must be positive")
		return
	}
	var request struct {
		Reason      string `json:"reason"`
		AnonymousID string `json:"anonymous_id"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if utf8.RuneCountInString(request.Reason) > commentdomain.MaxReportLength || utf8.RuneCountInString(request.AnonymousID) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_report", "report fields are too long")
		return
	}
	viewer, ok := commentViewer(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "invalid_identity", "the authenticated identity is invalid")
		return
	}
	viewer.AnonymousID = normalizedAnonymousID(request.AnonymousID)
	if err := a.comments.Report(r.Context(), r.PathValue("serial"), commentID, commentdomain.ReportInput{
		Reason: request.Reason, AnonymousID: viewer.AnonymousID, IP: clientIP(r), Viewer: viewer,
	}); err != nil {
		a.writeCommentError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusCreated, map[string]bool{"reported": true})
}

func (a *api) writeCommentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, commentdomain.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "post or comment was not found")
	case errors.Is(err, commentdomain.ErrInvalidParent):
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_parent",
			"the comment being replied to is gone or is already as deeply nested as replies go")
	case errors.Is(err, commentdomain.ErrRateLimit):
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many comments; please try again later")
	default:
		a.logger.Error("comment_request_failed",
			"request_id", requestIDFromContext(r.Context()), "path", r.URL.Path, "error", err,
		)
		writeError(w, r, http.StatusServiceUnavailable, "comments_unavailable", "comments are temporarily unavailable")
	}
}

func commentViewer(r *http.Request) (commentdomain.Viewer, bool) {
	identity, authenticated := auth.IdentityFromContext(r.Context())
	if !authenticated {
		return commentdomain.Viewer{}, true
	}
	userID, err := strconv.ParseInt(identity.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return commentdomain.Viewer{}, false
	}
	return commentdomain.Viewer{UserID: &userID}, true
}

// commentKeyHash returns the SHA-256 of the request's delete key, hex encoded, or an
// empty string when the request carries none.
//
// The database stores the hash rather than the key itself, so a leaked dump is not a
// pile of working credentials.
func commentKeyHash(r *http.Request) string {
	cookie, err := r.Cookie(commentKeyCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	return hex.EncodeToString(sum[:])
}

// ensureCommentKey returns the hash of the request's delete key, issuing one first if the
// browser has none.
//
// Issued here and nowhere else: only posting a comment creates something to delete, so
// only posting a comment justifies the cookie. Readers are given nothing.
func (a *api) ensureCommentKey(w http.ResponseWriter, r *http.Request) string {
	if hash := commentKeyHash(r); hash != "" {
		return hash
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		// Without a key the comment is still posted; it simply cannot be deleted from
		// this browser later. Failing the write instead would be the worse trade.
		a.logger.Error("comment_key_unavailable",
			"request_id", requestIDFromContext(r.Context()), "error", err,
		)
		return ""
	}
	key := hex.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{
		Name:     commentKeyCookieName,
		Value:    key,
		Path:     commentKeyCookiePath,
		MaxAge:   commentKeyCookieMaxAge,
		HttpOnly: true,
		Secure:   a.cookiesAreSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func normalizedAnonymousID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
