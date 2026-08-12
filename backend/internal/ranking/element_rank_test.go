package ranking

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// fakeRepository serves canned deltas keyed by the watermark it is asked about,
// so a second run can be given "nothing new" without any database.
type fakeRepository struct {
	completedWin  map[int64]RoundDelta
	completedLose map[int64]RoundDelta
	allWin        map[int64]RoundDelta
	allLose       map[int64]RoundDelta

	upserts   []Rank
	upsertErr error

	completedWinErr error
}

func (repository *fakeRepository) delta(source map[int64]RoundDelta, after int64) RoundDelta {
	if source == nil {
		return RoundDelta{}
	}
	return source[after]
}

func (repository *fakeRepository) CompletedWinDelta(_ context.Context, _, _, after int64) (RoundDelta, error) {
	if repository.completedWinErr != nil {
		return RoundDelta{}, repository.completedWinErr
	}
	return repository.delta(repository.completedWin, after), nil
}

func (repository *fakeRepository) CompletedLoseDelta(_ context.Context, _, _, after int64) (RoundDelta, error) {
	return repository.delta(repository.completedLose, after), nil
}

func (repository *fakeRepository) AllGamesWinDelta(_ context.Context, _, _, after int64) (RoundDelta, error) {
	return repository.delta(repository.allWin, after), nil
}

func (repository *fakeRepository) AllGamesLoseDelta(_ context.Context, _, _, after int64) (RoundDelta, error) {
	return repository.delta(repository.allLose, after), nil
}

func (repository *fakeRepository) UpsertRank(_ context.Context, rank Rank) error {
	if repository.upsertErr != nil {
		return repository.upsertErr
	}
	repository.upserts = append(repository.upserts, rank)
	return nil
}

func (repository *fakeRepository) ranksOfType(rankType RankType) []Rank {
	out := make([]Rank, 0)
	for _, rank := range repository.upserts {
		if rank.RankType == rankType {
			out = append(out, rank)
		}
	}
	return out
}

type fakeStats struct {
	stored  Stats
	present bool
	getErr  error
	putErr  error
	puts    int
}

func (store *fakeStats) Get(context.Context, int64, int64) (Stats, error) {
	if store.getErr != nil {
		return Stats{}, store.getErr
	}
	if !store.present {
		return Stats{}, nil
	}
	return store.stored, nil
}

func (store *fakeStats) Put(_ context.Context, _, _ int64, stats Stats) error {
	store.puts++
	if store.putErr != nil {
		return store.putErr
	}
	store.stored = stats
	store.present = true
	return nil
}

func taipei(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	return location
}

func newService(t *testing.T, repository Repository, stats StatsStore, now time.Time) *Service {
	t.Helper()
	service, err := NewService(Options{
		Repository: repository,
		Stats:      stats,
		Location:   taipei(t),
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestNewServiceRequiresDependencies(t *testing.T) {
	location := taipei(t)
	cases := map[string]Options{
		"no repository": {Stats: &fakeStats{}, Location: location},
		"no stats":      {Repository: &fakeRepository{}, Location: location},
		// An implicit timezone would file record_date under the wrong day for
		// eight hours out of every twenty-four.
		"no location": {Repository: &fakeRepository{}, Stats: &fakeStats{}},
	}
	for name, options := range cases {
		if _, err := NewService(options); err == nil {
			t.Errorf("NewService() should reject the %s case", name)
		}
	}
}

func TestUpdateElementRankRejectsMissingIdentifiers(t *testing.T) {
	service := newService(t, &fakeRepository{}, &fakeStats{}, time.Now())

	if err := service.UpdateElementRank(context.Background(), 0, 5); err == nil {
		t.Error("a zero post id must be rejected")
	}
	if err := service.UpdateElementRank(context.Background(), 5, 0); err == nil {
		t.Error("a zero element id must be rejected")
	}
}

// A cold memo means a full recount from id 0, which is the correct starting
// point and the reason the seven day TTL is safe.
func TestUpdateElementRankCountsFromZeroWithNoMemo(t *testing.T) {
	repository := &fakeRepository{
		completedWin:  map[int64]RoundDelta{0: {Count: 10, MaxID: 110, ChampionCount: 2}},
		completedLose: map[int64]RoundDelta{0: {Count: 5, MaxID: 108}},
		allWin:        map[int64]RoundDelta{0: {Count: 20, MaxID: 200}},
		allLose:       map[int64]RoundDelta{0: {Count: 30, MaxID: 205}},
	}
	stats := &fakeStats{}
	service := newService(t, repository, stats, time.Date(2026, 8, 5, 9, 0, 0, 0, taipei(t)))

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}

	champion := repository.ranksOfType(RankTypeChampion)
	if len(champion) != 1 {
		t.Fatalf("champion rows = %#v", champion)
	}
	if champion[0].WinCount != 2 || champion[0].RoundCount != 15 {
		t.Fatalf("champion = %#v, want wins 2 over 15 rounds", champion[0])
	}
	// 2/15 = 13.333... -> 13.33 for decimal(5,2)
	if champion[0].WinRate != 13.33 {
		t.Fatalf("champion win rate = %v, want 13.33", champion[0].WinRate)
	}

	pk := repository.ranksOfType(RankTypePKKing)
	if len(pk) != 1 {
		t.Fatalf("pk rows = %#v", pk)
	}
	if pk[0].WinCount != 20 || pk[0].RoundCount != 50 || pk[0].WinRate != 40 {
		t.Fatalf("pk = %#v, want 20/50 at 40%%", pk[0])
	}

	if stats.stored.ChampionMaxWinID != 110 || stats.stored.PKMaxLoseID != 205 {
		t.Fatalf("watermarks = %#v", stats.stored)
	}
}

// THE INVARIANT. The database write must be the absolute total, never an
// increment. Both failure orders between the memo and the rank row converge only
// because of this; an increment would double-count on any redelivery.
func TestUpdateElementRankWritesAbsoluteTotalsNotIncrements(t *testing.T) {
	repository := &fakeRepository{
		// The memo is already at these watermarks, and nothing new has arrived.
		completedWin:  map[int64]RoundDelta{110: {}},
		completedLose: map[int64]RoundDelta{108: {}},
		allWin:        map[int64]RoundDelta{200: {}},
		allLose:       map[int64]RoundDelta{205: {}},
	}
	stats := &fakeStats{
		present: true,
		stored: Stats{
			ChampionMaxWinID: 110, ChampionMaxLoseID: 108,
			ChampionRoundWins: 10, ChampionRoundLoses: 5, ChampionGameWins: 2,
			PKMaxWinID: 200, PKMaxLoseID: 205,
			PKWinCount: 20, PKLoseCount: 30,
		},
	}
	service := newService(t, repository, stats, time.Now())

	// Run three times. With no new rounds, every run must write the same values.
	for run := 1; run <= 3; run++ {
		if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
			t.Fatalf("run %d: UpdateElementRank() error = %v", run, err)
		}
	}

	for _, rank := range repository.ranksOfType(RankTypeChampion) {
		if rank.WinCount != 2 || rank.RoundCount != 15 {
			t.Fatalf("champion drifted across runs: %#v", rank)
		}
	}
	for _, rank := range repository.ranksOfType(RankTypePKKing) {
		if rank.WinCount != 20 || rank.RoundCount != 50 {
			t.Fatalf("pk drifted across runs: %#v", rank)
		}
	}
	if stats.stored.PKWinCount != 20 || stats.stored.ChampionRoundWins != 10 {
		t.Fatalf("memo drifted across runs: %#v", stats.stored)
	}
}

// The convergence case where the memo write failed after the rank was written:
// the next run recounts the same rounds from the old watermark and arrives at the
// same absolute total.
func TestUpdateElementRankConvergesWhenMemoWriteFailed(t *testing.T) {
	repository := &fakeRepository{
		completedWin:  map[int64]RoundDelta{0: {Count: 10, MaxID: 110, ChampionCount: 2}},
		completedLose: map[int64]RoundDelta{0: {Count: 5, MaxID: 108}},
		allWin:        map[int64]RoundDelta{0: {Count: 20, MaxID: 200}},
		allLose:       map[int64]RoundDelta{0: {Count: 30, MaxID: 205}},
	}
	stats := &fakeStats{putErr: errors.New("redis unreachable")}
	service := newService(t, repository, stats, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err == nil {
		t.Fatal("a failed memo write must be reported")
	}
	first := repository.ranksOfType(RankTypePKKing)
	if len(first) != 1 {
		t.Fatalf("expected the rank to have been written before the memo failed, got %#v", first)
	}

	// The memo is still empty, so the next run recounts from zero.
	stats.putErr = nil
	repository.upserts = nil
	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("second run error = %v", err)
	}

	second := repository.ranksOfType(RankTypePKKing)
	if len(second) != 1 {
		t.Fatalf("second run ranks = %#v", second)
	}
	if second[0].WinCount != first[0].WinCount || second[0].RoundCount != first[0].RoundCount {
		t.Fatalf("recount diverged: first %#v, second %#v", first[0], second[0])
	}
}

// The memo must be written only after the ranks, so a failed rank write leaves
// the watermark unadvanced and the work still pending.
func TestUpdateElementRankDoesNotAdvanceMemoWhenRankWriteFails(t *testing.T) {
	repository := &fakeRepository{
		allWin:    map[int64]RoundDelta{0: {Count: 20, MaxID: 200}},
		allLose:   map[int64]RoundDelta{0: {Count: 30, MaxID: 205}},
		upsertErr: errors.New("deadlock found when trying to get lock"),
	}
	stats := &fakeStats{}
	service := newService(t, repository, stats, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err == nil {
		t.Fatal("a failed rank write must be reported")
	}
	if stats.puts != 0 {
		t.Fatal("the memo must not be written when the rank write failed")
	}
	if stats.present {
		t.Fatalf("watermark advanced despite the failure: %#v", stats.stored)
	}
}

// Matches the Laravel guard: an element that played completed rounds but never
// won a game outright gets no champion row at all, rather than a zero one.
func TestUpdateElementRankSkipsChampionWithoutAnOutrightWin(t *testing.T) {
	repository := &fakeRepository{
		completedWin:  map[int64]RoundDelta{0: {Count: 10, MaxID: 110, ChampionCount: 0}},
		completedLose: map[int64]RoundDelta{0: {Count: 5, MaxID: 108}},
		allWin:        map[int64]RoundDelta{0: {Count: 10, MaxID: 110}},
		allLose:       map[int64]RoundDelta{0: {Count: 5, MaxID: 108}},
	}
	service := newService(t, repository, &fakeStats{}, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}

	if got := repository.ranksOfType(RankTypeChampion); len(got) != 0 {
		t.Fatalf("champion rows = %#v, want none", got)
	}
	if got := repository.ranksOfType(RankTypePKKing); len(got) != 1 {
		t.Fatalf("pk rows = %#v, want one", got)
	}
}

// An element with no rounds at all produces no rows.
func TestUpdateElementRankWritesNothingWithoutRounds(t *testing.T) {
	repository := &fakeRepository{}
	service := newService(t, repository, &fakeStats{}, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}
	if len(repository.upserts) != 0 {
		t.Fatalf("upserts = %#v, want none", repository.upserts)
	}
}

// A watermark must never move backwards, even if a delta reports a lower max id.
func TestUpdateElementRankNeverRewindsAWatermark(t *testing.T) {
	repository := &fakeRepository{
		allWin:  map[int64]RoundDelta{200: {Count: 1, MaxID: 150}},
		allLose: map[int64]RoundDelta{205: {}},
	}
	stats := &fakeStats{
		present: true,
		stored:  Stats{PKMaxWinID: 200, PKMaxLoseID: 205, PKWinCount: 20, PKLoseCount: 30},
	}
	service := newService(t, repository, stats, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}
	if stats.stored.PKMaxWinID != 200 {
		t.Fatalf("PKMaxWinID = %d, want it held at 200", stats.stored.PKMaxWinID)
	}
}

// record_date is a DATE and Laravel's today() is Asia/Taipei. A UTC clock would
// file evening rows under the previous day.
func TestRecordDateUsesTheApplicationTimezone(t *testing.T) {
	repository := &fakeRepository{
		allWin:  map[int64]RoundDelta{0: {Count: 1, MaxID: 1}},
		allLose: map[int64]RoundDelta{0: {}},
	}
	// 2026-08-05 00:30 in Taipei is still 2026-08-04 in UTC.
	instant := time.Date(2026, 8, 4, 16, 30, 0, 0, time.UTC)
	service := newService(t, repository, &fakeStats{}, instant)

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}

	got := repository.upserts[0].RecordDate
	if got.Year() != 2026 || got.Month() != time.August || got.Day() != 5 {
		t.Fatalf("record date = %s, want 2026-08-05 in Asia/Taipei", got.Format(time.RFC3339))
	}
}

func TestUpdateElementRankPropagatesRepositoryErrors(t *testing.T) {
	repository := &fakeRepository{completedWinErr: errors.New("connection reset")}
	service := newService(t, repository, &fakeStats{}, time.Now())

	if err := service.UpdateElementRank(context.Background(), 46, 2759); err == nil {
		t.Fatal("a repository failure must be reported")
	}
}

func TestUpdateElementRankPropagatesStatsReadError(t *testing.T) {
	stats := &fakeStats{getErr: errors.New("redis unreachable")}
	service := newService(t, &fakeRepository{}, stats, time.Now())

	// Reading the memo must not silently fall back to a full recount: that would
	// turn a Redis blip into a full re-aggregation of 45.9M rows per element.
	if err := service.UpdateElementRank(context.Background(), 46, 2759); err == nil {
		t.Fatal("a failed memo read must be reported")
	}
}

// win_rate is decimal(5,2); rounding in Go keeps the value identical to PHP's
// rather than letting MySQL truncate.
func TestWinRateRoundsToTwoDecimals(t *testing.T) {
	cases := []struct {
		wins, rounds int64
		want         float64
	}{
		{0, 0, 0},
		{0, 10, 0},
		{5, 0, 0},
		{1, 3, 33.33},
		{2, 3, 66.67},
		{2, 15, 13.33},
		{6, 627, 0.96},
		{42, 68, 61.76},
		{39, 61, 63.93},
		{10, 10, 100},
	}
	for _, test := range cases {
		if got := WinRate(test.wins, test.rounds); got != test.want {
			t.Errorf("WinRate(%d, %d) = %v, want %v", test.wins, test.rounds, got, test.want)
		}
	}
}

func TestStatsKeyMatchesTheLaravelCacheKey(t *testing.T) {
	if got := StatsKey(46, 2759); got != "element_rank_stats:46:2759" {
		t.Fatalf("StatsKey() = %q", got)
	}
}

func TestRankTypeValidity(t *testing.T) {
	if !RankTypeChampion.Valid() || !RankTypePKKing.Valid() {
		t.Fatal("the two enum values must be valid")
	}
	if RankType("king").Valid() {
		t.Fatal("an unknown rank type must be invalid")
	}
}

func quietRankLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
