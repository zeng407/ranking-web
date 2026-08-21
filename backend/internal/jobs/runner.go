package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/queue"
)

// LockKeyPrefix namespaces the worker's per-key locks. It must differ from the
// scheduler's prefix.
const LockKeyPrefix = "2pick:go:job-lock:"

// SerialLockTTL bounds how long a serialization lock is held if the process dies
// mid-handler. It exceeds the longest handler timeout so a healthy run never
// loses its own lock, while a crashed one does not block the key forever.
const SerialLockTTL = 5 * time.Minute

// Reserver is the consume side of the queue.
type Reserver interface {
	Reserve(ctx context.Context, queues []string, block time.Duration) (*queue.Reservation, error)
	RecoverProcessing(ctx context.Context, queues []string) (int, error)
}

// Locker serializes handlers that touch the same rows.
type Locker interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (redislock.Releaser, bool, error)
}

type Runner struct {
	reserver Reserver
	registry *Registry
	locker   Locker
	logger   *slog.Logger
	queues   []string
	// concurrency is how many handlers may run at once.
	concurrency int
	// jobTimeout is the ceiling applied when a registration does not set its own.
	jobTimeout time.Duration

	processed    atomic.Int64
	failed       atomic.Int64
	retried      atomic.Int64
	deadLettered atomic.Int64
	deferred     atomic.Int64
}

type RunnerOptions struct {
	Reserver    Reserver
	Registry    *Registry
	Locker      Locker
	Logger      *slog.Logger
	Queues      []string
	Concurrency int
	JobTimeout  time.Duration
}

func NewRunner(options RunnerOptions) (*Runner, error) {
	if options.Reserver == nil {
		return nil, errors.New("jobs: reserver is required")
	}
	if options.Registry == nil {
		return nil, errors.New("jobs: registry is required")
	}
	if options.Locker == nil {
		return nil, errors.New("jobs: locker is required")
	}
	if len(options.Queues) == 0 {
		return nil, errors.New("jobs: at least one queue is required")
	}
	if options.Concurrency < 1 {
		return nil, errors.New("jobs: concurrency must be at least 1")
	}
	if options.JobTimeout <= 0 {
		return nil, errors.New("jobs: job timeout must be positive")
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	return &Runner{
		reserver:    options.Reserver,
		registry:    options.Registry,
		locker:      options.Locker,
		logger:      options.Logger,
		queues:      options.Queues,
		concurrency: options.Concurrency,
		jobTimeout:  options.JobTimeout,
	}, nil
}

// Stats reports counters for logs and the health endpoint.
func (runner *Runner) Stats() (processed, failed, retried, deadLettered, deferred int64) {
	return runner.processed.Load(), runner.failed.Load(), runner.retried.Load(),
		runner.deadLettered.Load(), runner.deferred.Load()
}

// Run consumes until ctx is cancelled, then returns once every in-flight handler
// has finished.
//
// It first recovers anything stranded on the processing lists by a previous
// worker that was killed mid-handler; without that step those messages would
// never be delivered again.
func (runner *Runner) Run(ctx context.Context) error {
	recovered, err := runner.reserver.RecoverProcessing(ctx, runner.queues)
	if err != nil {
		return fmt.Errorf("jobs: recover stranded messages: %w", err)
	}
	if recovered > 0 {
		runner.logger.Warn("worker_recovered_stranded_messages", "count", recovered)
	}

	var waitGroup sync.WaitGroup
	for index := 0; index < runner.concurrency; index++ {
		waitGroup.Add(1)
		go func(slot int) {
			defer waitGroup.Done()
			runner.consume(ctx, slot)
		}(index)
	}
	waitGroup.Wait()
	return nil
}

func (runner *Runner) consume(ctx context.Context, slot int) {
	for {
		if ctx.Err() != nil {
			return
		}

		reservation, err := runner.reserver.Reserve(ctx, runner.queues, queue.ReserveBlockTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// An undecodable entry is already dead-lettered by Reserve; anything
			// else is a Redis problem. Either way, pause briefly so a persistent
			// failure does not become a hot loop.
			runner.logger.Error("worker_reserve_failed", "slot", slot, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(queue.ReserveFailurePause):
			}
			continue
		}
		if reservation == nil {
			continue
		}

		runner.process(ctx, reservation)
	}
}

func (runner *Runner) process(ctx context.Context, reservation *queue.Reservation) {
	message := reservation.Message
	logger := runner.logger.With(
		"type", message.Type,
		"queue", message.Queue,
		"attempt", reservation.Attempt(),
		"idempotency_key", message.IdempotencyKey,
	)

	registration, err := runner.registry.Lookup(message.Type)
	if err != nil {
		// No handler will ever appear for this type by retrying, so it goes
		// straight to the dead-letter queue where it stays visible.
		logger.Error("worker_unknown_message_type", "error", err)
		runner.deadLetter(ctx, reservation, logger)
		return
	}
	logger = logger.With("laravel_job", registration.LaravelJob)

	release, proceed := runner.acquireSerialLock(ctx, registration, reservation, logger)
	if !proceed {
		return
	}
	if release != nil {
		defer func() {
			if err := release.Release(context.WithoutCancel(ctx)); err != nil && !errors.Is(err, redislock.ErrNotHeld) {
				logger.Warn("worker_serial_lock_release_failed", "error", err)
			}
		}()
	}

	timeout := registration.Timeout
	if timeout > runner.jobTimeout {
		// The worker's ceiling wins: exceeding it would let a job outlive the
		// window inside which redelivery is assumed not to happen.
		timeout = runner.jobTimeout
	}

	// Detached from the shutdown context so a handler already running gets its
	// full timeout to finish rather than being cut off mid-write. Run() waits for
	// it, bounded by the process shutdown timeout.
	handlerContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	started := time.Now()
	handlerErr := registration.Handler.Handle(handlerContext, message)
	duration := time.Since(started)

	if handlerErr == nil {
		if err := reservation.Ack(context.WithoutCancel(ctx)); err != nil {
			logger.Error("worker_ack_failed", "error", err, "duration_ms", duration.Milliseconds())
			return
		}
		runner.processed.Add(1)
		logger.Info("worker_job_completed", "duration_ms", duration.Milliseconds())
		return
	}

	runner.failed.Add(1)
	logger = logger.With("error", handlerErr, "duration_ms", duration.Milliseconds())

	if IsPermanent(handlerErr) {
		// A malformed payload cannot be fixed by waiting, so it skips the retry
		// budget entirely instead of holding the queue behind five backoffs.
		logger.Error("worker_job_failed_permanently")
		runner.deadLetter(ctx, reservation, logger)
		return
	}
	// retry dead-letters on its own once the attempt budget is spent.
	runner.retry(ctx, reservation, registration, logger)
}

// retry serves the backoff and returns the message to its queue with an
// incremented attempt count.
func (runner *Runner) retry(
	ctx context.Context,
	reservation *queue.Reservation,
	registration Registration,
	logger *slog.Logger,
) {
	if reservation.Attempt() >= registration.MaxAttempts {
		logger.Error("worker_job_exhausted_attempts", "max_attempts", registration.MaxAttempts)
		runner.deadLetter(ctx, reservation, logger)
		return
	}

	delay := queue.RetryDelay(reservation.Attempt())
	logger.Warn("worker_job_retrying", "retry_in", delay.String())
	// The backoff is served before requeueing, so a failing job does not spin
	// through its whole attempt budget in milliseconds.
	select {
	case <-ctx.Done():
		// Shutting down: requeue immediately rather than dropping the message.
	case <-time.After(delay):
	}
	if err := reservation.Retry(context.WithoutCancel(ctx)); err != nil {
		logger.Error("worker_retry_failed", "error", err)
		return
	}
	runner.retried.Add(1)
}

// acquireSerialLock takes the per-key lock when the registration asks for one.
//
// Every path that returns proceed=false must hand the reservation back, or the
// message would sit on the processing list until the next startup recovery.
func (runner *Runner) acquireSerialLock(
	ctx context.Context,
	registration Registration,
	reservation *queue.Reservation,
	logger *slog.Logger,
) (redislock.Releaser, bool) {
	if registration.SerialKey == nil {
		return nil, true
	}

	key, err := registration.SerialKey(reservation.Message)
	if err != nil {
		// The key is derived from the payload, so a failure here means the payload
		// is malformed and no retry will help.
		logger.Error("worker_serial_key_failed", "error", err)
		runner.deadLetter(ctx, reservation, logger)
		return nil, false
	}
	if key == "" {
		return nil, true
	}

	release, acquired, err := runner.locker.Acquire(ctx, key, SerialLockTTL)
	if err != nil {
		logger.Error("worker_serial_lock_failed", "serial_key", key, "error", err)
		// Redis is unhealthy rather than the job being bad, so this consumes an
		// attempt and backs off instead of dead-lettering.
		runner.retry(ctx, reservation, registration, logger)
		return nil, false
	}
	if !acquired {
		// Another delivery is doing this exact work right now. The message is not
		// failing, so it is handed back without consuming an attempt.
		runner.deferred.Add(1)
		logger.Info("worker_deferred_serial_conflict", "serial_key", key)
		// A brief pause keeps a lone conflicting message from spinning between
		// reserve and requeue.
		select {
		case <-ctx.Done():
		case <-time.After(deferredRequeueDelay):
		}
		if err := reservation.Requeue(context.WithoutCancel(ctx)); err != nil {
			logger.Error("worker_requeue_failed", "error", err)
		}
		return nil, false
	}
	return release, true
}

// deferredRequeueDelay paces a message that keeps losing the serialization race.
const deferredRequeueDelay = time.Second

func (runner *Runner) deadLetter(ctx context.Context, reservation *queue.Reservation, logger *slog.Logger) {
	if reservation == nil {
		return
	}
	if err := reservation.DeadLetter(context.WithoutCancel(ctx)); err != nil {
		logger.Error("worker_dead_letter_failed", "error", err)
		return
	}
	runner.deadLettered.Add(1)
}
