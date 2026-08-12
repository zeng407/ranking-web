package ranking

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeReports struct {
	baseRanks    []BaseRank
	existing     map[int64]ExistingReport
	written      []ReportRow
	writtenPosts []int64
	hidden       int64

	baseErr     error
	existingErr error
	writeErr    error
}

func (repository *fakeReports) BaseRanks(context.Context, int64, time.Time) ([]BaseRank, error) {
	if repository.baseErr != nil {
		return nil, repository.baseErr
	}
	return repository.baseRanks, nil
}

func (repository *fakeReports) ExistingReports(context.Context, int64) (map[int64]ExistingReport, error) {
	if repository.existingErr != nil {
		return nil, repository.existingErr
	}
	if repository.existing == nil {
		return map[int64]ExistingReport{}, nil
	}
	return repository.existing, nil
}

func (repository *fakeReports) UpsertReports(_ context.Context, postID int64, rows []ReportRow) (int64, error) {
	if repository.writeErr != nil {
		return 0, repository.writeErr
	}
	repository.writtenPosts = append(repository.writtenPosts, postID)
	repository.written = append([]ReportRow(nil), rows...)
	return repository.hidden, nil
}

func pointerTo[T any](value T) *T { return &value }

func rowByElement(t *testing.T, rows []ReportRow, elementID int64) ReportRow {
	t.Helper()
	for _, row := range rows {
		if row.ElementID == elementID {
			return row
		}
	}
	t.Fatalf("no row for element %d in %#v", elementID, rows)
	return ReportRow{}
}

var reportNow = time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)

// Positions come from win_rate then win_count within each rank type.
func TestBuildReportRowsAssignsPositionsPerRankType(t *testing.T) {
	base := []BaseRank{
		{ElementID: 1, RankType: RankTypePKKing, WinRate: 50, WinCount: 5},
		{ElementID: 2, RankType: RankTypePKKing, WinRate: 90, WinCount: 9},
		{ElementID: 3, RankType: RankTypePKKing, WinRate: 50, WinCount: 8},
		{ElementID: 2, RankType: RankTypeChampion, WinRate: 10, WinCount: 1},
		{ElementID: 3, RankType: RankTypeChampion, WinRate: 20, WinCount: 2},
	}

	rows := BuildReportRows(7, base, nil, reportNow)

	// pk: 2 (90) then 3 (50, wins 8) then 1 (50, wins 5)
	if got := *rowByElement(t, rows, 2).WinPosition; got != 1 {
		t.Errorf("element 2 win position = %d, want 1", got)
	}
	if got := *rowByElement(t, rows, 3).WinPosition; got != 2 {
		t.Errorf("element 3 win position = %d, want 2: higher win_count breaks the rate tie", got)
	}
	if got := *rowByElement(t, rows, 1).WinPosition; got != 3 {
		t.Errorf("element 1 win position = %d, want 3", got)
	}

	// champion: 3 (20) then 2 (10); element 1 has no champion row at all
	if got := *rowByElement(t, rows, 3).FinalWinPosition; got != 1 {
		t.Errorf("element 3 final win position = %d, want 1", got)
	}
	if rowByElement(t, rows, 1).FinalWinPosition != nil {
		t.Error("element 1 must have no champion position")
	}
}

// Overall rank is win_rate then final_win_rate.
func TestBuildReportRowsAssignsOverallRank(t *testing.T) {
	base := []BaseRank{
		{ElementID: 1, RankType: RankTypePKKing, WinRate: 80},
		{ElementID: 2, RankType: RankTypePKKing, WinRate: 80},
		{ElementID: 3, RankType: RankTypePKKing, WinRate: 95},
		{ElementID: 1, RankType: RankTypeChampion, WinRate: 5},
		{ElementID: 2, RankType: RankTypeChampion, WinRate: 40},
	}

	rows := BuildReportRows(7, base, nil, reportNow)

	if got := rowByElement(t, rows, 3).Rank; got != 1 {
		t.Errorf("element 3 rank = %d, want 1", got)
	}
	// Tied on win_rate 80, so final_win_rate decides: 40 beats 5.
	if got := rowByElement(t, rows, 2).Rank; got != 2 {
		t.Errorf("element 2 rank = %d, want 2", got)
	}
	if got := rowByElement(t, rows, 1).Rank; got != 3 {
		t.Errorf("element 1 rank = %d, want 3", got)
	}
}

// An element with no rank today keeps its stored position and rate rather than
// dropping out of the report.
func TestBuildReportRowsFallsBackToStoredValues(t *testing.T) {
	existing := map[int64]ExistingReport{
		42: {
			ElementID:        42,
			FinalWinPosition: pointerTo(int64(3)),
			FinalWinRate:     pointerTo(12.5),
			WinPosition:      pointerTo(int64(7)),
			WinRate:          pointerTo(66.6),
			CreatedAt:        pointerTo(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	}

	rows := BuildReportRows(7, nil, existing, reportNow)

	row := rowByElement(t, rows, 42)
	if *row.FinalWinPosition != 3 || *row.WinPosition != 7 {
		t.Fatalf("positions = %#v", row)
	}
	if row.FinalWinRate != 12.5 || row.WinRate != 66.6 {
		t.Fatalf("rates = %#v", row)
	}
	// created_at must be preserved so an update does not look like an insert.
	if !row.CreatedAt.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("CreatedAt = %s, want the stored value", row.CreatedAt)
	}
	if !row.UpdatedAt.Equal(reportNow) {
		t.Fatalf("UpdatedAt = %s, want now", row.UpdatedAt)
	}
}

// A fresh rank overrides the stored one.
func TestBuildReportRowsPrefersFreshValues(t *testing.T) {
	existing := map[int64]ExistingReport{
		42: {ElementID: 42, WinPosition: pointerTo(int64(9)), WinRate: pointerTo(10.0)},
	}
	base := []BaseRank{{ElementID: 42, RankType: RankTypePKKing, WinRate: 77, WinCount: 3}}

	rows := BuildReportRows(7, base, existing, reportNow)

	row := rowByElement(t, rows, 42)
	if *row.WinPosition != 1 {
		t.Fatalf("WinPosition = %d, want the fresh position 1", *row.WinPosition)
	}
	if row.WinRate != 77 {
		t.Fatalf("WinRate = %v, want the fresh rate 77", row.WinRate)
	}
}

// An element with neither a rank nor a stored rate gets zero, matching the
// original, rather than NULL.
func TestBuildReportRowsDefaultsMissingRatesToZero(t *testing.T) {
	existing := map[int64]ExistingReport{42: {ElementID: 42}}

	row := rowByElement(t, BuildReportRows(7, nil, existing, reportNow), 42)

	if row.FinalWinRate != 0 || row.WinRate != 0 {
		t.Fatalf("rates = %#v, want zero", row)
	}
	if row.FinalWinPosition != nil || row.WinPosition != nil {
		t.Fatalf("positions = %#v, want nil", row)
	}
}

// Every element that has either a rank today or an existing report appears
// exactly once.
func TestBuildReportRowsUnionsElementsWithoutDuplicates(t *testing.T) {
	base := []BaseRank{
		{ElementID: 1, RankType: RankTypePKKing, WinRate: 10},
		{ElementID: 1, RankType: RankTypeChampion, WinRate: 10},
		{ElementID: 2, RankType: RankTypeChampion, WinRate: 20},
	}
	existing := map[int64]ExistingReport{1: {ElementID: 1}, 3: {ElementID: 3}}

	rows := BuildReportRows(7, base, existing, reportNow)

	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (elements 1, 2, 3)", len(rows))
	}
	seen := make(map[int64]int)
	for _, row := range rows {
		seen[row.ElementID]++
	}
	for _, elementID := range []int64{1, 2, 3} {
		if seen[elementID] != 1 {
			t.Errorf("element %d appears %d times, want 1", elementID, seen[elementID])
		}
	}
}

// Ranks must be a dense 1..N with no gaps or repeats.
func TestBuildReportRowsProducesDenseRanks(t *testing.T) {
	base := make([]BaseRank, 0, 20)
	for id := int64(1); id <= 20; id++ {
		base = append(base, BaseRank{ElementID: id, RankType: RankTypePKKing, WinRate: float64(id % 5)})
	}

	rows := BuildReportRows(7, base, nil, reportNow)

	seen := make(map[int]bool, len(rows))
	for _, row := range rows {
		if row.Rank < 1 || row.Rank > len(rows) {
			t.Fatalf("rank %d out of range for %d rows", row.Rank, len(rows))
		}
		if seen[row.Rank] {
			t.Fatalf("rank %d assigned twice", row.Rank)
		}
		seen[row.Rank] = true
	}
	if len(seen) != len(rows) {
		t.Fatalf("got %d distinct ranks for %d rows", len(seen), len(rows))
	}
}

// The PHP original leaves tied elements in an order MySQL never promised. This
// port must at least be self-consistent: the same input always gives the same
// ranks.
func TestBuildReportRowsIsDeterministicForTies(t *testing.T) {
	base := []BaseRank{
		{ElementID: 3, RankType: RankTypePKKing, WinRate: 50, WinCount: 1},
		{ElementID: 1, RankType: RankTypePKKing, WinRate: 50, WinCount: 1},
		{ElementID: 2, RankType: RankTypePKKing, WinRate: 50, WinCount: 1},
	}
	existing := map[int64]ExistingReport{1: {ElementID: 1}, 2: {ElementID: 2}, 3: {ElementID: 3}}

	first := BuildReportRows(7, base, existing, reportNow)
	for run := 0; run < 10; run++ {
		again := BuildReportRows(7, base, existing, reportNow)
		for _, row := range again {
			if want := rowByElement(t, first, row.ElementID).Rank; row.Rank != want {
				t.Fatalf("element %d rank changed between runs: %d then %d", row.ElementID, want, row.Rank)
			}
		}
	}
	// Ties resolve on element_id, so the lowest id ranks first.
	if rowByElement(t, first, 1).Rank != 1 {
		t.Fatalf("element 1 rank = %d, want 1 for the lowest tied id", rowByElement(t, first, 1).Rank)
	}
}

// Consistent lock ordering is what keeps concurrent runs from deadlocking.
func TestSortReportRowsForWriteOrdersByElementID(t *testing.T) {
	rows := []ReportRow{{ElementID: 9}, {ElementID: 2}, {ElementID: 5}}

	SortReportRowsForWrite(rows)

	for index := 1; index < len(rows); index++ {
		if rows[index-1].ElementID > rows[index].ElementID {
			t.Fatalf("rows not ordered by element_id: %#v", rows)
		}
	}
}

func newReportService(t *testing.T, reports ReportRepository) *Service {
	t.Helper()
	service, err := NewService(Options{
		Repository: &fakeRepository{},
		Reports:    reports,
		Stats:      &fakeStats{},
		Logger:     quietRankLogger(),
		Location:   taipei(t),
		Now:        func() time.Time { return reportNow },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestCreateRankReportsWritesRowsSortedForLockOrdering(t *testing.T) {
	reports := &fakeReports{
		baseRanks: []BaseRank{
			{ElementID: 9, RankType: RankTypePKKing, WinRate: 10},
			{ElementID: 2, RankType: RankTypePKKing, WinRate: 90},
			{ElementID: 5, RankType: RankTypePKKing, WinRate: 50},
		},
		hidden: 2,
	}
	service := newReportService(t, reports)

	if err := service.CreateRankReports(context.Background(), 7); err != nil {
		t.Fatalf("CreateRankReports() error = %v", err)
	}

	if len(reports.written) != 3 {
		t.Fatalf("wrote %d rows, want 3", len(reports.written))
	}
	for index := 1; index < len(reports.written); index++ {
		if reports.written[index-1].ElementID > reports.written[index].ElementID {
			t.Fatalf("rows reached the repository unordered: %#v", reports.written)
		}
	}
	// Ranking must survive the reordering: element 2 has the best rate.
	if got := rowByElement(t, reports.written, 2).Rank; got != 1 {
		t.Fatalf("element 2 rank = %d, want 1", got)
	}
	if reports.writtenPosts[0] != 7 {
		t.Fatalf("post id = %d, want 7", reports.writtenPosts[0])
	}
}

func TestCreateRankReportsRejectsMissingPost(t *testing.T) {
	service := newReportService(t, &fakeReports{})

	if err := service.CreateRankReports(context.Background(), 0); err == nil {
		t.Fatal("a zero post id must be rejected")
	}
}

// The report path needs its own repository; without it the call must fail rather
// than silently do nothing.
func TestCreateRankReportsRequiresReportRepository(t *testing.T) {
	service, err := NewService(Options{
		Repository: &fakeRepository{},
		Stats:      &fakeStats{},
		Logger:     quietRankLogger(),
		Location:   taipei(t),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.CreateRankReports(context.Background(), 7); err == nil {
		t.Fatal("CreateRankReports() should fail without a report repository")
	}
}

func TestCreateRankReportsPropagatesErrors(t *testing.T) {
	cases := map[string]*fakeReports{
		"base ranks":       {baseErr: errors.New("connection reset")},
		"existing reports": {existingErr: errors.New("connection reset")},
		"write":            {writeErr: errors.New("deadlock found when trying to get lock")},
	}
	for name, reports := range cases {
		service := newReportService(t, reports)
		if err := service.CreateRankReports(context.Background(), 7); err == nil {
			t.Errorf("%s failure must be reported", name)
		}
	}
}

// An empty post is valid: nothing to rank, and the hidden sweep must still run.
func TestCreateRankReportsHandlesAnEmptyPost(t *testing.T) {
	reports := &fakeReports{}
	service := newReportService(t, reports)

	if err := service.CreateRankReports(context.Background(), 7); err != nil {
		t.Fatalf("CreateRankReports() error = %v", err)
	}
	if len(reports.writtenPosts) != 1 {
		t.Fatal("the repository must still be called so deleted elements get hidden")
	}
	if len(reports.written) != 0 {
		t.Fatalf("wrote %#v, want no rows", reports.written)
	}
}
