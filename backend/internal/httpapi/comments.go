package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"2pick.app/backend/internal/auth"
	commentdomain "2pick.app/backend/internal/comments"
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
	viewer.AnonymousID = normalizedAnonymousID(request.AnonymousID)
	created, err := a.comments.Create(r.Context(), r.PathValue("serial"), commentdomain.CreateInput{
		Content: request.Content, Anonymous: request.Anonymous && viewer.UserID != nil,
		AnonymousID: viewer.AnonymousID, IP: clientIP(r), Viewer: viewer,
	})
	if err != nil {
		a.writeCommentError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusCreated, created)
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

func normalizedAnonymousID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
