package ranking

import (
	"context"
	"fmt"
	"time"
)

// dateLayout is the DATE column format.
const dateLayout = "2006-01-02"

// BuildAllHistory writes one cumulative history row per day for a report.
//
// Port of RankReportHistoryBuilder::buildAll.
//
// The shape of the algorithm is a carry-forward walk. `ranks` has at most one row
// per element per day per type, and days where nobody played have no row at all,
// so the totals from the last day that did have one are carried forward. That is
// why this cannot be expressed as a single SQL statement.
//
// Rows are written with rank = 0 and the dates are recorded as pending; the
// reorder pass assigns the real ranks later.
func (service *Service) BuildAllHistory(
	ctx context.Context,
	report RankReportRef,
	refresh bool,
	startAt time.Time,
) error {
	if err := service.requireHistory(report); err != nil {
		return err
	}

	if refresh {
		if _, err := service.history.SoftDeleteHistory(ctx, report.ID, HistoryRangeAll); err != nil {
			return fmt.Errorf("ranking: refresh all history for report %d: %w", report.ID, err)
		}
	}

	start, err := service.historyStartDate(ctx, report, HistoryRangeAll, startAt)
	if err != nil {
		return err
	}

	// The pk_king row anchors the walk. Its absence means the element has never
	// played, and the original bails out on sumRounds == 0.
	pkAnchor, err := service.history.FirstRankOnOrAfter(ctx, report.PostID, report.ElementID, start, RankTypePKKing)
	if err != nil {
		return fmt.Errorf("ranking: anchor pk rank for report %d: %w", report.ID, err)
	}

	var winCount, loseCount, rounds int64
	if pkAnchor != nil {
		winCount = pkAnchor.WinCount
		loseCount = pkAnchor.RoundCount - pkAnchor.WinCount
		rounds = pkAnchor.RoundCount
		// The walk starts at the anchor's date, not the requested start.
		start = pkAnchor.RecordDate
	}
	if rounds == 0 {
		// Nobody played in this window. Not an error.
		return nil
	}

	championAnchor, err := service.history.FirstRankOnOrAfter(ctx, report.PostID, report.ElementID, start, RankTypeChampion)
	if err != nil {
		return fmt.Errorf("ranking: anchor champion rank for report %d: %w", report.ID, err)
	}
	var championCount, gameCompleteCount int64
	if championAnchor != nil {
		championCount = championAnchor.WinCount
		gameCompleteCount = championAnchor.RoundCount
	}

	dailyRanks, err := service.history.RanksOnOrAfter(ctx, report.PostID, report.ElementID, start)
	if err != nil {
		return fmt.Errorf("ranking: ranks for report %d: %w", report.ID, err)
	}
	// Indexed by date so the walk does not rescan the slice per day, which the
	// original does with a Collection::where per day per type.
	pkByDate := make(map[string]DailyRank, len(dailyRanks))
	championByDate := make(map[string]DailyRank, len(dailyRanks))
	for _, rank := range dailyRanks {
		key := rank.RecordDate.Format(dateLayout)
		switch rank.RankType {
		case RankTypePKKing:
			if _, seen := pkByDate[key]; !seen {
				pkByDate[key] = rank
			}
		case RankTypeChampion:
			if _, seen := championByDate[key]; !seen {
				championByDate[key] = rank
			}
		}
	}

	present, err := service.history.HistoryDatesPresent(ctx, report.ID, HistoryRangeAll)
	if err != nil {
		return fmt.Errorf("ranking: existing history dates for report %d: %w", report.ID, err)
	}

	today := service.now().Truncate(0)
	todayKey := today.Format(dateLayout)

	rows := make([]HistoryRow, 0)
	pendingDates := make([]string, 0)

	// Walk day by day up to but excluding today, matching `while ($timeline->lt(today()))`.
	for timeline := start; ; timeline = timeline.AddDate(0, 0, 1) {
		key := timeline.Format(dateLayout)
		if key >= todayKey {
			break
		}

		if rank, ok := pkByDate[key]; ok {
			winCount = rank.WinCount
			loseCount = rank.RoundCount - rank.WinCount
			rounds = rank.RoundCount
		}
		if rank, ok := championByDate[key]; ok {
			championCount = rank.WinCount
			gameCompleteCount = rank.RoundCount
		}

		// An existing row is never rewritten. The original guards its
		// updateOrCreate with an exists() check, which makes the update branch
		// unreachable, so this is insert-if-absent.
		if _, exists := present[key]; exists {
			continue
		}
		if winCount <= 0 {
			continue
		}

		rows = append(rows, HistoryRow{
			RankReportID:      report.ID,
			PostID:            report.PostID,
			ElementID:         report.ElementID,
			TimeRange:         HistoryRangeAll,
			StartDate:         timeline,
			Rank:              0,
			WinCount:          winCount,
			LoseCount:         loseCount,
			WinRate:           WinRate(winCount, rounds),
			ChampionCount:     championCount,
			GameCompleteCount: gameCompleteCount,
			ChampionRate:      WinRate(championCount, gameCompleteCount),
		})
		pendingDates = append(pendingDates, key)
	}

	if len(rows) == 0 {
		return nil
	}
	if err := service.history.InsertHistoryRows(ctx, rows); err != nil {
		return fmt.Errorf("ranking: insert all history for report %d: %w", report.ID, err)
	}
	// Recorded after the rows exist. Recording first would let the reorder pass
	// look for dates that were never written.
	if err := service.pending.Add(ctx, report.PostID, HistoryRangeAll, pendingDates); err != nil {
		return fmt.Errorf("ranking: record pending dates for report %d: %w", report.ID, err)
	}

	service.logger.Info("rank_history_all_built",
		"rank_report_id", report.ID,
		"post_id", report.PostID,
		"element_id", report.ElementID,
		"start_date", start.Format(dateLayout),
		"rows_written", len(rows),
		"refresh", refresh,
	)
	return nil
}

// BuildThousandVotesHistory writes today's rolling-window row for a report.
//
// DIVERGENCE FROM THE ORIGINAL, DELIBERATE.
//
// RankReportHistoryBuilder::buildThousandVotes maintains the window incrementally
// in a Redis entry, adding new votes and subtracting outdated ones:
//
//	$winCount = max(0, $winCount - $winOutdated + $winNew);
//
// That is an increment, not a recount, so unlike createElementRank it does not
// self-heal: once the cached counts and the real rounds disagree the error
// persists until the 30 day TTL expires or a refresh clears it, and the max(0, …)
// silently clamps negative drift.
//
// This port recomputes the window from the rounds every time. isThousandVotes-
// UpdatedToday means at most one run per element per day, and
// idx_rounds_game_winner and idx_rounds_game_loser already exist, so the cost is
// one bounded index scan per element per day in exchange for removing a whole
// class of permanent drift.
//
// Consequence to expect during comparison: on elements whose cached window had
// already drifted, this produces different numbers from Laravel. That is the
// correction, not a defect.
func (service *Service) BuildThousandVotesHistory(
	ctx context.Context,
	report RankReportRef,
	refresh bool,
) error {
	if err := service.requireHistory(report); err != nil {
		return err
	}

	if refresh {
		if _, err := service.history.SoftDeleteHistory(ctx, report.ID, HistoryRangeThousandVotes); err != nil {
			return fmt.Errorf("ranking: refresh thousand votes history for report %d: %w", report.ID, err)
		}
	}

	today := service.now()
	todayKey := today.Format(dateLayout)

	if !refresh {
		present, err := service.history.HistoryDatesPresent(ctx, report.ID, HistoryRangeThousandVotes)
		if err != nil {
			return fmt.Errorf("ranking: existing thousand votes dates for report %d: %w", report.ID, err)
		}
		if _, exists := present[todayKey]; exists {
			// Already built today. The original returns here too, which is what
			// bounds this to one run per element per day.
			return nil
		}
	}

	votes, err := service.history.RecentVotes(ctx, report.PostID, report.ElementID, ThousandVotesWindow)
	if err != nil {
		return fmt.Errorf("ranking: recent votes for report %d: %w", report.ID, err)
	}
	if len(votes) == 0 {
		return nil
	}

	var winCount, loseCount int64
	for _, vote := range votes {
		if vote.Won {
			winCount++
		} else {
			loseCount++
		}
	}

	row := HistoryRow{
		RankReportID: report.ID,
		PostID:       report.PostID,
		ElementID:    report.ElementID,
		TimeRange:    HistoryRangeThousandVotes,
		StartDate:    today,
		Rank:         0,
		WinCount:     winCount,
		LoseCount:    loseCount,
		WinRate:      WinRate(winCount, winCount+loseCount),
		// The original writes zeros for these on the thousand-votes row.
		ChampionCount:     0,
		GameCompleteCount: 0,
		ChampionRate:      0,
	}
	if err := service.history.UpsertHistoryRow(ctx, row); err != nil {
		return fmt.Errorf("ranking: write thousand votes history for report %d: %w", report.ID, err)
	}
	if err := service.pending.Add(ctx, report.PostID, HistoryRangeThousandVotes, []string{todayKey}); err != nil {
		return fmt.Errorf("ranking: record pending date for report %d: %w", report.ID, err)
	}

	service.logger.Info("rank_history_thousand_votes_built",
		"rank_report_id", report.ID,
		"post_id", report.PostID,
		"element_id", report.ElementID,
		"start_date", todayKey,
		"votes_in_window", len(votes),
		"win_count", winCount,
		"lose_count", loseCount,
		"refresh", refresh,
	)
	return nil
}

// historyStartDate resolves where the walk begins: the newest recorded date,
// else the caller's start, else the post's creation date.
func (service *Service) historyStartDate(
	ctx context.Context,
	report RankReportRef,
	timeRange HistoryTimeRange,
	startAt time.Time,
) (time.Time, error) {
	latest, err := service.history.LatestHistoryStartDate(ctx, report.ID, timeRange)
	if err != nil {
		return time.Time{}, fmt.Errorf("ranking: latest history date for report %d: %w", report.ID, err)
	}
	if !latest.IsZero() {
		return latest, nil
	}
	if !startAt.IsZero() {
		return startAt, nil
	}
	if report.PostCreatedAt.IsZero() {
		return time.Time{}, fmt.Errorf("ranking: report %d has no history, no start date and no post creation date", report.ID)
	}
	return report.PostCreatedAt, nil
}

func (service *Service) requireHistory(report RankReportRef) error {
	if service.history == nil {
		return fmt.Errorf("ranking: history repository is not configured")
	}
	if service.pending == nil {
		return fmt.Errorf("ranking: pending dates store is not configured")
	}
	if report.ID <= 0 || report.PostID <= 0 || report.ElementID <= 0 {
		return fmt.Errorf("ranking: report needs id, post id and element id, got %+v", report)
	}
	return nil
}
