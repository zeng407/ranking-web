// Package publicpost rebuilds the public_posts table that the home page reads.
//
// It replaces PublicPostScheduleExecutor::updatePublicPosts, which the Kernel runs
// every minute. internal/publiccontent is the read side of the same table; this is
// the write side.
//
// THE BATCH CAP IS GONE ON PURPOSE.
//
// The PHP capped every pass at POST_BATCH_SIZE = 2000. That number bounded PHP's
// memory use rather than expressing a rule: the owner confirmed the limit exists
// because the old implementation consumed a lot of memory and the run took too long,
// and that processing the full set is correct if memory is handled efficiently. Left
// in, the cap silently truncates — only the newest 2,000 posts ever reach the listing,
// and the tail keeps whatever it had.
//
// So every pass here walks its whole source set, and memory is bounded by processing
// in chunks instead of by processing less. Nothing holds more than one chunk of
// assembled payloads at a time; the only whole-set structure is a slice of int64 ids,
// which is 8 bytes per post.
package publicpost

import (
	"context"
	"errors"
	"time"
)

// Queue is where the refresh travels, matching the Kernel entry's effective queue.
const Queue = "default"

// TypeRefresh rebuilds every position column.
const TypeRefresh = "public_post.refresh"

// LockKey serializes the refresh. The Kernel used withoutOverlapping(60), and the
// scheduler entry carries the same TTL; this is the worker-side half, so a message
// redelivered while one is running defers instead of interleaving.
const LockKey = "publicpost:refresh"

// MinimumElementCount is config('setting.post_min_element_count'): a post with fewer
// elements is not listed.
const MinimumElementCount = 8

// UnlistedPosition is the sentinel a post keeps when it is not in a pass's result.
// It sorts last, which is what keeps an unlisted post off the page.
const UnlistedPosition = 9999

// ChunkSize is how many posts are assembled and written at once.
//
// This is the memory bound that replaces the batch cap. Each chunk costs one round of
// queries and holds one round of payloads; the pass still covers everything.
const ChunkSize = 200

// PreviewCandidateLimit is the number of top-ranked elements the two preview
// elements are drawn from, from getRankReports($post, 5, 1).
const PreviewCandidateLimit = 5

// FreshnessTTL is how long a completed refresh suppresses the next one, from
// Cache::put('public_post_fresh', true, 60 * 10).
//
// It is a debounce rather than a schedule: PostUpdateTimestampSubscriber clears the
// flag whenever a post is created, updated or deleted, so a change is picked up on
// the next tick and an idle system rebuilds at most every ten minutes.
const FreshnessTTL = 10 * time.Minute

// FreshnessKey is Laravel's un-prefixed cache key.
const FreshnessKey = "public_post_fresh"

// PostResourceKeyPrefix is Laravel's per-post resource cache, cleared at the end of
// a refresh so the PHP endpoints stop serving the payload this run replaced.
const PostResourceKeyPrefix = "post_resource_"

var ErrUnknownPass = errors.New("publicpost: unknown pass")

// Pass is one of the four position columns.
type Pass string

const (
	// PassNew orders by post id descending: newest first.
	PassNew Pass = "new"
	// PassToday, PassWeek and PassMonth order by the hot trend position for their
	// window.
	PassToday Pass = "today"
	PassWeek  Pass = "week"
	PassMonth Pass = "month"
)

// Ordered returns the passes in the order the executor ran them. The order matters:
// each pass marks every row dirty and the last one to run decides which rows survive
// removeDirtyPublicPosts.
func Ordered() []Pass {
	return []Pass{PassNew, PassToday, PassWeek, PassMonth}
}

// PositionColumn is the column a pass writes.
func (pass Pass) PositionColumn() (string, error) {
	switch pass {
	case PassNew:
		return "new_position", nil
	case PassToday:
		return "day_position", nil
	case PassWeek:
		return "week_position", nil
	case PassMonth:
		return "month_position", nil
	}
	return "", errors.New("publicpost: pass " + string(pass) + " has no position column")
}

// TrendRange is the post_trends.time_range a pass reads, or "" for PassNew which
// reads posts directly.
func (pass Pass) TrendRange() string {
	switch pass {
	case PassToday:
		return "today"
	case PassWeek:
		return "week"
	case PassMonth:
		return "month"
	}
	return ""
}

func (pass Pass) Valid() bool {
	_, err := pass.PositionColumn()
	return err == nil
}

// Element is the preview element shape from PostResource::getElement, field for
// field. The order here is the PHP's, and the JSON tags are what
// internal/publiccontent decodes.
type Element struct {
	VideoSource *string `json:"video_source"`
	Type        *string `json:"type"`
	ID          *int64  `json:"id"`
	URL         *string `json:"url"`
	URL2        *string `json:"url2"`
	Title       *string `json:"title"`
	Previewable bool    `json:"previewable"`
}

// Resource is the payload stored in public_posts.data, matching
// PostResource::toArray.
type Resource struct {
	Title       string  `json:"title"`
	Serial      string  `json:"serial"`
	IsPrivate   bool    `json:"is_private"`
	Description string  `json:"description"`
	Element1    Element `json:"element1"`
	Element2    Element `json:"element2"`
	// CreatedAt and UpdatedAt are Carbon's toDateTimeString(), so "Y-m-d H:i:s" with
	// no timezone and no sub-second part.
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	PlayCount     int64    `json:"play_count"`
	ElementsCount int64    `json:"elements_count"`
	Tags          []string `json:"tags"`
	// IsCensored is an INTEGER on the wire, not a boolean, and this is not a choice.
	// posts.is_censored is tinyint(1) with no Eloquent cast declared, so PDO hands back
	// 0 or 1 and json_encode writes the number — while is_private and previewable are
	// PHP booleans from `===` comparisons and so encode as true/false. Confirmed
	// against the stored rows with JSON_TYPE: is_censored INTEGER, is_private BOOLEAN,
	// previewable BOOLEAN.
	//
	// internal/publiccontent decodes this field into an int, so writing false here
	// makes the whole listing fail to decode. The cross-package test caught exactly
	// that.
	IsCensored int `json:"is_censored"`
}

// PostRow is what the assembler needs about one post.
type PostRow struct {
	ID          int64
	Serial      string
	Title       string
	Description string
	IsPrivate   bool
	// IsCensored is an int for the reason given on Resource.IsCensored.
	IsCensored int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ElementRow is an element as stored, before it becomes a preview Element.
type ElementRow struct {
	ID             int64
	Title          *string
	Type           *string
	VideoSource    *string
	ThumbURL       *string
	MediumThumbURL *string
}

// Row is one assembled public_posts row.
type Row struct {
	PostID      int64
	Position    int
	Title       string
	Description string
	// Tags is the JSON array stored in the varchar column, which is separate from
	// Resource.Tags and is what the listing filters on.
	Tags     string
	Resource Resource
	// The candidate sets the preview is drawn from, carried on the row so the
	// repository can batch-load them while the service does the selection — that step
	// needs the injected shuffle, which does not belong in a query.
	rankedCandidates   []ElementRow
	fallbackCandidates []ElementRow
	tagNames           []string
}

// Candidates exposes the loaded preview candidates. Package-internal state made
// readable for tests rather than part of the contract.
func (row Row) Candidates() (ranked, fallback []ElementRow) {
	return row.rankedCandidates, row.fallbackCandidates
}

// Repository is the database surface for the write side.
type Repository interface {
	// ListedPostIDs returns the posts PassNew covers, newest first.
	ListedPostIDs(ctx context.Context) ([]int64, error)
	// TrendedPostIDs returns the posts a trend pass covers, in trend order.
	TrendedPostIDs(ctx context.Context, trendRange string, windowStart time.Time) ([]int64, error)
	// MarkAllDirty flags every existing row so the ones this pass does not write can
	// be pushed to the sentinel afterwards.
	MarkAllDirty(ctx context.Context) (int64, error)
	// PushDirtyToSentinel gives every still-dirty row the unlisted position.
	PushDirtyToSentinel(ctx context.Context, pass Pass) (int64, error)
	// LoadChunk assembles the rows for one chunk of post ids.
	LoadChunk(ctx context.Context, postIDs []int64) ([]Row, error)
	// UpsertChunk writes them, clearing is_dirty.
	UpsertChunk(ctx context.Context, pass Pass, rows []Row) (int64, error)
	// RemoveDirty deletes rows whose post no longer qualifies for the listing.
	RemoveDirty(ctx context.Context) (int64, error)
	// PublicPostIDs lists the posts whose Laravel resource cache should be cleared.
	PublicPostIDs(ctx context.Context) ([]int64, error)
}

// FreshnessStore is the debounce flag, shared with Laravel.
type FreshnessStore interface {
	// IsFresh reports whether a recent refresh means this one can be skipped.
	IsFresh(ctx context.Context) (bool, error)
	// MarkFresh records that a refresh completed.
	MarkFresh(ctx context.Context) error
}

// ResourceCache invalidates Laravel's per-post resource entries.
type ResourceCache interface {
	Clear(ctx context.Context, postIDs []int64) error
}

// NoResourceCache is used once the PHP endpoints no longer serve posts.
type NoResourceCache struct{}

func (NoResourceCache) Clear(context.Context, []int64) error { return nil }
