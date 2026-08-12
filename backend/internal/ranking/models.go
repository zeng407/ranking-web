// Package ranking computes the per-element rank rows that back the public
// leaderboards.
//
// It is a port of App\Services\RankService::createElementRank. Two properties of
// that implementation are load-bearing and must survive the port:
//
// The aggregation is incremental behind a watermark. The running totals and the
// highest game_1v1_rounds id already counted are memoised under
// element_rank_stats:{postID}:{elementID} with a seven day TTL, so each run only
// scans rounds newer than the watermark instead of re-aggregating a table that
// holds 45.9M rows.
//
// The database write is an absolute value, never an increment. That is what makes
// the memo safe. There is no transaction spanning Redis and MySQL, so either
// write can fail alone, and both orders still converge:
//
//	rank written, memo lost      next run recounts from the old watermark and
//	                             writes the same absolute total
//	memo written, rank lost      next run finds no new rounds and rewrites the
//	                             total already in the memo
//
// An "optimisation" to win_count = win_count + N would break both cases. The
// invariant is covered by a test.
package ranking

import (
	"context"
	"fmt"
	"time"
)

// RankType matches the enum('pk_king','champion') column on `ranks`.
type RankType string

const (
	// RankTypeChampion counts games an element actually won outright, over the
	// rounds of completed games only.
	RankTypeChampion RankType = "champion"
	// RankTypePKKing counts head-to-head wins across all games, complete or not.
	RankTypePKKing RankType = "pk_king"
)

func (rankType RankType) Valid() bool {
	return rankType == RankTypeChampion || rankType == RankTypePKKing
}

// Stats is the memoised aggregation state for one element in one post.
//
// The field names match the Laravel cache payload exactly so the two
// implementations can read each other's entries during the cutover.
type Stats struct {
	ChampionMaxWinID   int64 `json:"champion_max_win_id"`
	ChampionMaxLoseID  int64 `json:"champion_max_lose_id"`
	ChampionRoundWins  int64 `json:"champion_round_wins"`
	ChampionRoundLoses int64 `json:"champion_round_loses"`
	ChampionGameWins   int64 `json:"champion_game_wins"`
	PKMaxWinID         int64 `json:"pk_max_win_id"`
	PKMaxLoseID        int64 `json:"pk_max_lose_id"`
	PKWinCount         int64 `json:"pk_win_count"`
	PKLoseCount        int64 `json:"pk_lose_count"`
}

// StatsTTL matches CacheService::getElementRankStats.
//
// Expiry is safe by construction: the totals and the watermark live in the same
// entry, so they are lost together and the next run recounts from zero, which
// produces the same absolute values. Splitting them would not be safe.
const StatsTTL = 7 * 24 * time.Hour

// StatsKey is the memo key, matching the Laravel cache key so both
// implementations address the same entry.
func StatsKey(postID, elementID int64) string {
	return fmt.Sprintf("element_rank_stats:%d:%d", postID, elementID)
}

// RoundDelta is the aggregate of game_1v1_rounds newer than a watermark.
type RoundDelta struct {
	// Count is how many rounds matched.
	Count int64
	// MaxID is the highest id seen, becoming the new watermark.
	MaxID int64
	// ChampionCount is how many of those rounds were the final round of a game
	// (remain_elements = 1), i.e. an outright win. Only meaningful for the
	// completed-games winner query.
	ChampionCount int64
}

// Rank is one row of the `ranks` table.
type Rank struct {
	PostID     int64
	ElementID  int64
	RankType   RankType
	RecordDate time.Time
	WinCount   int64
	RoundCount int64
	// WinRate is a percentage stored as decimal(5,2).
	WinRate float64
}

// BaseRank is one `ranks` row feeding the report computation.
type BaseRank struct {
	ElementID int64
	RankType  RankType
	WinRate   float64
	WinCount  int64
}

// ExistingReport is the current `rank_reports` row for an element. Every value is
// a pointer because the columns are nullable and the port must distinguish
// "absent" from "zero": the Laravel code falls back to the stored value only when
// the fresh one is missing.
type ExistingReport struct {
	ElementID        int64
	FinalWinPosition *int64
	FinalWinRate     *float64
	WinPosition      *int64
	WinRate          *float64
	CreatedAt        *time.Time
}

// ReportRow is one row to write to `rank_reports`.
type ReportRow struct {
	PostID    int64
	ElementID int64
	// Rank is the overall position across the post, 1-based.
	Rank int
	// FinalWinPosition is the champion-table position; nil when the element has
	// never had one.
	FinalWinPosition *int64
	FinalWinRate     float64
	// WinPosition is the pk_king-table position; nil when the element has never
	// had one.
	WinPosition *int64
	WinRate     float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ReportUpsertChunkSize matches the 200-row chunks RankService::createRankReports
// writes. A post can carry thousands of elements, and one statement per element
// would be far slower while one statement for all of them would hold locks for
// too long.
const ReportUpsertChunkSize = 200

// ReportTransactionAttempts matches Laravel's DB::transaction($callback, 3).
// The write touches many rows of one post, so a deadlock against a concurrent
// run is expected rather than exceptional.
const ReportTransactionAttempts = 3

// StatsStore memoises the aggregation state.
type StatsStore interface {
	// Get returns the stored stats, or the zero value when absent. A missing
	// entry is not an error: it means a full recount, which is correct.
	Get(ctx context.Context, postID, elementID int64) (Stats, error)
	Put(ctx context.Context, postID, elementID int64, stats Stats) error
}

// Repository is the database side of the computation.
type Repository interface {
	// CompletedWinDelta aggregates winner rounds of completed games above the
	// watermark, including how many were outright wins.
	CompletedWinDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error)
	// CompletedLoseDelta aggregates loser rounds of completed games above the
	// watermark.
	CompletedLoseDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error)
	// AllGamesWinDelta aggregates winner rounds across all games above the
	// watermark.
	AllGamesWinDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error)
	// AllGamesLoseDelta aggregates loser rounds across all games above the
	// watermark.
	AllGamesLoseDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error)
	// UpsertRank writes one rank row by its natural key.
	UpsertRank(ctx context.Context, rank Rank) error
}

// ReportRepository is the database side of the rank report computation.
type ReportRepository interface {
	// BaseRanks returns the post's `ranks` rows for one record date, excluding
	// soft-deleted elements.
	BaseRanks(ctx context.Context, postID int64, recordDate time.Time) ([]BaseRank, error)
	// ExistingReports returns the post's current reports keyed by element,
	// excluding soft-deleted elements.
	ExistingReports(ctx context.Context, postID int64) (map[int64]ExistingReport, error)
	// UpsertReports writes the rows and hides reports whose element has been
	// soft-deleted, in one retrying transaction.
	UpsertReports(ctx context.Context, postID int64, rows []ReportRow) (hidden int64, err error)
}
