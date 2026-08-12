package scheduling

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/queue"
)

// stubReleaser records whether the runner gave the lock back.
type stubReleaser struct {
	mu       sync.Mutex
	releases int
	err      error
}

func (releaser *stubReleaser) Release(context.Context) error {
	releaser.mu.Lock()
	defer releaser.mu.Unlock()
	releaser.releases++
	return releaser.err
}

func (releaser *stubReleaser) count() int {
	releaser.mu.Lock()
	defer releaser.mu.Unlock()
	return releaser.releases
}

type stubLocker struct {
	mu       sync.Mutex
	refuse   bool
	err      error
	calls    int
	keys     []string
	releaser *stubReleaser
}

func (locker *stubLocker) Acquire(_ context.Context, key string, _ time.Duration) (redislock.Releaser, bool, error) {
	locker.mu.Lock()
	defer locker.mu.Unlock()
	locker.calls++
	locker.keys = append(locker.keys, key)
	if locker.err != nil {
		return nil, false, locker.err
	}
	if locker.refuse {
		return nil, false, nil
	}
	if locker.releaser == nil {
		locker.releaser = &stubReleaser{}
	}
	return locker.releaser, true, nil
}

type stubDispatcher struct {
	mu       sync.Mutex
	messages []queue.Message
	err      error
}

func (dispatcher *stubDispatcher) Publish(_ context.Context, messages ...queue.Message) error {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if dispatcher.err != nil {
		return dispatcher.err
	}
	dispatcher.messages = append(dispatcher.messages, messages...)
	return nil
}

func (dispatcher *stubDispatcher) published() []queue.Message {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	out := make([]queue.Message, len(dispatcher.messages))
	copy(out, dispatcher.messages)
	return out
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRunner(t *testing.T, locker Locker, dispatcher Dispatcher) *Runner {
	t.Helper()
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	runner, err := NewRunner(RunnerOptions{
		Locker:     locker,
		Dispatcher: dispatcher,
		Logger:     quietLogger(),
		Location:   taipei,
	})
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	return runner
}

func TestNewRunnerRequiresDependencies(t *testing.T) {
	taipei, _ := time.LoadLocation("Asia/Taipei")
	cases := map[string]RunnerOptions{
		"no locker":     {Dispatcher: &stubDispatcher{}, Location: taipei},
		"no dispatcher": {Locker: &stubLocker{}, Location: taipei},
		// An implicit timezone would silently shift every entry.
		"no location": {Locker: &stubLocker{}, Dispatcher: &stubDispatcher{}},
	}
	for name, options := range cases {
		if _, err := NewRunner(options); err == nil {
			t.Errorf("NewRunner() should reject the %s case", name)
		}
	}
}

func TestFireDispatchesWithAnIdempotencyKey(t *testing.T) {
	dispatcher := &stubDispatcher{}
	runner := newRunner(t, &stubLocker{}, dispatcher)

	entry := entryByName(t, "post-trend-all")
	runner.fire(entry)

	published := dispatcher.published()
	if len(published) != 1 {
		t.Fatalf("published = %#v", published)
	}
	if published[0].Type != "post_trend.create" || published[0].Queue != "default" {
		t.Fatalf("message = %#v", published[0])
	}
	if published[0].IdempotencyKey == "" {
		t.Fatal("a scheduled dispatch must carry an idempotency key")
	}
	if dispatched, _, _ := runner.Stats(); dispatched != 1 {
		t.Fatalf("dispatched = %d, want 1", dispatched)
	}
}

// The withoutOverlapping contract at the runner level: a held lock must skip the
// tick, and must not publish.
func TestFireSkipsWhenLockIsHeld(t *testing.T) {
	dispatcher := &stubDispatcher{}
	runner := newRunner(t, &stubLocker{refuse: true}, dispatcher)

	runner.fire(entryByName(t, "update-public-posts"))

	if published := dispatcher.published(); len(published) != 0 {
		t.Fatalf("a skipped tick must not publish, got %#v", published)
	}
	dispatched, skipped, failed := runner.Stats()
	if dispatched != 0 || skipped != 1 || failed != 0 {
		t.Fatalf("stats = (%d, %d, %d), want (0, 1, 0)", dispatched, skipped, failed)
	}
}

// A Redis failure while acquiring must not be mistaken for "no overlap" and let
// the work run unguarded.
func TestFireDoesNotDispatchWhenTheLockErrors(t *testing.T) {
	dispatcher := &stubDispatcher{}
	runner := newRunner(t, &stubLocker{err: errors.New("redis unreachable")}, dispatcher)

	runner.fire(entryByName(t, "post-trend-week"))

	if published := dispatcher.published(); len(published) != 0 {
		t.Fatalf("a failed lock must not publish, got %#v", published)
	}
	if _, _, failed := runner.Stats(); failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
}

func TestFireCountsDispatchFailure(t *testing.T) {
	dispatcher := &stubDispatcher{err: errors.New("redis unreachable")}
	runner := newRunner(t, &stubLocker{}, dispatcher)

	runner.fire(entryByName(t, "generate-sitemap"))

	dispatched, _, failed := runner.Stats()
	if dispatched != 0 || failed != 1 {
		t.Fatalf("stats = (%d, %d), want (0, 1)", dispatched, failed)
	}
}

func TestRegisterAddsOneCronEntryPerSchedule(t *testing.T) {
	runner := newRunner(t, &stubLocker{}, &stubDispatcher{})

	entries := Entries()
	if err := runner.Register(entries); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := runner.EntryCount(); got != len(entries) {
		t.Fatalf("EntryCount() = %d, want %d", got, len(entries))
	}
}

func TestRegisterRejectsAnInvalidSpec(t *testing.T) {
	runner := newRunner(t, &stubLocker{}, &stubDispatcher{})

	err := runner.Register([]Entry{{
		Name: "broken", Spec: "not a cron spec", LockTTL: time.Minute,
		Flag: "SCHEDULE_BROKEN", Message: sampleEntryMessage(),
	}})
	if err == nil {
		t.Fatal("Register() should reject an invalid cron spec")
	}
}

func TestRegisterRejectsDuplicateEntries(t *testing.T) {
	runner := newRunner(t, &stubLocker{}, &stubDispatcher{})

	entry := entryByName(t, "generate-sitemap")
	if err := runner.Register([]Entry{entry, entry}); err == nil {
		t.Fatal("Register() should reject duplicate entries")
	}
}

func TestStopWaitsForRunningTicks(t *testing.T) {
	runner := newRunner(t, &stubLocker{}, &stubDispatcher{})
	if err := runner.Register(Entries()); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	runner.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runner.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

// The lock must be given back as soon as the dispatch finishes. Holding it for
// the full TTL would turn the overlap guard into a rate limit, and
// update-public-posts fires every minute with a 60 minute TTL.
// The dispatch lock is keyed on the TICK, not on the entry, and is deliberately not
// released when the dispatch succeeds.
//
// Keying it on the entry name did not prevent anything: the lock was released as soon
// as the publish returned, so two replicas firing in the same second both acquired it
// in turn and both dispatched. A local rehearsal with two schedulers produced seven
// refreshes in four minutes where four were due.
func TestFireLocksTheTickAndKeepsIt(t *testing.T) {
	locker := &stubLocker{}
	runner := newRunner(t, locker, &stubDispatcher{})
	entry := entryByName(t, "update-public-posts")

	runner.fire(entry)

	locker.mu.Lock()
	keys := append([]string(nil), locker.keys...)
	locker.mu.Unlock()

	if len(keys) != 1 {
		t.Fatalf("acquired %d locks, want 1", len(keys))
	}
	// The key must identify the minute, not just the entry, or a second replica in the
	// same minute would be indistinguishable from the next tick.
	if keys[0] == entry.Name {
		t.Fatalf("lock key is the entry name %q; it must identify the tick", keys[0])
	}
	if !strings.HasPrefix(keys[0], entry.Name+":") {
		t.Fatalf("lock key %q should be the entry's idempotency key", keys[0])
	}

	// Held, not released: releasing would let a replica arriving a moment later
	// dispatch the same minute again.
	if got := locker.releaser.count(); got != 0 {
		t.Fatalf("released the tick lock %d times after a successful dispatch, want 0", got)
	}
}

// On failure the lock is released, so another replica can still get the tick out
// rather than the minute being lost to one Redis blip.
func TestFireReleasesTheTickLockWhenTheDispatchFails(t *testing.T) {
	locker := &stubLocker{}
	runner := newRunner(t, locker, &stubDispatcher{err: errors.New("redis unreachable")})

	runner.fire(entryByName(t, "update-public-posts"))

	if got := locker.releaser.count(); got != 1 {
		t.Fatalf("releases = %d, want 1 after a failed dispatch", got)
	}
	dispatched, _, failed := runner.Stats()
	if dispatched != 0 || failed != 1 {
		t.Fatalf("stats = (%d, %d), want (0, 1)", dispatched, failed)
	}
}

// A replica that loses the race for a tick skips it and does not count as a failure.
func TestFireSkipsATickAnotherReplicaAlreadyDispatched(t *testing.T) {
	locker := &stubLocker{refuse: true}
	runner := newRunner(t, locker, &stubDispatcher{})

	runner.fire(entryByName(t, "update-public-posts"))

	dispatched, skipped, failed := runner.Stats()
	if dispatched != 0 || skipped != 1 || failed != 0 {
		t.Fatalf("stats = (%d, %d, %d), want (0, 1, 0)", dispatched, skipped, failed)
	}
}

// Two schedulers, one tick: exactly one dispatch. This is the property the rehearsal
// found missing, modelled with a locker that behaves like Redis rather than always
// granting.
func TestTwoReplicasDispatchATickOnce(t *testing.T) {
	shared := &keyedLocker{held: map[string]bool{}}
	dispatcher := &stubDispatcher{}
	first := newRunner(t, shared, dispatcher)
	second := newRunner(t, shared, dispatcher)

	entry := entryByName(t, "update-public-posts")
	// Both fire for the same minute, which is what two replicas on the same cron do.
	first.fire(entry)
	second.fire(entry)

	if got := len(dispatcher.published()); got != 1 {
		t.Fatalf("published %d messages for one tick, want 1", got)
	}
	if dispatched, _, _ := first.Stats(); dispatched != 1 {
		t.Errorf("the first replica dispatched %d times, want 1", dispatched)
	}
	if _, skipped, _ := second.Stats(); skipped != 1 {
		t.Errorf("the second replica skipped %d times, want 1", skipped)
	}
}

// keyedLocker grants a key once, like a real SET NX.
type keyedLocker struct {
	mu   sync.Mutex
	held map[string]bool
}

func (locker *keyedLocker) Acquire(_ context.Context, key string, _ time.Duration) (redislock.Releaser, bool, error) {
	locker.mu.Lock()
	defer locker.mu.Unlock()
	if locker.held[key] {
		return nil, false, nil
	}
	locker.held[key] = true
	return &stubReleaser{}, true, nil
}

// A failed dispatch must still release, or one Redis blip would suppress every
// tick for the whole TTL.
func TestFireReleasesTheLockAfterAFailedDispatch(t *testing.T) {
	locker := &stubLocker{}
	runner := newRunner(t, locker, &stubDispatcher{err: errors.New("redis unreachable")})

	runner.fire(entryByName(t, "post-trend-month"))

	if got := locker.releaser.count(); got != 1 {
		t.Fatalf("releases = %d, want 1", got)
	}
}
