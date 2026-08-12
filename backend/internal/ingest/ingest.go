package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Adding media to a post, from Api\ElementController's createMedia and the handlers
// behind ElementService.
//
// Separate from internal/authoring, which reads and edits what is already there. This one
// fetches, sniffs, stores and writes new rows — a different set of dependencies (an object
// store, a queue, the network) and a different set of failure modes.

// Limits from config/setting.php.
const (
	// MaxFileBytes is upload_media_file_size_mb.
	MaxFileBytes = 4 * 1024 * 1024
	// MaxElements is post_max_element_count, the cap on a post's media.
	MaxElements = 1024
	// MaxTitleRunes is element_title_size.
	MaxTitleRunes = 100
	// RateLimitWindow, RateLimitBytes and RateLimitFiles are
	// upload_media_size_mb_at_a_time and upload_media_file_count_at_a_time: 30 MiB or
	// 50 files a minute, per account.
	RateLimitWindow = time.Minute
	RateLimitBytes  = 30 * 1024 * 1024
	RateLimitFiles  = 50
)

// Element types and video sources, matching the columns.
const (
	TypeImage = "image"
	TypeVideo = "video"
)

// Validation codes.
const (
	CodeRequired         = "required"
	CodeTooLarge         = "too_large"
	CodeUnsupportedMedia = "unsupported_media"
	CodePostFull         = "post_full"
	CodeRateLimited      = "rate_limited"
)

// ErrInvalid carries the per-field reason an upload was refused.
type ErrInvalid struct {
	Fields map[string][]string
}

func (err *ErrInvalid) Error() string {
	return fmt.Sprintf("ingest: rejected: %v", err.Fields)
}

func invalid(field, code string) error {
	return &ErrInvalid{Fields: map[string][]string{field: {code}}}
}

// ErrPostNotFound means no post with that serial belongs to the caller. The same error
// for "does not exist" and "belongs to someone else", as everywhere else in the editor.
var ErrPostNotFound = errors.New("ingest: post not found")

// ErrNotConfigured means the process has no object store to put media in.
var ErrNotConfigured = errors.New("ingest: not configured")

// ErrElementNotFound is also what a caller who does not own the element gets: the same
// answer for "no such element" and "not yours", so the reply reveals no ids.
var ErrElementNotFound = errors.New("ingest: element not found")

// NewElement is one row about to be written.
type NewElement struct {
	PostID int64
	// Path is the object key, for media this site stores. Empty for media that stays
	// where it is, which is every remote video.
	Path      string
	SourceURL string
	ThumbURL  string
	Title     string
	Type      string
	// VideoSource and VideoID identify a remote video: youtube, youtube_embed,
	// bilibili_video, twitch_video, twitch_clip, or url for a plain video file.
	VideoSource  string
	VideoID      string
	DurationSecs *int
	StartSecond  *int
	EndSecond    *int
}

// Stored is what an ingested element looks like to the caller.
type Stored struct {
	ID        int64
	SourceURL string
	ThumbURL  string
	Title     string
	Type      string
}

// Store is the persistence this package needs.
type Store interface {
	// PostForOwner resolves a serial the user owns to its id, with the number of
	// elements already on it. ErrPostNotFound when it is not theirs.
	PostForOwner(ctx context.Context, userID int64, serial string) (postID int64, elements int, err error)
	// CreateElement writes the row and attaches it to the post, in one transaction.
	CreateElement(ctx context.Context, element NewElement) (Stored, error)
	// ElementForOwner resolves an element the user owns through one of their posts and
	// answers with that post's serial (the object key directory) and the element's
	// current title, which replacing the media keeps. ErrElementNotFound when it is not
	// theirs.
	ElementForOwner(ctx context.Context, userID, elementID int64) (serial, title string, err error)
	// ReplaceElementMedia overwrites one element's media columns in place.
	ReplaceElementMedia(ctx context.Context, elementID int64, media ReplacementMedia) error
}

// ReplacementMedia is the new file behind an element that already exists. Everything a
// medium implies is in here so the store can write these columns and clear the others: an
// element that was a YouTube video and is now an uploaded image must keep no trace of the
// video it was.
type ReplacementMedia struct {
	Path      string
	SourceURL string
	ThumbURL  string
	Type      string

	// VideoSource is empty for an image, which is how the video columns get cleared.
	VideoSource string
}

// ObjectStore writes the media this site keeps a copy of.
//
// Declared here rather than imported from internal/media so this package does not depend
// on the thumbnail service; media.S3Store satisfies it as it stands.
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) (string, error)
}

// Thumbnailer queues the work a new element needs.
type Thumbnailer interface {
	// VideoThumbnail is what App\Listeners\MakeVideoThumbnail did on VideoElementCreated.
	VideoThumbnail(ctx context.Context, elementID int64) error
}

// RateLimiter is the per-account upload budget.
type RateLimiter interface {
	// Allow records an upload of size bytes and reports whether it was within the
	// budget. A refusal must not consume the budget.
	Allow(ctx context.Context, userID int64, size int) (bool, error)
}

// Service ingests media into a post.
type Service struct {
	store   Store
	objects ObjectStore
	thumbs  Thumbnailer
	limiter RateLimiter
	keyName func(directory, extension string) string
	// The URL side. Each may be nil, and a source whose dependency is missing reports
	// itself unavailable rather than taking the whole batch down.
	prober  Prober
	youtube YouTubeLookup
	pages   PageLookup
	fetcher Fetcher
}

type ServiceOptions struct {
	Store   Store
	Objects ObjectStore
	// Thumbs is optional: without it a video element is written and the daily sweep
	// makes its thumbnail later, which is better than refusing the upload.
	Thumbs Thumbnailer
	// Limiter is optional. Without it the per-minute budget is not enforced, which is
	// the state of a deployment with no Redis.
	Limiter RateLimiter
	// KeyName builds the object key. Defaults to the layout Laravel wrote,
	// "{post serial}/{uuid}.{ext}".
	KeyName func(directory, extension string) string
	// Prober decides what an unrecognised URL serves. Without it such a URL is simply
	// unrecognised, which is a narrower answer rather than a wrong one.
	Prober Prober
	// YouTube reads video metadata. Nil without an API key.
	YouTube YouTubeLookup
	// Pages reads a page's title and preview, for Bilibili.
	Pages PageLookup
	// Fetcher downloads remote images. It must refuse private addresses.
	Fetcher Fetcher
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Store == nil {
		return nil, errors.New("ingest: store is required")
	}
	if options.Objects == nil {
		return nil, errors.New("ingest: object store is required")
	}
	keyName := options.KeyName
	if keyName == nil {
		keyName = DefaultKey
	}
	return &Service{
		store:   options.Store,
		objects: options.Objects,
		thumbs:  options.Thumbs,
		limiter: options.Limiter,
		keyName: keyName,
		prober:  options.Prober,
		youtube: options.YouTube,
		pages:   options.Pages,
		fetcher: options.Fetcher,
	}, nil
}

// Upload stores one uploaded file and attaches it to the post.
//
// THE TYPE COMES FROM THE BYTES, NEVER FROM THE REQUEST. Laravel's `mimetypes` rule read
// the guessed type too, which was right; what this adds is that the extension written
// into the object key comes from the same sniff, so a file cannot be stored under a name
// that disagrees with what it is.
func (service *Service) Upload(
	ctx context.Context, userID int64, serial, fileName string, content []byte,
) (Stored, error) {
	if len(content) == 0 {
		return Stored{}, invalid("file", CodeRequired)
	}
	if len(content) > MaxFileBytes {
		return Stored{}, invalid("file", CodeTooLarge)
	}

	kind, ok := SniffUpload(content)
	if !ok {
		return Stored{}, invalid("file", CodeUnsupportedMedia)
	}

	postID, existing, err := service.store.PostForOwner(ctx, userID, serial)
	if err != nil {
		return Stored{}, err
	}
	if existing >= MaxElements {
		return Stored{}, invalid("file", CodePostFull)
	}

	// The budget is spent only once the upload is going to be attempted, so a request
	// refused for being too large or for the wrong type does not cost the author part of
	// their minute.
	if service.limiter != nil {
		allowed, err := service.limiter.Allow(ctx, userID, len(content))
		if err != nil {
			return Stored{}, err
		}
		if !allowed {
			return Stored{}, invalid("file", CodeRateLimited)
		}
	}

	key := service.keyName(serial, kind.Extension)
	url, err := service.objects.Put(ctx, key, content, kind.ContentType)
	if err != nil {
		return Stored{}, fmt.Errorf("ingest: store upload: %w", err)
	}

	element := NewElement{
		PostID:    postID,
		Path:      key,
		SourceURL: url,
		// The stored file is its own thumbnail until the thumbnail job replaces it,
		// which is what the uploaded-file handlers did.
		ThumbURL: url,
		Title:    TitleFromFileName(fileName),
		Type:     kind.Type,
	}
	if kind.Type == TypeVideo {
		element.VideoSource = VideoSourceFile
	}

	stored, err := service.store.CreateElement(ctx, element)
	if err != nil {
		return Stored{}, err
	}

	// Only videos: App\Listeners\MakeVideoThumbnail ran on VideoElementCreated, while
	// ImageElementCreated had no listeners at all — an uploaded image's thumbnails are
	// made by the make-thumbnails schedule, which is already ported and already running.
	if kind.Type == TypeVideo && service.thumbs != nil {
		if err := service.thumbs.VideoThumbnail(ctx, stored.ID); err != nil {
			// The element exists and is usable; the sweep will catch the thumbnail.
			return stored, nil
		}
	}
	return stored, nil
}

// ReplaceMedia swaps the file behind an element the caller owns, keeping its title, its
// place in every post it is on, and every vote it has already collected.
//
// One request, where Laravel took two: its POST .../upload stored the object, cached the
// path under a sha256 of itself for ten minutes and answered with that `path_id`, which a
// following PUT redeemed. The cache existed to let the old editor preview the new file
// before committing to it, which a SPA does from the File object with
// URL.createObjectURL and no server round trip at all. Dropping it removes the ten-minute
// window in which an abandoned preview left an object with no row pointing at it.
//
// The old object is left where it is, as Laravel left it: the same is true of deleting an
// element, and the storage sweep is what reclaims both.
func (service *Service) ReplaceMedia(
	ctx context.Context, userID, elementID int64, fileName string, content []byte,
) (Stored, error) {
	if len(content) == 0 {
		return Stored{}, invalid("file", CodeRequired)
	}
	if len(content) > MaxFileBytes {
		return Stored{}, invalid("file", CodeTooLarge)
	}

	// The type comes from the bytes here too, and the extension in the key comes from the
	// same sniff. See Upload.
	kind, ok := SniffUpload(content)
	if !ok {
		return Stored{}, invalid("file", CodeUnsupportedMedia)
	}

	serial, title, err := service.store.ElementForOwner(ctx, userID, elementID)
	if err != nil {
		return Stored{}, err
	}
	// No MaxElements check: a replacement adds no element to the post.

	if service.limiter != nil {
		allowed, err := service.limiter.Allow(ctx, userID, len(content))
		if err != nil {
			return Stored{}, err
		}
		if !allowed {
			return Stored{}, invalid("file", CodeRateLimited)
		}
	}

	key := service.keyName(serial, kind.Extension)
	url, err := service.objects.Put(ctx, key, content, kind.ContentType)
	if err != nil {
		return Stored{}, fmt.Errorf("ingest: store replacement: %w", err)
	}

	media := ReplacementMedia{
		Path:      key,
		SourceURL: url,
		// Its own thumbnail until the thumbnail job replaces it, as on a new upload.
		ThumbURL: url,
		Type:     kind.Type,
	}
	if kind.Type == TypeVideo {
		media.VideoSource = VideoSourceFile
	}
	if err := service.store.ReplaceElementMedia(ctx, elementID, media); err != nil {
		return Stored{}, err
	}

	stored := Stored{
		ID: elementID, SourceURL: url, ThumbURL: url, Title: title, Type: kind.Type,
	}
	if kind.Type == TypeVideo && service.thumbs != nil {
		if err := service.thumbs.VideoThumbnail(ctx, elementID); err != nil {
			// The element is usable; the sweep will catch the thumbnail.
			return stored, nil
		}
	}
	return stored, nil
}

// VideoSourceFile is what a directly uploaded or linked video file is recorded as. It is
// the `url` value in production, on 2,205 elements.
const VideoSourceFile = "url"

// TitleFromFileName is Laravel's parseTitle: the base name without its extension, capped,
// and with the line breaks a pasted name can carry stripped out.
func TitleFromFileName(fileName string) string {
	name := fileName
	if index := strings.LastIndexAny(name, "/\\"); index >= 0 {
		name = name[index+1:]
	}
	if index := strings.LastIndex(name, "."); index > 0 {
		name = name[:index]
	}
	name = strings.Map(func(letter rune) rune {
		switch letter {
		case '\n', '\r', '\t':
			return -1
		}
		return letter
	}, name)
	name = strings.TrimSpace(name)
	if name == "" {
		// What the file handlers used when there was nothing to read.
		return "untitled"
	}
	return TrimTitle(name)
}

// TrimTitle caps a title at the column's limit, counting runes.
func TrimTitle(title string) string {
	if utf8.RuneCountInString(title) <= MaxTitleRunes {
		return title
	}
	runes := []rune(title)
	return string(runes[:MaxTitleRunes])
}
