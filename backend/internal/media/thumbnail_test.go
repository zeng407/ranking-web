package media

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type fakeElements struct {
	element  *Element
	findErr  error
	setCalls []setCall
	setErr   error
	missing  []Element
	deleted  []Element
	cleared  []int64
	clearErr error
}

type setCall struct {
	elementID int64
	column    string
	url       string
}

func (repository *fakeElements) FindElement(context.Context, int64) (*Element, error) {
	if repository.findErr != nil {
		return nil, repository.findErr
	}
	return repository.element, nil
}

func (repository *fakeElements) SetThumbnailURL(_ context.Context, elementID int64, column, url string) error {
	if repository.setErr != nil {
		return repository.setErr
	}
	repository.setCalls = append(repository.setCalls, setCall{elementID, column, url})
	return nil
}

func (repository *fakeElements) ElementsMissingThumbnail(context.Context, string, int) ([]Element, error) {
	return repository.missing, nil
}

func (repository *fakeElements) DeletedElementsWithFiles(context.Context, int) ([]Element, error) {
	return repository.deleted, nil
}

func (repository *fakeElements) ClearElementPath(_ context.Context, elementID int64) error {
	if repository.clearErr != nil {
		return repository.clearErr
	}
	repository.cleared = append(repository.cleared, elementID)
	return nil
}

type fakeStore struct {
	put     map[string][]byte
	deleted []string
	present map[string]bool
	putErr  error
	base    string
}

func newFakeStore() *fakeStore {
	return &fakeStore{put: map[string][]byte{}, present: map[string]bool{}, base: "https://file.example.com"}
}

func (store *fakeStore) Put(_ context.Context, key string, body []byte, _ string) (string, error) {
	if store.putErr != nil {
		return "", store.putErr
	}
	store.put[key] = body
	return store.base + "/" + key, nil
}

func (store *fakeStore) Delete(_ context.Context, key string) error {
	store.deleted = append(store.deleted, key)
	delete(store.present, key)
	return nil
}

func (store *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	return store.present[key], nil
}

func (store *fakeStore) URLToKey(rawURL string) string {
	base := store.base + "/"
	if !strings.HasPrefix(rawURL, base) {
		return ""
	}
	return strings.TrimPrefix(rawURL, base)
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func stringPointer(value string) *string { return &value }

func newService(t *testing.T, elements ElementRepository, store ObjectStore) *ThumbnailService {
	t.Helper()
	transcoder, err := NewTranscoder()
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	service, err := NewThumbnailService(ServiceOptions{
		Elements: elements, Store: store, Transcoder: transcoder, Logger: quietLogger(),
	})
	if err != nil {
		t.Fatalf("NewThumbnailService() error = %v", err)
	}
	return service
}

func TestNewThumbnailServiceRequiresDependencies(t *testing.T) {
	transcoder, err := NewTranscoder()
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	for name, options := range map[string]ServiceOptions{
		"no elements":   {Store: newFakeStore(), Transcoder: transcoder},
		"no store":      {Elements: &fakeElements{}, Transcoder: transcoder},
		"no transcoder": {Elements: &fakeElements{}, Store: newFakeStore()},
	} {
		if _, err := NewThumbnailService(options); err == nil {
			t.Errorf("NewThumbnailService() should reject the %s case", name)
		}
	}
}

// An element deleted between dispatch and execution is a no-op, not a failure.
func TestMakeImageThumbnailIgnoresMissingElement(t *testing.T) {
	elements := &fakeElements{element: nil}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 1, LowThumbnailSpec()); err != nil {
		t.Fatalf("MakeImageThumbnail() error = %v", err)
	}
	if len(elements.setCalls) != 0 {
		t.Fatalf("wrote %#v, want nothing", elements.setCalls)
	}
}

// Already generated: the column is set and differs from thumb_url.
func TestMakeImageThumbnailSkipsWhenAlreadyGenerated(t *testing.T) {
	elements := &fakeElements{element: &Element{
		ID: 1, Type: "image",
		ThumbURL:    stringPointer("https://example.com/original.png"),
		LowThumbURL: stringPointer("https://file.example.com/low/400x225/abc.webp"),
	}}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 1, LowThumbnailSpec()); err != nil {
		t.Fatalf("MakeImageThumbnail() error = %v", err)
	}
	if len(elements.setCalls) != 0 {
		t.Fatalf("wrote %#v, want nothing", elements.setCalls)
	}
}

// A column equal to thumb_url means an earlier fallback wrote it, so the element
// must be retried rather than treated as done. This is why the original compares
// the two instead of only checking for null.
func TestMakeImageThumbnailRetriesWhenColumnEqualsThumbURL(t *testing.T) {
	elements := &fakeElements{element: &Element{
		ID: 1, Type: "image",
		ThumbURL:    stringPointer("http://127.0.0.1/blocked.png"),
		LowThumbURL: stringPointer("http://127.0.0.1/blocked.png"),
		SourceURL:   stringPointer("https://example.com/source.png"),
	}}
	service := newService(t, elements, newFakeStore())

	// The fetch is refused by the SSRF guard, so it falls back to source_url,
	// proving it did not take the "already generated" path.
	if err := service.MakeImageThumbnail(context.Background(), 1, LowThumbnailSpec()); err != nil {
		t.Fatalf("MakeImageThumbnail() error = %v", err)
	}
	if len(elements.setCalls) != 1 {
		t.Fatalf("setCalls = %#v, want one fallback write", elements.setCalls)
	}
	if elements.setCalls[0].url != "https://example.com/source.png" {
		t.Fatalf("wrote %q, want the source_url", elements.setCalls[0].url)
	}
}

func TestMakeImageThumbnailRejectsNonImage(t *testing.T) {
	elements := &fakeElements{element: &Element{ID: 1, Type: "video"}}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 1, LowThumbnailSpec()); err == nil {
		t.Fatal("MakeImageThumbnail() should reject a video element")
	}
}

// An unfetchable source falls back to source_url and reports success: the element
// now has a usable URL, and retrying would repeat a fetch that already failed.
func TestMakeImageThumbnailFallsBackWhenFetchIsRefused(t *testing.T) {
	elements := &fakeElements{element: &Element{
		ID: 7, Type: "image",
		ThumbURL:  stringPointer("http://169.254.169.254/latest/meta-data/"),
		SourceURL: stringPointer("https://example.com/source.png"),
	}}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 7, LowThumbnailSpec()); err != nil {
		t.Fatalf("MakeImageThumbnail() error = %v", err)
	}
	if len(elements.setCalls) != 1 || elements.setCalls[0].column != LowThumbColumn {
		t.Fatalf("setCalls = %#v", elements.setCalls)
	}
	if elements.setCalls[0].url != "https://example.com/source.png" {
		t.Fatalf("wrote %q, want the source_url", elements.setCalls[0].url)
	}
}

// With no source_url there is nothing to fall back to, so the failure surfaces.
func TestMakeImageThumbnailFailsWithoutFallback(t *testing.T) {
	elements := &fakeElements{element: &Element{
		ID: 7, Type: "image",
		ThumbURL: stringPointer("http://127.0.0.1/blocked.png"),
	}}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 7, LowThumbnailSpec()); err == nil {
		t.Fatal("MakeImageThumbnail() should fail when there is no source_url")
	}
}

func TestMakeImageThumbnailRequiresThumbURL(t *testing.T) {
	elements := &fakeElements{element: &Element{ID: 7, Type: "image"}}
	service := newService(t, elements, newFakeStore())

	if err := service.MakeImageThumbnail(context.Background(), 7, LowThumbnailSpec()); err == nil {
		t.Fatal("MakeImageThumbnail() should fail with no thumb_url")
	}
}

func TestSpecForColumn(t *testing.T) {
	low, err := SpecForColumn(LowThumbColumn)
	if err != nil || low.MaxWidth != 400 || low.Prefix != "low" {
		t.Fatalf("low spec = %#v, err = %v", low, err)
	}
	medium, err := SpecForColumn(MediumThumbColumn)
	if err != nil || medium.MaxWidth != 800 || medium.Prefix != "medium" {
		t.Fatalf("medium spec = %#v, err = %v", medium, err)
	}
	if _, err := SpecForColumn("thumb_url"); err == nil {
		t.Fatal("SpecForColumn() should reject a column the schedule does not generate")
	}
}

// --- video ---

func TestMakeVideoThumbnailSkipsNonURLSources(t *testing.T) {
	for _, element := range []*Element{
		{ID: 1, Type: "image"},
		{ID: 1, Type: "video", VideoSource: stringPointer("youtube")},
		{ID: 1, Type: "video"},
	} {
		elements := &fakeElements{element: element}
		service := newService(t, elements, newFakeStore())
		if err := service.MakeVideoThumbnail(context.Background(), 1); err != nil {
			t.Fatalf("MakeVideoThumbnail() error = %v", err)
		}
		if len(elements.setCalls) != 0 {
			t.Fatalf("element %#v produced writes %#v", element, elements.setCalls)
		}
	}
}

// The source choice looks backwards but is deliberate: before a thumbnail exists,
// thumb_url holds the uploaded video.
func TestVideoSourceSelection(t *testing.T) {
	service := newService(t, &fakeElements{}, newFakeStore())

	cases := []struct {
		name     string
		element  *Element
		expected string
	}{
		{
			"thumb_url is a video file",
			&Element{ThumbURL: stringPointer("https://f.example.com/a.mp4"), SourceURL: stringPointer("https://s.example.com/b.mp4")},
			"https://f.example.com/a.mp4",
		},
		{
			"thumb_url is an image so source_url wins",
			&Element{ThumbURL: stringPointer("https://f.example.com/a.jpg"), SourceURL: stringPointer("https://s.example.com/b.mp4")},
			"https://s.example.com/b.mp4",
		},
		{
			"no thumb_url",
			&Element{SourceURL: stringPointer("https://s.example.com/b.webm")},
			"https://s.example.com/b.webm",
		},
		{
			// The original uses pathinfo, which keeps the query string in the
			// extension and would miss this.
			"query string is ignored",
			&Element{ThumbURL: stringPointer("https://f.example.com/a.mp4?token=x"), SourceURL: stringPointer("https://s.example.com/b.mp4")},
			"https://f.example.com/a.mp4?token=x",
		},
	}
	for _, test := range cases {
		if got := service.videoSourceFor(test.element); got != test.expected {
			t.Errorf("%s: videoSourceFor() = %q, want %q", test.name, got, test.expected)
		}
	}
}

func TestIsVideoFileURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://e.com/a.mp4", "https://e.com/a.webm", "https://e.com/a.ogg",
		"https://e.com/a.MP4", "https://e.com/a.mp4?x=1", "https://e.com/a.mp4#t=1",
	} {
		if !isVideoFileURL(rawURL) {
			t.Errorf("isVideoFileURL(%q) = false, want true", rawURL)
		}
	}
	for _, rawURL := range []string{
		"https://e.com/a.jpg", "https://e.com/a", "https://e.com/a.mov", "",
	} {
		if isVideoFileURL(rawURL) {
			t.Errorf("isVideoFileURL(%q) = true, want false", rawURL)
		}
	}
}

// --- cleanup ---

func TestRemoveDeletedElementFilesDeletesOwnObjectsOnly(t *testing.T) {
	store := newFakeStore()
	store.present["uploads/original.png"] = true
	store.present["low/400x225/low.webp"] = true

	elements := &fakeElements{deleted: []Element{{
		ID:   3,
		Path: stringPointer("uploads/original.png"),
		// Belongs to the store.
		LowThumbURL: stringPointer("https://file.example.com/low/400x225/low.webp"),
		// An external fallback URL: not ours, must not be touched.
		ThumbURL: stringPointer("https://someone-else.example.net/image.png"),
	}}}
	service := newService(t, elements, store)

	cleaned, err := service.RemoveDeletedElementFiles(context.Background(), 10)
	if err != nil {
		t.Fatalf("RemoveDeletedElementFiles() error = %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
	if len(store.deleted) != 2 {
		t.Fatalf("deleted = %#v, want the path and the low thumb", store.deleted)
	}
	for _, key := range store.deleted {
		if strings.Contains(key, "someone-else") {
			t.Fatalf("deleted a foreign URL: %q", key)
		}
	}
	// Clearing path is what stops the element being reconsidered next run.
	if len(elements.cleared) != 1 || elements.cleared[0] != 3 {
		t.Fatalf("cleared = %#v, want element 3", elements.cleared)
	}
}

func TestRemoveDeletedElementFilesSkipsAbsentObjects(t *testing.T) {
	store := newFakeStore()
	elements := &fakeElements{deleted: []Element{{
		ID: 3, Path: stringPointer("uploads/gone.png"),
	}}}
	service := newService(t, elements, store)

	cleaned, err := service.RemoveDeletedElementFiles(context.Background(), 10)
	if err != nil {
		t.Fatalf("RemoveDeletedElementFiles() error = %v", err)
	}
	if cleaned != 1 || len(store.deleted) != 0 {
		t.Fatalf("cleaned = %d, deleted = %#v", cleaned, store.deleted)
	}
	if len(elements.cleared) != 1 {
		t.Fatal("path must still be cleared when there was nothing to delete")
	}
}

func TestRemoveDeletedElementFilesRequiresPositiveLimit(t *testing.T) {
	service := newService(t, &fakeElements{}, newFakeStore())

	if _, err := service.RemoveDeletedElementFiles(context.Background(), 0); err == nil {
		t.Fatal("RemoveDeletedElementFiles() should reject a zero limit")
	}
}

func TestRemoveDeletedElementFilesStopsOnClearFailure(t *testing.T) {
	store := newFakeStore()
	elements := &fakeElements{
		deleted:  []Element{{ID: 3, Path: stringPointer("uploads/a.png")}},
		clearErr: errors.New("deadlock"),
	}
	service := newService(t, elements, store)

	if _, err := service.RemoveDeletedElementFiles(context.Background(), 10); err == nil {
		t.Fatal("a failure to clear path must be reported")
	}
}

func TestPendingThumbnailsRequiresPositiveLimit(t *testing.T) {
	service := newService(t, &fakeElements{}, newFakeStore())

	if _, err := service.PendingThumbnails(context.Background(), LowThumbnailSpec(), 0); err == nil {
		t.Fatal("PendingThumbnails() should reject a zero limit")
	}
}

func TestThumbnailKeyLayout(t *testing.T) {
	key := ThumbnailKey("low", 400, 225, "webp")
	if !strings.HasPrefix(key, "low/400x225/") || !strings.HasSuffix(key, ".webp") {
		t.Fatalf("ThumbnailKey() = %q, want low/400x225/<uuid>.webp", key)
	}
	// A fresh name each call, so a regenerated thumbnail never overwrites the old
	// object while a CDN might still be serving it.
	if key == ThumbnailKey("low", 400, 225, "webp") {
		t.Fatal("ThumbnailKey() must not repeat")
	}
	if got := VideoThumbnailKey(); !strings.HasPrefix(got, "video-thumbnails/") || !strings.HasSuffix(got, ".jpg") {
		t.Fatalf("VideoThumbnailKey() = %q", got)
	}
}
