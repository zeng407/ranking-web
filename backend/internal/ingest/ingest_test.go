package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The upload path's rules, against in-memory stores.

type memoryStore struct {
	postID   int64
	elements int
	postErr  error

	created   []NewElement
	createErr error
	nextID    int64
}

func (store *memoryStore) PostForOwner(_ context.Context, _ int64, _ string) (int64, int, error) {
	if store.postErr != nil {
		return 0, 0, store.postErr
	}
	return store.postID, store.elements, nil
}

func (store *memoryStore) CreateElement(_ context.Context, element NewElement) (Stored, error) {
	if store.createErr != nil {
		return Stored{}, store.createErr
	}
	store.created = append(store.created, element)
	store.nextID++
	return Stored{
		ID: store.nextID, SourceURL: element.SourceURL, ThumbURL: element.ThumbURL,
		Title: element.Title, Type: element.Type,
	}, nil
}

type memoryObjects struct {
	keys   []string
	types  []string
	sizes  []int
	putErr error
}

func (objects *memoryObjects) Put(
	_ context.Context, key string, body []byte, contentType string,
) (string, error) {
	objects.keys = append(objects.keys, key)
	objects.types = append(objects.types, contentType)
	objects.sizes = append(objects.sizes, len(body))
	if objects.putErr != nil {
		return "", objects.putErr
	}
	return "https://file.2pick.test/" + key, nil
}

type memoryThumbs struct {
	queued []int64
	err    error
}

func (thumbs *memoryThumbs) VideoThumbnail(_ context.Context, elementID int64) error {
	thumbs.queued = append(thumbs.queued, elementID)
	return thumbs.err
}

type memoryLimiter struct {
	allow bool
	calls []int
	err   error
}

func (limiter *memoryLimiter) Allow(_ context.Context, _ int64, size int) (bool, error) {
	limiter.calls = append(limiter.calls, size)
	return limiter.allow, limiter.err
}

type harness struct {
	service *Service
	store   *memoryStore
	objects *memoryObjects
	thumbs  *memoryThumbs
	limiter *memoryLimiter
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := &memoryStore{postID: 42}
	objects := &memoryObjects{}
	thumbs := &memoryThumbs{}
	limiter := &memoryLimiter{allow: true}
	service, err := NewService(ServiceOptions{
		Store: store, Objects: objects, Thumbs: thumbs, Limiter: limiter,
		// A fixed key, so a test can assert what was written rather than a UUID.
		KeyName: func(directory, extension string) string {
			return directory + "/fixed." + extension
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &harness{service: service, store: store, objects: objects, thumbs: thumbs, limiter: limiter}
}

func codeFor(t *testing.T, err error, field string) string {
	t.Helper()
	var refused *ErrInvalid
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want an ErrInvalid", err)
	}
	codes := refused.Fields[field]
	if len(codes) != 1 {
		t.Fatalf("fields = %v, want one code for %q", refused.Fields, field)
	}
	return codes[0]
}

func png(size int) []byte {
	image := []byte("\x89PNG\r\n\x1a\n")
	if size <= len(image) {
		return image
	}
	return append(image, make([]byte, size-len(image))...)
}

func mp4() []byte {
	return append([]byte("\x00\x00\x00\x18ftypmp42"), make([]byte, 32)...)
}

func TestUploadStoresTheFileAndWritesTheElement(t *testing.T) {
	harness := newHarness(t)

	stored, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "holiday.PNG", png(64))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if stored.Type != TypeImage {
		t.Errorf("type = %q, want image", stored.Type)
	}
	if harness.objects.keys[0] != "abcdefgh/fixed.png" {
		t.Errorf("key = %q, want it under the post's serial", harness.objects.keys[0])
	}
	if harness.objects.types[0] != "image/png" {
		t.Errorf("content type = %q", harness.objects.types[0])
	}
	written := harness.store.created[0]
	if written.PostID != 42 {
		t.Errorf("post = %d, want 42", written.PostID)
	}
	if written.Path != "abcdefgh/fixed.png" {
		t.Errorf("path = %q", written.Path)
	}
	// The stored file is its own thumbnail until the sweep replaces it, which is what
	// the uploaded-file handlers did.
	if written.ThumbURL != written.SourceURL {
		t.Errorf("thumb = %q, source = %q", written.ThumbURL, written.SourceURL)
	}
	if written.Title != "holiday" {
		t.Errorf("title = %q, want the file name without its extension", written.Title)
	}
}

/*
THE EXTENSION COMES FROM THE BYTES, NOT THE NAME.

An upload called holiday.png that is really a video would otherwise be stored under a .png
key and served with an image content type — and the browser would refuse to play it while
the thumbnail job tried to read it as a picture.
*/
func TestTheStoredExtensionFollowsTheContentNotTheFileName(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.Upload(
		context.Background(), 7, "abcdefgh", "pretending.png", mp4()); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if harness.objects.keys[0] != "abcdefgh/fixed.mp4" {
		t.Errorf("key = %q, want the sniffed extension", harness.objects.keys[0])
	}
	if harness.objects.types[0] != "video/mp4" {
		t.Errorf("content type = %q, want video/mp4", harness.objects.types[0])
	}
	if harness.store.created[0].Type != TypeVideo {
		t.Errorf("type = %q, want video", harness.store.created[0].Type)
	}
}

// MakeVideoThumbnail ran on VideoElementCreated; ImageElementCreated had no listeners at
// all, and an uploaded image's thumbnails come from the make-thumbnails sweep instead.
func TestOnlyAVideoQueuesAThumbnail(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.png", png(32)); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if len(harness.thumbs.queued) != 0 {
		t.Errorf("an image queued %v", harness.thumbs.queued)
	}

	stored, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.mp4", mp4())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if len(harness.thumbs.queued) != 1 || harness.thumbs.queued[0] != stored.ID {
		t.Errorf("queued %v, want [%d]", harness.thumbs.queued, stored.ID)
	}
}

// The element is already written and usable by then; refusing the upload over a queue
// failure would throw away work the author has done.
func TestAFailedThumbnailQueueDoesNotFailTheUpload(t *testing.T) {
	harness := newHarness(t)
	harness.thumbs.err = errors.New("redis is down")

	if _, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.mp4", mp4()); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if len(harness.store.created) != 1 {
		t.Error("the element was not written")
	}
}

func TestUploadRefusesWhatItCannotServe(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		want    string
	}{
		{"empty", nil, CodeRequired},
		{"too large", png(MaxFileBytes + 1), CodeTooLarge},
		{"html", []byte("<html><body>hi</body></html>"), CodeUnsupportedMedia},
		{"svg, which browsers run script from", []byte(`<svg xmlns="http://www.w3.org/2000/svg">`), CodeUnsupportedMedia},
		{"php", []byte("<?php system($_GET['c']); ?>"), CodeUnsupportedMedia},
		{"a truncated png header", []byte("\x89PNG"), CodeUnsupportedMedia},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newHarness(t)

			_, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "x", testCase.content)
			if code := codeFor(t, err, "file"); code != testCase.want {
				t.Errorf("code = %q, want %q", code, testCase.want)
			}
			if len(harness.objects.keys) != 0 {
				t.Error("a refused upload reached the bucket")
			}
			if len(harness.store.created) != 0 {
				t.Error("a refused upload was written")
			}
		})
	}
}

func TestUploadAcceptsEveryFormatTheOriginalAccepted(t *testing.T) {
	cases := []struct {
		name        string
		content     []byte
		wantType    string
		contentType string
	}{
		{"png", png(32), TypeImage, "image/png"},
		{"jpeg", append([]byte("\xff\xd8\xff\xe0"), make([]byte, 20)...), TypeImage, "image/jpeg"},
		{"gif", []byte("GIF89a and the rest"), TypeImage, "image/gif"},
		{"bmp", append([]byte("BM"), make([]byte, 20)...), TypeImage, "image/bmp"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), TypeImage, "image/webp"},
		{"avi", []byte("RIFF\x00\x00\x00\x00AVI LIST"), TypeVideo, "video/x-msvideo"},
		{"mp4", mp4(), TypeVideo, "video/mp4"},
		{"mpeg", append([]byte("\x00\x00\x01\xba"), make([]byte, 20)...), TypeVideo, "video/mpeg"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newHarness(t)

			stored, err := harness.service.Upload(
				context.Background(), 7, "abcdefgh", "x", testCase.content)
			if err != nil {
				t.Fatalf("Upload() error = %v", err)
			}
			if stored.Type != testCase.wantType {
				t.Errorf("type = %q, want %q", stored.Type, testCase.wantType)
			}
			if harness.objects.types[0] != testCase.contentType {
				t.Errorf("content type = %q, want %q", harness.objects.types[0], testCase.contentType)
			}
		})
	}
}

func TestUploadRefusesAPostThatIsFull(t *testing.T) {
	harness := newHarness(t)
	harness.store.elements = MaxElements

	_, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.png", png(32))
	if code := codeFor(t, err, "file"); code != CodePostFull {
		t.Errorf("code = %q, want %q", code, CodePostFull)
	}
	if len(harness.objects.keys) != 0 {
		t.Error("the bucket was written to for a full post")
	}
}

func TestUploadingToSomeoneElsesPostIsNotFound(t *testing.T) {
	harness := newHarness(t)
	harness.store.postErr = ErrPostNotFound

	_, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.png", png(32))
	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("error = %v, want ErrPostNotFound", err)
	}
	if len(harness.objects.keys) != 0 {
		t.Error("a stranger's upload reached the bucket")
	}
}

func TestUploadRefusesOverTheRateLimit(t *testing.T) {
	harness := newHarness(t)
	harness.limiter.allow = false

	_, err := harness.service.Upload(context.Background(), 7, "abcdefgh", "a.png", png(32))
	if code := codeFor(t, err, "file"); code != CodeRateLimited {
		t.Errorf("code = %q, want %q", code, CodeRateLimited)
	}
	if len(harness.objects.keys) != 0 {
		t.Error("a rate-limited upload reached the bucket")
	}
}

/*
THE BUDGET IS SPENT ONLY ON AN UPLOAD THAT IS GOING TO BE ATTEMPTED.

Checking it first would let a run of oversized or wrong-typed files — a folder dragged in
by mistake, say — burn an author's whole minute on requests that stored nothing.
*/
func TestARefusedFileDoesNotSpendTheBudget(t *testing.T) {
	harness := newHarness(t)

	_, _ = harness.service.Upload(context.Background(), 7, "abcdefgh", "x", []byte("<?php ?>"))
	_, _ = harness.service.Upload(context.Background(), 7, "abcdefgh", "x", png(MaxFileBytes+1))

	if len(harness.limiter.calls) != 0 {
		t.Errorf("the limiter was charged %v for uploads that stored nothing", harness.limiter.calls)
	}
}

// The row is what makes the object reachable, so writing the object first and the row
// second is the order that can only leak storage — the other way round would leave an
// element pointing at nothing.
func TestNothingIsWrittenWhenTheBucketRefuses(t *testing.T) {
	harness := newHarness(t)
	harness.objects.putErr = errors.New("bucket unreachable")

	if _, err := harness.service.Upload(
		context.Background(), 7, "abcdefgh", "a.png", png(32)); err == nil {
		t.Fatal("Upload() returned no error although the bucket refused")
	}
	if len(harness.store.created) != 0 {
		t.Error("an element was written for an object that does not exist")
	}
}

func TestTitleFromFileName(t *testing.T) {
	cases := map[string]string{
		"holiday.png":             "holiday",
		"holiday":                 "holiday",
		"/tmp/upload/holiday.png": "holiday",
		`C:\photos\holiday.png`:   "holiday",
		"two.dots.in.it.png":      "two.dots.in.it",
		"with\na break.png":       "witha break",
		".hidden":                 ".hidden",
		"":                        "untitled",
		"   ":                     "untitled",
		strings.Repeat("あ", 150):  strings.Repeat("あ", MaxTitleRunes),
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			if got := TitleFromFileName(name); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	if _, err := NewService(ServiceOptions{Objects: &memoryObjects{}}); err == nil {
		t.Error("NewService() accepted a missing store")
	}
	if _, err := NewService(ServiceOptions{Store: &memoryStore{}}); err == nil {
		t.Error("NewService() accepted a missing object store")
	}
	// The queue and the limiter may be absent: a deployment without Redis still uploads.
	if _, err := NewService(ServiceOptions{
		Store: &memoryStore{}, Objects: &memoryObjects{}}); err != nil {
		t.Errorf("NewService() without a queue or a limiter: %v", err)
	}
}
