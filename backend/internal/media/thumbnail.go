package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
)

// Thumbnail sizes and path prefixes, transcribed from ThumbnailExecutor.
const (
	LowThumbMaxWidth   = 400
	LowThumbMaxHeight  = 400
	LowThumbPrefix     = "low"
	LowThumbColumn     = "lowthumb_url"
	MediumThumbMaxSide = 800
	MediumThumbPrefix  = "medium"
	MediumThumbColumn  = "mediumthumb_url"
)

// videoFileExtensions matches MakeVideoThumbnail::isVideoType.
var videoFileExtensions = map[string]struct{}{
	"mp4": {}, "webm": {}, "ogg": {},
}

// Element is the subset of the elements row the media jobs need.
type Element struct {
	ID             int64
	Type           string
	VideoSource    *string
	SourceURL      *string
	ThumbURL       *string
	LowThumbURL    *string
	MediumThumbURL *string
	Path           *string
}

// ElementRepository is the database side of the media jobs.
type ElementRepository interface {
	// FindElement returns one element, or nil when it does not exist.
	FindElement(ctx context.Context, elementID int64) (*Element, error)
	// SetThumbnailURL writes one of the thumbnail URL columns.
	SetThumbnailURL(ctx context.Context, elementID int64, column, url string) error
	// ElementsMissingThumbnail returns image elements whose column is null, newest
	// first, capped at limit.
	ElementsMissingThumbnail(ctx context.Context, column string, limit int) ([]Element, error)
	// DeletedElementsWithFiles returns soft-deleted elements that still have a
	// stored path, capped at limit.
	DeletedElementsWithFiles(ctx context.Context, limit int) ([]Element, error)
	// ClearElementPath sets path to null, marking the element as cleaned up.
	ClearElementPath(ctx context.Context, elementID int64) error
}

// ThumbnailService generates and cleans up media derivatives.
type ThumbnailService struct {
	elements   ElementRepository
	store      ObjectStore
	fetcher    *Fetcher
	transcoder *Transcoder
	logger     *slog.Logger
}

type ServiceOptions struct {
	Elements   ElementRepository
	Store      ObjectStore
	Fetcher    *Fetcher
	Transcoder *Transcoder
	Logger     *slog.Logger
}

func NewThumbnailService(options ServiceOptions) (*ThumbnailService, error) {
	if options.Elements == nil {
		return nil, errors.New("media: element repository is required")
	}
	if options.Store == nil {
		return nil, errors.New("media: object store is required")
	}
	if options.Transcoder == nil {
		return nil, errors.New("media: transcoder is required")
	}
	fetcher := options.Fetcher
	if fetcher == nil {
		fetcher = NewFetcher()
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &ThumbnailService{
		elements:   options.Elements,
		store:      options.Store,
		fetcher:    fetcher,
		transcoder: options.Transcoder,
		logger:     logger,
	}, nil
}

// ThumbnailSpec names one derivative.
type ThumbnailSpec struct {
	Column    string
	Prefix    string
	MaxWidth  int
	MaxHeight int
}

// LowThumbnailSpec and MediumThumbnailSpec are the two the schedule generates.
func LowThumbnailSpec() ThumbnailSpec {
	return ThumbnailSpec{Column: LowThumbColumn, Prefix: LowThumbPrefix, MaxWidth: LowThumbMaxWidth, MaxHeight: LowThumbMaxHeight}
}

func MediumThumbnailSpec() ThumbnailSpec {
	return ThumbnailSpec{Column: MediumThumbColumn, Prefix: MediumThumbPrefix, MaxWidth: MediumThumbMaxSide, MaxHeight: MediumThumbMaxSide}
}

// SpecForColumn resolves a spec from a column name.
func SpecForColumn(column string) (ThumbnailSpec, error) {
	switch column {
	case LowThumbColumn:
		return LowThumbnailSpec(), nil
	case MediumThumbColumn:
		return MediumThumbnailSpec(), nil
	default:
		return ThumbnailSpec{}, fmt.Errorf("media: unknown thumbnail column %q", column)
	}
}

// MakeImageThumbnail generates one derivative for one element.
//
// Merges ImageThumbnailService::makeThumbnail and ResizeElementImage::handle. The
// original splits them across a service and a queued job, which downloads the
// source twice: once to read its dimensions and once to resize it. This fetches
// once.
//
// The fallbacks are preserved. A source that serves nothing, or that cannot be
// decoded, results in the column being set to source_url rather than the job
// failing, because a broken derivative is worse than pointing at the original.
func (service *ThumbnailService) MakeImageThumbnail(ctx context.Context, elementID int64, spec ThumbnailSpec) error {
	element, err := service.elements.FindElement(ctx, elementID)
	if err != nil {
		return fmt.Errorf("media: load element %d: %w", elementID, err)
	}
	if element == nil {
		// Deleted between dispatch and execution. Nothing to do.
		return nil
	}
	if element.Type != "image" {
		return fmt.Errorf("media: element %d is type %q, not image", elementID, element.Type)
	}

	current := deref(columnValue(element, spec.Column))
	thumbURL := deref(element.ThumbURL)

	// Already generated. The original compares against thumb_url because the
	// fallback path writes the source into the column, and such a row must be
	// retried rather than treated as done.
	if current != "" && current != thumbURL {
		return nil
	}
	if thumbURL == "" {
		return fmt.Errorf("media: element %d has no thumb_url to derive from", elementID)
	}

	source, err := service.fetcher.Fetch(ctx, thumbURL)
	if err != nil {
		return service.fallBackToSource(ctx, element, spec, err)
	}

	dimensions, err := service.transcoder.ProbeImage(ctx, source)
	if err != nil {
		return service.fallBackToSource(ctx, element, spec, err)
	}

	width, height := FitBox(dimensions.Width, dimensions.Height, spec.MaxWidth, spec.MaxHeight)
	if width <= 0 || height <= 0 {
		return service.fallBackToSource(ctx, element, spec,
			fmt.Errorf("media: element %d probed as %dx%d", elementID, dimensions.Width, dimensions.Height))
	}

	encoded, err := service.transcoder.ResizeToWebP(ctx, source, width, height)
	if err != nil {
		return service.fallBackToSource(ctx, element, spec, err)
	}

	key := ThumbnailKey(spec.Prefix, width, height, "webp")
	storedURL, err := service.store.Put(ctx, key, encoded, "image/webp")
	if err != nil {
		// A storage failure is infrastructure, not bad input: it must retry rather
		// than degrade the row to source_url.
		return fmt.Errorf("media: store thumbnail for element %d: %w", elementID, err)
	}
	if err := service.elements.SetThumbnailURL(ctx, elementID, spec.Column, storedURL); err != nil {
		return fmt.Errorf("media: record thumbnail for element %d: %w", elementID, err)
	}

	service.logger.Info("media_thumbnail_generated",
		"element_id", elementID,
		"column", spec.Column,
		"source_size", dimensions.Width*dimensions.Height,
		"width", width, "height", height,
		"bytes", len(encoded),
		"key", key,
	)
	return nil
}

// fallBackToSource points the column at source_url, as the original does whenever
// the derivative cannot be produced.
//
// It returns nil when the fallback succeeds: the element now has a usable URL, so
// retrying would only repeat a fetch that already failed.
func (service *ThumbnailService) fallBackToSource(
	ctx context.Context, element *Element, spec ThumbnailSpec, cause error,
) error {
	sourceURL := deref(element.SourceURL)
	if sourceURL == "" {
		return fmt.Errorf("media: element %d has no source_url to fall back to: %w", element.ID, cause)
	}
	if deref(columnValue(element, spec.Column)) == sourceURL {
		// Already pointing there; nothing to write.
		service.logger.Warn("media_thumbnail_fallback_unchanged",
			"element_id", element.ID, "column", spec.Column, "reason", cause.Error())
		return nil
	}
	if err := service.elements.SetThumbnailURL(ctx, element.ID, spec.Column, sourceURL); err != nil {
		return fmt.Errorf("media: fall back to source_url for element %d: %w", element.ID, err)
	}

	service.logger.Warn("media_thumbnail_fell_back_to_source",
		"element_id", element.ID, "column", spec.Column, "reason", cause.Error())
	return nil
}

// MakeVideoThumbnail extracts a still frame and stores it as the element's
// thumb_url.
//
// Port of App\Jobs\MakeVideoThumbnail, with one deliberate change: that class does
// not implement ShouldQueue, so despite being called through dispatch() it runs
// inline in whatever process asks for it, putting an ffmpeg run and an upload
// inside a web request. Here it is a real queued job.
//
// It only applies to elements whose video_source is "url"; hosted platforms supply
// their own thumbnails.
func (service *ThumbnailService) MakeVideoThumbnail(ctx context.Context, elementID int64) error {
	element, err := service.elements.FindElement(ctx, elementID)
	if err != nil {
		return fmt.Errorf("media: load element %d: %w", elementID, err)
	}
	if element == nil {
		return nil
	}
	if element.Type != "video" || deref(element.VideoSource) != "url" {
		// The original filters these in its query, so a non-matching element is a
		// no-op rather than an error.
		service.logger.Info("media_video_thumbnail_skipped",
			"element_id", elementID, "type", element.Type, "video_source", deref(element.VideoSource))
		return nil
	}

	sourceURL := service.videoSourceFor(element)
	if sourceURL == "" {
		return fmt.Errorf("media: element %d has no video url", elementID)
	}

	frame, err := service.transcoder.ExtractVideoFrame(ctx, sourceURL)
	if err != nil {
		return fmt.Errorf("media: extract frame for element %d: %w", elementID, err)
	}

	key := VideoThumbnailKey()
	storedURL, err := service.store.Put(ctx, key, frame, "image/jpeg")
	if err != nil {
		return fmt.Errorf("media: store video thumbnail for element %d: %w", elementID, err)
	}
	if err := service.elements.SetThumbnailURL(ctx, elementID, "thumb_url", storedURL); err != nil {
		return fmt.Errorf("media: record video thumbnail for element %d: %w", elementID, err)
	}

	service.logger.Info("media_video_thumbnail_generated",
		"element_id", elementID, "source", sourceURL, "bytes", len(frame), "key", key)
	return nil
}

// videoSourceFor picks which URL to read the frame from.
//
// Port of the isVideoType check: if thumb_url already looks like a video file it is
// used, otherwise source_url. That looks backwards but is deliberate — before a
// thumbnail exists, thumb_url holds the uploaded video itself.
func (service *ThumbnailService) videoSourceFor(element *Element) string {
	thumbURL := deref(element.ThumbURL)
	if thumbURL != "" && isVideoFileURL(thumbURL) {
		return thumbURL
	}
	return deref(element.SourceURL)
}

func isVideoFileURL(rawURL string) bool {
	// The query string is dropped before looking at the extension; the original
	// uses pathinfo, which does not.
	if index := strings.IndexAny(rawURL, "?#"); index >= 0 {
		rawURL = rawURL[:index]
	}
	extension := strings.ToLower(strings.TrimPrefix(path.Ext(rawURL), "."))
	_, ok := videoFileExtensions[extension]
	return ok
}

// RemoveDeletedElementFiles deletes the stored files of soft-deleted elements.
//
// Port of ElementScheduleExecutor::removeDeletedFiles. path is cleared afterwards,
// which is what stops the same element being reconsidered on the next run.
//
// A URL that does not belong to this store is left alone. That matters: an element
// whose thumbnail generation fell back to an external source_url has a foreign URL
// in the column, and the original's unanchored str_replace plus an existence check
// happens to skip it too, but only by accident.
func (service *ThumbnailService) RemoveDeletedElementFiles(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("media: limit must be positive, got %d", limit)
	}

	elements, err := service.elements.DeletedElementsWithFiles(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("media: list deleted elements: %w", err)
	}

	cleaned := 0
	for _, element := range elements {
		removed := 0

		// The stored original, held as a key rather than a URL.
		if key := deref(element.Path); key != "" {
			deleted, err := service.deleteIfPresent(ctx, key)
			if err != nil {
				return cleaned, err
			}
			if deleted {
				removed++
			}
		}

		for _, rawURL := range []string{
			deref(element.ThumbURL), deref(element.LowThumbURL), deref(element.MediumThumbURL),
		} {
			key := service.store.URLToKey(rawURL)
			if key == "" {
				continue
			}
			deleted, err := service.deleteIfPresent(ctx, key)
			if err != nil {
				return cleaned, err
			}
			if deleted {
				removed++
			}
		}

		if err := service.elements.ClearElementPath(ctx, element.ID); err != nil {
			return cleaned, fmt.Errorf("media: clear path for element %d: %w", element.ID, err)
		}
		cleaned++

		service.logger.Info("media_deleted_element_files_removed",
			"element_id", element.ID, "objects_removed", removed)
	}
	return cleaned, nil
}

func (service *ThumbnailService) deleteIfPresent(ctx context.Context, key string) (bool, error) {
	exists, err := service.store.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("media: check %q: %w", key, err)
	}
	if !exists {
		return false, nil
	}
	if err := service.store.Delete(ctx, key); err != nil {
		return false, err
	}
	return true, nil
}

// PendingThumbnails returns the elements the schedule should generate for.
func (service *ThumbnailService) PendingThumbnails(ctx context.Context, spec ThumbnailSpec, limit int) ([]Element, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("media: limit must be positive, got %d", limit)
	}
	elements, err := service.elements.ElementsMissingThumbnail(ctx, spec.Column, limit)
	if err != nil {
		return nil, fmt.Errorf("media: list elements missing %s: %w", spec.Column, err)
	}
	return elements, nil
}

func columnValue(element *Element, column string) *string {
	switch column {
	case LowThumbColumn:
		return element.LowThumbURL
	case MediumThumbColumn:
		return element.MediumThumbURL
	case "thumb_url":
		return element.ThumbURL
	default:
		return nil
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
