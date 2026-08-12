// Package posttrend builds the hot-post rankings that feed the home page.
//
// It replaces two Laravel pieces:
//
//	make:post-trend {range}   (CreatePostTrend + PostTrendScheduleExecutor)
//	  -> post_trend.create
//	UpdatePostTrendsPosition  -> post_trend.update_positions
//
// The two are separate messages for the same reason Laravel dispatched a job at the
// end of the command: counting plays scans the games table, assigning positions
// scans the statistics, and a failure in the second must not force the first to be
// redone.
package posttrend

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Queue is where these messages travel. Same queue the Kernel's console commands
// effectively used.
const Queue = "default"

const (
	// TypeCreate recomputes play counts for one range.
	TypeCreate = "post_trend.create"
	// TypeUpdatePositions turns those play counts into ranked positions.
	TypeUpdatePositions = "post_trend.update_positions"
)

// TrendTypeHot is the only trend_type in the data. Kept named rather than inlined
// so a second type is a visible change.
const TrendTypeHot = "hot"

// TimeRange is the value stored in post_trends.time_range and
// post_statistics.time_range.
type TimeRange string

const (
	RangeAll   TimeRange = "all"
	RangeYear  TimeRange = "year"
	RangeMonth TimeRange = "month"
	RangeWeek  TimeRange = "week"
	// RangeToday is spelled "today" in the database even though the schedule and the
	// artisan command both say "day". See RangeFromScheduleArgument.
	RangeToday TimeRange = "today"
)

func (value TimeRange) Valid() bool {
	switch value {
	case RangeAll, RangeYear, RangeMonth, RangeWeek, RangeToday:
		return true
	}
	return false
}

// ErrUnknownRange means the payload named a range that is not one of the five.
var ErrUnknownRange = errors.New("posttrend: unknown time range")

// RangeFromScheduleArgument maps the schedule's argument to the stored enum.
//
// A REAL TRAP, not a formality. The Kernel runs `make:post-trend day`, whose switch
// calls createTodayPostTrends(), which passes TrendTimeRange::TODAY — the string
// "today". So the wire value is "day" and the column value is "today", and the four
// other ranges happen to spell the same both ways. Treating them as identical would
// write a time_range nothing reads and leave the real "today" trend frozen.
func RangeFromScheduleArgument(argument string) (TimeRange, error) {
	switch argument {
	case "day", "today":
		return RangeToday, nil
	case "week":
		return RangeWeek, nil
	case "month":
		return RangeMonth, nil
	case "year":
		return RangeYear, nil
	case "all":
		return RangeAll, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownRange, argument)
}

// WindowStart is the first day the range covers, or nil for RangeAll.
//
// Mirrors the switch in PostTrendScheduleExecutor::createHotTrendPost, evaluated in
// the application timezone because these are DATE values and Laravel's today() uses
// the app timezone rather than UTC.
//
// The week starts on Monday, which is Carbon's ISO-8601 default. Confirmed against
// the data rather than assumed: all 135,530 week rows in post_trends and all 284,696
// in post_statistics fall on a Monday.
func WindowStart(rangeValue TimeRange, now time.Time) *time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch rangeValue {
	case RangeAll:
		// No window: every game counts, and the statistics row is keyed by each
		// post's own creation date instead.
		return nil
	case RangeYear:
		start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
		return &start
	case RangeMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return &start
	case RangeWeek:
		// Sunday is weekday 0 in Go and the last day of an ISO week, so it needs the
		// full six-day step back rather than none.
		offset := (int(today.Weekday()) + 6) % 7
		start := today.AddDate(0, 0, -offset)
		return &start
	case RangeToday:
		return &today
	}
	return nil
}

// PlayCount is one post's play total for a range.
type PlayCount struct {
	PostID int64
	// StartDate is the statistics key. For every range but RangeAll it is the window
	// start; for RangeAll it is the post's own creation date, which is what
	// `$startDate ?: $post->created_at->toDateString()` evaluates to.
	StartDate time.Time
	Count     int64
}

// TrendPosition is one ranked row.
type TrendPosition struct {
	PostID   int64
	Position int
}

// Repository is the database surface.
type Repository interface {
	// PlayCounts counts, for every live post, the games in the window that reached
	// the minimum vote count.
	PlayCounts(ctx context.Context, rangeValue TimeRange, windowStart *time.Time) ([]PlayCount, error)
	// UpsertPlayCounts writes those counts to post_statistics.
	UpsertPlayCounts(ctx context.Context, rangeValue TimeRange, counts []PlayCount) (int64, error)
	// ResetPositions pushes every row in the group to the unranked sentinel, so a
	// post that has dropped out does not keep a stale position.
	ResetPositions(ctx context.Context, rangeValue TimeRange, windowStart *time.Time) (int64, error)
	// RankedPosts reads the top posts for the range, ordered as the original did.
	RankedPosts(ctx context.Context, rangeValue TimeRange, windowStart *time.Time, limit int) ([]int64, error)
	// UpsertPositions writes the ranked rows.
	UpsertPositions(ctx context.Context, rangeValue TimeRange, windowStart *time.Time, positions []TrendPosition) (int64, error)
}

// MinimumVoteCount is the games filter from the executor: a game with fewer votes
// than this does not count as a play.
const MinimumVoteCount = 4

// UnrankedPosition is the sentinel every row is reset to before ranking, and the
// value a post keeps when it falls outside the top RankedLimit.
const UnrankedPosition = 9999

// RankedLimit is how many posts get a real position, from the limit(1000) in
// UpdatePostTrendsPosition.
const RankedLimit = 1000

// UpsertChunkSize bounds one multi-row statement. The counts cover every live post,
// around 6,200 of them, which is too many for a single statement to stay readable in
// a slow query log.
const UpsertChunkSize = 500
