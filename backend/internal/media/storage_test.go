package media

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// The URL-to-key mapping is pure, so it is tested without an endpoint. It is the
// part that decides whether the cleanup job deletes an object, so an unanchored
// match here would delete files that are not ours.
func TestURLToKeyRequiresThePrefix(t *testing.T) {
	store := &S3Store{publicURL: "https://file.2pick.app"}

	cases := map[string]string{
		"https://file.2pick.app/low/400x225/abc.webp":     "low/400x225/abc.webp",
		"https://file.2pick.app/video-thumbnails/a.jpg":   "video-thumbnails/a.jpg",
		"https://file.2pick.app/low/400x225/abc.webp?v=2": "low/400x225/abc.webp",
		"https://file.2pick.app//leading-slash.webp":      "leading-slash.webp",
		// Not ours: an element whose thumbnail fell back to an external source.
		"https://someone-else.example.net/image.png": "",
		// Contains the base but does not start with it. The original's
		// str_replace would mangle this into a bogus key and could delete it.
		"https://evil.example.com/?u=https://file.2pick.app/low/a.webp": "",
		// A different host that merely shares a prefix string.
		"https://file.2pick.app.evil.com/low/a.webp": "",
		"":    "",
		"   ": "",
	}
	for rawURL, want := range cases {
		if got := store.URLToKey(rawURL); got != want {
			t.Errorf("URLToKey(%q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestNewS3StoreValidatesConfiguration(t *testing.T) {
	client := s3.New(s3.Options{Region: "us-east-1"})

	if _, err := NewS3Store(nil, S3Config{Bucket: "b", PublicURL: "https://u"}); err == nil {
		t.Error("a nil client must be rejected")
	}
	if _, err := NewS3Store(client, S3Config{PublicURL: "https://u"}); err == nil {
		t.Error("a missing bucket must be rejected")
	}
	if _, err := NewS3Store(client, S3Config{Bucket: "b"}); err == nil {
		t.Error("a missing public url must be rejected")
	}
}

// testStore connects to an S3-compatible endpoint only when one is configured. The
// release image builds with no endpoint available, so this must skip there.
func testStore(t *testing.T) *S3Store {
	t.Helper()
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT is not set; skipping object store integration test")
	}

	bucket := os.Getenv("S3_TEST_BUCKET")
	if bucket == "" {
		bucket = "media-test"
	}

	client := s3.New(s3.Options{
		Region:       envOr("S3_TEST_REGION", "us-east-1"),
		BaseEndpoint: aws.String(endpoint),
		// MinIO needs path-style addressing; a virtual-host style request would try
		// to resolve bucket.minio.
		UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(
			envOr("S3_TEST_ACCESS_KEY", "sail"),
			envOr("S3_TEST_SECRET_KEY", "password"),
			"",
		),
	})

	ctx := context.Background()
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		// Already existing is fine; anything else means the endpoint is unusable.
		if !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") &&
			!strings.Contains(err.Error(), "BucketAlreadyExists") {
			t.Fatalf("create bucket %q at %s: %v", bucket, endpoint, err)
		}
	}

	store, err := NewS3Store(client, S3Config{Bucket: bucket, PublicURL: "https://file.test.local"})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	return store
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func TestS3StorePutExistsDeleteRoundTrip(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	key := "low/400x225/" + "roundtrip-test.webp"
	t.Cleanup(func() { _ = store.Delete(context.Background(), key) })

	body := []byte("not really a webp, but bytes are bytes")
	url, err := store.Put(ctx, key, body, "image/webp")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if url != "https://file.test.local/"+key {
		t.Fatalf("Put() url = %q", url)
	}

	// The URL the database records must map back to the key that was written.
	if got := store.URLToKey(url); got != key {
		t.Fatalf("URLToKey(%q) = %q, want %q", url, got, key)
	}

	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatal("Exists() = false right after Put")
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() after delete error = %v", err)
	}
	if exists {
		t.Fatal("Exists() = true after Delete")
	}
}

// A missing key must read as absent, not as an error. The cleanup job checks every
// key before deleting, so an error here would abort the whole batch.
func TestS3StoreExistsReportsAbsentKeyWithoutError(t *testing.T) {
	store := testStore(t)

	exists, err := store.Exists(context.Background(), "low/400x225/definitely-not-there.webp")
	if err != nil {
		t.Fatalf("Exists() error = %v, want a clean false", err)
	}
	if exists {
		t.Fatal("Exists() = true for a key that was never written")
	}
}

// Deleting something that is not there is not an error either.
func TestS3StoreDeleteIsIdempotent(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if err := store.Delete(ctx, "low/400x225/never-existed.webp"); err != nil {
		t.Fatalf("Delete() of a missing key error = %v", err)
	}
	if err := store.Delete(ctx, ""); err != nil {
		t.Fatalf("Delete(\"\") error = %v", err)
	}
}

func TestS3StoreRejectsEmptyPut(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if _, err := store.Put(ctx, "low/a.webp", nil, "image/webp"); err == nil {
		t.Error("Put() with no body must be rejected")
	}
	if _, err := store.Put(ctx, "", []byte("x"), "image/webp"); err == nil {
		t.Error("Put() with no key must be rejected")
	}
}
