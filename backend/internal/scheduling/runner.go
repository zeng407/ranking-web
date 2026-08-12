package scheduling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/queue"
	"github.com/robfig/cron/v3"
)

// LockKeyPrefix namespaces the scheduler's locks. It must differ from the
// worker's prefix and from Laravel's own scheduler mutexes.
const LockKeyPrefix = "2pick:go:schedule-lock:"

// Locker takes the single-flight lock for an entry. Acquired=false means another
// run holds it and this tick must be skipped.
type Locker interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (redislock.Releaser, bool, error)
}

// Dispatcher publishes an entry's message. The scheduler owns no database
// writes, so a plain publish is correct here; work that must be transactional
// belongs in a worker job.
type Dispatcher interface {
	Publish(ctx context.Context, messages ...queue.Message) error
}

// DispatchTimeout bounds one dispatch. Enqueuing is a single Redis round trip,
// so anything slower means Redis is unhealthy and the tick should fail rather
// than pile up.
const DispatchTimeout = 10 * time.Second

type Runner struct {
	cron       *cron.Cron
	locker     Locker
	dispatcher Dispatcher
	logger     *slog.Logger
	location   *time.Location

	dispatched atomic.Int64
	skipped    atomic.Int64
	failed     atomic.Int64
}

type RunnerOptions struct {
	Locker     Locker
	Dispatcher Dispatcher
	Logger     *slog.Logger
	// Location must be explicit. Cron expressions are meaningless without it and
	// the container default is UTC while the application runs on Asia/Taipei.
	Location *time.Location
}

func NewRunner(options RunnerOptions) (*Runner, error) {
	if options.Locker == nil {
		return nil, errors.New("scheduling: locker is required")
	}
	if options.Dispatcher == nil {
		return nil, errors.New("scheduling: dispatcher is required")
	}
	if options.Location == nil {
		return nil, errors.New("scheduling: an explicit timezone is required")
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	return &Runner{
		cron:       cron.New(cron.WithLocation(options.Location)),
		locker:     options.Locker,
		dispatcher: options.Dispatcher,
		logger:     options.Logger,
		location:   options.Location,
	}, nil
}

// Register adds the entries to the cron schedule. Callers pass only the enabled
// ones.
func (runner *Runner) Register(entries []Entry) error {
	if err := Validate(entries); err != nil {
		return err
	}
	for _, entry := range entries {
		entry := entry
		if _, err := runner.cron.AddFunc(entry.Spec, func() {
			runner.fire(entry)
		}); err != nil {
			return fmt.Errorf("scheduling: register %q with spec %q: %w", entry.Name, entry.Spec, err)
		}
	}
	return nil
}

func (runner *Runner) Start() {
	runner.cron.Start()
}

// Stop stops scheduling new ticks and waits for running ones to finish, bounded
// by ctx.
func (runner *Runner) Stop(ctx context.Context) error {
	stopped := runner.cron.Stop()
	select {
	case <-stopped.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Entries reports the number of registered cron entries.
func (runner *Runner) EntryCount() int {
	return len(runner.cron.Entries())
}

// Stats reports counters for the health endpoint and logs.
func (runner *Runner) Stats() (dispatched, skipped, failed int64) {
	return runner.dispatched.Load(), runner.skipped.Load(), runner.failed.Load()
}

// fire runs one tick: take the lock, publish, release.
//
// The lock is released as soon as the dispatch completes rather than being held
// for the full TTL. Holding it longer would make the TTL a rate limit instead of
// an overlap guard, and update-public-posts runs every minute with a 60 minute
// TTL.
func (runner *Runner) fire(entry Entry) {
	firedAt := time.Now().In(runner.location)
	logger := runner.logger.With(
		"entry", entry.Name,
		"laravel_entry", entry.LaravelEntry,
		"fired_at", firedAt.Format(time.RFC3339),
	)

	ctx, cancel := context.WithTimeout(context.Background(), DispatchTimeout)
	defer cancel()

	message := entry.Message
	message.IdempotencyKey = IdempotencyKey(entry.Name, firedAt)

	// Locked on the tick, not on the entry.
	//
	// Keying this on entry.Name did not do what the comment claimed. The lock is
	// released a few milliseconds later, as soon as the publish returns, so two
	// replicas firing in the same second both acquire it in turn and both dispatch.
	// A local rehearsal with two schedulers produced seven refreshes in four minutes
	// where four were due, and the worker's own per-key lock had to absorb the
	// duplicates.
	//
	// The tick key makes the dispatch itself idempotent: whichever replica gets there
	// first owns that minute, and the other finds the key taken and skips. A later
	// tick has a different key, so a legitimate fire is never suppressed.
	//
	// The entry's TTL is kept as the expiry. It is far longer than needed — these keys
	// could expire in seconds — but it is also the crash-safety bound, and one key per
	// tick per entry costs nothing.
	lock, acquired, err := runner.locker.Acquire(ctx, message.IdempotencyKey, entry.LockTTL)
	if err != nil {
		runner.failed.Add(1)
		logger.Error("schedule_lock_failed", "error", err)
		return
	}
	if !acquired {
		// Another replica already dispatched this tick.
		runner.skipped.Add(1)
		logger.Info("schedule_skipped_duplicate_tick", "idempotency_key", message.IdempotencyKey)
		return
	}

	if err := runner.dispatcher.Publish(ctx, message); err != nil {
		runner.failed.Add(1)
		logger.Error("schedule_dispatch_failed", "error", err)

		// Released only on failure, so another replica can still get this tick out.
		// Holding it here would lose the minute entirely for the sake of a Redis blip.
		if err := lock.Release(ctx); err != nil && !errors.Is(err, redislock.ErrNotHeld) {
			logger.Warn("schedule_lock_release_failed", "error", err)
		}
		return
	}

	// Deliberately NOT released. The key marks this tick as dispatched and has to
	// outlive the tick, or a replica arriving a moment later would find it free and
	// dispatch the same minute again — which is the bug this replaced.
	runner.dispatched.Add(1)
	logger.Info("schedule_dispatched",
		"queue", message.Queue,
		"type", message.Type,
		"idempotency_key", message.IdempotencyKey,
	)
}
