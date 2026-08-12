package ranking

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// BuildReportRows turns the day's rank rows and the existing reports into the
// rows to write. It is pure so the ordering rules can be tested without a
// database.
//
// Port of the first half of RankService::createRankReports.
func BuildReportRows(
	postID int64,
	baseRanks []BaseRank,
	existing map[int64]ExistingReport,
	now time.Time,
) []ReportRow {
	championPosition, championRate := positionsFor(baseRanks, RankTypeChampion)
	pkPosition, pkRate := positionsFor(baseRanks, RankTypePKKing)

	// Every element that either has a rank today or already has a report. An
	// element that stopped being played keeps its stored values rather than
	// dropping out of the table.
	elementIDs := make([]int64, 0, len(existing)+len(championPosition)+len(pkPosition))
	seen := make(map[int64]struct{}, len(elementIDs))
	appendID := func(id int64) {
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		elementIDs = append(elementIDs, id)
	}
	for id := range existing {
		appendID(id)
	}
	for id := range championPosition {
		appendID(id)
	}
	for id := range pkPosition {
		appendID(id)
	}
	// Iterating a Go map is randomised, so the collected order is sorted before
	// anything depends on it. The PHP original inherits an unspecified database
	// order here; see the note on rankAll.
	sort.Slice(elementIDs, func(left, right int) bool { return elementIDs[left] < elementIDs[right] })

	rows := make([]ReportRow, 0, len(elementIDs))
	for _, elementID := range elementIDs {
		report, hasReport := existing[elementID]

		row := ReportRow{
			PostID:    postID,
			ElementID: elementID,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// A fresh position wins; otherwise the stored one is kept, so an element
		// with no games today does not lose its standing.
		if position, ok := championPosition[elementID]; ok {
			value := position
			row.FinalWinPosition = &value
		} else if hasReport {
			row.FinalWinPosition = report.FinalWinPosition
		}
		if position, ok := pkPosition[elementID]; ok {
			value := position
			row.WinPosition = &value
		} else if hasReport {
			row.WinPosition = report.WinPosition
		}

		// Rates fall back to the stored value, then to zero. The column is
		// nullable but the original writes 0, so this does too.
		if rate, ok := championRate[elementID]; ok {
			row.FinalWinRate = rate
		} else if hasReport && report.FinalWinRate != nil {
			row.FinalWinRate = *report.FinalWinRate
		}
		if rate, ok := pkRate[elementID]; ok {
			row.WinRate = rate
		} else if hasReport && report.WinRate != nil {
			row.WinRate = *report.WinRate
		}

		// created_at is preserved so an update does not look like an insert.
		if hasReport && report.CreatedAt != nil {
			row.CreatedAt = *report.CreatedAt
		}

		rows = append(rows, row)
	}

	rankAll(rows)
	return rows
}

// positionsFor ranks one rank_type by win_rate then win_count and returns each
// element's 1-based position along with its rate.
func positionsFor(baseRanks []BaseRank, rankType RankType) (map[int64]int64, map[int64]float64) {
	ofType := make([]BaseRank, 0, len(baseRanks))
	for _, rank := range baseRanks {
		if rank.RankType == rankType {
			ofType = append(ofType, rank)
		}
	}

	sort.Slice(ofType, func(left, right int) bool {
		if ofType[left].WinRate != ofType[right].WinRate {
			return ofType[left].WinRate > ofType[right].WinRate
		}
		if ofType[left].WinCount != ofType[right].WinCount {
			return ofType[left].WinCount > ofType[right].WinCount
		}
		// Deterministic tie-break; see the note on rankAll.
		return ofType[left].ElementID < ofType[right].ElementID
	})

	positions := make(map[int64]int64, len(ofType))
	rates := make(map[int64]float64, len(ofType))
	for index, rank := range ofType {
		positions[rank.ElementID] = int64(index + 1)
		rates[rank.ElementID] = rank.WinRate
	}
	return positions, rates
}

// rankAll assigns the overall 1-based rank by win_rate then final_win_rate.
//
// PARITY NOTE. The PHP original sorts with usort, which is stable on PHP 8, over
// an array whose order came from a query with no ORDER BY. Elements tied on both
// rates therefore receive ranks in an order MySQL never promised and that nothing
// can reproduce. This port breaks such ties on element_id so the same input always
// yields the same ranks. For tied elements the assigned rank may differ from the
// PHP value; the set of ranks and every untied element's rank are identical.
func rankAll(rows []ReportRow) {
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].WinRate != rows[right].WinRate {
			return rows[left].WinRate > rows[right].WinRate
		}
		if rows[left].FinalWinRate != rows[right].FinalWinRate {
			return rows[left].FinalWinRate > rows[right].FinalWinRate
		}
		return rows[left].ElementID < rows[right].ElementID
	})
	for index := range rows {
		rows[index].Rank = index + 1
	}
}

// SortReportRowsForWrite orders rows by element_id before writing.
//
// This is deliberate, not cosmetic: two concurrent runs that lock rows in the
// same order deadlock far less often than two that interleave. The original does
// the same immediately before its upsert.
func SortReportRowsForWrite(rows []ReportRow) {
	sort.Slice(rows, func(left, right int) bool { return rows[left].ElementID < rows[right].ElementID })
}

// CreateRankReports recomputes the post's rank_reports from today's rank rows.
//
// Port of RankService::createRankReports.
func (service *Service) CreateRankReports(ctx context.Context, postID int64) error {
	if postID <= 0 {
		return fmt.Errorf("ranking: post id is required, got %d", postID)
	}
	if service.reports == nil {
		return fmt.Errorf("ranking: report repository is not configured")
	}

	recordDate := service.now()

	baseRanks, err := service.reports.BaseRanks(ctx, postID, recordDate)
	if err != nil {
		return fmt.Errorf("ranking: base ranks for post %d: %w", postID, err)
	}
	existing, err := service.reports.ExistingReports(ctx, postID)
	if err != nil {
		return fmt.Errorf("ranking: existing reports for post %d: %w", postID, err)
	}

	rows := BuildReportRows(postID, baseRanks, existing, recordDate)
	SortReportRowsForWrite(rows)

	hidden, err := service.reports.UpsertReports(ctx, postID, rows)
	if err != nil {
		return fmt.Errorf("ranking: write reports for post %d: %w", postID, err)
	}

	service.logger.Info("rank_reports_updated",
		"post_id", postID,
		"record_date", recordDate.Format("2006-01-02"),
		"base_ranks", len(baseRanks),
		"existing_reports", len(existing),
		"rows_written", len(rows),
		"hidden_for_deleted_elements", hidden,
	)
	return nil
}
