package ingest

import (
	"context"
	"strings"
	"testing"
)

// The classifier, against the URL shapes production actually holds. Every sample below
// was taken from the elements table rather than invented, because the shapes that matter
// are the ones authors have really pasted.

type staticProber struct {
	contentType string
	size        int64
	calls       []string
}

func (prober *staticProber) Probe(_ context.Context, rawURL string) (string, int64) {
	prober.calls = append(prober.calls, rawURL)
	return prober.contentType, prober.size
}

func TestClassifyRecognisesTheShapesProductionHolds(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		source Source
		id     string
	}{
		{"watch", "https://www.youtube.com/watch?v=kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"watch over http", "http://www.youtube.com/watch?v=--41OGPMurU", SourceYouTube, "--41OGPMurU"},
		// 4,909 elements. The original's own regex refused this host; it only worked
		// because ?v= was read before the host was looked at.
		{"music", "https://music.youtube.com/watch?v=kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"mobile", "https://m.youtube.com/watch?v=kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"short link", "https://youtu.be/kAP_x0krk7A?si=ISJ4-xWCOqXZ1B7g", SourceYouTube, "kAP_x0krk7A"},
		{"shorts", "https://www.youtube.com/shorts/kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"shorts with no www", "https://youtube.com/shorts/kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"embed url", "https://www.youtube.com/embed/kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"bare id", "kSNYTZXj_G8", SourceYouTube, "kSNYTZXj_G8"},
		{"bilibili mobile", "https://m.bilibili.com/video/BV117421K7tp?buvid=Y745", SourceBilibili, "BV117421K7tp"},
		{"bilibili", "https://www.bilibili.com/video/BV117421K7tp", SourceBilibili, "BV117421K7tp"},
		// No www, and /clip/ singular — both as stored.
		{"twitch clip", "https://twitch.tv/amamiharuma/clip/CrypticArbitraryToad", SourceTwitch, ""},
		{"twitch video", "https://www.twitch.tv/videos/1618357349", SourceTwitch, ""},
		{"jpg", "https://example.test/a/photo.jpg", SourceImage, ""},
		{"png with a query", "https://example.test/a/photo.png?w=100", SourceImage, ""},
		{"gif in caps", "https://example.test/a/PHOTO.GIF", SourceImage, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// No prober: every one of these must be settled by pattern alone, because a
			// network call per pasted URL is what makes a hundred-URL batch time out.
			got := Classify(context.Background(), testCase.value, nil)
			if got.Source != testCase.source {
				t.Errorf("source = %q, want %q", got.Source, testCase.source)
			}
			if got.ID != testCase.id {
				t.Errorf("id = %q, want %q", got.ID, testCase.id)
			}
		})
	}
}

// The embed form, as stored: a whole iframe with the id inside its src.
func TestClassifyReadsAYouTubeEmbed(t *testing.T) {
	embed := `<iframe width="100%" height="270" src="https://www.youtube.com/embed/0f_wN2EoBlY?si=abc" ` +
		`title="YouTube video player" frameborder="0" allowfullscreen></iframe>`

	got := Classify(context.Background(), embed, nil)

	if got.Source != SourceYouTubeEmbed {
		t.Fatalf("source = %q, want %q", got.Source, SourceYouTubeEmbed)
	}
	if got.ID != "0f_wN2EoBlY" {
		t.Errorf("id = %q, want the part before the query", got.ID)
	}
}

func TestAnEmbedThatIsNotOneIsRefused(t *testing.T) {
	cases := map[string]string{
		"not an iframe":      `<script src="https://www.youtube.com/embed/abc"></script>`,
		"not youtube":        `<iframe src="https://evil.test/embed/abc"></iframe>`,
		"trailing markup":    `<iframe src="https://www.youtube.com/embed/abc"></iframe><script>x()</script>`,
		"an id far too long": `<iframe src="https://www.youtube.com/embed/` + strings.Repeat("a", 130) + `"></iframe>`,
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := YouTubeEmbedID(value); ok {
				t.Error("accepted something that is not a YouTube embed")
			}
		})
	}
}

/*
THE DEFECT THIS PORT DOES NOT REPRODUCE.

parseVideoId read ?v= out of the query before it looked at the host, and returned it if
present. Pasting any URL that happened to carry a `v` parameter produced a YouTube element
whose id was whatever that parameter said — which then failed its metadata lookup, so the
author was told their URL was broken rather than that it had been misread.
*/
func TestAForeignURLWithAVParameterIsNotAYouTubeVideo(t *testing.T) {
	got := Classify(context.Background(), "https://cdn.example.test/clip.mp4?v=7", nil)

	if got.Source == SourceYouTube {
		t.Fatalf("source = %q with id %q; a v parameter on someone else's host is not a video id",
			got.Source, got.ID)
	}
}

// The network is a last resort: a batch may carry a hundred URLs, and a probe each would
// be a hundred round trips before a single element is written.
func TestThePatternsSettleEverythingWithoutTheNetwork(t *testing.T) {
	prober := &staticProber{contentType: "image/png"}

	for _, value := range []string{
		"https://www.youtube.com/watch?v=kSNYTZXj_G8",
		"https://m.bilibili.com/video/BV117421K7tp",
		"https://www.twitch.tv/videos/1618357349",
		"https://example.test/photo.jpg",
	} {
		Classify(context.Background(), value, prober)
	}

	if len(prober.calls) != 0 {
		t.Errorf("the prober was called for %v", prober.calls)
	}
}

// A URL with no extension and no recognisable host is what the probe is for: 2,205
// production elements are direct video files, and the discordapp CDN links among them
// carry no extension at all.
func TestAnUnrecognisedURLIsProbed(t *testing.T) {
	const link = "https://cdn.discordapp.test/attachments/1185143733524627487/1355948632914001980"

	video := Classify(context.Background(), link, &staticProber{contentType: "video/mp4"})
	if video.Source != SourceVideoFile {
		t.Errorf("source = %q, want %q", video.Source, SourceVideoFile)
	}

	image := Classify(context.Background(), link, &staticProber{contentType: "image/webp"})
	if image.Source != SourceImage {
		t.Errorf("source = %q, want %q", image.Source, SourceImage)
	}

	unknown := Classify(context.Background(), link, &staticProber{contentType: "text/html"})
	if unknown.Source != SourceUnknown {
		t.Errorf("source = %q, want it unrecognised", unknown.Source)
	}
}

func TestWithoutAProberAnUnrecognisedURLStaysUnknown(t *testing.T) {
	got := Classify(context.Background(), "https://example.test/mystery", nil)

	if got.Source != SourceUnknown {
		t.Errorf("source = %q, want it unrecognised", got.Source)
	}
}

func TestClassifyRefusesRubbish(t *testing.T) {
	for _, value := range []string{"", "   ", "not a url", "javascript:alert(1)"} {
		if got := Classify(context.Background(), value, nil); got.Source != SourceUnknown {
			t.Errorf("%q classified as %q", value, got.Source)
		}
	}
}

func TestSplitTitledURLsSplitsOnCommasAndTheFirstSpace(t *testing.T) {
	got := SplitTitledURLs(
		" https://a.test/1.jpg  ,https://b.test/2.jpg a title with spaces , , https://a.test/1.jpg ")

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2 — the blank and the repeat drop out: %+v", len(got), got)
	}
	if got[0].URL != "https://a.test/1.jpg" || got[0].Title != "" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].URL != "https://b.test/2.jpg" || got[1].Title != "a title with spaces" {
		t.Errorf("second = %+v", got[1])
	}
}
