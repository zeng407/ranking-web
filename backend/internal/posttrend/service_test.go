package posttrend

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

type recordedUpsert struct {
	rangeValue TimeRange
	counts     []PlayCount
}

type recordedPositions struct {
	rangeValue TimeRange
	window     *time.Time
	positions  []TrendPosition
}

type fakeRepository struct {
	mu sync.Mutex

	// countsFor is keyed by range so a test can control what each pass sees.
	countsFor map[TimeRange][]PlayCount

	upserts      []recordedUpsert
	resets       []TimeRange
	rankedCalls  []TimeRange
	positionRuns []recordedPositions

	rankedIDs []int64

	countsErr    error
	upsertErr    error
	resetErr     error
	rankedErr    error
	positionsErr error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		countsFor: map[TimeRange][]PlayCount{},
		rankedIDs: []int64{31, 17, 4},
	}
}

func (repository *fakeRepository) PlayCounts(
	_ context.Context, rangeValue TimeRange, windowStart *time.Time,
) ([]PlayCount, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.countsErr != nil {
		return nil, repository.countsErr
	}
	if counts, ok := repository.countsFor[rangeValue]; ok {
		return counts, nil
	}
	date := time.Date(2026, time.August, 6, 0, 0, 0, 0, time.UTC)
	if windowStart != nil {
		date = *windowStart
	}
	return []PlayCount{{PostID: 1, StartDate: date, Count: 9}}, nil
}

func (repository *fakeRepository) UpsertPlayCounts(
	_ context.Context, rangeValue TimeRange, counts []PlayCount,
) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.upsertErr != nil {
		return 0, repository.upsertErr
	}
	repository.upserts = append(repository.upserts, recordedUpsert{rangeValue: rangeValue, counts: counts})
	return int64(len(counts)), nil
}

func (repository *fakeRepository) ResetPositions(
	_ context.Context, rangeValue TimeRange, _ *time.Time,
) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.resetErr != nil {
		return 0, repository.resetErr
	}
	repository.resets = append(repository.resets, rangeValue)
	return 12, nil
}

func (repository *fakeRepository) RankedPosts(
	_ context.Context, rangeValue TimeRange, _ *time.Time, _ int,
) ([]int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.rankedErr != nil {
		return nil, repository.rankedErr
	}
	repository.rankedCalls = append(repository.rankedCalls, rangeValue)
	return repository.rankedIDs, nil
}

func (repository *fakeRepository) UpsertPositions(
	_ context.Context, rangeValue TimeRange, windowStart *time.Time, positions []TrendPosition,
) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.positionsErr != nil {
		return 0, repository.positionsErr
	}
	repository.positionRuns = append(repository.positionRuns,
		recordedPositions{rangeValue: rangeValue, window: windowStart, positions: positions})
	return int64(len(positions)), nil
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

// ---------- harness ----------

type harness struct {
	service    *Service
	repository *fakeRepository
	transport  *fakeTransport
}

// pinnedNow is a Thursday, so the week window is the Monday before it.
var pinnedNow = time.Date(2026, time.August, 6, 14, 0, 0, 0, time.UTC)

func newHarness(t *testing.T) *harness {
	t.Helper()

	repository := newFakeRepository()
	transport := &fakeTransport{}
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	service, err := NewService(Options{
		Repository: repository,
		Publisher:  publisher,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:   taipei,
		Now:        func() time.Time { return pinnedNow },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &harness{service: service, repository: repository, transport: transport}
}

func (h *harness) create(t *testing.T, argument string) error {
	t.Helper()
	body, err := json.Marshal(CreatePayload{Range: argument})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return h.service.handleCreate(context.Background(),
		queue.Message{Queue: Queue, Type: TypeCreate, Payload: body})
}

func (h *harness) positions(t *testing.T, message queue.Message) error {
	t.Helper()
	return h.service.handlePositions(context.Background(), message)
}

// ---------- tests ----------

func TestCreateWritesCountsThenAsksForPositions(t *testing.T) {
	h := newHarness(t)
	if err := h.create(t, "day"); err != nil {
		t.Fatalf("handleCreate() error = %v", err)
	}

	if len(h.repository.upserts) != 1 {
		t.Fatalf("wrote %d batches, want 1", len(h.repository.upserts))
	}
	// The stored enum, not the schedule argument.
	if got := h.repository.upserts[0].rangeValue; got != RangeToday {
		t.Errorf("wrote range %q, want %q", got, RangeToday)
	}

	published := h.transport.published()
	if len(published) != 1 {
		t.Fatalf("published %d messages, want 1", len(published))
	}
	if published[0].Type != TypeUpdatePositions {
		t.Errorf("published %q, want %q", published[0].Type, TypeUpdatePositions)
	}

	var payload PositionsPayload
	if err := json.Unmarshal(published[0].Payload, &payload); err != nil {
		t.Fatalf("decode follow-up payload: %v", err)
	}
	if payload.Range != RangeToday {
		t.Errorf("follow-up range = %q, want %q", payload.Range, RangeToday)
	}
	// The window is computed in Asia/Taipei, where the pinned instant is already the
	// 6th at 22:00.
	if payload.StartDate != "2026-08-06" {
		t.Errorf("follow-up start_date = %q, want 2026-08-06", payload.StartDate)
	}
}

// The positions pass reads exactly the rows the create pass writes, so publishing
// before the write would rank the previous run's counts.
func TestCreateDoesNotAskForPositionsWhenTheWriteFails(t *testing.T) {
	h := newHarness(t)
	h.repository.upsertErr = errors.New("deadlock")

	if err := h.create(t, "day"); err == nil {
		t.Fatal("handleCreate() should fail when the upsert fails")
	}
	if len(h.transport.published()) != 0 {
		t.Fatal("no positions message should be published when the counts were not written")
	}
}

func TestCreateForTheAllRangeCarriesNoStartDate(t *testing.T) {
	h := newHarness(t)
	if err := h.create(t, "all"); err != nil {
		t.Fatalf("handleCreate() error = %v", err)
	}

	var payload PositionsPayload
	if err := json.Unmarshal(h.transport.published()[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Range != RangeAll {
		t.Errorf("range = %q, want %q", payload.Range, RangeAll)
	}
	if payload.StartDate != "" {
		t.Errorf("start_date = %q, want empty for the all-time range", payload.StartDate)
	}
}

func TestPositionsResetsBeforeRankingAndNumbersFromOne(t *testing.T) {
	h := newHarness(t)
	message, err := PositionsMessage(RangeWeek, windowFor(t, "2026-08-03"))
	if err != nil {
		t.Fatalf("PositionsMessage() error = %v", err)
	}
	if err := h.positions(t, message); err != nil {
		t.Fatalf("handlePositions() error = %v", err)
	}

	if len(h.repository.resets) != 1 {
		t.Fatalf("reset %d times, want 1", len(h.repository.resets))
	}
	if len(h.repository.positionRuns) != 1 {
		t.Fatalf("wrote %d position batches, want 1", len(h.repository.positionRuns))
	}

	written := h.repository.positionRuns[0].positions
	// The fake returns three ids in ranked order.
	want := []TrendPosition{{PostID: 31, Position: 1}, {PostID: 17, Position: 2}, {PostID: 4, Position: 3}}
	if len(written) != len(want) {
		t.Fatalf("wrote %d positions, want %d", len(written), len(want))
	}
	for index, entry := range want {
		if written[index] != entry {
			t.Errorf("position %d = %+v, want %+v", index, written[index], entry)
		}
	}
	if window := h.repository.positionRuns[0].window; window == nil || window.Format("2006-01-02") != "2026-08-03" {
		t.Errorf("wrote window %v, want 2026-08-03", window)
	}
}

// A post that has dropped out of the top RankedLimit gets no upsert, so the reset is
// the only thing that stops it keeping last hour's position and outranking posts that
// are actually being played.
func TestPositionsStillResetsWhenNothingRanks(t *testing.T) {
	h := newHarness(t)
	h.repository.rankedIDs = nil

	message, err := PositionsMessage(RangeMonth, windowFor(t, "2026-08-01"))
	if err != nil {
		t.Fatalf("PositionsMessage() error = %v", err)
	}
	if err := h.positions(t, message); err != nil {
		t.Fatalf("handlePositions() error = %v", err)
	}
	if len(h.repository.resets) != 1 {
		t.Fatal("the group must be reset even when no post ranks")
	}
	if len(h.repository.positionRuns) != 1 || len(h.repository.positionRuns[0].positions) != 0 {
		t.Fatalf("wrote %+v, want an empty batch", h.repository.positionRuns)
	}
}

func TestPositionsRejectsAMismatchedWindow(t *testing.T) {
	h := newHarness(t)

	// A dated range with no start date cannot identify a group.
	missing, err := json.Marshal(PositionsPayload{Range: RangeWeek})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = h.positions(t, queue.Message{Queue: Queue, Type: TypeUpdatePositions, Payload: missing})
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("a dated range with no start date must be permanent, got %v", err)
	}

	// The all-time range carries none, so a start date means the payload was built
	// wrong and would rank a group that does not exist.
	extra, err := json.Marshal(PositionsPayload{Range: RangeAll, StartDate: "2026-08-01"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = h.positions(t, queue.Message{Queue: Queue, Type: TypeUpdatePositions, Payload: extra})
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("the all-time range with a start date must be permanent, got %v", err)
	}
}

func TestMalformedPayloadsAreNotRetried(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	cases := map[string]struct {
		messageType string
		payload     string
	}{
		"create with broken json":    {TypeCreate, `{`},
		"create with unknown range":  {TypeCreate, `{"range":"fortnight"}`},
		"create with no range":       {TypeCreate, `{}`},
		"positions with broken json": {TypeUpdatePositions, `{`},
		"positions unknown range":    {TypeUpdatePositions, `{"range":"day"}`},
		"positions bad date":         {TypeUpdatePositions, `{"range":"week","start_date":"not-a-date"}`},
	}
	for name, test := range cases {
		message := queue.Message{Queue: Queue, Type: test.messageType, Payload: json.RawMessage(test.payload)}
		var err error
		if test.messageType == TypeCreate {
			err = h.service.handleCreate(ctx, message)
		} else {
			err = h.service.handlePositions(ctx, message)
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

// "day" is the schedule's spelling and "today" the column's; a payload carrying
// either must reach the same group, or a manual re-run would write a second trend.
func TestPositionsRejectsTheScheduleSpelling(t *testing.T) {
	h := newHarness(t)
	payload := json.RawMessage(`{"range":"day","start_date":"2026-08-06"}`)
	err := h.service.handlePositions(context.Background(),
		queue.Message{Queue: Queue, Type: TypeUpdatePositions, Payload: payload})
	if err == nil || !jobs.IsPermanent(err) {
		t.Fatalf("the positions payload must carry the stored value, got %v", err)
	}
}

func TestBackendFailuresStayRetryable(t *testing.T) {
	failures := map[string]func(*harness){
		"counting plays fails": func(h *harness) { h.repository.countsErr = errors.New("lock wait timeout") },
		"writing counts fails": func(h *harness) { h.repository.upsertErr = errors.New("deadlock") },
		"resetting fails":      func(h *harness) { h.repository.resetErr = errors.New("connection reset") },
		"reading ranked fails": func(h *harness) { h.repository.rankedErr = errors.New("gone away") },
		"writing ranked fails": func(h *harness) { h.repository.positionsErr = errors.New("deadlock") },
	}
	for name, breakIt := range failures {
		h := newHarness(t)
		breakIt(h)

		var err error
		switch name {
		case "counting plays fails", "writing counts fails":
			err = h.create(t, "day")
		default:
			message, buildErr := PositionsMessage(RangeToday, windowFor(t, "2026-08-06"))
			if buildErr != nil {
				t.Fatalf("%s: PositionsMessage() error = %v", name, buildErr)
			}
			err = h.positions(t, message)
		}

		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if jobs.IsPermanent(err) {
			t.Errorf("%s: dead-lettered a transient failure: %v", name, err)
		}
	}
}

func TestRegistrationsStateTheirContract(t *testing.T) {
	h := newHarness(t)
	registry := jobs.NewRegistry()
	registry.MustRegister(h.service.CreateRegistration(), h.service.PositionsRegistration())

	for _, messageType := range []string{TypeCreate, TypeUpdatePositions} {
		registration, err := registry.Lookup(messageType)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v", messageType, err)
		}
		if registration.Timeout <= 0 || registration.MaxAttempts < 1 {
			t.Errorf("%s has no usable contract: %+v", messageType, registration)
		}
		if registration.SerialKey == nil {
			t.Errorf("%s must serialize per range", messageType)
		}
	}
}

// Two runs for one range would both reset to the sentinel and interleave their
// upserts, briefly leaving the home page ranked by nothing. Different ranges must not
// contend, or the four hourly schedules would queue behind each other.
func TestSerialKeysScopePerRange(t *testing.T) {
	h := newHarness(t)

	keyFor := func(argument string) string {
		body, err := json.Marshal(CreatePayload{Range: argument})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		key, err := h.service.CreateRegistration().SerialKey(
			queue.Message{Queue: Queue, Type: TypeCreate, Payload: body})
		if err != nil {
			t.Fatalf("SerialKey(%q) error = %v", argument, err)
		}
		return key
	}

	if keyFor("day") == keyFor("week") {
		t.Fatal("two ranges share one lock key")
	}
	// The two spellings of the same range must collapse onto one key.
	if keyFor("day") != keyFor("today") {
		t.Fatalf("\"day\" and \"today\" produced different keys: %q vs %q", keyFor("day"), keyFor("today"))
	}

	positionsKey := func(rangeValue TimeRange, date string) string {
		payload := PositionsPayload{Range: rangeValue, StartDate: date}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		key, err := h.service.PositionsRegistration().SerialKey(
			queue.Message{Queue: Queue, Type: TypeUpdatePositions, Payload: body})
		if err != nil {
			t.Fatalf("SerialKey error = %v", err)
		}
		return key
	}
	// Matches uniqueId() in the Laravel job: startDate . range.
	if positionsKey(RangeWeek, "2026-08-03") == positionsKey(RangeWeek, "2026-08-10") {
		t.Fatal("two weeks share one lock key")
	}
	if positionsKey(RangeWeek, "2026-08-03") == positionsKey(RangeMonth, "2026-08-03") {
		t.Fatal("two ranges on the same date share one lock key")
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	publisher, err := queue.NewPublisher(&fakeTransport{})
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	complete := Options{
		Repository: newFakeRepository(),
		Publisher:  publisher,
		Location:   time.UTC,
	}
	for name, mutate := range map[string]func(*Options){
		"no repository": func(o *Options) { o.Repository = nil },
		"no publisher":  func(o *Options) { o.Publisher = nil },
		// A missing location would silently compute the DATE windows in UTC, which is
		// eight hours behind the application timezone and lands on the wrong day for
		// part of every night.
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

func windowFor(t *testing.T, date string) *time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("parse %q: %v", date, err)
	}
	return &parsed
}
