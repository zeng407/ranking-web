package publiccontent

import (
	"strings"
	"testing"
)

func TestPostFiltersAlwaysExcludeIncompleteReadModels(t *testing.T) {
	where, arguments := postFilters("")
	if where != " WHERE data IS NOT NULL" || len(arguments) != 0 {
		t.Fatalf("where = %q, arguments = %#v", where, arguments)
	}

	where, arguments = postFilters("#anime music")
	if !strings.Contains(where, "data IS NOT NULL") || strings.Count(where, "title LIKE ?") != 2 {
		t.Fatalf("where = %q", where)
	}
	if len(arguments) != 6 || arguments[0] != "%anime%" || arguments[3] != "%music%" {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestPostSortColumnUsesOnlyKnownColumns(t *testing.T) {
	for _, testCase := range []struct {
		sort, dateRange, expected string
	}{
		{"new", "week", "new_position"},
		{"hot", "day", "day_position"},
		{"hot", "month", "month_position"},
		{"hot", "all", "week_position"},
		{"unexpected", "unexpected", "week_position"},
	} {
		if actual := postSortColumn(testCase.sort, testCase.dateRange); actual != testCase.expected {
			t.Fatalf("postSortColumn(%q, %q) = %q", testCase.sort, testCase.dateRange, actual)
		}
	}
}

func TestRecentGroupReportUsesLatestValidSnapshot(t *testing.T) {
	currentRank := int64(2)
	current := &RankReport{
		Rank:    &currentRank,
		Element: RankElement{ID: 42},
	}
	history := []RankHistory{
		{Rank: 7, WinRate: "61.5", Date: "2026-08-04"},
		{Rank: 9, WinRate: "58.0", Date: "2026-08-03"},
	}

	report := recentGroupReport(current, history)
	if report == nil || report.Rank == nil || *report.Rank != 7 || report.WinRate != "61.5" || report.Date != "2026-08-04" || report.Element.ID != 42 {
		t.Fatalf("report = %#v", report)
	}
	if recentGroupReport(current, nil) != nil {
		t.Fatal("empty history should not produce a recent rank")
	}
	if recentGroupReport(current, []RankHistory{{Rank: 0}}) != nil {
		t.Fatal("an unreordered rank=0 snapshot should not be public")
	}
}

func TestChampionsQueryLeadsWithUserGameResults(t *testing.T) {
	// Left to the optimizer, this join starts from posts and materialises ~3.2M
	// rows into an on-disk temp table before sorting them down to five, which
	// took 9-13s against a 972k-row user_game_results. The hint plus the
	// ORDER BY on the driving table's primary key is what turns that into a
	// backward index scan; removing either brings the slow plan back.
	if !strings.Contains(championsQuery, "/*+ JOIN_PREFIX(ugr) */") {
		t.Fatal("champions query lost the JOIN_PREFIX(ugr) hint that keeps it off the filesort plan")
	}
	if !strings.Contains(championsQuery, "FROM user_game_results ugr") {
		t.Fatal("JOIN_PREFIX names ugr, so user_game_results has to stay the first table in FROM")
	}
	if !strings.Contains(championsQuery, "ORDER BY ugr.id DESC") {
		t.Fatal("ordering by the driving table's primary key is what removes the sort")
	}
	if !strings.Contains(championsQuery, "LIMIT ?") {
		t.Fatal("the limit has to stay bound so the scan can stop early")
	}
}

func TestRankVisibilityAdaptsToWhicheverColumnsExist(t *testing.T) {
	// rank_reports does not carry the same columns in every schema snapshot: one
	// restore has deleted_at and no hidden, another has hidden and no deleted_at.
	// Hard-coding either one made /api/v1/ranks fail with
	// "Unknown column 'rr.deleted_at' in 'where clause'" and a 503.
	for _, testCase := range []struct {
		name                    string
		hasDeletedAt, hasHidden bool
		expected                string
	}{
		{"both columns", true, true, " AND rr.deleted_at IS NULL AND rr.hidden = 0"},
		{"soft delete only", true, false, " AND rr.deleted_at IS NULL"},
		{"moderation flag only", false, true, " AND rr.hidden = 0"},
		{"neither column", false, false, ""},
	} {
		if got := rankVisibility(testCase.hasDeletedAt, testCase.hasHidden); got != testCase.expected {
			t.Errorf("%s: rankVisibility(%t, %t) = %q, want %q",
				testCase.name, testCase.hasDeletedAt, testCase.hasHidden, got, testCase.expected)
		}
	}
}

func TestRankQueriesDoNotHardcodeRankReportColumns(t *testing.T) {
	// The ranking queries must take their rank_reports filter from the probe
	// above; an inlined rr.deleted_at or rr.hidden reintroduces the 503 on any
	// database whose schema lacks that column.
	for _, query := range []string{rankReportSelect, recentRankReportSelect, cumulativeRankReportSelect} {
		if strings.Contains(query, "rr.deleted_at") || strings.Contains(query, "rr.hidden") {
			t.Errorf("query hardcodes a rank_reports column that is not present in every schema:\n%s", query)
		}
	}
}

func TestCumulativeRanksCarryTheThousandVoteStandingWithoutDroppingRows(t *testing.T) {
	// One table shows both standings, so the recent join must never decide which
	// elements appear: an element the snapshot left out still has an all-time
	// rank, and an inner join would silently shorten the page.
	if !strings.Contains(cumulativeRankReportSelect, "LEFT JOIN rank_report_histories recent") {
		t.Fatalf("the recent standing must be joined left:\n%s", cumulativeRankReportSelect)
	}
	// Rank 0 means "counted but not placed"; showing it as #0 reads as a real
	// standing, which is why the recent group elsewhere skips those rows too.
	if !strings.Contains(cumulativeRankReportSelect, "recent.rank > 0") {
		t.Fatalf("unplaced snapshot rows must not become a recent rank:\n%s", cumulativeRankReportSelect)
	}
	if !strings.Contains(cumulativeRankReportSelect, "recent.time_range = 'thousand_votes'") {
		t.Fatalf("the recent standing must come from the thousand-vote range:\n%s", cumulativeRankReportSelect)
	}
	// Both the join subquery and the caller's own filter take a post id, and the
	// subquery is read first: swapping them ranks a different post.
	if strings.Count(cumulativeRankReportSelect, "?") != 1 {
		t.Fatalf("the select must take exactly the post id of the snapshot subquery:\n%s", cumulativeRankReportSelect)
	}
}

func TestRecentGroupReportSkipsUnrankedSnapshots(t *testing.T) {
	rank := int64(3)
	current := &RankReport{Rank: &rank, WinRate: "80.4", Date: "2026-08-04"}

	// rank == 0 means "in this snapshot but not ranked", and it is usually the
	// newest row: 186 of 200 elements on one post had a zero-rank newest entry.
	// Reading history[0] blindly reported "no data" for nearly every element even
	// though the list endpoint showed a real rank one snapshot earlier.
	report := recentGroupReport(current, []RankHistory{
		{Rank: 0, WinRate: "80.4", Date: "2026-07-29"},
		{Rank: 4, WinRate: "80.4", Date: "2026-07-28"},
		{Rank: 4, WinRate: "80.8", Date: "2026-07-27"},
	})
	if report == nil {
		t.Fatal("a zero-rank newest snapshot must not hide the ranked snapshot behind it")
	}
	if report.Rank == nil || *report.Rank != 4 || report.Date != "2026-07-28" {
		t.Fatalf("report = %+v, want the newest ranked snapshot (#4 on 2026-07-28)", report)
	}

	if got := recentGroupReport(current, []RankHistory{{Rank: 0, Date: "2026-07-29"}}); got != nil {
		t.Fatalf("with no ranked snapshot at all the group stays empty, got %+v", got)
	}
	if got := recentGroupReport(nil, []RankHistory{{Rank: 4, Date: "2026-07-28"}}); got != nil {
		t.Fatalf("without a current report there is no element to attach, got %+v", got)
	}
}
