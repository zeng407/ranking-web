package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The batch's rules, against in-memory providers.

type memoryYouTube struct {
	info VideoInfo
	err  error
	ids  []string
}

func (lookup *memoryYouTube) Video(_ context.Context, videoID string) (VideoInfo, error) {
	lookup.ids = append(lookup.ids, videoID)
	return lookup.info, lookup.err
}

type memoryPages struct {
	info VideoInfo
	err  error
	urls []string
}

func (lookup *memoryPages) Page(_ context.Context, rawURL string) (VideoInfo, error) {
	lookup.urls = append(lookup.urls, rawURL)
	return lookup.info, lookup.err
}

type memoryFetcher struct {
	content []byte
	err     error
	urls    []string
}

func (fetcher *memoryFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	fetcher.urls = append(fetcher.urls, rawURL)
	return fetcher.content, fetcher.err
}

type urlHarness struct {
	service *Service
	store   *memoryStore
	objects *memoryObjects
	thumbs  *memoryThumbs
	youtube *memoryYouTube
	pages   *memoryPages
	fetcher *memoryFetcher
	prober  *staticProber
}

func newURLHarness(t *testing.T) *urlHarness {
	t.Helper()
	store := &memoryStore{postID: 42}
	objects := &memoryObjects{}
	thumbs := &memoryThumbs{}
	youtube := &memoryYouTube{info: VideoInfo{
		Title: "a youtube video", ThumbURL: "https://i.ytimg.test/high.jpg", DurationSecs: 245,
	}}
	pages := &memoryPages{info: VideoInfo{
		Title: "a bilibili video", ThumbURL: "//i0.hdslb.test/preview.jpg@100w_100h_1c.png",
	}}
	fetcher := &memoryFetcher{content: png(64)}
	prober := &staticProber{}

	service, err := NewService(ServiceOptions{
		Store: store, Objects: objects, Thumbs: thumbs,
		YouTube: youtube, Pages: pages, Fetcher: fetcher, Prober: prober,
		KeyName: func(directory, extension string) string { return directory + "/fixed." + extension },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &urlHarness{
		service: service, store: store, objects: objects, thumbs: thumbs,
		youtube: youtube, pages: pages, fetcher: fetcher, prober: prober,
	}
}

func TestAddURLsStoresAYouTubeVideoWithItsMetadata(t *testing.T) {
	harness := newURLHarness(t)

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://www.youtube.com/watch?v=kSNYTZXj_G8")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Added) != 1 || len(result.Failed) != 0 {
		t.Fatalf("added %d, failed %d", len(result.Added), len(result.Failed))
	}

	written := harness.store.created[0]
	if written.VideoSource != VideoSourceYouTube || written.VideoID != "kSNYTZXj_G8" {
		t.Errorf("source = %q, id = %q", written.VideoSource, written.VideoID)
	}
	if written.Type != TypeVideo {
		t.Errorf("type = %q, want video", written.Type)
	}
	if written.Title != "a youtube video" {
		t.Errorf("title = %q, want the one YouTube gave", written.Title)
	}
	if written.ThumbURL != "https://i.ytimg.test/high.jpg" {
		t.Errorf("thumb = %q", written.ThumbURL)
	}
	if written.DurationSecs == nil || *written.DurationSecs != 245 {
		t.Errorf("duration = %v, want 245", written.DurationSecs)
	}
	// A remote video's thumbnail comes from its own service; nothing is generated.
	if len(harness.thumbs.queued) != 0 {
		t.Errorf("queued %v for a remote video", harness.thumbs.queued)
	}
}

// An embed is the same video by a different route, and the column records which route it
// came in by — 20,093 rows say youtube_embed.
func TestAnEmbedIsStoredAsAnEmbed(t *testing.T) {
	harness := newURLHarness(t)
	embed := `<iframe src="https://www.youtube.com/embed/0f_wN2EoBlY?si=abc"></iframe>`

	if _, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh", embed); err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	written := harness.store.created[0]
	if written.VideoSource != VideoSourceYouTubeEmbed {
		t.Errorf("source = %q, want %q", written.VideoSource, VideoSourceYouTubeEmbed)
	}
	if written.VideoID != "0f_wN2EoBlY" {
		t.Errorf("id = %q", written.VideoID)
	}
}

// A title typed next to the URL wins over the one the service reports: the author is
// naming their own entry.
func TestASuppliedTitleWinsOverTheOneTheServiceGives(t *testing.T) {
	harness := newURLHarness(t)

	if _, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://www.youtube.com/watch?v=kSNYTZXj_G8 my own name"); err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	if got := harness.store.created[0].Title; got != "my own name" {
		t.Errorf("title = %q, want the supplied one", got)
	}
}

/*
BILIBILI'S PREVIEW IS COPIED INTO THIS SITE'S OWN STORAGE.

Their CDN refuses requests carrying a foreign referer, so a hotlinked preview is a broken
image on every card. The original downloaded it too. The two URL fix-ups come with it: a
protocol-relative address gains https, and the @100w_100h_1c.png crop suffix is dropped so
the full image is fetched.
*/
func TestABilibiliPreviewIsDownloadedAndNormalised(t *testing.T) {
	harness := newURLHarness(t)

	if _, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://m.bilibili.com/video/BV117421K7tp"); err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	written := harness.store.created[0]
	if written.VideoSource != VideoSourceBilibili || written.VideoID != "BV117421K7tp" {
		t.Errorf("source = %q, id = %q", written.VideoSource, written.VideoID)
	}
	if written.ThumbURL != "https://file.2pick.test/abcdefgh/fixed.png" {
		t.Errorf("thumb = %q, want the stored copy", written.ThumbURL)
	}
	if len(harness.fetcher.urls) != 1 {
		t.Fatalf("fetched %v", harness.fetcher.urls)
	}
	if harness.fetcher.urls[0] != "https://i0.hdslb.test/preview.jpg" {
		t.Errorf("fetched %q; the // prefix and the crop suffix must both be fixed",
			harness.fetcher.urls[0])
	}
}

func TestAnImageURLIsDownloadedAndStored(t *testing.T) {
	harness := newURLHarness(t)

	if _, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://example.test/photos/holiday.png"); err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	written := harness.store.created[0]
	if written.Type != TypeImage {
		t.Errorf("type = %q, want image", written.Type)
	}
	if written.Path != "abcdefgh/fixed.png" {
		t.Errorf("path = %q, want the stored key", written.Path)
	}
	if written.Title != "holiday" {
		t.Errorf("title = %q, want it from the file name", written.Title)
	}
	// The source stays what the author pasted; the copy is the thumbnail.
	if written.SourceURL != "https://example.test/photos/holiday.png" {
		t.Errorf("source = %q", written.SourceURL)
	}
}

/*
WHAT IS STORED IS WHAT WAS RECEIVED, NOT WHAT WAS PROMISED.

A URL ending in .png, or answering image/png to a HEAD, is free to serve anything at all
to the GET that follows. The bytes are sniffed after the download for the same reason an
upload is: this file is served back to every visitor.
*/
func TestAnImageURLThatServesSomethingElseIsRefused(t *testing.T) {
	harness := newURLHarness(t)
	harness.fetcher.content = []byte("<?php system($_GET['c']); ?>")

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://example.test/photos/holiday.png")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != ReasonUnavailable {
		t.Fatalf("failures = %+v, want one unavailable", result.Failed)
	}
	if len(harness.objects.keys) != 0 {
		t.Error("it reached the bucket anyway")
	}
	if len(harness.store.created) != 0 {
		t.Error("an element was written for it")
	}
}

// A direct video is left where it is: 2,205 production rows are exactly this, with no
// stored copy and the source as its own thumbnail.
func TestADirectVideoIsLinkedRatherThanCopied(t *testing.T) {
	harness := newURLHarness(t)
	harness.prober.contentType = "video/mp4"
	harness.prober.size = 3 * 1024 * 1024

	if _, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://cdn.example.test/attachments/12345"); err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	written := harness.store.created[0]
	if written.Type != TypeVideo || written.VideoSource != VideoSourceFile {
		t.Errorf("type = %q, source = %q", written.Type, written.VideoSource)
	}
	if written.Path != "" {
		t.Errorf("path = %q, want nothing stored", written.Path)
	}
	if written.ThumbURL != written.SourceURL {
		t.Errorf("thumb = %q, want the source itself", written.ThumbURL)
	}
	if len(harness.fetcher.urls) != 0 {
		t.Errorf("a linked video was downloaded: %v", harness.fetcher.urls)
	}
}

// The HEAD already said how big it is, so the limit is applied without pulling it.
func TestAnOversizedLinkIsRefusedWithoutDownloadingIt(t *testing.T) {
	harness := newURLHarness(t)
	harness.prober.contentType = "video/mp4"
	harness.prober.size = MaxFileBytes + 1

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://cdn.example.test/attachments/12345")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != ReasonTooLarge {
		t.Fatalf("failures = %+v, want one too_large", result.Failed)
	}
	if len(harness.fetcher.urls) != 0 {
		t.Errorf("it was downloaded anyway: %v", harness.fetcher.urls)
	}
}

/*
ONE BAD URL DOES NOT LOSE THE REST OF THE BATCH.

An author pasting thirty links will have one that is dead or private. Failing the whole
request over it would throw away the twenty-nine that worked, and the original did not
either — it collected the failures and reported them.
*/
func TestABatchKeepsWhatWorkedAndNamesWhatDidNot(t *testing.T) {
	harness := newURLHarness(t)

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh", strings.Join([]string{
		"https://www.youtube.com/watch?v=kSNYTZXj_G8",
		"not a url at all",
		"https://example.test/photo.png",
	}, ","))
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("added %d, want 2", len(result.Added))
	}
	if len(result.Failed) != 1 {
		t.Fatalf("failed %+v, want one", result.Failed)
	}
	if result.Failed[0].URL != "not" || result.Failed[0].Reason != ReasonUnrecognised {
		// "not a url at all" splits on the first space, so the URL part is "not".
		t.Errorf("failure = %+v", result.Failed[0])
	}
}

func TestAVideoWhoseMetadataCannotBeReadIsReportedNotStored(t *testing.T) {
	harness := newURLHarness(t)
	harness.youtube.err = ErrVideoNotFound

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://www.youtube.com/watch?v=kSNYTZXj_G8")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != ReasonUnavailable {
		t.Fatalf("failures = %+v", result.Failed)
	}
	if len(harness.store.created) != 0 {
		t.Error("an element was written for a video that could not be read")
	}
}

// A deployment with no API key still ingests images and links; YouTube reports itself
// unavailable rather than taking the batch down.
func TestASourceWithNoProviderIsUnavailableRatherThanFatal(t *testing.T) {
	store := &memoryStore{postID: 42}
	service, err := NewService(ServiceOptions{
		Store: store, Objects: &memoryObjects{}, Fetcher: &memoryFetcher{content: png(64)},
		KeyName: func(directory, extension string) string { return directory + "/fixed." + extension },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://www.youtube.com/watch?v=kSNYTZXj_G8,https://example.test/photo.png")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Added) != 1 {
		t.Errorf("added %d, want the image", len(result.Added))
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != ReasonUnavailable {
		t.Errorf("failures = %+v", result.Failed)
	}
}

func TestABatchStopsAddingAtThePostsLimit(t *testing.T) {
	harness := newURLHarness(t)
	harness.store.elements = MaxElements - 1

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://example.test/1.png,https://example.test/2.png,https://example.test/3.png")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Added) != 1 {
		t.Errorf("added %d, want the one that fit", len(result.Added))
	}
	if len(result.Failed) != 2 {
		t.Fatalf("failed %+v, want two", result.Failed)
	}
	for _, failure := range result.Failed {
		if failure.Reason != ReasonPostFull {
			t.Errorf("reason = %q, want %q", failure.Reason, ReasonPostFull)
		}
	}
}

func TestABatchRefusesTooManyURLs(t *testing.T) {
	harness := newURLHarness(t)
	urls := make([]string, MaxURLsPerRequest+1)
	for index := range urls {
		urls[index] = "https://example.test/photo.png?n=" + strings.Repeat("x", index+1)
	}

	_, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh", strings.Join(urls, ","))
	if code := codeFor(t, err, "urls"); code != CodeTooMany {
		t.Errorf("code = %q, want %q", code, CodeTooMany)
	}
	if len(harness.store.created) != 0 {
		t.Error("an oversized batch wrote elements")
	}
}

func TestAnEmptyBatchIsRefused(t *testing.T) {
	harness := newURLHarness(t)

	for _, list := range []string{"", "   ", " , , "} {
		_, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh", list)
		if code := codeFor(t, err, "urls"); code != CodeRequired {
			t.Errorf("%q: code = %q, want %q", list, code, CodeRequired)
		}
	}
}

func TestAddingToSomeoneElsesPostIsNotFound(t *testing.T) {
	harness := newURLHarness(t)
	harness.store.postErr = ErrPostNotFound

	_, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://example.test/photo.png")
	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("error = %v, want ErrPostNotFound", err)
	}
	if len(harness.fetcher.urls) != 0 {
		t.Error("a stranger's batch reached the network")
	}
}

// Twitch is recognised and deliberately not ingested — 42 elements a year, and no
// credentials in this deployment. It must say so rather than look like a broken link.
func TestTwitchIsRecognisedAndReportedUnavailable(t *testing.T) {
	harness := newURLHarness(t)

	result, err := harness.service.AddURLs(context.Background(), 7, "abcdefgh",
		"https://www.twitch.tv/videos/1618357349")
	if err != nil {
		t.Fatalf("AddURLs() error = %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != ReasonUnavailable {
		t.Errorf("failures = %+v", result.Failed)
	}
}
