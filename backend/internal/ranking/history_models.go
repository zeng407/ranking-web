package ranking

import (
	"context"
	"time"
)

// HistoryTimeRange is the `rank_report_histories.time_range` value.
//
// Only two of Laravel's five RankReportTimeRange values are ever written:
// buildWeek, buildMonth and buildYear have no callers, and buildMonth and
// buildYear have empty bodies. The table confirms it — 9,581,596 `all` rows and
// 3,566,315 `thousand_votes` rows, and zero for the other three. They are not
// ported.
type HistoryTimeRange string

const (
	// HistoryRangeAll is the cumulative history, one row per day.
	HistoryRangeAll HistoryTimeRange = "all"
	// HistoryRangeThousandVotes is the rolling window of the most recent
	// ThousandVotesWindow rounds, one row per day.
	HistoryRangeThousandVotes HistoryTimeRange = "thousand_votes"
)

func (timeRange HistoryTimeRange) Valid() bool {
	return timeRange == HistoryRangeAll || timeRange == HistoryRangeThousandVotes
}

// ThousandVotesWindow is the size of the rolling vote window.
const ThousandVotesWindow = 1000

// RankReportRef identifies the report a history row belongs to. The natural key
// of `rank_report_histories` is
// (element_id, post_id, rank_report_id, time_range, start_date).
type RankReportRef struct {
	ID        int64
	PostID    int64
	ElementID int64
	// PostCreatedAt is the fallback start date when the report has no history and
	// no explicit start was given.
	PostCreatedAt time.Time
}

// DailyRank is one `ranks` row, used to walk the timeline.
type DailyRank struct {
	RecordDate time.Time
	RankType   RankType
	WinCount   int64
	RoundCount int64
}

// HistoryRow is one row of `rank_report_histories`.
type HistoryRow struct {
	RankReportID int64
	PostID       int64
	ElementID    int64
	TimeRange    HistoryTimeRange
	StartDate    time.Time
	// Rank is written as 0 and assigned later by the reorder pass, matching the
	// original's "we mark the rank as 0, then update the rank later".
	Rank              int64
	WinCount          int64
	LoseCount         int64
	WinRate           float64
	ChampionCount     int64
	GameCompleteCount int64
	ChampionRate      float64
}

// VoteOutcome is one round from the element's point of view.
type VoteOutcome struct {
	RoundID int64
	// Won is true when the element was the winner of the round.
	Won bool
}

// HistoryRepository is the database side of the history builder.
type HistoryRepository interface {
	// LatestHistoryStartDate returns the newest start_date already recorded for
	// the report and range, or the zero time when there is none.
	LatestHistoryStartDate(ctx context.Context, rankReportID int64, timeRange HistoryTimeRange) (time.Time, error)
	// SoftDeleteHistory marks the report's rows for a range as deleted. It is a
	// soft delete because the table carries deleted_at and the refresh path is
	// expected to leave the old rows behind as history.
	SoftDeleteHistory(ctx context.Context, rankReportID int64, timeRange HistoryTimeRange) (int64, error)
	// FirstRankOnOrAfter returns the earliest rank row for the element at or after
	// a date, optionally restricted to one rank type.
	FirstRankOnOrAfter(ctx context.Context, postID, elementID int64, onOrAfter time.Time, rankType RankType) (*DailyRank, error)
	// RanksOnOrAfter returns every rank row for the element at or after a date,
	// ordered by record_date.
	RanksOnOrAfter(ctx context.Context, postID, elementID int64, onOrAfter time.Time) ([]DailyRank, error)
	// HistoryDatesPresent returns the start_dates already recorded for the report
	// and range, so the builder can skip them without a query per day.
	HistoryDatesPresent(ctx context.Context, rankReportID int64, timeRange HistoryTimeRange) (map[string]struct{}, error)
	// InsertHistoryRows writes rows that are known to be absent.
	InsertHistoryRows(ctx context.Context, rows []HistoryRow) error
	// UpsertHistoryRow writes one row by its natural key.
	UpsertHistoryRow(ctx context.Context, row HistoryRow) error
	// RecentVotes returns the element's most recent rounds for the post, newest
	// first, capped at limit.
	RecentVotes(ctx context.Context, postID, elementID int64, limit int) ([]VoteOutcome, error)
}

// PendingDatesStore records which history dates still need their rank assigned.
//
// The builder writes rank = 0 and the reorder pass fills it in, so the set of
// dates touched has to survive between the two jobs.
type PendingDatesStore interface {
	// Add records dates as needing a rank update.
	Add(ctx context.Context, postID int64, timeRange HistoryTimeRange, dates []string) error
	// Pull returns and clears the recorded dates.
	Pull(ctx context.Context, postID int64, timeRange HistoryTimeRange) ([]string, error)
}

// PendingDatesTTL matches CacheService::putRankHistoryNeededUpdateDatesCache.
const PendingDatesTTL = 30 * 24 * time.Hour
