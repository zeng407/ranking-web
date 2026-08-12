package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"2pick.app/backend/internal/media"
)

// The three things that answer questions about a URL: YouTube's API, a page's own HTML,
// and a HEAD request.

// ProviderTimeout bounds each outbound call. A batch may carry a hundred URLs, so one
// slow host must not hold the request open indefinitely.
const ProviderTimeout = 10 * time.Second

// ---------- YouTube ----------

// YouTubeAPI reads video metadata from the YouTube Data API.
type YouTubeAPI struct {
	key    string
	client *http.Client
	// endpoint is overridable so the tests can answer without leaving the machine.
	endpoint string
}

func NewYouTubeAPI(key string) (*YouTubeAPI, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("ingest: a YouTube API key is required")
	}
	return &YouTubeAPI{
		key:      key,
		client:   &http.Client{Timeout: ProviderTimeout},
		endpoint: "https://www.googleapis.com/youtube/v3/videos",
	}, nil
}

// ErrVideoNotFound means the id resolved to nothing: deleted, private, or never real.
var ErrVideoNotFound = errors.New("ingest: video not found")

func (api *YouTubeAPI) Video(ctx context.Context, videoID string) (VideoInfo, error) {
	query := url.Values{
		"part": {"snippet,contentDetails"},
		"id":   {videoID},
		"key":  {api.key},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		api.endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return VideoInfo{}, fmt.Errorf("ingest: build youtube request: %w", err)
	}

	response, err := api.client.Do(request)
	if err != nil {
		return VideoInfo{}, fmt.Errorf("ingest: youtube lookup %q: %w", videoID, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return VideoInfo{}, fmt.Errorf("ingest: youtube lookup %q: status %d", videoID, response.StatusCode)
	}

	var payload struct {
		Items []struct {
			Snippet struct {
				Title      string `json:"title"`
				Thumbnails map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return VideoInfo{}, fmt.Errorf("ingest: decode youtube response: %w", err)
	}
	if len(payload.Items) == 0 {
		return VideoInfo{}, ErrVideoNotFound
	}

	item := payload.Items[0]
	return VideoInfo{
		Title:        item.Snippet.Title,
		ThumbURL:     bestThumbnail(item.Snippet.Thumbnails),
		DurationSecs: ParseISODuration(item.ContentDetails.Duration),
	}, nil
}

// bestThumbnail picks in the order the original did: high, then medium, standard, maxres,
// default. Not the largest available — "high" is 480x360, which is what every card on the
// site is sized for, and maxres would be a 1280-wide image behind a 160-wide thumbnail.
func bestThumbnail(thumbnails map[string]struct {
	URL string `json:"url"`
}) string {
	for _, size := range []string{"high", "medium", "standard", "maxres", "default"} {
		if thumbnail, ok := thumbnails[size]; ok && thumbnail.URL != "" {
			return thumbnail.URL
		}
	}
	return ""
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// ParseISODuration reads YouTube's PT#H#M#S into seconds, and answers 0 for anything it
// does not recognise — a live stream reports P0D, which has no length to store.
func ParseISODuration(value string) int {
	matches := isoDuration.FindStringSubmatch(value)
	if matches == nil {
		return 0
	}
	number := func(part string) int {
		if part == "" {
			return 0
		}
		parsed, _ := strconv.Atoi(part)
		return parsed
	}
	return number(matches[1])*3600 + number(matches[2])*60 + number(matches[3])
}

// ---------- a page's own HTML ----------

// PageScraper reads a page's title and preview image, for the sources with no API.
//
// It fetches through the SSRF-safe fetcher for the same reason everything else here does:
// the URL comes from whoever is pasting it.
type PageScraper struct {
	fetcher Fetcher
}

func NewPageScraper(fetcher Fetcher) (*PageScraper, error) {
	if fetcher == nil {
		return nil, errors.New("ingest: a fetcher is required")
	}
	return &PageScraper{fetcher: fetcher}, nil
}

var (
	openGraphImage = regexp.MustCompile(
		`(?is)<meta[^>]+property\s*=\s*["']og:image["'][^>]+content\s*=\s*["']([^"']+)["']`)
	openGraphImageReversed = regexp.MustCompile(
		`(?is)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+property\s*=\s*["']og:image["']`)
	headingText = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	markup      = regexp.MustCompile(`(?s)<[^>]*>`)
)

func (scraper *PageScraper) Page(ctx context.Context, rawURL string) (VideoInfo, error) {
	body, err := scraper.fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return VideoInfo{}, fmt.Errorf("ingest: read page %q: %w", rawURL, err)
	}
	html := string(body)

	// Reported as the page wrote it. Making it usable — absolute, and without the CDN's
	// crop suffix — is NormalizeThumbnailURL's job, and it belongs on the one path that
	// consumes the value rather than in each thing that could produce one.
	return VideoInfo{
		Title:    firstHeading(html),
		ThumbURL: firstOpenGraphImage(html),
	}, nil
}

func firstOpenGraphImage(html string) string {
	// Both attribute orders, because a page is free to write either and the original's
	// DOM query did not care which came first.
	if matches := openGraphImage.FindStringSubmatch(html); matches != nil {
		return strings.TrimSpace(matches[1])
	}
	if matches := openGraphImageReversed.FindStringSubmatch(html); matches != nil {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func firstHeading(html string) string {
	matches := headingText.FindStringSubmatch(html)
	if matches == nil {
		return ""
	}
	// Any nested markup is dropped rather than rendered: an <h1> often wraps a <span>,
	// and the tag names are not part of the title.
	text := markup.ReplaceAllString(matches[1], "")
	return strings.TrimSpace(unescapeHTML(text))
}

// unescapeHTML handles the entities a title realistically carries. A full parser is not
// warranted for a value that is about to be truncated to a hundred runes.
func unescapeHTML(value string) string {
	return strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ",
	).Replace(value)
}

// NormalizeThumbnailURL applies the two fix-ups the original applied to Bilibili's
// preview: a protocol-relative URL gains https, and the @100w_100h_1c.png suffix — which
// asks their CDN for a 100-pixel-wide crop — is dropped so the full image is used.
func NormalizeThumbnailURL(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	return strings.Replace(value, "@100w_100h_1c.png", "", 1)
}

// ---------- HEAD ----------

// HeadProber answers what a URL serves with one HEAD request.
type HeadProber struct {
	client *http.Client
}

// NewHeadProber builds a prober that refuses private addresses, because the URLs it is
// pointed at come from anyone with an account.
func NewHeadProber() *HeadProber {
	return &HeadProber{client: &http.Client{
		Timeout:   ProviderTimeout,
		Transport: media.NewSafeTransport(),
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("ingest: too many redirects")
			}
			return media.ValidateSourceURL(request.URL.String())
		},
	}}
}

func (prober *HeadProber) Probe(ctx context.Context, rawURL string) (string, int64) {
	if err := media.ValidateSourceURL(rawURL); err != nil {
		return "", -1
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", -1
	}
	response, err := prober.client.Do(request)
	if err != nil {
		return "", -1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", -1
	}

	// The parameters are dropped: "image/jpeg; charset=binary" is still an image.
	contentType := strings.TrimSpace(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0])
	size := response.ContentLength
	if size == 0 {
		// A HEAD that reports nothing is unknown, not empty.
		size = -1
	}
	return strings.ToLower(contentType), size
}
