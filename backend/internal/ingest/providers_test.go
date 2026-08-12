package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"2pick.app/backend/internal/media"
)

// The three providers, against local servers rather than the real ones — what is being
// checked is how their answers are read, not that Google is up.

func TestTheYouTubeLookupReadsTitleThumbnailAndDuration(t *testing.T) {
	var asked *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"snippet":{"title":"a video","thumbnails":{
				"default":{"url":"https://i.ytimg.test/default.jpg"},
				"medium":{"url":"https://i.ytimg.test/medium.jpg"},
				"high":{"url":"https://i.ytimg.test/high.jpg"},
				"maxres":{"url":"https://i.ytimg.test/maxres.jpg"}}},
			"contentDetails":{"duration":"PT1H2M3S"}}]}`))
	}))
	defer server.Close()

	api, err := NewYouTubeAPI("a-key")
	if err != nil {
		t.Fatalf("NewYouTubeAPI() error = %v", err)
	}
	api.endpoint = server.URL

	info, err := api.Video(context.Background(), "kSNYTZXj_G8")
	if err != nil {
		t.Fatalf("Video() error = %v", err)
	}
	if info.Title != "a video" {
		t.Errorf("title = %q", info.Title)
	}
	// high, not maxres: the cards are 480 wide at most, and maxres would be a 1280-pixel
	// image behind a thumbnail. That is the order the original picked in.
	if info.ThumbURL != "https://i.ytimg.test/high.jpg" {
		t.Errorf("thumbnail = %q, want the high one", info.ThumbURL)
	}
	if info.DurationSecs != 3723 {
		t.Errorf("duration = %d, want 3723", info.DurationSecs)
	}
	if asked.URL.Query().Get("id") != "kSNYTZXj_G8" || asked.URL.Query().Get("key") != "a-key" {
		t.Errorf("asked %s", asked.URL.RawQuery)
	}
}

func TestTheYouTubeLookupFallsDownTheThumbnailSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"snippet":{"title":"t","thumbnails":{
			"default":{"url":"https://i.ytimg.test/default.jpg"}}},
			"contentDetails":{"duration":"PT30S"}}]}`))
	}))
	defer server.Close()

	api, _ := NewYouTubeAPI("a-key")
	api.endpoint = server.URL

	info, err := api.Video(context.Background(), "x")
	if err != nil {
		t.Fatalf("Video() error = %v", err)
	}
	if info.ThumbURL != "https://i.ytimg.test/default.jpg" {
		t.Errorf("thumbnail = %q", info.ThumbURL)
	}
}

// A deleted or private video comes back as an empty list, not an error status.
func TestAnUnknownVideoIsErrVideoNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	api, _ := NewYouTubeAPI("a-key")
	api.endpoint = server.URL

	if _, err := api.Video(context.Background(), "gone"); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("error = %v, want ErrVideoNotFound", err)
	}
}

func TestAnAPIFailureIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	api, _ := NewYouTubeAPI("a-key")
	api.endpoint = server.URL

	if _, err := api.Video(context.Background(), "x"); err == nil {
		t.Fatal("Video() reported no error for a 403")
	}
}

func TestParseISODuration(t *testing.T) {
	cases := map[string]int{
		"PT30S":    30,
		"PT2M":     120,
		"PT1H":     3600,
		"PT1H2M3S": 3723,
		"PT10M30S": 630,
		// A live stream reports this, and it has no length to store.
		"P0D":      0,
		"":         0,
		"nonsense": 0,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			if got := ParseISODuration(value); got != want {
				t.Errorf("got %d, want %d", got, want)
			}
		})
	}
}

type staticFetcher struct {
	body []byte
	err  error
}

func (fetcher staticFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return fetcher.body, fetcher.err
}

func TestThePageScraperReadsTheTitleAndPreview(t *testing.T) {
	html := `<html><head>
		<meta property="og:image" content="//i0.hdslb.test/preview.jpg@100w_100h_1c.png">
		</head><body><h1 class="video-title"><span>an &amp; interesting</span> video</h1></body></html>`

	scraper, err := NewPageScraper(staticFetcher{body: []byte(html)})
	if err != nil {
		t.Fatalf("NewPageScraper() error = %v", err)
	}

	info, err := scraper.Page(context.Background(), "https://www.bilibili.test/video/BV1")
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	// Nested markup dropped, the entity decoded.
	if info.Title != "an & interesting video" {
		t.Errorf("title = %q", info.Title)
	}
	// Reported as written; making it usable is the caller's step.
	if info.ThumbURL != "//i0.hdslb.test/preview.jpg@100w_100h_1c.png" {
		t.Errorf("thumbnail = %q", info.ThumbURL)
	}
}

// A page is free to write the attributes either way round, and the original's DOM query
// did not care which came first.
func TestThePageScraperAcceptsEitherAttributeOrder(t *testing.T) {
	html := `<meta content="https://example.test/p.jpg" property="og:image"><h1>a title</h1>`
	scraper, _ := NewPageScraper(staticFetcher{body: []byte(html)})

	info, err := scraper.Page(context.Background(), "https://example.test")
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	if info.ThumbURL != "https://example.test/p.jpg" {
		t.Errorf("thumbnail = %q", info.ThumbURL)
	}
}

func TestThePageScraperSurvivesAPageWithNeither(t *testing.T) {
	scraper, _ := NewPageScraper(staticFetcher{body: []byte(`<html><body>nothing</body></html>`)})

	info, err := scraper.Page(context.Background(), "https://example.test")
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	if info.Title != "" || info.ThumbURL != "" {
		t.Errorf("info = %+v, want both empty", info)
	}
}

func TestNormalizeThumbnailURL(t *testing.T) {
	cases := map[string]string{
		"//i0.hdslb.test/a.jpg":                  "https://i0.hdslb.test/a.jpg",
		"//i0.hdslb.test/a.jpg@100w_100h_1c.png": "https://i0.hdslb.test/a.jpg",
		"https://i0.hdslb.test/a.jpg":            "https://i0.hdslb.test/a.jpg",
		"":                                       "",
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			if got := NormalizeThumbnailURL(value); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestTheHeadProberReportsTypeAndSize(t *testing.T) {
	// httptest listens on loopback, which the prober refuses by design. The escape hatch
	// is read per call for exactly this, and media's own tests use it the same way.
	t.Setenv(media.AllowPrivateSourcesEnv, "true")

	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Header().Set("Content-Type", "video/mp4; charset=binary")
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	contentType, size := NewHeadProber().Probe(context.Background(), server.URL)

	if method != http.MethodHead {
		t.Errorf("method = %s, want HEAD — not downloading it is the whole point", method)
	}
	// The parameters are dropped: "video/mp4; charset=binary" is still a video.
	if contentType != "video/mp4" {
		t.Errorf("type = %q, want video/mp4", contentType)
	}
	if size != 1048576 {
		t.Errorf("size = %d, want 1048576", size)
	}
}

// The block is the reason this is safe to point at a URL an author pasted.
func TestTheHeadProberRefusesAPrivateAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	contentType, size := NewHeadProber().Probe(context.Background(), server.URL)

	if contentType != "" || size != -1 {
		t.Errorf("a loopback address was probed: type %q, size %d", contentType, size)
	}
}

func TestTheHeadProberRefusesWhatIsNotAURL(t *testing.T) {
	for _, value := range []string{"", "not a url", "file:///etc/passwd", "http://169.254.169.254/"} {
		contentType, size := NewHeadProber().Probe(context.Background(), value)
		if contentType != "" || size != -1 {
			t.Errorf("%q was probed: type %q, size %d", value, contentType, size)
		}
	}
}

func TestNewYouTubeAPIRequiresAKey(t *testing.T) {
	if _, err := NewYouTubeAPI("  "); err == nil {
		t.Error("NewYouTubeAPI() accepted an empty key")
	}
}
