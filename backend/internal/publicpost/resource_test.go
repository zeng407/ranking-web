package publicpost

import (
	"encoding/json"
	"testing"
	"time"
)

func pointer[T any](value T) *T { return &value }

// noShuffle leaves the order alone, so a test can predict which elements are picked.
func noShuffle(int, func(i, j int)) {}

// reverseShuffle is a deterministic permutation, used to prove the selection actually
// depends on the shuffle rather than ignoring it.
func reverseShuffle(length int, swap func(i, j int)) {
	for index := 0; index < length/2; index++ {
		swap(index, length-1-index)
	}
}

func TestPreviewableAcceptsImagesAndKnownVideoSources(t *testing.T) {
	if !Previewable(ElementRow{Type: pointer(ElementTypeImage)}) {
		t.Error("an image element must be previewable")
	}
	for _, source := range []string{
		VideoSourceYouTube, VideoSourceYouTubeEmbed, VideoSourceBilibili,
		VideoSourceTwitchVideo, VideoSourceTwitchClip,
	} {
		if !Previewable(ElementRow{Type: pointer("video"), VideoSource: pointer(source)}) {
			t.Errorf("video source %q must be previewable", source)
		}
	}
	for _, source := range []string{"vimeo", "soundcloud", "unknown"} {
		if Previewable(ElementRow{Type: pointer("video"), VideoSource: pointer(source)}) {
			t.Errorf("video source %q must not be previewable", source)
		}
	}
	if Previewable(ElementRow{Type: pointer("video")}) {
		t.Error("a video with no source must not be previewable")
	}
}

// The subtle clause: a plain URL source is previewable only when its thumb looks like
// an image file, which is how a link to an image renders while a link to a page does
// not.
func TestPreviewableChecksTheThumbExtensionForPlainURLs(t *testing.T) {
	previewable := map[string]bool{
		"https://file.2pick.app/low/a.webp": true,
		"https://file.2pick.app/low/a.jpg":  true,
		"https://file.2pick.app/low/a.jpeg": true,
		"https://file.2pick.app/low/a.png":  true,
		"https://file.2pick.app/low/a.gif":  true,
		// Not an image extension.
		"https://example.com/page":   false,
		"https://example.com/a.mp4":  false,
		"https://example.com/a.webm": false,
		// The PHP pattern is anchored at the end and has no /i flag, so neither of
		// these matched there either. Copying the looseness matters more than fixing
		// it, because the value decides what the browser tries to render.
		"https://example.com/a.PNG":         false,
		"https://example.com/a.png?size=90": false,
	}
	for url, want := range previewable {
		got := Previewable(ElementRow{
			Type: pointer("video"), VideoSource: pointer(VideoSourceURL), ThumbURL: pointer(url),
		})
		if got != want {
			t.Errorf("thumb %q previewable = %v, want %v", url, got, want)
		}
	}
	if Previewable(ElementRow{Type: pointer("video"), VideoSource: pointer(VideoSourceURL)}) {
		t.Error("a plain URL source with no thumb must not be previewable")
	}
}

func TestThumbURLFallbacks(t *testing.T) {
	// getDefaultThumbUrl has no fallback.
	if url := DefaultThumbURL(ElementRow{MediumThumbURL: pointer("m.webp")}); url != nil {
		t.Errorf("DefaultThumbURL = %q, want nil when thumb_url is unset", *url)
	}
	if url := DefaultThumbURL(ElementRow{ThumbURL: pointer("t.webp")}); url == nil || *url != "t.webp" {
		t.Errorf("DefaultThumbURL = %v, want t.webp", url)
	}

	// getMediumThumbUrl falls back to the plain thumb.
	if url := MediumThumbURL(ElementRow{ThumbURL: pointer("t.webp")}); url == nil || *url != "t.webp" {
		t.Errorf("MediumThumbURL = %v, want the plain thumb", url)
	}
	if url := MediumThumbURL(ElementRow{
		ThumbURL: pointer("t.webp"), MediumThumbURL: pointer("m.webp"),
	}); url == nil || *url != "m.webp" {
		t.Errorf("MediumThumbURL = %v, want m.webp", url)
	}
	// PHP's ?: treats "" as falsy, so an empty medium thumb falls back too.
	if url := MediumThumbURL(ElementRow{
		ThumbURL: pointer("t.webp"), MediumThumbURL: pointer(""),
	}); url == nil || *url != "t.webp" {
		t.Errorf("MediumThumbURL with an empty medium = %v, want the plain thumb", url)
	}
}

// The placeholder shape matters: internal/publiccontent recognises it and drops the
// element, so a post with too few usable elements renders without a preview instead of
// with a broken one.
func TestBuildElementProducesTheNullPlaceholder(t *testing.T) {
	placeholder := BuildElement(nil)
	if placeholder.ID != nil || placeholder.URL != nil || placeholder.URL2 != nil ||
		placeholder.Title != nil || placeholder.Type != nil || placeholder.VideoSource != nil {
		t.Fatalf("placeholder should be all null, got %+v", placeholder)
	}
	if placeholder.Previewable {
		t.Error("the placeholder must not be previewable")
	}

	encoded, err := json.Marshal(placeholder)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Every key must still be present; a missing key is not the same as a null one to
	// a consumer reading fields off the object.
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"video_source", "type", "id", "url", "url2", "title", "previewable"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("the placeholder is missing %q", field)
		}
	}
}

// The ranked set is used only when it can supply both elements, never mixed with the
// fallback — `if ($ranks->count() >= 2)`.
func TestSelectPreviewPrefersRankedElementsOnlyWhenTwoExist(t *testing.T) {
	ranked := []ElementRow{{ID: 1}, {ID: 2}, {ID: 3}}
	all := []ElementRow{{ID: 90}, {ID: 91}, {ID: 92}}

	first, second := SelectPreviewElements(ranked, all, noShuffle)
	if first == nil || second == nil {
		t.Fatal("two ranked elements must yield two previews")
	}
	for _, element := range []*ElementRow{first, second} {
		if element.ID > 10 {
			t.Errorf("picked fallback element %d while ranked elements were available", element.ID)
		}
	}

	// One ranked element is not enough, and it must not be combined with a fallback.
	first, second = SelectPreviewElements(ranked[:1], all, noShuffle)
	if first == nil || second == nil {
		t.Fatal("the fallback must supply two previews")
	}
	for _, element := range []*ElementRow{first, second} {
		if element.ID < 10 {
			t.Errorf("mixed ranked element %d into the fallback selection", element.ID)
		}
	}
}

// The selection is random by design. This proves it actually consumes the shuffle
// rather than always taking the same two, which is what a port that dropped the
// shuffle would do.
func TestSelectPreviewUsesTheShuffle(t *testing.T) {
	ranked := []ElementRow{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}

	plainFirst, plainSecond := SelectPreviewElements(ranked, nil, noShuffle)
	reversedFirst, reversedSecond := SelectPreviewElements(ranked, nil, reverseShuffle)

	if plainFirst.ID == reversedFirst.ID && plainSecond.ID == reversedSecond.ID {
		t.Fatalf("the shuffle had no effect: both runs picked %d and %d",
			plainFirst.ID, plainSecond.ID)
	}
	// Whatever the order, the two must be distinct.
	if plainFirst.ID == plainSecond.ID || reversedFirst.ID == reversedSecond.ID {
		t.Fatal("the two previews must be different elements")
	}
}

func TestSelectPreviewDoesNotMutateItsInput(t *testing.T) {
	ranked := []ElementRow{{ID: 1}, {ID: 2}, {ID: 3}}
	SelectPreviewElements(ranked, nil, reverseShuffle)
	for index, want := range []int64{1, 2, 3} {
		if ranked[index].ID != want {
			t.Fatalf("input was reordered: %+v", ranked)
		}
	}
}

func TestSelectPreviewHandlesTooFewElements(t *testing.T) {
	// One element: one preview and one placeholder.
	first, second := SelectPreviewElements(nil, []ElementRow{{ID: 7}}, noShuffle)
	if first == nil || first.ID != 7 {
		t.Errorf("first = %v, want element 7", first)
	}
	if second != nil {
		t.Errorf("second = %v, want nil", second)
	}

	// None at all: two placeholders.
	first, second = SelectPreviewElements(nil, nil, noShuffle)
	if first != nil || second != nil {
		t.Errorf("got %v and %v, want two placeholders", first, second)
	}
}

func TestBuildResourceMatchesTheResourceShape(t *testing.T) {
	created := time.Date(2026, time.August, 5, 14, 3, 9, 123456789, time.UTC)
	updated := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	post := PostRow{
		ID: 42, Serial: "abc123", Title: "標題", Description: "說明",
		IsPrivate: true, IsCensored: 1, CreatedAt: created, UpdatedAt: updated,
	}
	element := ElementRow{
		ID: 9, Title: pointer("元素"), Type: pointer(ElementTypeImage),
		ThumbURL: pointer("t.webp"), MediumThumbURL: pointer("m.webp"),
	}

	resource := BuildResource(post, []string{"a", "b"}, 12, 345, &element, nil)

	if resource.Title != "標題" || resource.Serial != "abc123" || resource.Description != "說明" {
		t.Errorf("scalar fields wrong: %+v", resource)
	}
	if !resource.IsPrivate || resource.IsCensored != 1 {
		t.Error("is_private and is_censored must be carried through")
	}
	// Carbon's toDateTimeString(): no timezone, no sub-second part.
	if resource.CreatedAt != "2026-08-05 14:03:09" {
		t.Errorf("created_at = %q, want 2026-08-05 14:03:09", resource.CreatedAt)
	}
	if resource.UpdatedAt != "2026-08-06 09:00:00" {
		t.Errorf("updated_at = %q", resource.UpdatedAt)
	}
	if resource.PlayCount != 345 || resource.ElementsCount != 12 {
		t.Errorf("counts wrong: %+v", resource)
	}
	if resource.Element1.ID == nil || *resource.Element1.ID != 9 {
		t.Errorf("element1 = %+v", resource.Element1)
	}
	if resource.Element2.ID != nil {
		t.Errorf("element2 should be the placeholder, got %+v", resource.Element2)
	}
}

// A null tag list would break the frontend, which iterates the array.
func TestBuildResourceEncodesAnEmptyTagListAsAnArray(t *testing.T) {
	resource := BuildResource(PostRow{}, nil, 0, 0, nil, nil)
	encoded, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Tags == nil {
		t.Fatalf("tags decoded as null; payload was %s", encoded)
	}
	if len(decoded.Tags) != 0 {
		t.Errorf("tags = %v, want empty", decoded.Tags)
	}
}

// The payload has to survive the round trip through the reader that serves it, or the
// listing breaks in a way only the browser would show.
func TestResourceRoundTripsThroughTheStoredForm(t *testing.T) {
	element := ElementRow{
		ID: 5, Title: pointer("t"), Type: pointer("video"),
		VideoSource: pointer(VideoSourceYouTube), ThumbURL: pointer("t.webp"),
	}
	resource := BuildResource(
		PostRow{Serial: "s", Title: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		[]string{"標籤", "tag"}, 9, 100, &element, &element,
	)

	encoded, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Resource
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Serial != resource.Serial || back.PlayCount != resource.PlayCount {
		t.Errorf("round trip lost data: %+v", back)
	}
	if back.Element1.ID == nil || *back.Element1.ID != 5 || !back.Element1.Previewable {
		t.Errorf("element1 did not survive: %+v", back.Element1)
	}
	if len(back.Tags) != 2 || back.Tags[0] != "標籤" {
		t.Errorf("tags did not survive: %v", back.Tags)
	}
}

// The wire types are load-bearing and they are not uniform: is_censored comes straight
// from a tinyint column and encodes as a number, while is_private and previewable are
// PHP booleans. internal/publiccontent decodes is_censored into an int, so getting this
// wrong makes the entire listing fail to decode rather than showing one wrong flag.
func TestResourceWireTypesMatchTheStoredPayload(t *testing.T) {
	element := ElementRow{ID: 1, Type: pointer(ElementTypeImage)}
	resource := BuildResource(PostRow{IsCensored: 1, IsPrivate: true}, nil, 8, 3, &element, nil)

	encoded, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// A number, not true/false.
	if got := string(decoded["is_censored"]); got != "1" {
		t.Errorf("is_censored encoded as %s, want 1", got)
	}
	var censored int
	if err := json.Unmarshal(decoded["is_censored"], &censored); err != nil {
		t.Errorf("is_censored must decode into an int: %v", err)
	}

	// These two are booleans.
	if got := string(decoded["is_private"]); got != "true" {
		t.Errorf("is_private encoded as %s, want true", got)
	}
	var previewable struct {
		Previewable json.RawMessage `json:"previewable"`
	}
	if err := json.Unmarshal(decoded["element1"], &previewable); err != nil {
		t.Fatalf("decode element1: %v", err)
	}
	if got := string(previewable.Previewable); got != "true" {
		t.Errorf("previewable encoded as %s, want true", got)
	}

	// And the counts are numbers.
	for _, field := range []string{"play_count", "elements_count"} {
		var value int64
		if err := json.Unmarshal(decoded[field], &value); err != nil {
			t.Errorf("%s must decode into a number: %v", field, err)
		}
	}
}
