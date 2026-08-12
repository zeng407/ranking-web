package authoring

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

// The elements inside a post, from Api\ElementController's read and edit paths.
//
// Adding elements — uploads, URL batches, YouTube embeds — is not here. That is a media
// ingestion pipeline with seven source handlers behind it, and it belongs with the media
// package that already fetches and thumbnails.

// MaxElementTitleRunes is config/setting.php's element_title_size.
const MaxElementTitleRunes = 100

// Element is one item in a post, as the editor draws it. Mirrors PostElementResource.
type Element struct {
	ID           int64
	SourceURL    string
	ThumbURL     string
	MediumURL    string
	LowURL       string
	Title        string
	Type         string
	VideoSource  string
	VideoID      string
	DurationSecs *int
	StartSecond  *int
	EndSecond    *int
	CreatedAt    time.Time
	// Rank is the element's standing in this post, or nil when it has never been voted
	// on. The original embedded the whole rank_report row; this carries the two numbers
	// the editor shows.
	Rank *ElementRank
}

// ElementRank is one element's standing in one post, from its rank_reports row.
//
// Three of that row's columns, not all ten. WinRate is the running rate and FinalWinRate
// counts only the matches that decided a champion, which is the pair the rank page
// already shows side by side.
type ElementRank struct {
	Rank         int
	WinRate      float64
	FinalWinRate float64
}

// ElementEdit is what the editor can change without touching the media.
type ElementEdit struct {
	// Title nil means "leave it", matching the original's `sometimes` rule.
	Title *string
	// StartSecond and EndSecond trim a video. Nil means leave.
	StartSecond *int
	EndSecond   *int
}

// ElementPage is one page of a post's elements.
type ElementPage struct {
	Elements []Element
	Total    int
	Page     int
	PerPage  int
}

// ElementQuery is the listing's filter, sort and pagination.
type ElementQuery struct {
	// TitleLike filters on the element title. The only filter the original allowed
	// through — ElementFilter::TITLE_LIKE was the whole allow-list.
	TitleLike string
	// SortBy is "id" or "title"; anything else falls back to id.
	SortBy string
	// Descending defaults true, matching the original's ['id' => 'desc'].
	Descending bool
	Page       int
	PerPage    int
}

// Normalized fills in the defaults and clamps what the caller sent.
//
// THE ORIGINAL'S PAGINATION WAS BROKEN AND IS NOT REPRODUCED. It read the page number
// out of the wrong key — `'page' => $input['per_page'] ?? 1` — so asking for 50 per page
// asked for page 50, and a caller who sent per_page at all could never see page one.
// Every editor request would have hit it, which is how a defect like that survives:
// the client only ever sent the defaults.
func (query ElementQuery) Normalized() ElementQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PerPage < 1 || query.PerPage > ElementsPerPage {
		query.PerPage = ElementsPerPage
	}
	if query.SortBy != "title" {
		query.SortBy = "id"
	}
	query.TitleLike = strings.TrimSpace(query.TitleLike)
	return query
}

// ElementStore is the element persistence.
type ElementStore interface {
	// Elements reads one page of a post's elements. The post must belong to the user.
	Elements(ctx context.Context, userID int64, serial string, query ElementQuery) (ElementPage, error)
	// UpdateElement writes an edit. The element must belong to a post the user owns.
	UpdateElement(ctx context.Context, userID int64, elementID int64, edit ElementEdit) (Element, error)
	// DeleteElement detaches the element from its posts, soft-deletes it and removes its
	// rank reports. It answers with the ids of the posts whose reports it invalidated.
	DeleteElement(ctx context.Context, userID int64, elementID int64) ([]int64, error)
}

// Elements lists one page of a post's elements.
func (service *Service) Elements(
	ctx context.Context, userID int64, serial string, query ElementQuery,
) (ElementPage, error) {
	return service.elements.Elements(ctx, userID, serial, query.Normalized())
}

// EditElement changes a title or a video's trim points.
func (service *Service) EditElement(
	ctx context.Context, userID int64, elementID int64, edit ElementEdit,
) (Element, error) {
	if edit.Title != nil {
		trimmed := strings.TrimSpace(*edit.Title)
		if utf8.RuneCountInString(trimmed) > MaxElementTitleRunes {
			return Element{}, invalid("title", CodeTooLong)
		}
		edit.Title = &trimmed
	}
	// A negative trim point would be stored in an UNSIGNED column, where MySQL in
	// non-strict mode writes 0 and in strict mode errors. Refused here so the answer is
	// the same either way.
	if edit.StartSecond != nil && *edit.StartSecond < 0 {
		return Element{}, invalid("video_start_second", CodeInvalidRange)
	}
	if edit.EndSecond != nil && *edit.EndSecond < 0 {
		return Element{}, invalid("video_end_second", CodeInvalidRange)
	}
	if edit.StartSecond != nil && edit.EndSecond != nil && *edit.EndSecond <= *edit.StartSecond {
		// A clip that ends before it starts plays as nothing at all. The original stored
		// it, and the player then showed an element nobody could see.
		return Element{}, invalid("video_end_second", CodeInvalidRange)
	}
	return service.elements.UpdateElement(ctx, userID, elementID, edit)
}

// CodeInvalidRange is a trim that cannot be played.
const CodeInvalidRange = "invalid_range"

// DeleteElement removes an element from the post it belongs to.
func (service *Service) DeleteElement(ctx context.Context, userID int64, elementID int64) error {
	affected, err := service.elements.DeleteElement(ctx, userID, elementID)
	if err != nil {
		return err
	}
	// One refresh per post the element was ranked in, which is what DeleteElementRank
	// dispatched.
	for _, postID := range affected {
		service.refreshRank(ctx, postID)
	}
	return nil
}
