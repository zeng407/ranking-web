package admin

import (
	"context"
	"strings"
	"unicode/utf8"
)

// CarouselTypeVideo is the only home_carousel_items.type the site draws. The column is a
// plain string and the original validated `in:video`, so this is the whole allow-list.
const CarouselTypeVideo = "video"

// MaxCarouselTextRunes bounds the title and description, which are VARCHAR(255).
//
// The original validated neither, so a long title was truncated by MySQL in non-strict
// mode and rejected as a 500 in strict mode. Refused here, so the moderator is told.
const MaxCarouselTextRunes = 255

// CarouselItem is one slide of the home carousel.
type CarouselItem struct {
	ID          int64
	Position    int
	Type        string
	Title       string
	Description string
	ImageURL    string
	VideoURL    string
	VideoSource string
	VideoID     string
	StartSecond *int
	EndSecond   *int
	Active      bool
}

// CarouselDraft is what creating a slide accepts.
type CarouselDraft struct {
	Type  string
	Title string
	// Description defaults to the title, which is what HomeCarouselService did.
	Description string
	// ImageURL overrides the thumbnail the video lookup finds.
	ImageURL string
	VideoURL string
	// StartSecond and EndSecond trim the clip the home page plays.
	StartSecond *int
	EndSecond   *int
	// Active nil means active, matching the column's default.
	Active *bool
}

// CarouselEdit changes a slide without re-resolving its video. Nil means "leave it".
type CarouselEdit struct {
	Title       *string
	Description *string
	StartSecond *int
	EndSecond   *int
	Active      *bool
}

// CarouselPosition is one entry of a reorder.
type CarouselPosition struct {
	ID       int64
	Position int
}

// ResolvedVideo is what a video URL turns out to be.
type ResolvedVideo struct {
	Title    string
	ThumbURL string
	// Source is the video_source column: youtube, twitch, and so on.
	Source string
	// ID is the platform's own video id, empty for a source that has none.
	ID string
	// URL is the canonical URL to store, which for a lookup that followed a redirect or
	// normalised an embed is not the URL that was submitted.
	URL string
}

// CarouselItems reads the carousel in the admin screen's order: inactive slides last,
// then by position.
func (service *Service) CarouselItems(ctx context.Context) ([]CarouselItem, error) {
	items, err := service.store.CarouselItems(ctx)
	return items, wrap("list carousel items", err)
}

// CreateCarouselItem adds a slide, resolving the video's title and thumbnail from its URL.
//
// The new slide takes position 1, which is what HomeCarouselService did: positions are
// not unique, so this puts it at the front without renumbering anything, and the reorder
// endpoint is how a moderator arranges them afterwards.
func (service *Service) CreateCarouselItem(
	ctx context.Context, draft CarouselDraft,
) (CarouselItem, error) {
	draft.Type = strings.TrimSpace(draft.Type)
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.VideoURL = strings.TrimSpace(draft.VideoURL)
	draft.ImageURL = strings.TrimSpace(draft.ImageURL)

	fields := FieldErrors{}
	if draft.Type == "" {
		fields["type"] = []string{authoringRequired}
	} else if draft.Type != CarouselTypeVideo {
		fields["type"] = []string{authoringInvalidPolicy}
	}
	if draft.VideoURL == "" {
		fields["video_url"] = []string{authoringRequired}
	}
	validateCarouselText(draft.Title, "title", fields)
	validateCarouselText(draft.Description, "description", fields)
	if err := validateTrim(draft.StartSecond, draft.EndSecond); err != nil {
		return CarouselItem{}, err
	}
	if len(fields) > 0 {
		return CarouselItem{}, &ErrInvalid{Fields: fields}
	}

	item := CarouselItem{
		Position:    1,
		Type:        draft.Type,
		Title:       draft.Title,
		Description: draft.Description,
		ImageURL:    draft.ImageURL,
		VideoURL:    draft.VideoURL,
		StartSecond: draft.StartSecond,
		EndSecond:   draft.EndSecond,
		Active:      draft.Active == nil || *draft.Active,
	}

	if service.videos != nil {
		resolved, err := service.videos.Resolve(ctx, draft.VideoURL)
		if err != nil {
			// The URL is the one thing a slide cannot be guessed from: a slide with no
			// playable video is a blank panel on the home page.
			return CarouselItem{}, invalid("video_url", CodeUnresolvable)
		}
		item.VideoSource = resolved.Source
		item.VideoID = resolved.ID
		if resolved.URL != "" {
			item.VideoURL = resolved.URL
		}
		if item.Title == "" {
			item.Title = resolved.Title
		}
		if item.ImageURL == "" {
			item.ImageURL = resolved.ThumbURL
		}
	}
	if item.ImageURL == "" {
		// Without a lookup the moderator has to supply the still themselves; the carousel
		// draws the image before the video is playable.
		return CarouselItem{}, invalid("image_url", authoringRequired)
	}
	if item.Description == "" {
		// What HomeCarouselService stored: the description defaults to the title.
		item.Description = item.Title
	}

	stored, err := service.store.CreateCarouselItem(ctx, item)
	if err != nil {
		return CarouselItem{}, wrap("create a carousel item", err)
	}
	service.forgetCarouselCache(ctx)
	return stored, nil
}

// UpdateCarouselItem changes a slide's text, trim points or visibility.
func (service *Service) UpdateCarouselItem(
	ctx context.Context, itemID int64, edit CarouselEdit,
) (CarouselItem, error) {
	fields := FieldErrors{}
	if edit.Title != nil {
		trimmed := strings.TrimSpace(*edit.Title)
		edit.Title = &trimmed
		validateCarouselText(trimmed, "title", fields)
	}
	if edit.Description != nil {
		trimmed := strings.TrimSpace(*edit.Description)
		edit.Description = &trimmed
		validateCarouselText(trimmed, "description", fields)
	}
	if len(fields) > 0 {
		return CarouselItem{}, &ErrInvalid{Fields: fields}
	}

	// The trim points are checked against the stored ones as well as each other, so
	// moving only the start cannot leave a clip that ends before it begins.
	existing, err := service.store.CarouselItem(ctx, itemID)
	if err != nil {
		return CarouselItem{}, wrap("read the carousel item", err)
	}
	start, end := existing.StartSecond, existing.EndSecond
	if edit.StartSecond != nil {
		start = edit.StartSecond
	}
	if edit.EndSecond != nil {
		end = edit.EndSecond
	}
	if err := validateTrim(start, end); err != nil {
		return CarouselItem{}, err
	}

	item, err := service.store.UpdateCarouselItem(ctx, itemID, edit)
	if err != nil {
		return CarouselItem{}, wrap("update the carousel item", err)
	}
	service.forgetCarouselCache(ctx)
	return item, nil
}

// DeleteCarouselItem removes a slide.
func (service *Service) DeleteCarouselItem(ctx context.Context, itemID int64) error {
	if err := service.store.DeleteCarouselItem(ctx, itemID); err != nil {
		return wrap("delete the carousel item", err)
	}
	service.forgetCarouselCache(ctx)
	return nil
}

// ReorderCarouselItems writes a new order.
func (service *Service) ReorderCarouselItems(ctx context.Context, positions []CarouselPosition) error {
	if len(positions) == 0 {
		return invalid("items", authoringRequired)
	}
	seen := make(map[int64]struct{}, len(positions))
	for _, entry := range positions {
		if entry.ID <= 0 {
			return invalid("items", CodeInvalidRange)
		}
		if entry.Position < 0 {
			// The column is UNSIGNED; a negative would be written as 0 or error
			// depending on the server's strict mode.
			return invalid("items", CodeInvalidRange)
		}
		if _, repeated := seen[entry.ID]; repeated {
			// Two positions for one slide: the last write would win silently, and which
			// one that is depends on the order of a JSON array.
			return invalid("items", CodeDuplicate)
		}
		seen[entry.ID] = struct{}{}
	}

	if err := service.store.ReorderCarouselItems(ctx, positions); err != nil {
		return wrap("reorder carousel items", err)
	}
	service.forgetCarouselCache(ctx)
	return nil
}

// Validation codes this package adds to the authoring set.
const (
	// CodeUnresolvable is a video URL nothing could be read from.
	CodeUnresolvable = "unresolvable"
	// CodeDuplicate is the same id twice in one request.
	CodeDuplicate = "duplicate"
	// CodeInvalidRange is a number the column cannot hold, or a clip that ends before it
	// starts.
	CodeInvalidRange = "invalid_range"
)

// The authoring package's codes, aliased so this file reads in one vocabulary.
const (
	authoringRequired      = "required"
	authoringTooLong       = "too_long"
	authoringInvalidPolicy = "invalid_policy"
)

func validateCarouselText(value, field string, fields FieldErrors) {
	if utf8.RuneCountInString(value) > MaxCarouselTextRunes {
		fields[field] = []string{authoringTooLong}
	}
}

func validateTrim(start, end *int) error {
	if start != nil && *start < 0 {
		return invalid("video_start_second", CodeInvalidRange)
	}
	if end != nil && *end < 0 {
		return invalid("video_end_second", CodeInvalidRange)
	}
	if start != nil && end != nil && *end <= *start {
		return invalid("video_end_second", CodeInvalidRange)
	}
	return nil
}

func (service *Service) forgetCarouselCache(ctx context.Context) {
	if service.carouselCache == nil {
		return
	}
	// Swallowed for the same reason the role cache delete is: the row is written, and a
	// stale home page corrects itself when the cache expires.
	_ = service.carouselCache.ForgetCarousels(ctx)
}
