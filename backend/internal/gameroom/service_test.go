package gameroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/realtime"
)

// ---------- fakes ----------

type fakeRepository struct {
	mu sync.Mutex

	rooms map[string]Room

	settled       []BetOutcome
	recomputes    []int64
	rankPasses    []int64
	leaderboardOf int64

	board Leaderboard

	// recomputeHook runs inside RecomputeTotals, which is how a test lands a vote
	// mid-flight: the version rises after the refresh has already read it.
	recomputeHook func()

	settleErr     error
	recomputeErr  error
	rankErr       error
	boardErr      error
	roomLookupErr error
}

func newFakeRepository(serial string) *fakeRepository {
	return &fakeRepository{
		rooms: map[string]Room{serial: {ID: 77, Serial: serial}},
		board: Leaderboard{TotalUsers: 3, Top10: []Player{{UserID: "a", Rank: 1}}},
	}
}

func (repository *fakeRepository) RoomBySerial(_ context.Context, serial string) (Room, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.roomLookupErr != nil {
		return Room{}, repository.roomLookupErr
	}
	room, ok := repository.rooms[serial]
	if !ok {
		return Room{}, fmt.Errorf("%w: serial %q", ErrNotFound, serial)
	}
	return room, nil
}

func (repository *fakeRepository) SettleBets(_ context.Context, outcome BetOutcome) (SettleResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.settleErr != nil {
		return SettleResult{}, repository.settleErr
	}
	repository.settled = append(repository.settled, outcome)
	return SettleResult{Won: 2, Lost: 1, Discarded: 3}, nil
}

func (repository *fakeRepository) RecomputeTotals(_ context.Context, roomID int64) (int64, error) {
	repository.mu.Lock()
	if repository.recomputeErr != nil {
		err := repository.recomputeErr
		repository.mu.Unlock()
		return 0, err
	}
	repository.recomputes = append(repository.recomputes, roomID)
	hook := repository.recomputeHook
	repository.mu.Unlock()

	// Called outside the lock so the hook may re-enter the fakes.
	if hook != nil {
		hook()
	}
	return 5, nil
}

func (repository *fakeRepository) AssignRanks(_ context.Context, roomID int64) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.rankErr != nil {
		return 0, repository.rankErr
	}
	repository.rankPasses = append(repository.rankPasses, roomID)
	return 4, nil
}

func (repository *fakeRepository) Leaderboard(_ context.Context, roomID int64) (Leaderboard, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.boardErr != nil {
		return Leaderboard{}, repository.boardErr
	}
	repository.leaderboardOf = roomID
	return repository.board, nil
}

func (repository *fakeRepository) Standings(context.Context, int64) ([]Standing, error) {
	return nil, nil
}

func (repository *fakeRepository) BetsByPlayer(context.Context, int64) (map[int64][]Bet, error) {
	return nil, nil
}

func (repository *fakeRepository) recomputeCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return len(repository.recomputes)
}

// fakeTracker is an in-memory RefreshTracker with the same monotonic guarantee as
// the Redis one.
type fakeTracker struct {
	mu      sync.Mutex
	version map[string]int64
	applied map[string]int64

	outstandingErr error
	markAppliedErr error
	markChangedErr error
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{version: map[string]int64{}, applied: map[string]int64{}}
}

func (tracker *fakeTracker) MarkChanged(_ context.Context, serial string) (int64, error) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.markChangedErr != nil {
		return 0, tracker.markChangedErr
	}
	tracker.version[serial]++
	return tracker.version[serial], nil
}

func (tracker *fakeTracker) Outstanding(_ context.Context, serial string) (Outstanding, error) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.outstandingErr != nil {
		return Outstanding{}, tracker.outstandingErr
	}
	return Outstanding{Version: tracker.version[serial], Applied: tracker.applied[serial]}, nil
}

func (tracker *fakeTracker) MarkApplied(_ context.Context, serial string, version int64) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.markAppliedErr != nil {
		return tracker.markAppliedErr
	}
	if version > tracker.applied[serial] {
		tracker.applied[serial] = version
	}
	return nil
}

type broadcast struct {
	channel string
	event   string
	payload any
}

type fakeBroadcaster struct {
	mu   sync.Mutex
	sent []broadcast
	err  error
}

func (broadcaster *fakeBroadcaster) Publish(_ context.Context, channel, event string, payload any) error {
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()
	if broadcaster.err != nil {
		return broadcaster.err
	}
	broadcaster.sent = append(broadcaster.sent, broadcast{channel: channel, event: event, payload: payload})
	return nil
}

func (broadcaster *fakeBroadcaster) count() int {
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()
	return len(broadcaster.sent)
}

type fakeTransport struct {
	mu       sync.Mutex
	messages []queue.Message
}

func (transport *fakeTransport) Publish(_ context.Context, messages []queue.Message) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.messages = append(transport.messages, messages...)
	return nil
}

func (transport *fakeTransport) published() []queue.Message {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return append([]queue.Message(nil), transport.messages...)
}

type recordingLegacyCache struct {
	mu           sync.Mutex
	invalidated  []string
	flagsCleared []string
	err          error
}

func (cache *recordingLegacyCache) InvalidateLeaderboard(_ context.Context, serial string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.err != nil {
		return cache.err
	}
	cache.invalidated = append(cache.invalidated, serial)
	return nil
}

func (cache *recordingLegacyCache) ClearUpdatingFlag(_ context.Context, serial string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.err != nil {
		return cache.err
	}
	cache.flagsCleared = append(cache.flagsCleared, serial)
	return nil
}

func (cache *recordingLegacyCache) clearedCount() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return len(cache.flagsCleared)
}

// ---------- harness ----------

const testSerial = "room-abc123"

type harness struct {
	service     *Service
	repository  *fakeRepository
	tracker     *fakeTracker
	broadcaster *fakeBroadcaster
	transport   *fakeTransport
	legacy      *recordingLegacyCache
	votes       *fakeVotes
}

// fakeVotes stands in for the tally the room's REST endpoint reads.
type fakeVotes struct {
	tally      VoteTally
	inProgress bool
	roomID     int64
	gameSerial string
	calls      int
	err        error
}

func (fake *fakeVotes) CurrentVotes(_ context.Context, roomID int64, gameSerial string) (VoteTally, bool, error) {
	fake.calls++
	fake.roomID, fake.gameSerial = roomID, gameSerial
	if fake.err != nil {
		return VoteTally{}, false, fake.err
	}
	return fake.tally, fake.inProgress, nil
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	repository := newFakeRepository(testSerial)
	tracker := newFakeTracker()
	broadcaster := &fakeBroadcaster{}
	transport := &fakeTransport{}
	legacy := &recordingLegacyCache{}
	votes := &fakeVotes{}

	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	service, err := NewService(Options{
		Repository:  repository,
		Tracker:     tracker,
		Legacy:      legacy,
		Broadcaster: broadcaster,
		Publisher:   publisher,
		Votes:       votes,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &harness{
		service: service, repository: repository, tracker: tracker,
		broadcaster: broadcaster, transport: transport, legacy: legacy, votes: votes,
	}
}

func (h *harness) settle(t *testing.T, round int) error {
	t.Helper()
	message, err := SettleMessage(SettlePayload{
		RoomSerial: testSerial, WinnerID: 1, LoserID: 2,
		CurrentRound: round, OfRound: 8, RemainElements: 7,
	})
	if err != nil {
		t.Fatalf("SettleMessage() error = %v", err)
	}
	return h.service.handleSettle(context.Background(), message)
}

func (h *harness) round(t *testing.T, gameSerial string) error {
	t.Helper()
	message, err := RoundMessage(RoundPayload{RoomSerial: testSerial, GameSerial: gameSerial})
	if err != nil {
		t.Fatalf("RoundMessage() error = %v", err)
	}
	return h.service.handleRound(context.Background(), message)
}

func (h *harness) refresh(t *testing.T) error {
	t.Helper()
	message, err := RefreshMessage(testSerial)
	if err != nil {
		t.Fatalf("RefreshMessage() error = %v", err)
	}
	return h.service.handleRefresh(context.Background(), message)
}

// ---------- tests ----------

func TestSettleAppliesTheRoundThenAsksForARefresh(t *testing.T) {
	h := newHarness(t)
	if err := h.settle(t, 3); err != nil {
		t.Fatalf("handleSettle() error = %v", err)
	}

	if len(h.repository.settled) != 1 {
		t.Fatalf("settled %d rounds, want 1", len(h.repository.settled))
	}
	outcome := h.repository.settled[0]
	if outcome.RoomID != 77 || outcome.CurrentRound != 3 || outcome.RemainElements != 7 {
		t.Errorf("settled the wrong round: %+v", outcome)
	}

	published := h.transport.published()
	if len(published) != 1 {
		t.Fatalf("published %d messages, want 1", len(published))
	}
	if published[0].Type != TypeRankRefresh || published[0].Queue != Queue {
		t.Errorf("published %q on %q, want %q on %q",
			published[0].Type, published[0].Queue, TypeRankRefresh, Queue)
	}

	if outstanding, _ := h.tracker.Outstanding(context.Background(), testSerial); !outstanding.Pending() {
		t.Error("a settled round must leave a refresh outstanding")
	}
}

// The ordering that makes the version counter safe. If MarkChanged ran before the
// settlement committed, a refresh slipping in between would tally the room without
// this round and then mark that version applied, and the round's scores would never
// be counted.
func TestSettleDoesNotRaiseTheVersionWhenTheWriteFails(t *testing.T) {
	h := newHarness(t)
	h.repository.settleErr = errors.New("deadlock")

	if err := h.settle(t, 1); err == nil {
		t.Fatal("handleSettle() should fail when the settlement fails")
	}
	outstanding, _ := h.tracker.Outstanding(context.Background(), testSerial)
	if outstanding.Version != 0 {
		t.Fatalf("version = %d after a failed settlement, want 0", outstanding.Version)
	}
	if len(h.transport.published()) != 0 {
		t.Fatal("no refresh should be published when the settlement failed")
	}
}

func TestRefreshRecomputesRanksAndBroadcasts(t *testing.T) {
	h := newHarness(t)
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	if h.repository.recomputeCount() != 1 {
		t.Errorf("recomputed %d times, want 1", h.repository.recomputeCount())
	}
	if len(h.repository.rankPasses) != 1 {
		t.Errorf("ran %d rank passes, want 1", len(h.repository.rankPasses))
	}
	if h.broadcaster.count() != 1 {
		t.Fatalf("sent %d broadcasts, want 1", h.broadcaster.count())
	}

	sent := h.broadcaster.sent[0]
	if sent.channel != "game-room."+testSerial {
		t.Errorf("broadcast on %q, want %q", sent.channel, "game-room."+testSerial)
	}
	if sent.event != BroadcastEvent {
		t.Errorf("broadcast event %q, want %q", sent.event, BroadcastEvent)
	}
	if _, ok := sent.payload.(Leaderboard); !ok {
		t.Errorf("broadcast payload is %T, want Leaderboard", sent.payload)
	}
	if h.repository.leaderboardOf != 77 {
		t.Errorf("read the leaderboard of room %d, want 77", h.repository.leaderboardOf)
	}
}

// Every backend failure in the refresh path is transient by nature — a deadlock, a
// dropped connection, an unreachable Redis — so none of them may dead-letter a
// vote's score. Each is also checked for leaving the work outstanding, because a
// failure that silently records progress loses the vote instead.
func TestEveryBackendFailureIsRetryableAndKeepsTheWorkOutstanding(t *testing.T) {
	failures := map[string]func(*harness){
		"recompute fails": func(h *harness) { h.repository.recomputeErr = errors.New("deadlock") },
		"rank pass fails": func(h *harness) { h.repository.rankErr = errors.New("lock wait timeout") },
		"leaderboard read fails": func(h *harness) {
			h.repository.boardErr = errors.New("connection reset")
		},
		"room lookup fails": func(h *harness) {
			h.repository.roomLookupErr = errors.New("connection refused")
		},
		"reading the version fails": func(h *harness) {
			h.tracker.outstandingErr = errors.New("redis unreachable")
		},
		"recording the version fails": func(h *harness) {
			h.tracker.markAppliedErr = errors.New("redis unreachable")
		},
	}

	for name, breakIt := range failures {
		h := newHarness(t)
		if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
			t.Fatalf("%s: MarkChanged() error = %v", name, err)
		}
		breakIt(h)

		err := h.refresh(t)
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if jobs.IsPermanent(err) {
			t.Errorf("%s: dead-lettered a transient failure: %v", name, err)
		}

		// Clear the Redis failure so the state can be read back; a database failure
		// leaves the tracker untouched anyway.
		h.tracker.outstandingErr = nil
		outstanding, readErr := h.tracker.Outstanding(context.Background(), testSerial)
		if readErr != nil {
			t.Errorf("%s: Outstanding() error = %v", name, readErr)
			continue
		}
		if !outstanding.Pending() {
			t.Errorf("%s: the refresh recorded progress it did not make", name)
		}
	}
}

// The settlement committed but the version bump did not. The message must retry:
// the UPDATEs are idempotent, so re-running them is safe, and without the bump no
// refresh would ever be triggered for this round.
func TestSettleRetriesWhenTheVersionBumpFails(t *testing.T) {
	h := newHarness(t)
	h.tracker.markChangedErr = errors.New("redis unreachable")

	err := h.settle(t, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	if jobs.IsPermanent(err) {
		t.Fatalf("dead-lettered a transient Redis failure: %v", err)
	}
	if len(h.transport.published()) != 0 {
		t.Fatal("no refresh should be published when the version was not raised")
	}

	h.tracker.markChangedErr = nil
	if err := h.settle(t, 1); err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if outstanding, _ := h.tracker.Outstanding(context.Background(), testSerial); !outstanding.Pending() {
		t.Fatal("the retry must leave a refresh outstanding")
	}
}

// THE POINT OF THE REWRITE. A burst of votes must produce one recompute, not one
// per vote, and it must do so without a delayed re-dispatch job.
func TestRefreshCoalescesABurstOfVotes(t *testing.T) {
	h := newHarness(t)

	// Ten votes land. Each settles and publishes its own refresh request.
	for round := 1; round <= 10; round++ {
		if err := h.settle(t, round); err != nil {
			t.Fatalf("handleSettle(%d) error = %v", round, err)
		}
	}
	if got := len(h.transport.published()); got != 10 {
		t.Fatalf("published %d refresh requests, want 10", got)
	}

	// The refreshes then run one after another, as the per-room lock arranges.
	for index := 0; index < 10; index++ {
		if err := h.refresh(t); err != nil {
			t.Fatalf("handleRefresh() error = %v", err)
		}
	}

	if got := h.repository.recomputeCount(); got != 1 {
		t.Fatalf("recomputed %d times for 10 coalesced votes, want 1", got)
	}
	if got := h.broadcaster.count(); got != 1 {
		t.Fatalf("broadcast %d times, want 1", got)
	}
}

// The other half of coalescing: a vote arriving after a refresh has read the
// version must still be picked up, or the burst's tail is silently dropped. This is
// the case ReUpdateGameRoomRank was built for.
func TestRefreshPicksUpAVoteThatArrivesDuringTheWork(t *testing.T) {
	h := newHarness(t)
	if err := h.settle(t, 1); err != nil {
		t.Fatalf("handleSettle() error = %v", err)
	}

	// A second vote lands while the first refresh is between reading the version and
	// finishing, modelled by settling again before the second refresh runs.
	if err := h.refresh(t); err != nil {
		t.Fatalf("first handleRefresh() error = %v", err)
	}
	if err := h.settle(t, 2); err != nil {
		t.Fatalf("second handleSettle() error = %v", err)
	}
	if err := h.refresh(t); err != nil {
		t.Fatalf("second handleRefresh() error = %v", err)
	}

	if got := h.repository.recomputeCount(); got != 2 {
		t.Fatalf("recomputed %d times, want 2: the later vote must trigger its own pass", got)
	}
}

// A repeated broadcast only makes clients redraw. A missed one leaves the room on a
// stale leaderboard until the next vote, so the broadcast must happen before the
// version is recorded.
func TestRefreshDoesNotRecordProgressWhenTheBroadcastFails(t *testing.T) {
	h := newHarness(t)
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	h.broadcaster.err = errors.New("soketi unreachable")

	if err := h.refresh(t); err == nil {
		t.Fatal("handleRefresh() should fail when the broadcast fails")
	}
	outstanding, _ := h.tracker.Outstanding(context.Background(), testSerial)
	if !outstanding.Pending() {
		t.Fatal("a failed broadcast must leave the refresh outstanding so the retry redoes it")
	}

	// The retry succeeds and does broadcast.
	h.broadcaster.err = nil
	if err := h.refresh(t); err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if h.broadcaster.count() != 1 {
		t.Fatalf("sent %d broadcasts after the retry, want 1", h.broadcaster.count())
	}
}

func TestRefreshInvalidatesTheLegacyCacheBeforeBroadcasting(t *testing.T) {
	h := newHarness(t)
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	if len(h.legacy.invalidated) != 1 || h.legacy.invalidated[0] != testSerial {
		t.Errorf("invalidated %v, want [%s]", h.legacy.invalidated, testSerial)
	}
	if h.legacy.clearedCount() != 1 {
		t.Errorf("cleared the updating flag %d times, want 1", h.legacy.clearedCount())
	}
}

// A stale legacy cache is a nuisance; repeating the whole recompute to fix it is
// worse, so the failure must not fail the job.
func TestRefreshSurvivesALegacyCacheFailure(t *testing.T) {
	h := newHarness(t)
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	h.legacy.err = errors.New("redis unreachable")

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v, want nil", err)
	}
	if h.broadcaster.count() != 1 {
		t.Error("the broadcast must still happen when the legacy cache cannot be cleared")
	}
}

// The updating flag is what the PHP endpoint reports as rank_updating. It must stay
// set while work is outstanding, or the UI stops showing that a refresh is coming.
func TestRefreshLeavesTheUpdatingFlagSetWhileWorkRemains(t *testing.T) {
	h := newHarness(t)
	// Two votes, one refresh: the second is still outstanding when the first
	// finishes, so the flag must not be cleared yet.
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}

	// Raise the version again after the refresh reads it, by wrapping the repository
	// so the extra vote lands mid-recompute.
	h.repository.recomputeHook = func() {
		if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
			t.Errorf("mid-flight MarkChanged() error = %v", err)
		}
	}

	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if h.legacy.clearedCount() != 0 {
		t.Fatal("the updating flag was cleared while a later vote was still outstanding")
	}
}

func TestRefreshOnAnUpToDateRoomDoesNothing(t *testing.T) {
	h := newHarness(t)
	// No votes: version and applied are both zero.
	if err := h.refresh(t); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if h.repository.recomputeCount() != 0 || h.broadcaster.count() != 0 {
		t.Fatal("a refresh with nothing outstanding must not touch the database or broadcast")
	}
}

func TestMalformedPayloadsAreNotRetried(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	cases := map[string]queue.Message{
		"settle with broken json":  {Queue: Queue, Type: TypeBetSettled, Payload: json.RawMessage(`{`)},
		"refresh with broken json": {Queue: Queue, Type: TypeRankRefresh, Payload: json.RawMessage(`{`)},
		"settle with no serial": {Queue: Queue, Type: TypeBetSettled,
			Payload: json.RawMessage(`{"winner_id":1,"loser_id":2}`)},
		"settle with the same element on both sides": {Queue: Queue, Type: TypeBetSettled,
			Payload: json.RawMessage(`{"room_serial":"x","winner_id":1,"loser_id":1}`)},
		"refresh with no serial": {Queue: Queue, Type: TypeRankRefresh, Payload: json.RawMessage(`{}`)},
	}
	for name, message := range cases {
		var err error
		if message.Type == TypeBetSettled {
			err = h.service.handleSettle(ctx, message)
		} else {
			err = h.service.handleRefresh(ctx, message)
		}
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if !jobs.IsPermanent(err) {
			t.Errorf("%s: error is retryable, want permanent: %v", name, err)
		}
	}
}

func TestUnknownRoomIsPermanent(t *testing.T) {
	h := newHarness(t)
	message, err := SettleMessage(SettlePayload{
		RoomSerial: "does-not-exist", WinnerID: 1, LoserID: 2, CurrentRound: 1, OfRound: 2, RemainElements: 1,
	})
	if err != nil {
		t.Fatalf("SettleMessage() error = %v", err)
	}
	err = h.service.handleSettle(context.Background(), message)
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("an unknown room must be permanent, got %v", err)
	}
}

// A transient database failure must stay retryable, or a deadlock would dead-letter
// a vote's score.
func TestDatabaseFailuresStayRetryable(t *testing.T) {
	h := newHarness(t)
	if _, err := h.tracker.MarkChanged(context.Background(), testSerial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	h.repository.recomputeErr = errors.New("Deadlock found when trying to get lock")

	err := h.refresh(t)
	if err == nil {
		t.Fatal("expected an error")
	}
	if jobs.IsPermanent(err) {
		t.Fatalf("a deadlock must be retried, not dead-lettered: %v", err)
	}
}

func TestRegistrationsStateTheirContract(t *testing.T) {
	h := newHarness(t)
	registry := jobs.NewRegistry()
	registry.MustRegister(h.service.SettleRegistration(), h.service.RefreshRegistration())

	for _, messageType := range []string{TypeBetSettled, TypeRankRefresh} {
		registration, err := registry.Lookup(messageType)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v", messageType, err)
		}
		if registration.Timeout <= 0 || registration.MaxAttempts < 1 {
			t.Errorf("%s has no usable contract: %+v", messageType, registration)
		}
		if registration.SerialKey == nil {
			t.Fatalf("%s must serialize per room", messageType)
		}
	}
}

// The two locks must be different keys, or every vote in a busy room queues behind
// the refresh it triggered.
func TestSerialKeysAreScopedPerRoomAndPerConcern(t *testing.T) {
	h := newHarness(t)

	settleMessage, err := SettleMessage(SettlePayload{
		RoomSerial: testSerial, WinnerID: 1, LoserID: 2, CurrentRound: 1, OfRound: 2, RemainElements: 1,
	})
	if err != nil {
		t.Fatalf("SettleMessage() error = %v", err)
	}
	refreshMessage, err := RefreshMessage(testSerial)
	if err != nil {
		t.Fatalf("RefreshMessage() error = %v", err)
	}

	settleKey, err := h.service.SettleRegistration().SerialKey(settleMessage)
	if err != nil {
		t.Fatalf("settle serial key: %v", err)
	}
	refreshKey, err := h.service.RefreshRegistration().SerialKey(refreshMessage)
	if err != nil {
		t.Fatalf("refresh serial key: %v", err)
	}

	if settleKey == refreshKey {
		t.Fatalf("both concerns share the lock key %q", settleKey)
	}
	for _, key := range []string{settleKey, refreshKey} {
		if !containsSerial(key, testSerial) {
			t.Errorf("lock key %q does not scope to the room", key)
		}
	}

	// A different room must not contend.
	otherMessage, err := RefreshMessage("room-other")
	if err != nil {
		t.Fatalf("RefreshMessage() error = %v", err)
	}
	otherKey, err := h.service.RefreshRegistration().SerialKey(otherMessage)
	if err != nil {
		t.Fatalf("other serial key: %v", err)
	}
	if otherKey == refreshKey {
		t.Fatal("two rooms share one refresh lock")
	}
}

func containsSerial(key, serial string) bool {
	return len(key) > len(serial) && key[len(key)-len(serial):] == serial
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	publisher, err := queue.NewPublisher(&fakeTransport{})
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	complete := Options{
		Repository:  newFakeRepository(testSerial),
		Tracker:     newFakeTracker(),
		Broadcaster: &fakeBroadcaster{},
		Publisher:   publisher,
		Votes:       &fakeVotes{},
	}
	for name, mutate := range map[string]func(*Options){
		"no repository":  func(o *Options) { o.Repository = nil },
		"no tracker":     func(o *Options) { o.Tracker = nil },
		"no broadcaster": func(o *Options) { o.Broadcaster = nil },
		"no publisher":   func(o *Options) { o.Publisher = nil },
		"no vote reader": func(o *Options) { o.Votes = nil },
	} {
		options := complete
		mutate(&options)
		if _, err := NewService(options); err == nil {
			t.Errorf("NewService() should reject the %q case", name)
		}
	}

	// Legacy cache and scoring are optional and default.
	service, err := NewService(complete)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service.scoring != DefaultScoring() {
		t.Errorf("scoring = %+v, want the production values", service.scoring)
	}
}

// ---------- the pairing broadcast ----------

/**
 * What a participant needs when the host moves on: the game the room now follows and the
 * tally for the pair inside it. Both come from the same query the room's REST endpoint
 * answers with, so a pushed pairing and a polled one cannot disagree.
 */
func TestRoundBroadcastsTheTallyForThePairingOnScreen(t *testing.T) {
	h := newHarness(t)
	h.votes.inProgress = true
	h.votes.tally = VoteTally{
		FirstCandidate: 11, SecondCandidate: 22,
		FirstCandidateVotes: 2, SecondCandidateVote: 1,
		TotalVotes: 3, CurrentRound: 5, OfRound: 8, RemainElements: 4,
	}

	if err := h.round(t, "game-serial"); err != nil {
		t.Fatalf("handleRound() error = %v", err)
	}

	if h.votes.calls != 1 || h.votes.roomID != 77 || h.votes.gameSerial != "game-serial" {
		t.Errorf("read votes %d times for room %d game %q, want once for room 77 game-serial",
			h.votes.calls, h.votes.roomID, h.votes.gameSerial)
	}
	if len(h.broadcaster.sent) != 1 {
		t.Fatalf("%d broadcasts, want 1", len(h.broadcaster.sent))
	}
	sent := h.broadcaster.sent[0]
	if sent.channel != realtime.GameRoomChannel(testSerial) || sent.event != RoundEvent {
		t.Errorf("broadcast %q on %q, want %q on the room channel", sent.event, sent.channel, RoundEvent)
	}
	payload, ok := sent.payload.(RoundBroadcast)
	if !ok {
		t.Fatalf("payload = %T, want RoundBroadcast", sent.payload)
	}
	if payload.GameSerial != "game-serial" {
		t.Errorf("game serial = %q, want the one the message named", payload.GameSerial)
	}
	if payload.Votes == nil || *payload.Votes != h.votes.tally {
		t.Errorf("votes = %+v, want the tally the store returned", payload.Votes)
	}
}

// Between rounds there is no pair to tally, and the event still goes out: the game serial
// alone is the news, because a participant holding a stale one has to reload the room.
func TestRoundBroadcastsWithoutATallyBetweenRounds(t *testing.T) {
	h := newHarness(t)
	h.votes.inProgress = false

	if err := h.round(t, "game-serial"); err != nil {
		t.Fatalf("handleRound() error = %v", err)
	}
	if len(h.broadcaster.sent) != 1 {
		t.Fatalf("%d broadcasts, want 1 even with no round in progress", len(h.broadcaster.sent))
	}
	payload, ok := h.broadcaster.sent[0].payload.(RoundBroadcast)
	if !ok {
		t.Fatalf("payload = %T, want RoundBroadcast", h.broadcaster.sent[0].payload)
	}
	if payload.Votes != nil {
		t.Errorf("votes = %+v, want nothing when no round is in progress", payload.Votes)
	}
}

// A message with no game serial names no pairing, so no retry can fix it. RoundMessage
// refuses to build one, so this comes in over the wire — an older publisher, or a hand-fed
// queue — and the handler has to refuse it too.
func TestRoundRejectsAPayloadWithoutAGame(t *testing.T) {
	h := newHarness(t)
	message := queue.Message{
		Type:    TypeRoundChanged,
		Queue:   Queue,
		Payload: []byte(`{"room_serial":"` + testSerial + `"}`),
	}

	err := h.service.handleRound(context.Background(), message)
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("handleRound() error = %v, want a permanent failure", err)
	}
	if h.broadcaster.count() != 0 || h.votes.calls != 0 {
		t.Error("a malformed round message must not read or broadcast anything")
	}
}

// A room that is gone stays gone: retrying the broadcast cannot bring it back.
func TestRoundIsPermanentForARoomThatIsGone(t *testing.T) {
	h := newHarness(t)
	message, err := RoundMessage(RoundPayload{RoomSerial: "room-missing", GameSerial: "game-serial"})
	if err != nil {
		t.Fatalf("RoundMessage() error = %v", err)
	}

	if err := h.service.handleRound(context.Background(), message); err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("handleRound() error = %v, want a permanent failure", err)
	}
}

// A broadcast failure is retryable — Pusher being briefly unreachable is not the message's
// fault, and the pairing is still worth delivering a moment later.
func TestRoundRetriesABroadcastFailure(t *testing.T) {
	h := newHarness(t)
	h.broadcaster.err = errors.New("pusher is unreachable")

	err := h.round(t, "game-serial")
	if err == nil {
		t.Fatal("handleRound() should fail when the broadcast fails")
	}
	if jobs.IsPermanent(err) {
		t.Errorf("error = %v, want it retryable", err)
	}
}
