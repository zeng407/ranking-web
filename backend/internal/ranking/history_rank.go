package ranking

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// HistoryRetentionDays matches RemoveOutdateRankHistory's now()->subDays(93).
const HistoryRetentionDays = 93

// HistoryPurgeBatchSize matches the job's limit(1000). Deleting in bounded batches
// keeps one run from holding locks across a huge range of a 13.1M row table.
const HistoryPurgeBatchSize = 1000

// HistoryRankChunkSize bounds one batched rank UPDATE.
const HistoryRankChunkSize = 500

// RankedHistoryRow is a history row participating in the rank assignment.
type RankedHistoryRow struct {
	ID           int64
	ElementID    int64
	WinRate      float64
	ChampionRate float64
	WinCount     int64
	Rank         int64
}

// HistoryRankRepository is the database side of rank assignment and purging.
type HistoryRankRepository interface {
	// HistoryRowsForRanking returns the live rows for one post, range and date.
	HistoryRowsForRanking(ctx context.Context, postID int64, timeRange HistoryTimeRange, startDate time.Time) ([]RankedHistoryRow, error)
	// ApplyHistoryRanks writes the assigned ranks.
	ApplyHistoryRanks(ctx context.Context, rows []RankedHistoryRow) error
	// PurgeHistoryOlderThan hard-deletes up to limit rows whose start_date is
	// before the cutoff, returning how many went.
	PurgeHistoryOlderThan(ctx context.Context, postID int64, cutoff time.Time, limit int) (int64, error)
}

// AssignHistoryRanks fills in the rank for one post, range and date.
//
// Port of App\Jobs\UpdateRankForReportHistory. The builders write rank = 0 and
// this is the pass that turns those into positions.
//
// ORDER. win_rate, then champion_rate, then win_count, all descending, exactly as
// the original. A deterministic tie-break on element_id is added for the same
// reason as in the report ordering: the original has no final tie-break, so tied
// rows are ordered however MySQL happens to return them, which nothing can
// reproduce.
func (service *Service) AssignHistoryRanks(
	ctx context.Context,
	postID int64,
	timeRange HistoryTimeRange,
	startDate time.Time,
) error {
	if service.historyRanks == nil {
		return fmt.Errorf("ranking: history rank repository is not configured")
	}
	if postID <= 0 {
		return fmt.Errorf("ranking: post id is required, got %d", postID)
	}
	if !timeRange.Valid() {
		return fmt.Errorf("ranking: unknown history time range %q", timeRange)
	}

	rows, err := service.historyRanks.HistoryRowsForRanking(ctx, postID, timeRange, startDate)
	if err != nil {
		return fmt.Errorf("ranking: read history rows for post %d %s %s: %w",
			postID, timeRange, startDate.Format(dateLayout), err)
	}
	if len(rows) == 0 {
		return nil
	}

	SortHistoryRowsForRanking(rows)

	changed := make([]RankedHistoryRow, 0, len(rows))
	for index := range rows {
		assigned := int64(index + 1)
		if rows[index].Rank == assigned {
			// Already correct. Skipping keeps a re-run from rewriting every row of
			// a large post for nothing.
			continue
		}
		rows[index].Rank = assigned
		changed = append(changed, rows[index])
	}
	if len(changed) == 0 {
		return nil
	}

	if err := service.historyRanks.ApplyHistoryRanks(ctx, changed); err != nil {
		return fmt.Errorf("ranking: apply history ranks for post %d %s %s: %w",
			postID, timeRange, startDate.Format(dateLayout), err)
	}

	service.logger.Info("rank_history_ranks_assigned",
		"post_id", postID,
		"time_range", string(timeRange),
		"start_date", startDate.Format(dateLayout),
		"rows", len(rows),
		"rows_changed", len(changed),
	)
	return nil
}

// SortHistoryRowsForRanking orders rows the way the rank is assigned.
func SortHistoryRowsForRanking(rows []RankedHistoryRow) {
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].WinRate != rows[right].WinRate {
			return rows[left].WinRate > rows[right].WinRate
		}
		if rows[left].ChampionRate != rows[right].ChampionRate {
			return rows[left].ChampionRate > rows[right].ChampionRate
		}
		if rows[left].WinCount != rows[right].WinCount {
			return rows[left].WinCount > rows[right].WinCount
		}
		return rows[left].ElementID < rows[right].ElementID
	})
}

// ReorderHistoryRanks consumes the pending dates for a post and returns the work
// items that need ranking, one per (range, date).
//
// Port of RankService::updateRankReportHistoryRank, called for both ranges the way
// ReorderRankReportHistory does.
//
// DIVERGENCE, DELIBERATE. The original also consults a counter of in-flight
// builder jobs and, when it is non-zero, re-dispatches itself with a growing
// delay, trying to wait until every builder has finished. This port does not,
// because the pending-dates set already makes that unnecessary: a builder that
// finishes after this pass adds its date back to the set, so the next pass ranks
// it. Ranking a date twice is harmless — the second pass reads every live row for
// that date and reassigns from scratch. Dropping the counter removes a
// self-rescheduling loop whose termination depended on a cache value that nothing
// guarantees ever reaches zero.
func (service *Service) ReorderHistoryRanks(ctx context.Context, postID int64) ([]HistoryRankTarget, error) {
	if service.pending == nil {
		return nil, fmt.Errorf("ranking: pending dates store is not configured")
	}
	if postID <= 0 {
		return nil, fmt.Errorf("ranking: post id is required, got %d", postID)
	}

	targets := make([]HistoryRankTarget, 0)
	for _, timeRange := range []HistoryTimeRange{HistoryRangeAll, HistoryRangeThousandVotes} {
		dates, err := service.pending.Pull(ctx, postID, timeRange)
		if err != nil {
			return nil, fmt.Errorf("ranking: pull pending dates for post %d %s: %w", postID, timeRange, err)
		}
		// Sorted so the emitted work is stable and reads chronologically in logs.
		sort.Strings(dates)
		for _, date := range dates {
			parsed, err := time.Parse(dateLayout, date)
			if err != nil {
				// A malformed entry cannot be ranked; log and drop rather than
				// failing the whole post.
				service.logger.Warn("rank_history_pending_date_invalid",
					"post_id", postID, "time_range", string(timeRange), "value", date)
				continue
			}
			targets = append(targets, HistoryRankTarget{
				PostID:    postID,
				TimeRange: timeRange,
				StartDate: parsed,
			})
		}
	}

	service.logger.Info("rank_history_reorder_collected",
		"post_id", postID, "targets", len(targets))
	return targets, nil
}

// HistoryRankTarget names one rank assignment: a post, a range and a date.
type HistoryRankTarget struct {
	PostID    int64
	TimeRange HistoryTimeRange
	StartDate time.Time
}

// RemoveOutdatedHistory hard-deletes one batch of history older than the retention
// window.
//
// Port of App\Jobs\RemoveOutdateRankHistory. It returns the number removed so a
// caller can tell whether more remain.
//
// NOTE ON THE BACKLOG. The original removes at most 1000 rows per run and the
// schedule fires once a day, so a post that produces more than 1000 rows a day
// never catches up. The restored dump shows it: `all` rows go back to 2026-03-11,
// about 140 days before the dump, against a 93 day retention. This port keeps the
// batch size, because raising it changes the lock profile on a 13.1M row table,
// but it reports the count so the backlog becomes visible instead of silent.
func (service *Service) RemoveOutdatedHistory(ctx context.Context, postID int64) (int64, error) {
	if service.historyRanks == nil {
		return 0, fmt.Errorf("ranking: history rank repository is not configured")
	}
	if postID <= 0 {
		return 0, fmt.Errorf("ranking: post id is required, got %d", postID)
	}

	cutoff := service.now().AddDate(0, 0, -HistoryRetentionDays)
	removed, err := service.historyRanks.PurgeHistoryOlderThan(ctx, postID, cutoff, HistoryPurgeBatchSize)
	if err != nil {
		return 0, fmt.Errorf("ranking: purge history for post %d: %w", postID, err)
	}
	if removed == 0 {
		return 0, nil
	}

	service.logger.Info("rank_history_purged",
		"post_id", postID,
		"cutoff", cutoff.Format(dateLayout),
		"removed", removed,
		"batch_size", HistoryPurgeBatchSize,
		// True means the batch was filled, so more rows are almost certainly
		// waiting and one run per day will not drain them.
		"batch_full", removed == int64(HistoryPurgeBatchSize),
	)
	return removed, nil
}
