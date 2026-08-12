package ranking

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeHistoryRanks struct {
	rows     []RankedHistoryRow
	applied  []RankedHistoryRow
	purged   int64
	readErr  error
	applyErr error
	purgeErr error
	// purgeCutoff records what the service asked for.
	purgeCutoff time.Time
	purgeLimit  int
}

func (repository *fakeHistoryRanks) HistoryRowsForRanking(
	context.Context, int64, HistoryTimeRange, time.Time,
) ([]RankedHistoryRow, error) {
	if repository.readErr != nil {
		return nil, repository.readErr
	}
	out := make([]RankedHistoryRow, len(repository.rows))
	copy(out, repository.rows)
	return out, nil
}

func (repository *fakeHistoryRanks) ApplyHistoryRanks(_ context.Context, rows []RankedHistoryRow) error {
	if repository.applyErr != nil {
		return repository.applyErr
	}
	repository.applied = append(repository.applied, rows...)
	return nil
}

func (repository *fakeHistoryRanks) PurgeHistoryOlderThan(
	_ context.Context, _ int64, cutoff time.Time, limit int,
) (int64, error) {
	repository.purgeCutoff = cutoff
	repository.purgeLimit = limit
	if repository.purgeErr != nil {
		return 0, repository.purgeErr
	}
	return repository.purged, nil
}

func newRankService(t *testing.T, ranks HistoryRankRepository, pending PendingDatesStore, now time.Time) *Service {
	t.Helper()
	service, err := NewService(Options{
		Repository:   &fakeRepository{},
		Stats:        &fakeStats{},
		HistoryRanks: ranks,
		Pending:      pending,
		Logger:       quietRankLogger(),
		Location:     taipei(t),
		Now:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func appliedByID(rows []RankedHistoryRow) map[int64]int64 {
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Rank
	}
	return out
}

// The documented order: win_rate, then champion_rate, then win_count, all
// descending.
func TestAssignHistoryRanksOrdersByRateThenChampionRateThenWinCount(t *testing.T) {
	ranks := &fakeHistoryRanks{rows: []RankedHistoryRow{
		{ID: 1, ElementID: 10, WinRate: 50, ChampionRate: 5, WinCount: 100},
		{ID: 2, ElementID: 20, WinRate: 90, ChampionRate: 1, WinCount: 10},
		{ID: 3, ElementID: 30, WinRate: 50, ChampionRate: 9, WinCount: 1},
		{ID: 4, ElementID: 40, WinRate: 50, ChampionRate: 5, WinCount: 200},
	}}
	service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))

	err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5))
	if err != nil {
		t.Fatalf("AssignHistoryRanks() error = %v", err)
	}

	got := appliedByID(ranks.applied)
	// 2 (rate 90); then rate 50 group: 3 (champion 9), then champion 5 group:
	// 4 (win 200) before 1 (win 100).
	for id, want := range map[int64]int64{2: 1, 3: 2, 4: 3, 1: 4} {
		if got[id] != want {
			t.Errorf("row %d rank = %d, want %d (applied %#v)", id, got[id], want, got)
		}
	}
}

// Ties resolve on element_id so the same input always gives the same ranks. The
// original has no final tie-break and inherits MySQL's unspecified order.
func TestAssignHistoryRanksIsDeterministicForTies(t *testing.T) {
	rows := []RankedHistoryRow{
		{ID: 3, ElementID: 30, WinRate: 50, ChampionRate: 5, WinCount: 10},
		{ID: 1, ElementID: 10, WinRate: 50, ChampionRate: 5, WinCount: 10},
		{ID: 2, ElementID: 20, WinRate: 50, ChampionRate: 5, WinCount: 10},
	}
	for run := 0; run < 5; run++ {
		ranks := &fakeHistoryRanks{rows: rows}
		service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))
		if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5)); err != nil {
			t.Fatalf("run %d error = %v", run, err)
		}
		got := appliedByID(ranks.applied)
		for id, want := range map[int64]int64{1: 1, 2: 2, 3: 3} {
			if got[id] != want {
				t.Fatalf("run %d: row %d rank = %d, want %d", run, id, got[id], want)
			}
		}
	}
}

// Rows already carrying the right rank are not rewritten. A large post would
// otherwise rewrite every row on every pass.
func TestAssignHistoryRanksSkipsRowsAlreadyCorrect(t *testing.T) {
	ranks := &fakeHistoryRanks{rows: []RankedHistoryRow{
		{ID: 1, ElementID: 10, WinRate: 90, Rank: 1},
		{ID: 2, ElementID: 20, WinRate: 50, Rank: 99},
	}}
	service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))

	if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5)); err != nil {
		t.Fatalf("AssignHistoryRanks() error = %v", err)
	}
	if len(ranks.applied) != 1 {
		t.Fatalf("applied %#v, want only the row whose rank changed", ranks.applied)
	}
	if ranks.applied[0].ID != 2 || ranks.applied[0].Rank != 2 {
		t.Fatalf("applied = %#v", ranks.applied[0])
	}
}

func TestAssignHistoryRanksWritesNothingWhenAllCorrect(t *testing.T) {
	ranks := &fakeHistoryRanks{rows: []RankedHistoryRow{
		{ID: 1, ElementID: 10, WinRate: 90, Rank: 1},
		{ID: 2, ElementID: 20, WinRate: 50, Rank: 2},
	}}
	service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))

	if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5)); err != nil {
		t.Fatalf("AssignHistoryRanks() error = %v", err)
	}
	if len(ranks.applied) != 0 {
		t.Fatalf("applied %#v, want nothing", ranks.applied)
	}
}

// Ranks must be a dense 1..N.
func TestAssignHistoryRanksProducesDenseRanks(t *testing.T) {
	rows := make([]RankedHistoryRow, 0, 20)
	for id := int64(1); id <= 20; id++ {
		rows = append(rows, RankedHistoryRow{ID: id, ElementID: id, WinRate: float64(id % 4)})
	}
	ranks := &fakeHistoryRanks{rows: rows}
	service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))

	if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5)); err != nil {
		t.Fatalf("AssignHistoryRanks() error = %v", err)
	}

	seen := make(map[int64]bool, len(rows))
	for _, row := range ranks.applied {
		if row.Rank < 1 || row.Rank > int64(len(rows)) {
			t.Fatalf("rank %d out of range", row.Rank)
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

func TestAssignHistoryRanksValidatesArguments(t *testing.T) {
	service := newRankService(t, &fakeHistoryRanks{}, &fakePending{}, day(2026, time.July, 8))
	ctx := context.Background()

	if err := service.AssignHistoryRanks(ctx, 0, HistoryRangeAll, day(2026, time.July, 5)); err == nil {
		t.Error("a zero post id must be rejected")
	}
	if err := service.AssignHistoryRanks(ctx, 46, HistoryTimeRange("week"), day(2026, time.July, 5)); err == nil {
		t.Error("a range with no build path must be rejected")
	}
}

func TestAssignHistoryRanksRequiresRepository(t *testing.T) {
	service, err := NewService(Options{
		Repository: &fakeRepository{}, Stats: &fakeStats{},
		Logger: quietRankLogger(), Location: taipei(t),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, time.Now()); err == nil {
		t.Fatal("AssignHistoryRanks() should fail without a repository")
	}
}

func TestAssignHistoryRanksPropagatesErrors(t *testing.T) {
	for name, ranks := range map[string]*fakeHistoryRanks{
		"read":  {readErr: errors.New("connection reset")},
		"apply": {rows: []RankedHistoryRow{{ID: 1, ElementID: 1, WinRate: 1}}, applyErr: errors.New("deadlock")},
	} {
		service := newRankService(t, ranks, &fakePending{}, day(2026, time.July, 8))
		if err := service.AssignHistoryRanks(context.Background(), 46, HistoryRangeAll, day(2026, time.July, 5)); err == nil {
			t.Errorf("%s failure must be reported", name)
		}
	}
}

// --- reorder ---

type recordingPending struct {
	stored map[HistoryTimeRange][]string
	pulled map[HistoryTimeRange]int
	err    error
}

func (store *recordingPending) Add(_ context.Context, _ int64, timeRange HistoryTimeRange, dates []string) error {
	if store.stored == nil {
		store.stored = make(map[HistoryTimeRange][]string)
	}
	store.stored[timeRange] = append(store.stored[timeRange], dates...)
	return nil
}

func (store *recordingPending) Pull(_ context.Context, _ int64, timeRange HistoryTimeRange) ([]string, error) {
	if store.err != nil {
		return nil, store.err
	}
	if store.pulled == nil {
		store.pulled = make(map[HistoryTimeRange]int)
	}
	store.pulled[timeRange]++
	if store.stored == nil {
		// Nothing was ever added; a nil map cannot be assigned to.
		return nil, nil
	}
	dates := store.stored[timeRange]
	delete(store.stored, timeRange)
	return dates, nil
}

func TestReorderHistoryRanksCollectsBothRanges(t *testing.T) {
	pending := &recordingPending{}
	ctx := context.Background()
	if err := pending.Add(ctx, 46, HistoryRangeAll, []string{"2026-07-06", "2026-07-05"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := pending.Add(ctx, 46, HistoryRangeThousandVotes, []string{"2026-07-08"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	service := newRankService(t, &fakeHistoryRanks{}, pending, day(2026, time.July, 8))

	targets, err := service.ReorderHistoryRanks(ctx, 46)
	if err != nil {
		t.Fatalf("ReorderHistoryRanks() error = %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("targets = %#v, want three", targets)
	}
	// Both ranges must be pulled, matching the original calling
	// updateRankReportHistoryRank for ALL and THOUSAND_VOTES.
	if pending.pulled[HistoryRangeAll] != 1 || pending.pulled[HistoryRangeThousandVotes] != 1 {
		t.Fatalf("pulled = %#v, want one pull per range", pending.pulled)
	}
	// Dates within a range come out sorted.
	if targets[0].StartDate.Format(dateLayout) != "2026-07-05" {
		t.Fatalf("first target = %s, want the earliest date", targets[0].StartDate.Format(dateLayout))
	}
}

func TestReorderHistoryRanksReturnsNothingWhenNoPendingDates(t *testing.T) {
	service := newRankService(t, &fakeHistoryRanks{}, &recordingPending{}, day(2026, time.July, 8))

	targets, err := service.ReorderHistoryRanks(context.Background(), 46)
	if err != nil {
		t.Fatalf("ReorderHistoryRanks() error = %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("targets = %#v, want none", targets)
	}
}

// A malformed stored date must not fail the whole post.
func TestReorderHistoryRanksSkipsInvalidDates(t *testing.T) {
	pending := &recordingPending{}
	ctx := context.Background()
	_ = pending.Add(ctx, 46, HistoryRangeAll, []string{"not-a-date", "2026-07-05"})
	service := newRankService(t, &fakeHistoryRanks{}, pending, day(2026, time.July, 8))

	targets, err := service.ReorderHistoryRanks(ctx, 46)
	if err != nil {
		t.Fatalf("ReorderHistoryRanks() error = %v", err)
	}
	if len(targets) != 1 || targets[0].StartDate.Format(dateLayout) != "2026-07-05" {
		t.Fatalf("targets = %#v, want only the valid date", targets)
	}
}

func TestReorderHistoryRanksPropagatesPullError(t *testing.T) {
	service := newRankService(t, &fakeHistoryRanks{}, &recordingPending{err: errors.New("redis unreachable")}, day(2026, time.July, 8))

	if _, err := service.ReorderHistoryRanks(context.Background(), 46); err == nil {
		t.Fatal("a pull failure must be reported")
	}
}

// --- purge ---

// The cutoff is today minus the retention window, resolved in the application
// timezone.
func TestRemoveOutdatedHistoryUsesTheRetentionWindow(t *testing.T) {
	ranks := &fakeHistoryRanks{purged: 12}
	now := day(2026, time.July, 8)
	service := newRankService(t, ranks, &fakePending{}, now)

	removed, err := service.RemoveOutdatedHistory(context.Background(), 46)
	if err != nil {
		t.Fatalf("RemoveOutdatedHistory() error = %v", err)
	}
	if removed != 12 {
		t.Fatalf("removed = %d, want 12", removed)
	}
	want := now.AddDate(0, 0, -HistoryRetentionDays).Format(dateLayout)
	if got := ranks.purgeCutoff.Format(dateLayout); got != want {
		t.Fatalf("cutoff = %s, want %s", got, want)
	}
	if ranks.purgeLimit != HistoryPurgeBatchSize {
		t.Fatalf("limit = %d, want %d", ranks.purgeLimit, HistoryPurgeBatchSize)
	}
}

// The batch size matches the original's limit(1000). This is the reason the purge
// cannot keep up on busy posts, so it is pinned by a test rather than left to
// drift.
func TestHistoryPurgeBatchSizeMatchesTheOriginal(t *testing.T) {
	if HistoryPurgeBatchSize != 1000 {
		t.Fatalf("HistoryPurgeBatchSize = %d, want 1000", HistoryPurgeBatchSize)
	}
	if HistoryRetentionDays != 93 {
		t.Fatalf("HistoryRetentionDays = %d, want 93", HistoryRetentionDays)
	}
}

func TestRemoveOutdatedHistoryValidatesArguments(t *testing.T) {
	service := newRankService(t, &fakeHistoryRanks{}, &fakePending{}, day(2026, time.July, 8))

	if _, err := service.RemoveOutdatedHistory(context.Background(), 0); err == nil {
		t.Fatal("a zero post id must be rejected")
	}
}

func TestRemoveOutdatedHistoryPropagatesError(t *testing.T) {
	service := newRankService(t, &fakeHistoryRanks{purgeErr: errors.New("lock wait timeout")}, &fakePending{}, day(2026, time.July, 8))

	if _, err := service.RemoveOutdatedHistory(context.Background(), 46); err == nil {
		t.Fatal("a purge failure must be reported")
	}
}
