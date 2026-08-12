package ranking

import (
	"context"
	"testing"
	"time"
)

// The queries must run against the real schema. `rank` is a reserved word in
// MySQL 8, so an unquoted identifier fails only here, never in a unit test.
func TestReportQueriesRunAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLReportRepository(database)
	postID, _ := fixtureIDs(t)
	ctx := context.Background()

	// A date with no rows is still a valid query.
	if _, err := repository.BaseRanks(ctx, postID, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("BaseRanks() error = %v", err)
	}
	if _, err := repository.ExistingReports(ctx, postID); err != nil {
		t.Fatalf("ExistingReports() error = %v", err)
	}
}

// Soft-deleted elements must not contribute ranks or reports, matching the
// whereHas('element') filters in the original.
func TestReportQueriesExcludeSoftDeletedElements(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLReportRepository(database)
	postID, _ := fixtureIDs(t)
	ctx := context.Background()

	reports, err := repository.ExistingReports(ctx, postID)
	if err != nil {
		t.Fatalf("ExistingReports() error = %v", err)
	}
	if len(reports) == 0 {
		t.Skipf("post %d has no rank reports in this database", postID)
	}

	var deletedIncluded int
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rank_reports rr
		  JOIN elements e ON e.id = rr.element_id
		 WHERE rr.post_id = ? AND e.deleted_at IS NOT NULL`, postID).Scan(&deletedIncluded)
	if err != nil {
		t.Fatalf("count deleted-element reports: %v", err)
	}
	if deletedIncluded == 0 {
		t.Skipf("post %d has no reports for soft-deleted elements to exclude", postID)
	}

	// Whatever the query returned must contain none of them.
	rows, err := database.QueryContext(ctx, `
		SELECT rr.element_id FROM rank_reports rr
		  JOIN elements e ON e.id = rr.element_id
		 WHERE rr.post_id = ? AND e.deleted_at IS NOT NULL`, postID)
	if err != nil {
		t.Fatalf("list deleted-element reports: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var elementID int64
		if err := rows.Scan(&elementID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, present := reports[elementID]; present {
			t.Fatalf("element %d is soft deleted but appeared in ExistingReports", elementID)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}

// End to end against the real schema, on a post with real rank rows: the report
// is rewritten and a second run is byte-identical apart from updated_at.
func TestCreateRankReportsAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	reportRepository := NewMySQLReportRepository(database)
	postID, _ := fixtureIDs(t)
	ctx := context.Background()

	// The report is built from ranks for a single record_date, so pick one the
	// post actually has rather than today's, which may be empty.
	var recordDate time.Time
	err := database.QueryRowContext(ctx,
		`SELECT MAX(record_date) FROM ranks WHERE post_id = ?`, postID).Scan(&recordDate)
	if err != nil || recordDate.IsZero() {
		t.Skipf("post %d has no rank rows in this database (%v)", postID, err)
	}

	baseRanks, err := reportRepository.BaseRanks(ctx, postID, recordDate)
	if err != nil {
		t.Fatalf("BaseRanks() error = %v", err)
	}
	if len(baseRanks) == 0 {
		t.Skipf("post %d has no non-deleted ranks on %s", postID, recordDate.Format("2006-01-02"))
	}

	service, err := NewService(Options{
		Repository: NewMySQLRepository(database),
		Reports:    reportRepository,
		Stats:      &fakeStats{},
		Logger:     quietRankLogger(),
		Location:   taipei(t),
		Now:        func() time.Time { return recordDate },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.CreateRankReports(ctx, postID); err != nil {
		t.Fatalf("CreateRankReports() error = %v", err)
	}

	type reportRow struct {
		rank             int
		finalWinPosition *int64
		winPosition      *int64
		finalWinRate     float64
		winRate          float64
	}
	// Only the rows the pass manages. It excludes soft-deleted elements from
	// ExistingReports, mirroring the PHP whereHas('element'), so their reports are
	// never rebuilt and keep whatever rank they last had — often NULL. Reading them
	// back here would assert a property this job does not establish; the invariant
	// below holds over the live rows. See TestDeletedElementReportsAreHiddenNotRanked
	// for what happens to the excluded ones.
	read := func() map[int64]reportRow {
		t.Helper()
		rows, err := database.QueryContext(ctx,
			"SELECT rr.element_id, rr.`rank`, rr.final_win_position, rr.final_win_rate,"+
				" rr.win_position, rr.win_rate"+
				" FROM rank_reports rr"+
				" JOIN elements e ON e.id = rr.element_id AND e.deleted_at IS NULL"+
				" WHERE rr.post_id = ?", postID)
		if err != nil {
			t.Fatalf("read reports: %v", err)
		}
		defer rows.Close()
		out := make(map[int64]reportRow)
		for rows.Next() {
			var (
				elementID        int64
				rank             *int
				finalWinPosition *int64
				finalWinRate     *float64
				winPosition      *int64
				winRate          *float64
			)
			if err := rows.Scan(&elementID, &rank, &finalWinPosition, &finalWinRate, &winPosition, &winRate); err != nil {
				t.Fatalf("scan report: %v", err)
			}
			value := reportRow{finalWinPosition: finalWinPosition, winPosition: winPosition}
			if rank != nil {
				value.rank = *rank
			}
			if finalWinRate != nil {
				value.finalWinRate = *finalWinRate
			}
			if winRate != nil {
				value.winRate = *winRate
			}
			out[elementID] = value
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("rows: %v", err)
		}
		return out
	}

	first := read()
	if len(first) == 0 {
		t.Fatal("no reports were written")
	}

	// Ranks must be a dense 1..N across the post.
	seen := make(map[int]bool, len(first))
	for elementID, row := range first {
		if row.rank < 1 || row.rank > len(first) {
			t.Fatalf("element %d rank %d out of range for %d rows", elementID, row.rank, len(first))
		}
		if seen[row.rank] {
			t.Fatalf("rank %d assigned twice", row.rank)
		}
		seen[row.rank] = true
	}

	// Recomputing from the same static data must land on the same values.
	if err := service.CreateRankReports(ctx, postID); err != nil {
		t.Fatalf("second CreateRankReports() error = %v", err)
	}
	second := read()
	if len(second) != len(first) {
		t.Fatalf("row count changed: %d then %d", len(first), len(second))
	}
	for elementID, want := range first {
		got, ok := second[elementID]
		if !ok {
			t.Fatalf("element %d disappeared on the second run", elementID)
		}
		if got.rank != want.rank || got.winRate != want.winRate || got.finalWinRate != want.finalWinRate {
			t.Fatalf("element %d drifted: %#v then %#v", elementID, want, got)
		}
	}
}

// A report for a soft-deleted element is hidden rather than re-ranked.
//
// Two things have to hold together and neither is obvious from the code alone: the
// element is excluded from the rank sequence, so it cannot consume a position in
// 1..N; and its row is flagged hidden, so nothing reads it. Before this test the
// dense-rank assertion above accidentally covered these rows, which made it fail on
// a post whose elements had been deleted rather than reporting the real behaviour.
func TestDeletedElementReportsAreHiddenNotRanked(t *testing.T) {
	database := testDatabase(t)
	ctx := context.Background()

	// Any post that has both live and deleted elements with reports. Picking from the
	// data rather than a fixture id keeps this meaningful on any restore.
	var postID int64
	err := database.QueryRowContext(ctx, `
		SELECT rr.post_id
		  FROM rank_reports rr
		  JOIN elements e ON e.id = rr.element_id
		 WHERE e.deleted_at IS NOT NULL
		 GROUP BY rr.post_id
		 ORDER BY COUNT(*) DESC
		 LIMIT 1`).Scan(&postID)
	if err != nil {
		t.Skipf("no post has reports for deleted elements in this database (%v)", err)
	}

	reportRepository := NewMySQLReportRepository(database)
	service, err := NewService(Options{
		Repository: NewMySQLRepository(database),
		Reports:    reportRepository,
		Stats:      &fakeStats{},
		Logger:     quietRankLogger(),
		Location:   taipei(t),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.CreateRankReports(ctx, postID); err != nil {
		t.Fatalf("CreateRankReports() error = %v", err)
	}

	// Every report for a deleted element must be hidden.
	var visibleDeleted int
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM rank_reports rr
		  JOIN elements e ON e.id = rr.element_id
		 WHERE rr.post_id = ? AND e.deleted_at IS NOT NULL AND rr.hidden = 0`, postID).Scan(&visibleDeleted)
	if err != nil {
		t.Fatalf("count visible deleted reports: %v", err)
	}
	if visibleDeleted != 0 {
		t.Errorf("post %d has %d visible reports for deleted elements", postID, visibleDeleted)
	}

	// And the live rows must still be a dense 1..N, which is only possible if the
	// deleted ones took no position in the sequence.
	rows, err := database.QueryContext(ctx,
		"SELECT rr.`rank` FROM rank_reports rr"+
			" JOIN elements e ON e.id = rr.element_id AND e.deleted_at IS NULL"+
			" WHERE rr.post_id = ?", postID)
	if err != nil {
		t.Fatalf("read live ranks: %v", err)
	}
	defer rows.Close()

	seen := make(map[int]bool)
	total := 0
	for rows.Next() {
		var rank *int
		if err := rows.Scan(&rank); err != nil {
			t.Fatalf("scan rank: %v", err)
		}
		total++
		if rank == nil {
			t.Fatalf("a live element has no rank after the pass")
		}
		if seen[*rank] {
			t.Fatalf("rank %d assigned twice", *rank)
		}
		seen[*rank] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for rank := 1; rank <= total; rank++ {
		if !seen[rank] {
			t.Fatalf("rank %d is missing from a sequence of %d live reports", rank, total)
		}
	}
	t.Logf("post %d: %d live reports ranked 1..%d, every deleted element's report hidden",
		postID, total, total)
}
