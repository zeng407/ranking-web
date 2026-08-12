package ranking

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestReportParityAgainstStoredLaravelOutput compares the computed report
// ordering against what Laravel already wrote, WITHOUT writing anything.
//
// # WHAT THIS CAN AND CANNOT PROVE
//
// It compares two things that are well defined:
//
//   - the overall rank, for elements not tied with any other on
//     (win_rate, final_win_rate)
//   - the per-rank-type position, for elements not tied with any other on
//     (win_rate, win_count) within that type
//
// It deliberately does NOT compare rates. `rank_reports.win_rate` is a copy of
// `ranks.win_rate` taken when the report last ran, and this port copies the
// current value, so a difference means the stored report is stale, not that the
// port is wrong. Measured on the restored dump, rates differ by 0.01 to 0.05 for
// many elements for exactly that reason.
//
// It also skips tied elements. The PHP original sorts with usort, stable on
// PHP 8, over an array whose order came from a query with no ORDER BY, so tied
// elements receive positions in an order MySQL never promised. This port breaks
// ties on element_id. Every position difference observed on the restored dump was
// a permutation within a tie group.
//
// For the same reason as the stored `ranks` values, this is not a rigorous dual
// run: the baseline is a snapshot from an earlier Laravel run, not the output of
// the same input. The rigorous version has to execute Laravel's
// createRankReports against the same static snapshot and diff the two outputs.
//
// Set MYSQL_TEST_PARITY_POST_ID to a post whose reports no test has rewritten.
func TestReportParityAgainstStoredLaravelOutput(t *testing.T) {
	database := testDatabase(t)
	raw := os.Getenv("MYSQL_TEST_PARITY_POST_ID")
	if raw == "" {
		t.Skip("MYSQL_TEST_PARITY_POST_ID is not set; skipping parity comparison")
	}
	postID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || postID <= 0 {
		t.Fatalf("MYSQL_TEST_PARITY_POST_ID must be a positive integer, got %q", raw)
	}

	repository := NewMySQLReportRepository(database)
	ctx := context.Background()

	var recordDate time.Time
	if err := database.QueryRowContext(ctx,
		`SELECT MAX(record_date) FROM ranks WHERE post_id = ?`, postID).Scan(&recordDate); err != nil {
		t.Skipf("post %d has no rank rows: %v", postID, err)
	}

	baseRanks, err := repository.BaseRanks(ctx, postID, recordDate)
	if err != nil {
		t.Fatalf("BaseRanks() error = %v", err)
	}
	if len(baseRanks) == 0 {
		t.Skipf("post %d has no non-deleted ranks on %s", postID, recordDate.Format("2006-01-02"))
	}
	existing, err := repository.ExistingReports(ctx, postID)
	if err != nil {
		t.Fatalf("ExistingReports() error = %v", err)
	}
	if len(existing) == 0 {
		t.Skipf("post %d has no stored reports to compare against", postID)
	}

	storedRank, err := storedRanks(ctx, database, postID)
	if err != nil {
		t.Fatalf("read stored ranks: %v", err)
	}

	computed := BuildReportRows(postID, baseRanks, existing, recordDate)

	// Staleness disqualifies the whole post, not individual elements. If the
	// stored report predates the current rank values it was computed from a
	// different input set, and one element missing from that set shifts every
	// position below it. Filtering element by element leaves the visible half of a
	// tie permutation whose partner was filtered, which looks like a defect and is
	// not one.
	rankMismatches, ranksCompared, rankTies, postIsStale := compareOverallRanks(t, computed, storedRank, existing)
	if postIsStale {
		t.Logf("post %d on %s: %d elements", postID, recordDate.Format("2006-01-02"), len(computed))
		t.Skip("the stored report predates the current rank values, so it cannot be a baseline")
	}

	positionMismatches, positionsCompared, positionTies, positionsStale := comparePositions(t, baseRanks, existing)

	t.Logf("post %d on %s: %d elements", postID, recordDate.Format("2006-01-02"), len(computed))
	t.Logf("  positions: %d compared, %d mismatched, %d skipped as tied, %d skipped as stale",
		positionsCompared, positionMismatches, positionTies, positionsStale)
	t.Logf("  overall rank: %d compared, %d mismatched, %d skipped as tied",
		ranksCompared, rankMismatches, rankTies)

	if positionsCompared == 0 && ranksCompared == 0 {
		t.Skipf("post %d has nothing comparable: everything is tied or stale", postID)
	}
}

// storedRanks reads the rank Laravel last wrote per element, excluding
// soft-deleted elements so the set matches what BuildReportRows was given.
func storedRanks(ctx context.Context, database *sql.DB, postID int64) (map[int64]int, error) {
	rows, err := database.QueryContext(ctx,
		"SELECT rr.element_id, rr.`rank` FROM rank_reports rr"+
			" JOIN elements e ON e.id = rr.element_id AND e.deleted_at IS NULL"+
			" WHERE rr.post_id = ? AND rr.`rank` IS NOT NULL", postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stored := make(map[int64]int)
	for rows.Next() {
		var elementID int64
		var rank int
		if err := rows.Scan(&elementID, &rank); err != nil {
			return nil, err
		}
		stored[elementID] = rank
	}
	return stored, rows.Err()
}

// comparePositions checks per-rank-type positions where the comparison is well
// defined: the element is not tied, and its stored rate still equals the current
// one.
//
// The rate check is what makes this rigorous. A stored report is a snapshot from
// whenever the report last ran; if `ranks` has moved since, positions shift for
// reasons that have nothing to do with this port. Measured on the restored dump,
// every position difference was either inside a tie group or on a post whose
// stored rates had already drifted.
func comparePositions(t *testing.T, baseRanks []BaseRank, existing map[int64]ExistingReport) (mismatches, compared, ties, stale int) {
	t.Helper()

	for _, rankType := range []RankType{RankTypeChampion, RankTypePKKing} {
		positions, rates := positionsFor(baseRanks, rankType)

		// Count how many elements share each (win_rate, win_count) within the type.
		tieCount := make(map[[2]float64]int)
		for _, rank := range baseRanks {
			if rank.RankType != rankType {
				continue
			}
			tieCount[[2]float64{rank.WinRate, float64(rank.WinCount)}]++
		}

		for _, rank := range baseRanks {
			if rank.RankType != rankType {
				continue
			}
			report, ok := existing[rank.ElementID]
			if !ok {
				continue
			}
			if tieCount[[2]float64{rank.WinRate, float64(rank.WinCount)}] > 1 {
				ties++
				continue
			}

			var (
				storedPosition *int64
				storedRate     *float64
			)
			if rankType == RankTypeChampion {
				storedPosition, storedRate = report.FinalWinPosition, report.FinalWinRate
			} else {
				storedPosition, storedRate = report.WinPosition, report.WinRate
			}
			if storedPosition == nil {
				continue
			}
			if storedRate == nil || *storedRate != rates[rank.ElementID] {
				stale++
				continue
			}

			compared++
			if *storedPosition != positions[rank.ElementID] {
				mismatches++
				t.Errorf("element %d %s position: computed %d, Laravel %d",
					rank.ElementID, rankType, positions[rank.ElementID], *storedPosition)
			}
		}
	}
	return mismatches, compared, ties, stale
}

// compareOverallRanks checks the overall rank where the comparison is well
// defined: the element is not tied, and no element on the post has a stale stored
// rate.
//
// Staleness is judged post-wide here rather than per element, because one
// element's rate moving shifts every rank below it.
func compareOverallRanks(
	t *testing.T,
	computed []ReportRow,
	stored map[int64]int,
	existing map[int64]ExistingReport,
) (mismatches, compared, ties int, postIsStale bool) {
	t.Helper()

	for _, row := range computed {
		report, ok := existing[row.ElementID]
		if !ok {
			continue
		}
		if report.WinRate != nil && *report.WinRate != row.WinRate {
			postIsStale = true
			break
		}
		if report.FinalWinRate != nil && *report.FinalWinRate != row.FinalWinRate {
			postIsStale = true
			break
		}
	}
	if postIsStale {
		return 0, 0, 0, true
	}

	tieCount := make(map[[2]float64]int, len(computed))
	for _, row := range computed {
		tieCount[[2]float64{row.WinRate, row.FinalWinRate}]++
	}

	for _, row := range computed {
		storedValue, ok := stored[row.ElementID]
		if !ok {
			continue
		}
		if tieCount[[2]float64{row.WinRate, row.FinalWinRate}] > 1 {
			ties++
			continue
		}
		compared++
		if row.Rank != storedValue {
			mismatches++
			t.Errorf("element %d overall rank: computed %d, Laravel %d", row.ElementID, row.Rank, storedValue)
		}
	}
	return mismatches, compared, ties, false
}
