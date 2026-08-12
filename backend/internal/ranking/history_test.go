package ranking

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeHistory struct {
	latestStart  time.Time
	anchors      map[RankType]*DailyRank
	daily        []DailyRank
	present      map[string]struct{}
	recentVotes  []VoteOutcome
	inserted     []HistoryRow
	upserted     []HistoryRow
	softDeleted  []HistoryTimeRange
	insertErr    error
	upsertErr    error
	recentErr    error
	softDeleteEr error
}

func (repository *fakeHistory) LatestHistoryStartDate(context.Context, int64, HistoryTimeRange) (time.Time, error) {
	return repository.latestStart, nil
}

func (repository *fakeHistory) SoftDeleteHistory(_ context.Context, _ int64, timeRange HistoryTimeRange) (int64, error) {
	if repository.softDeleteEr != nil {
		return 0, repository.softDeleteEr
	}
	repository.softDeleted = append(repository.softDeleted, timeRange)
	return 1, nil
}

func (repository *fakeHistory) FirstRankOnOrAfter(_ context.Context, _, _ int64, _ time.Time, rankType RankType) (*DailyRank, error) {
	if repository.anchors == nil {
		return nil, nil
	}
	return repository.anchors[rankType], nil
}

func (repository *fakeHistory) RanksOnOrAfter(context.Context, int64, int64, time.Time) ([]DailyRank, error) {
	return repository.daily, nil
}

func (repository *fakeHistory) HistoryDatesPresent(context.Context, int64, HistoryTimeRange) (map[string]struct{}, error) {
	if repository.present == nil {
		return map[string]struct{}{}, nil
	}
	return repository.present, nil
}

func (repository *fakeHistory) InsertHistoryRows(_ context.Context, rows []HistoryRow) error {
	if repository.insertErr != nil {
		return repository.insertErr
	}
	repository.inserted = append(repository.inserted, rows...)
	return nil
}

func (repository *fakeHistory) UpsertHistoryRow(_ context.Context, row HistoryRow) error {
	if repository.upsertErr != nil {
		return repository.upsertErr
	}
	repository.upserted = append(repository.upserted, row)
	return nil
}

func (repository *fakeHistory) RecentVotes(_ context.Context, _, _ int64, limit int) ([]VoteOutcome, error) {
	if repository.recentErr != nil {
		return nil, repository.recentErr
	}
	if len(repository.recentVotes) > limit {
		return repository.recentVotes[:limit], nil
	}
	return repository.recentVotes, nil
}

type fakePending struct {
	added map[HistoryTimeRange][]string
	err   error
}

func (store *fakePending) Add(_ context.Context, _ int64, timeRange HistoryTimeRange, dates []string) error {
	if store.err != nil {
		return store.err
	}
	if store.added == nil {
		store.added = make(map[HistoryTimeRange][]string)
	}
	store.added[timeRange] = append(store.added[timeRange], dates...)
	return nil
}

func (store *fakePending) Pull(context.Context, int64, HistoryTimeRange) ([]string, error) {
	return nil, nil
}

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

var historyReport = RankReportRef{ID: 7, PostID: 46, ElementID: 2759, PostCreatedAt: day(2026, time.July, 1)}

func newHistoryService(t *testing.T, history HistoryRepository, pending PendingDatesStore, now time.Time) *Service {
	t.Helper()
	service, err := NewService(Options{
		Repository: &fakeRepository{},
		Stats:      &fakeStats{},
		History:    history,
		Pending:    pending,
		Logger:     quietRankLogger(),
		Location:   taipei(t),
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestBuildAllHistoryRequiresConfiguredDependencies(t *testing.T) {
	service, err := NewService(Options{
		Repository: &fakeRepository{}, Stats: &fakeStats{},
		Logger: quietRankLogger(), Location: taipei(t),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err == nil {
		t.Fatal("BuildAllHistory() should fail without a history repository")
	}
}

func TestBuildAllHistoryRejectsIncompleteReport(t *testing.T) {
	service := newHistoryService(t, &fakeHistory{}, &fakePending{}, day(2026, time.July, 10))

	for name, report := range map[string]RankReportRef{
		"no id":         {PostID: 1, ElementID: 1},
		"no post id":    {ID: 1, ElementID: 1},
		"no element id": {ID: 1, PostID: 1},
	} {
		if err := service.BuildAllHistory(context.Background(), report, false, time.Time{}); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

// An element that never played produces nothing, matching the sumRounds == 0
// bail-out.
func TestBuildAllHistoryWritesNothingWhenNobodyPlayed(t *testing.T) {
	history := &fakeHistory{}
	pending := &fakePending{}
	service := newHistoryService(t, history, pending, day(2026, time.July, 10))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	if len(history.inserted) != 0 {
		t.Fatalf("inserted %#v, want nothing", history.inserted)
	}
	if len(pending.added) != 0 {
		t.Fatal("no dates may be recorded when nothing was written")
	}
}

// The core behaviour: days with no rank row inherit the previous day's totals.
func TestBuildAllHistoryCarriesTotalsForward(t *testing.T) {
	history := &fakeHistory{
		anchors: map[RankType]*DailyRank{
			RankTypePKKing:   {RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
			RankTypeChampion: {RecordDate: day(2026, time.July, 5), RankType: RankTypeChampion, WinCount: 1, RoundCount: 8},
		},
		daily: []DailyRank{
			{RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
			{RecordDate: day(2026, time.July, 5), RankType: RankTypeChampion, WinCount: 1, RoundCount: 8},
			// No rows for the 6th or 7th.
			{RecordDate: day(2026, time.July, 8), RankType: RankTypePKKing, WinCount: 30, RoundCount: 50},
		},
	}
	pending := &fakePending{}
	service := newHistoryService(t, history, pending, day(2026, time.July, 9))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}

	// Days 5, 6, 7, 8 — today (the 9th) is excluded.
	if len(history.inserted) != 4 {
		t.Fatalf("inserted %d rows, want 4: %#v", len(history.inserted), history.inserted)
	}
	byDate := make(map[string]HistoryRow, len(history.inserted))
	for _, row := range history.inserted {
		byDate[row.StartDate.Format(dateLayout)] = row
	}

	fifth := byDate["2026-07-05"]
	if fifth.WinCount != 10 || fifth.LoseCount != 10 || fifth.WinRate != 50 {
		t.Fatalf("5th = %#v, want 10 wins of 20 at 50%%", fifth)
	}
	// The 6th and 7th have no rank row, so they repeat the 5th.
	for _, key := range []string{"2026-07-06", "2026-07-07"} {
		row := byDate[key]
		if row.WinCount != 10 || row.LoseCount != 10 {
			t.Fatalf("%s = %#v, want the 5th's totals carried forward", key, row)
		}
		if row.ChampionCount != 1 || row.GameCompleteCount != 8 {
			t.Fatalf("%s champion = %#v, want the 5th's carried forward", key, row)
		}
	}
	eighth := byDate["2026-07-08"]
	if eighth.WinCount != 30 || eighth.LoseCount != 20 || eighth.WinRate != 60 {
		t.Fatalf("8th = %#v, want 30 wins of 50 at 60%%", eighth)
	}
	// The champion values have no row on the 8th, so they still carry forward.
	if eighth.ChampionCount != 1 || eighth.GameCompleteCount != 8 {
		t.Fatalf("8th champion = %#v, want carried forward", eighth)
	}

	if got := len(pending.added[HistoryRangeAll]); got != 4 {
		t.Fatalf("recorded %d pending dates, want 4", got)
	}
}

// Rank is written as 0 for the reorder pass to fill in.
func TestBuildAllHistoryWritesRankZero(t *testing.T) {
	history := &fakeHistory{
		anchors: map[RankType]*DailyRank{
			RankTypePKKing: {RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
		},
		daily: []DailyRank{{RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20}},
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 6))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	for _, row := range history.inserted {
		if row.Rank != 0 {
			t.Fatalf("row %#v must be written with rank 0", row)
		}
	}
}

// An existing date is never rewritten.
func TestBuildAllHistorySkipsDatesAlreadyPresent(t *testing.T) {
	history := &fakeHistory{
		anchors: map[RankType]*DailyRank{
			RankTypePKKing: {RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
		},
		daily:   []DailyRank{{RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20}},
		present: map[string]struct{}{"2026-07-05": {}, "2026-07-06": {}},
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	if len(history.inserted) != 1 {
		t.Fatalf("inserted %#v, want only the 7th", history.inserted)
	}
	if got := history.inserted[0].StartDate.Format(dateLayout); got != "2026-07-07" {
		t.Fatalf("inserted %s, want 2026-07-07", got)
	}
}

// A day whose carried-forward win count is zero is skipped.
func TestBuildAllHistorySkipsDaysWithNoWins(t *testing.T) {
	history := &fakeHistory{
		anchors: map[RankType]*DailyRank{
			// Played 20 rounds, won none.
			RankTypePKKing: {RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 0, RoundCount: 20},
		},
		daily: []DailyRank{{RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 0, RoundCount: 20}},
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	if len(history.inserted) != 0 {
		t.Fatalf("inserted %#v, want nothing", history.inserted)
	}
}

func TestBuildAllHistoryRefreshSoftDeletesFirst(t *testing.T) {
	history := &fakeHistory{}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildAllHistory(context.Background(), historyReport, true, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	if len(history.softDeleted) != 1 || history.softDeleted[0] != HistoryRangeAll {
		t.Fatalf("softDeleted = %#v, want one all-range delete", history.softDeleted)
	}
}

// Pending dates must not be recorded when the insert failed, or the reorder pass
// would look for rows that do not exist.
func TestBuildAllHistoryDoesNotRecordPendingDatesWhenInsertFails(t *testing.T) {
	history := &fakeHistory{
		anchors: map[RankType]*DailyRank{
			RankTypePKKing: {RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
		},
		daily:     []DailyRank{{RecordDate: day(2026, time.July, 5), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20}},
		insertErr: errors.New("deadlock found when trying to get lock"),
	}
	pending := &fakePending{}
	service := newHistoryService(t, history, pending, day(2026, time.July, 8))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err == nil {
		t.Fatal("the insert failure must be reported")
	}
	if len(pending.added) != 0 {
		t.Fatalf("recorded %#v, want nothing", pending.added)
	}
}

// The walk starts from the newest recorded date rather than the post's creation.
func TestBuildAllHistoryResumesFromTheLatestRecordedDate(t *testing.T) {
	history := &fakeHistory{
		latestStart: day(2026, time.July, 6),
		anchors: map[RankType]*DailyRank{
			RankTypePKKing: {RecordDate: day(2026, time.July, 6), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20},
		},
		daily:   []DailyRank{{RecordDate: day(2026, time.July, 6), RankType: RankTypePKKing, WinCount: 10, RoundCount: 20}},
		present: map[string]struct{}{"2026-07-06": {}},
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildAllHistory(context.Background(), historyReport, false, time.Time{}); err != nil {
		t.Fatalf("BuildAllHistory() error = %v", err)
	}
	if len(history.inserted) != 1 || history.inserted[0].StartDate.Format(dateLayout) != "2026-07-07" {
		t.Fatalf("inserted %#v, want only the 7th", history.inserted)
	}
}

// --- thousand votes ---

func votes(wins, loses int) []VoteOutcome {
	out := make([]VoteOutcome, 0, wins+loses)
	id := int64(1000)
	for index := 0; index < wins; index++ {
		out = append(out, VoteOutcome{RoundID: id, Won: true})
		id--
	}
	for index := 0; index < loses; index++ {
		out = append(out, VoteOutcome{RoundID: id, Won: false})
		id--
	}
	return out
}

// The recompute: counts come straight from the returned rounds.
func TestBuildThousandVotesHistoryRecomputesFromRounds(t *testing.T) {
	history := &fakeHistory{recentVotes: votes(700, 300)}
	pending := &fakePending{}
	service := newHistoryService(t, history, pending, day(2026, time.July, 8))

	if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err != nil {
		t.Fatalf("BuildThousandVotesHistory() error = %v", err)
	}

	if len(history.upserted) != 1 {
		t.Fatalf("upserted %#v, want one row", history.upserted)
	}
	row := history.upserted[0]
	if row.WinCount != 700 || row.LoseCount != 300 || row.WinRate != 70 {
		t.Fatalf("row = %#v, want 700/300 at 70%%", row)
	}
	if row.TimeRange != HistoryRangeThousandVotes || row.Rank != 0 {
		t.Fatalf("row = %#v", row)
	}
	// The original writes zeros for the champion columns on this range.
	if row.ChampionCount != 0 || row.GameCompleteCount != 0 || row.ChampionRate != 0 {
		t.Fatalf("champion columns = %#v, want zero", row)
	}
	if got := pending.added[HistoryRangeThousandVotes]; len(got) != 1 || got[0] != "2026-07-08" {
		t.Fatalf("pending = %#v", got)
	}
}

// The window is capped, and the repository is asked for no more than the cap.
func TestBuildThousandVotesHistoryCapsTheWindow(t *testing.T) {
	history := &fakeHistory{recentVotes: votes(900, 400)}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err != nil {
		t.Fatalf("BuildThousandVotesHistory() error = %v", err)
	}
	row := history.upserted[0]
	if row.WinCount+row.LoseCount != ThousandVotesWindow {
		t.Fatalf("window = %d, want %d", row.WinCount+row.LoseCount, ThousandVotesWindow)
	}
}

// Recomputing is convergent: running it repeatedly produces identical values,
// which is the whole reason for diverging from the incremental original.
func TestBuildThousandVotesHistoryIsConvergent(t *testing.T) {
	for run := 1; run <= 3; run++ {
		history := &fakeHistory{recentVotes: votes(700, 300)}
		service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))
		if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err != nil {
			t.Fatalf("run %d error = %v", run, err)
		}
		row := history.upserted[0]
		if row.WinCount != 700 || row.LoseCount != 300 {
			t.Fatalf("run %d drifted: %#v", run, row)
		}
	}
}

// Already built today, so nothing happens. This is what bounds the recompute to
// once per element per day.
func TestBuildThousandVotesHistorySkipsWhenAlreadyBuiltToday(t *testing.T) {
	history := &fakeHistory{
		present:     map[string]struct{}{"2026-07-08": {}},
		recentVotes: votes(700, 300),
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err != nil {
		t.Fatalf("BuildThousandVotesHistory() error = %v", err)
	}
	if len(history.upserted) != 0 {
		t.Fatalf("upserted %#v, want nothing", history.upserted)
	}
}

// Refresh overrides the once-a-day guard.
func TestBuildThousandVotesHistoryRefreshRebuildsToday(t *testing.T) {
	history := &fakeHistory{
		present:     map[string]struct{}{"2026-07-08": {}},
		recentVotes: votes(700, 300),
	}
	service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))

	if err := service.BuildThousandVotesHistory(context.Background(), historyReport, true); err != nil {
		t.Fatalf("BuildThousandVotesHistory() error = %v", err)
	}
	if len(history.softDeleted) != 1 || history.softDeleted[0] != HistoryRangeThousandVotes {
		t.Fatalf("softDeleted = %#v", history.softDeleted)
	}
	if len(history.upserted) != 1 {
		t.Fatalf("upserted %#v, want one row", history.upserted)
	}
}

func TestBuildThousandVotesHistoryWritesNothingWithoutVotes(t *testing.T) {
	history := &fakeHistory{}
	pending := &fakePending{}
	service := newHistoryService(t, history, pending, day(2026, time.July, 8))

	if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err != nil {
		t.Fatalf("BuildThousandVotesHistory() error = %v", err)
	}
	if len(history.upserted) != 0 || len(pending.added) != 0 {
		t.Fatal("no votes means no row and no pending date")
	}
}

func TestBuildThousandVotesHistoryPropagatesErrors(t *testing.T) {
	cases := map[string]*fakeHistory{
		"recent votes": {recentErr: errors.New("connection reset")},
		"upsert":       {recentVotes: votes(1, 0), upsertErr: errors.New("deadlock")},
	}
	for name, history := range cases {
		service := newHistoryService(t, history, &fakePending{}, day(2026, time.July, 8))
		if err := service.BuildThousandVotesHistory(context.Background(), historyReport, false); err == nil {
			t.Errorf("%s failure must be reported", name)
		}
	}
}

func TestHistoryTimeRangeValidity(t *testing.T) {
	if !HistoryRangeAll.Valid() || !HistoryRangeThousandVotes.Valid() {
		t.Fatal("the two written ranges must be valid")
	}
	// week, month and year are never written; treating them as valid would invite
	// a caller to try.
	for _, dead := range []HistoryTimeRange{"week", "month", "year"} {
		if dead.Valid() {
			t.Errorf("%q must not be valid: it has no build path and no data", dead)
		}
	}
}
