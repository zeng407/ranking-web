package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ObjectStore writes generated thumbnails and removes files for deleted elements.
type ObjectStore interface {
	// Put stores bytes at key and returns the public URL.
	Put(ctx context.Context, key string, body []byte, contentType string) (string, error)
	// Delete removes a key. A missing key is not an error.
	Delete(ctx context.Context, key string) error
	// Exists reports whether a key is present.
	Exists(ctx context.Context, key string) (bool, error)
	// URLToKey maps a stored URL back to its key, or "" when the URL does not
	// belong to this store.
	URLToKey(rawURL string) string
}

// S3Config configures the store.
type S3Config struct {
	Bucket string
	// PublicURL is the base the stored URL is built from, matching Laravel's
	// AWS_URL and what Storage::url() returns.
	PublicURL string
}

// S3Store implements ObjectStore against S3 or any S3-compatible endpoint.
//
// The bucket stays private: nothing here sets an ACL. Objects are served through
// the CDN in front of the bucket, which is why only the URL is written to the
// database.
type S3Store struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

// ClientConfig is the SDK plumbing needed to reach the endpoint.
//
// Its own type because two commands build the same client — the worker for thumbnails
// and the api for avatars — and the endpoint-and-credentials dance is exactly the kind
// of thing that drifts when it is written twice.
type ClientConfig struct {
	// Endpoint is empty for real S3 and set for MinIO.
	Endpoint string
	Region   string
	// AccessKey empty leaves credential discovery to the SDK, which is how it works
	// under an instance role.
	AccessKey string
	SecretKey string
	// UsePathStyle is required by MinIO; real S3 uses virtual-host addressing.
	UsePathStyle bool
}

// NewS3Client builds the SDK client.
func NewS3Client(configuration ClientConfig) *s3.Client {
	options := s3.Options{
		Region:       configuration.Region,
		UsePathStyle: configuration.UsePathStyle,
	}
	if configuration.Endpoint != "" {
		options.BaseEndpoint = aws.String(configuration.Endpoint)
	}
	if configuration.AccessKey != "" {
		options.Credentials = credentials.NewStaticCredentialsProvider(
			configuration.AccessKey, configuration.SecretKey, "")
	}
	return s3.New(options)
}

func NewS3Store(client *s3.Client, configuration S3Config) (*S3Store, error) {
	if client == nil {
		return nil, errors.New("media: s3 client is required")
	}
	if strings.TrimSpace(configuration.Bucket) == "" {
		return nil, errors.New("media: s3 bucket is required")
	}
	if strings.TrimSpace(configuration.PublicURL) == "" {
		return nil, errors.New("media: public url is required to build stored urls")
	}
	return &S3Store{
		client:    client,
		bucket:    configuration.Bucket,
		publicURL: strings.TrimRight(configuration.PublicURL, "/"),
	}, nil
}

func (store *S3Store) Put(ctx context.Context, key string, body []byte, contentType string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("media: object key is required")
	}
	if len(body) == 0 {
		return "", errors.New("media: refusing to store an empty object")
	}

	_, err := store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(store.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("media: put %q: %w", key, err)
	}
	return store.publicURL + "/" + key, nil
}

func (store *S3Store) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("media: delete %q: %w", key, err)
	}
	return nil
}

func (store *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, nil
	}
	_, err := store.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}

	var notFound *s3NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	// The SDK reports a missing key as NotFound or NoSuchKey depending on the
	// endpoint, and MinIO differs from S3 here, so the message is inspected as a
	// fallback rather than trusting one shape.
	message := err.Error()
	if strings.Contains(message, "NotFound") || strings.Contains(message, "NoSuchKey") ||
		strings.Contains(message, "status code: 404") {
		return false, nil
	}
	return false, fmt.Errorf("media: head %q: %w", key, err)
}

// s3NotFound exists only so errors.As has a concrete target; the SDK's own
// types.NotFound is matched by the message check above.
type s3NotFound struct{ error }

// URLToKey strips the public base from a stored URL.
//
// This replaces the original's str_replace(Storage::url(”), ”, $url). That
// replacement is unanchored, so a URL merely containing the base anywhere would be
// mangled; this requires a prefix and returns "" for anything else, which the
// caller treats as "not ours, leave it alone". Elements whose thumb_url fell back
// to an external source_url land in that case, and must not be deleted.
func (store *S3Store) URLToKey(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	base := store.publicURL + "/"
	if !strings.HasPrefix(trimmed, base) {
		return ""
	}
	key := strings.TrimPrefix(trimmed, base)
	// A stored URL can carry a query string from a CDN; the key must not.
	if parsed, err := url.Parse(key); err == nil {
		key = parsed.Path
	}
	return strings.TrimPrefix(key, "/")
}

// ThumbnailKey builds the object key for a generated thumbnail.
//
// The layout matches the original: "{prefix}/{width}x{height}/{uuid}.{ext}", where
// the name is a UUIDv4 from FileHelper::generateFileName and the extension comes
// from the encoded format.
func ThumbnailKey(prefix string, width, height int, extension string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
	name := uuid.NewString()
	if extension != "" {
		name += "." + extension
	}
	if prefix == "" {
		return fmt.Sprintf("%dx%d/%s", width, height, name)
	}
	return fmt.Sprintf("%s/%dx%d/%s", prefix, width, height, name)
}

// AvatarKey builds the object key for an uploaded avatar.
//
// The directory matches the 329 avatars already in the bucket, which Laravel wrote with
// store('avatars'). The name is a UUIDv4 rather than Laravel's 40-character hashName,
// which is the convention the rest of this package already writes.
func AvatarKey(extension string) string {
	extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
	name := uuid.NewString()
	if extension != "" {
		name += "." + extension
	}
	return "avatars/" + name
}

// VideoThumbnailKey builds the object key for a video frame, matching the
// original's 'video-thumbnails' directory.
func VideoThumbnailKey() string {
	return "video-thumbnails/" + uuid.NewString() + ".jpg"
}
