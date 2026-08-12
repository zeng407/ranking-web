package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"2pick.app/backend/internal/publiccontent"
)

const (
	publicBrowserCache = "public, max-age=0"
	publicEdgeCache    = "public, max-age=60, stale-while-revalidate=30, stale-if-error=3600"
)

var allowedRankRanges = map[string]struct{}{
	"week": {}, "month": {}, "year": {}, "all": {}, "thousand_votes": {},
}

func (a *api) tags(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if utf8.RuneCountInString(keyword) > 30 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "keyword must contain at most 30 characters")
		return
	}
	tags, err := a.publicContent.Tags(r.Context(), keyword, 10)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	writePublicJSON(w, r, tags)
}

func (a *api) hotTags(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	tags, err := a.publicContent.HotTags(r.Context(), 30)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	writePublicJSON(w, r, tags)
}

func (a *api) carouselItems(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	items, err := a.publicContent.CarouselItems(r.Context())
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	writePublicJSON(w, r, items)
}

func (a *api) posts(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	query := r.URL.Query()
	sort := defaultString(query.Get("sort_by"), "hot")
	dateRange := defaultString(query.Get("range"), "week")
	if sort != "hot" && sort != "new" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "sort_by must be hot or new")
		return
	}
	if !isOneOf(dateRange, "all", "year", "month", "week", "day") {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "range must be all, year, month, week, or day")
		return
	}
	page, ok := positiveQueryInt(query.Get("page"), 1, 1, 1_000_000)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "page must be a positive integer")
		return
	}
	perPage, ok := positiveQueryInt(query.Get("per_page"), 15, 1, 15)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "per_page must be between 1 and 15")
		return
	}
	keyword := strings.TrimSpace(query.Get("k"))
	if utf8.RuneCountInString(keyword) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "k must contain at most 255 characters")
		return
	}

	result, err := a.publicContent.Posts(r.Context(), publiccontent.PostsQuery{
		Sort: sort, Range: dateRange, Keyword: keyword, Page: page, PerPage: perPage,
	})
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	writePublicJSON(w, r, result)
}

func (a *api) champions(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	champions, err := a.publicContent.Champions(r.Context(), 5)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	writePublicJSON(w, r, champions)
}

func (a *api) ranks(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	query := r.URL.Query()
	postSerial := strings.TrimSpace(query.Get("post_serial"))
	if postSerial == "" || utf8.RuneCountInString(postSerial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "post_serial is required and must contain at most 255 characters")
		return
	}
	page, ok := positiveQueryInt(query.Get("page"), 1, 1, 1_000_000)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "page must be a positive integer")
		return
	}
	perPage, ok := positiveQueryInt(query.Get("per_page"), 20, 1, 50)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "per_page must be between 1 and 50")
		return
	}
	group := publiccontent.RankGroup(defaultString(query.Get("group"), string(publiccontent.RankGroupCumulative)))
	if !group.Valid() {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "group must be cumulative or recent_1000")
		return
	}

	caller := a.callerFor(r)
	reports, err := a.publicContent.Ranks(r.Context(), postSerial, group, page, perPage, caller)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	a.writeScopedJSON(w, r, caller, postSerial, reports)
}

func (a *api) searchRanks(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	postSerial := strings.TrimSpace(r.URL.Query().Get("post_serial"))
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if postSerial == "" || utf8.RuneCountInString(postSerial) > 255 || keyword == "" || utf8.RuneCountInString(keyword) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "post_serial and keyword are required and must contain at most 255 characters")
		return
	}
	caller := a.callerFor(r)
	reports, err := a.publicContent.SearchRanks(r.Context(), postSerial, keyword, 10, caller)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	a.writeScopedJSON(w, r, caller, postSerial, reports)
}

func (a *api) rank(w http.ResponseWriter, r *http.Request) {
	if !a.requirePublicContent(w, r) {
		return
	}
	query := r.URL.Query()
	postSerial := strings.TrimSpace(query.Get("post_serial"))
	elementID, ok := positiveInt64(query.Get("element_id"))
	if postSerial == "" || utf8.RuneCountInString(postSerial) > 255 || !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "post_serial and a positive element_id are required")
		return
	}
	ranges := append([]string{}, query["time"]...)
	ranges = append(ranges, query["time[]"]...)
	ranges = uniqueNonEmpty(ranges)
	if len(ranges) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "at least one time range is required")
		return
	}
	for _, dateRange := range ranges {
		if _, allowed := allowedRankRanges[dateRange]; !allowed {
			writeError(w, r, http.StatusUnprocessableEntity, "invalid_query", "invalid time range")
			return
		}
	}
	caller := a.callerFor(r)
	details, err := a.publicContent.Rank(r.Context(), postSerial, elementID, ranges, caller)
	if err != nil {
		a.writePublicContentError(w, r, err)
		return
	}
	a.writeScopedJSON(w, r, caller, postSerial, details)
}

func (a *api) requirePublicContent(w http.ResponseWriter, r *http.Request) bool {
	if a.publicContent == nil {
		writeError(w, r, http.StatusServiceUnavailable, "content_not_configured", "public content database is not configured")
		return false
	}
	return true
}

func (a *api) writePublicContentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, publiccontent.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "public content was not found")
		return
	}
	a.logger.Error("public_content_query_failed",
		"request_id", requestIDFromContext(r.Context()), "path", r.URL.Path, "error", err,
	)
	writeError(w, r, http.StatusServiceUnavailable, "content_unavailable", "public content is temporarily unavailable")
}

func writePublicJSON(w http.ResponseWriter, r *http.Request, data any) {
	w.Header().Set("Cache-Control", publicBrowserCache)
	w.Header().Set("Cloudflare-CDN-Cache-Control", publicEdgeCache)
	writeJSON(w, r, http.StatusOK, envelope{Data: data})
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func positiveQueryInt(value string, fallback, minimum, maximum int) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed >= minimum && parsed <= maximum
}

func positiveInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil && parsed > 0
}

func isOneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func uniqueNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
