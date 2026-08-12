package publicpost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// ---------- fakes ----------

type upsertCall struct {
	pass Pass
	rows []Row
}

type fakeRepository struct {
	mu sync.Mutex

	listed  []int64
	trended map[string][]int64

	markedDirty int
	sentinels   []Pass
	upserts     []upsertCall
	loadedIDs   [][]int64
	removed     int
	publicIDs   []int64

	// order records every mutating call so a test can assert the sequence.
	order []string

	listedErr   error
	trendedErr  error
	dirtyErr    error
	loadErr     error
	upsertErr   error
	sentinelErr error
	removeErr   error
	publicErr   error

	failPass Pass
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		listed:    []int64{31, 30, 29},
		trended:   map[string][]int64{"today": {7, 8}, "week": {7}, "month": {9}},
		publicIDs: []int64{29, 30, 31},
	}
}

func (repository *fakeRepository) record(event string) {
	repository.order = append(repository.order, event)
}

func (repository *fakeRepository) ListedPostIDs(context.Context) ([]int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.listedErr != nil {
		return nil, repository.listedErr
	}
	return repository.listed, nil
}

func (repository *fakeRepository) TrendedPostIDs(
	_ context.Context, trendRange string, _ time.Time,
) ([]int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.trendedErr != nil {
		return nil, repository.trendedErr
	}
	return repository.trended[trendRange], nil
}

func (repository *fakeRepository) MarkAllDirty(context.Context) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.dirtyErr != nil {
		return 0, repository.dirtyErr
	}
	repository.markedDirty++
	repository.record("mark-dirty")
	return 5, nil
}

func (repository *fakeRepository) PushDirtyToSentinel(_ context.Context, pass Pass) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.sentinelErr != nil {
		return 0, repository.sentinelErr
	}
	repository.sentinels = append(repository.sentinels, pass)
	repository.record("sentinel:" + string(pass))
	return 2, nil
}

func (repository *fakeRepository) LoadChunk(_ context.Context, postIDs []int64) ([]Row, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.loadErr != nil {
		return nil, repository.loadErr
	}
	repository.loadedIDs = append(repository.loadedIDs, append([]int64(nil), postIDs...))

	rows := make([]Row, 0, len(postIDs))
	for _, postID := range postIDs {
		rows = append(rows, Row{PostID: postID, Title: "t", Description: "d"})
	}
	return rows, nil
}

func (repository *fakeRepository) UpsertChunk(_ context.Context, pass Pass, rows []Row) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.upsertErr != nil {
		return 0, repository.upsertErr
	}
	if repository.failPass != "" && pass == repository.failPass {
		return 0, errors.New("upsert failed for " + string(pass))
	}
	repository.upserts = append(repository.upserts, upsertCall{pass: pass, rows: append([]Row(nil), rows...)})
	repository.record("upsert:" + string(pass))
	return int64(len(rows)), nil
}

func (repository *fakeRepository) RemoveDirty(context.Context) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.removeErr != nil {
		return 0, repository.removeErr
	}
	repository.removed++
	repository.record("remove-dirty")
	return 1, nil
}

func (repository *fakeRepository) PublicPostIDs(context.Context) ([]int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.publicErr != nil {
		return nil, repository.publicErr
	}
	return repository.publicIDs, nil
}

func (repository *fakeRepository) upsertsFor(pass Pass) []upsertCall {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var matched []upsertCall
	for _, call := range repository.upserts {
		if call.pass == pass {
			matched = append(matched, call)
		}
	}
	return matched
}

type fakeFreshness struct {
	mu     sync.Mutex
	fresh  bool
	marked int

	readErr error
	markErr error
}

func (store *fakeFreshness) IsFresh(context.Context) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.readErr != nil {
		return false, store.readErr
	}
	return store.fresh, nil
}

func (store *fakeFreshness) MarkFresh(context.Context) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.markErr != nil {
		return store.markErr
	}
	store.marked++
	return nil
}

func (store *fakeFreshness) markedCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.marked
}

type fakeCache struct {
	mu      sync.Mutex
	cleared []int64
	err     error
}

func (cache *fakeCache) Clear(_ context.Context, postIDs []int64) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.err != nil {
		return cache.err
	}
	cache.cleared = append(cache.cleared, postIDs...)
	return nil
}

// ---------- harness ----------

type harness struct {
	service    *Service
	repository *fakeRepository
	freshness  *fakeFreshness
	cache      *fakeCache
}

// pinnedNow is a Thursday, so the week window is the Monday before it.
var pinnedNow = time.Date(2026, time.August, 6, 14, 0, 0, 0, time.UTC)

func newHarness(t *testing.T) *harness {
	t.Helper()

	repository := newFakeRepository()
	freshness := &fakeFreshness{}
	cache := &fakeCache{}
	service, err := NewService(Options{
		Repository: repository,
		Freshness:  freshness,
		Cache:      cache,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:   time.UTC,
		Now:        func() time.Time { return pinnedNow },
		Shuffle:    noShuffle,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &harness{service: service, repository: repository, freshness: freshness, cache: cache}
}

func (h *harness) refresh(t *testing.T) error {
	t.Helper()
	message, err := RefreshMessage()
	if err != nil {
		t.Fatalf("RefreshMessage() error = %v", err)
	}
	return h.service.handleRefresh(context.Background(), message)
}

// ---------- tests ----------

func TestRefreshRunsEveryPassThenRemovesStaleRows(t *testing.T) {
	h := newHarness(t)
	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	// One mark-dirty and one sentinel push per pass, in the executor's order.
	if h.repository.markedDirty != 4 {
		t.Errorf("marked dirty %d times, want 4 (one per pass)", h.repository.markedDirty)
	}
	wantSentinels := []Pass{PassNew, PassToday, PassWeek, PassMonth}
	if len(h.repository.sentinels) != len(wantSentinels) {
		t.Fatalf("pushed %d sentinels, want %d", len(h.repository.sentinels), len(wantSentinels))
	}
	for index, want := range wantSentinels {
		if h.repository.sentinels[index] != want {
			t.Errorf("sentinel %d = %q, want %q", index, h.repository.sentinels[index], want)
		}
	}

	if h.repository.removed != 1 {
		t.Errorf("removed stale rows %d times, want 1", h.repository.removed)
	}
	if h.freshness.markedCount() != 1 {
		t.Errorf("marked fresh %d times, want 1", h.freshness.markedCount())
	}
	if len(h.cache.cleared) != 3 {
		t.Errorf("cleared %d resource cache entries, want 3", len(h.cache.cleared))
	}
}

// The order within a pass is what makes the sentinel meaningful: mark everything
// dirty, write the ones that belong, then push whatever is still dirty. Any other
// order either blanks rows that should stay or leaves stale positions in place.
func TestRefreshMarksDirtyBeforeWritingAndPushesSentinelAfter(t *testing.T) {
	h := newHarness(t)
	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	// The first pass is `new`, so the first three events must be its own.
	want := []string{"mark-dirty", "upsert:new", "sentinel:new"}
	if len(h.repository.order) < len(want) {
		t.Fatalf("only %d events recorded: %v", len(h.repository.order), h.repository.order)
	}
	for index, event := range want {
		if h.repository.order[index] != event {
			t.Fatalf("event %d = %q, want %q (full order: %v)",
				index, h.repository.order[index], event, h.repository.order)
		}
	}
	// removeDirtyPublicPosts runs once, after every pass.
	if last := h.repository.order[len(h.repository.order)-1]; last != "remove-dirty" {
		t.Errorf("last event = %q, want remove-dirty", last)
	}
}

// THE GUARD THAT PROTECTS THE HOME PAGE. An empty source set must not mark everything
// dirty and then write nothing, which would push the whole listing to the sentinel and
// empty the page.
func TestRefreshSkipsAPassWithNoQualifyingPosts(t *testing.T) {
	h := newHarness(t)
	h.repository.trended["week"] = nil

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	for _, pass := range h.repository.sentinels {
		if pass == PassWeek {
			t.Fatal("the week pass pushed its sentinel despite having no posts")
		}
	}
	if len(h.repository.upsertsFor(PassWeek)) != 0 {
		t.Fatal("the week pass wrote rows despite having no posts")
	}
	// Three passes did run, so three mark-dirty calls rather than four.
	if h.repository.markedDirty != 3 {
		t.Errorf("marked dirty %d times, want 3", h.repository.markedDirty)
	}
	// And the refresh still counts as successful.
	if h.freshness.markedCount() != 1 {
		t.Error("an empty pass must not fail the refresh")
	}
}

// Positions are the position within the whole pass, not within a chunk. The PHP
// incremented one counter across its entire loop, so a chunk boundary restarting it
// would give several posts position 1.
func TestRefreshNumbersPositionsAcrossChunkBoundaries(t *testing.T) {
	h := newHarness(t)

	// More posts than one chunk holds.
	total := ChunkSize + 37
	listed := make([]int64, 0, total)
	for index := 0; index < total; index++ {
		listed = append(listed, int64(1000+index))
	}
	h.repository.listed = listed

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	calls := h.repository.upsertsFor(PassNew)
	if len(calls) != 2 {
		t.Fatalf("wrote %d chunks for %d posts, want 2", len(calls), total)
	}

	seen := make(map[int]int64, total)
	for _, call := range calls {
		for _, row := range call.rows {
			if previous, exists := seen[row.Position]; exists {
				t.Fatalf("position %d assigned to both post %d and post %d",
					row.Position, previous, row.PostID)
			}
			seen[row.Position] = row.PostID
		}
	}
	if len(seen) != total {
		t.Fatalf("assigned %d positions, want %d", len(seen), total)
	}
	// Dense 1..N in source order.
	for index, postID := range listed {
		if seen[index+1] != postID {
			t.Fatalf("position %d = post %d, want %d", index+1, seen[index+1], postID)
		}
	}
}

// The four try/catch blocks in the executor mean a broken pass must not stop the
// others. The job still has to fail so it retries, and must not mark itself fresh —
// otherwise a half-built listing would be locked in for the debounce window.
func TestRefreshContinuesAfterAFailedPassButStillReportsFailure(t *testing.T) {
	h := newHarness(t)
	h.repository.failPass = PassToday

	err := h.refresh(t)
	if err == nil {
		t.Fatal("handleRefresh() should fail when a pass fails")
	}
	if jobs.IsPermanent(err) {
		t.Errorf("a failed pass must be retryable: %v", err)
	}

	// The later passes still ran.
	for _, pass := range []Pass{PassWeek, PassMonth} {
		if len(h.repository.upsertsFor(pass)) == 0 {
			t.Errorf("the %q pass did not run after an earlier pass failed", pass)
		}
	}
	if h.freshness.markedCount() != 0 {
		t.Error("a refresh with a failed pass must not mark itself fresh")
	}
}

func TestRefreshSkipsWhenStillFresh(t *testing.T) {
	h := newHarness(t)
	h.freshness.fresh = true

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if h.repository.markedDirty != 0 || len(h.repository.upserts) != 0 {
		t.Fatal("a fresh listing must not be rebuilt")
	}
	if h.freshness.markedCount() != 0 {
		t.Error("a skipped refresh must not re-mark freshness")
	}
}

// A stale Laravel resource cache is a nuisance; failing the job over it would repeat
// the whole rebuild, which is the expensive part.
func TestRefreshSurvivesAResourceCacheFailure(t *testing.T) {
	h := newHarness(t)
	h.cache.err = errors.New("redis unreachable")

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v, want nil", err)
	}
	if h.freshness.markedCount() != 1 {
		t.Error("the refresh completed, so it must still be marked fresh")
	}
}

func TestRefreshFailsWhenTheCleanupFails(t *testing.T) {
	h := newHarness(t)
	h.repository.removeErr = errors.New("deadlock")

	err := h.refresh(t)
	if err == nil {
		t.Fatal("handleRefresh() should fail when the cleanup fails")
	}
	if h.freshness.markedCount() != 0 {
		t.Error("a failed cleanup must not mark the listing fresh")
	}
}

func TestBackendFailuresStayRetryable(t *testing.T) {
	failures := map[string]func(*harness){
		"listing the source fails": func(h *harness) { h.repository.listedErr = errors.New("gone away") },
		"reading trends fails":     func(h *harness) { h.repository.trendedErr = errors.New("gone away") },
		"marking dirty fails":      func(h *harness) { h.repository.dirtyErr = errors.New("deadlock") },
		"loading a chunk fails":    func(h *harness) { h.repository.loadErr = errors.New("connection reset") },
		"upserting fails":          func(h *harness) { h.repository.upsertErr = errors.New("deadlock") },
		"pushing the sentinel fails": func(h *harness) {
			h.repository.sentinelErr = errors.New("lock wait timeout")
		},
		"reading the freshness flag fails": func(h *harness) {
			h.freshness.readErr = errors.New("redis unreachable")
		},
		"marking fresh fails": func(h *harness) { h.freshness.markErr = errors.New("redis unreachable") },
	}
	for name, breakIt := range failures {
		h := newHarness(t)
		breakIt(h)

		err := h.refresh(t)
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if jobs.IsPermanent(err) {
			t.Errorf("%s: dead-lettered a transient failure: %v", name, err)
		}
	}
}

func TestRefreshRejectsAMalformedPayload(t *testing.T) {
	h := newHarness(t)
	err := h.service.handleRefresh(context.Background(), queue.Message{
		Queue: Queue, Type: TypeRefresh, Payload: json.RawMessage(`{`),
	})
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("a malformed payload must be permanent, got %v", err)
	}
}

func TestTrendWindowStartMatchesTheExecutor(t *testing.T) {
	// pinnedNow is Thursday 2026-08-06.
	cases := map[Pass]string{
		PassToday: "2026-08-06",
		PassWeek:  "2026-08-03",
		PassMonth: "2026-08-01",
	}
	for pass, want := range cases {
		got, err := TrendWindowStart(pass, pinnedNow)
		if err != nil {
			t.Errorf("TrendWindowStart(%q) error = %v", pass, err)
			continue
		}
		if formatted := got.Format("2006-01-02"); formatted != want {
			t.Errorf("TrendWindowStart(%q) = %s, want %s", pass, formatted, want)
		}
	}
	if _, err := TrendWindowStart(PassNew, pinnedNow); !errors.Is(err, ErrUnknownPass) {
		t.Errorf("PassNew has no trend window; error = %v", err)
	}
}

// Sunday is weekday 0 in Go but the last day of an ISO week, so it must step back six
// days rather than none.
func TestTrendWindowStartTreatsSundayAsTheEndOfTheWeek(t *testing.T) {
	sunday := time.Date(2026, time.August, 9, 23, 0, 0, 0, time.UTC)
	got, err := TrendWindowStart(PassWeek, sunday)
	if err != nil {
		t.Fatalf("TrendWindowStart() error = %v", err)
	}
	if formatted := got.Format("2006-01-02"); formatted != "2026-08-03" {
		t.Fatalf("Sunday's week start = %s, want 2026-08-03", formatted)
	}
	if got.Weekday() != time.Monday {
		t.Fatalf("week start is a %s, want Monday", got.Weekday())
	}
}

func TestPassColumnsAreDistinct(t *testing.T) {
	seen := make(map[string]Pass, 4)
	for _, pass := range Ordered() {
		column, err := pass.PositionColumn()
		if err != nil {
			t.Fatalf("PositionColumn(%q) error = %v", pass, err)
		}
		if previous, exists := seen[column]; exists {
			t.Fatalf("passes %q and %q share the column %q", previous, pass, column)
		}
		seen[column] = pass
		if !pass.Valid() {
			t.Errorf("%q should be valid", pass)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("got %d columns, want 4", len(seen))
	}
	// The day column is named after the range, not after the pass.
	if column, _ := PassToday.PositionColumn(); column != "day_position" {
		t.Errorf("PassToday writes %q, want day_position", column)
	}
	if !Pass("nonsense").Valid() {
		return
	}
	t.Error("an unknown pass must not be valid")
}

// The tags column is JSON in a varchar, and Laravel wrote it with
// JSON_UNESCAPED_UNICODE. Escaping the CJK names would change every stored value.
func TestEncodeTagsLeavesNonASCIIUnescaped(t *testing.T) {
	encoded := encodeTags([]string{"標籤", "a&b"})
	if encoded != `["標籤","a&b"]` {
		t.Fatalf("encodeTags = %s, want [\"標籤\",\"a&b\"]", encoded)
	}
	if encodeTags(nil) != "[]" {
		t.Fatalf("encodeTags(nil) = %s, want []", encodeTags(nil))
	}

	// And it must still be valid JSON the reader can decode.
	var back []string
	if err := json.Unmarshal([]byte(encoded), &back); err != nil {
		t.Fatalf("the tags column is not decodable: %v", err)
	}
	if len(back) != 2 || back[0] != "標籤" {
		t.Fatalf("decoded %v", back)
	}
}

func TestRegistrationStatesItsContract(t *testing.T) {
	h := newHarness(t)
	registry := jobs.NewRegistry()
	registry.MustRegister(h.service.Registration())

	registration, err := registry.Lookup(TypeRefresh)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if registration.Timeout <= 0 || registration.MaxAttempts < 1 {
		t.Errorf("no usable contract: %+v", registration)
	}
	// One lock for the whole table: the passes share is_dirty, so two refreshes
	// interleaving would clear each other's flags.
	key, err := registration.SerialKey(queue.Message{})
	if err != nil {
		t.Fatalf("SerialKey() error = %v", err)
	}
	if key != LockKey {
		t.Errorf("serial key = %q, want %q", key, LockKey)
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	complete := Options{
		Repository: newFakeRepository(),
		Freshness:  &fakeFreshness{},
		Location:   time.UTC,
	}
	for name, mutate := range map[string]func(*Options){
		"no repository": func(o *Options) { o.Repository = nil },
		"no freshness":  func(o *Options) { o.Freshness = nil },
		// Without it the trend windows would be computed in UTC, eight hours behind the
		// application timezone, landing on the wrong day for part of every night.
		"no location": func(o *Options) { o.Location = nil },
	} {
		options := complete
		mutate(&options)
		if _, err := NewService(options); err == nil {
			t.Errorf("NewService() should reject the %q case", name)
		}
	}
	if _, err := NewService(complete); err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
}
