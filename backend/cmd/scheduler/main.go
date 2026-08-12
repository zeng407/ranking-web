// Command scheduler fires the recurring entries that Laravel's console Kernel
// currently owns.
//
// It only enqueues; every heavy operation runs in the worker. That keeps this
// process stateless and single-task, which is what lets a distributed lock make
// "withoutOverlapping" mean the same thing across replicas.
//
// It deliberately holds no database handle. If a schedule needs to read rows to
// decide what to enqueue, that decision belongs in a worker job, not here.
//
// Every entry is gated by its own feature flag, defaulting to off, because the
// same schedule running here and in Laravel would double-count trends and
// double-write public posts. Cutover is one entry at a time.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/health"
	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/platform/redisstore"
	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/scheduling"
	"github.com/redis/go-redis/v9"
)

var (
	version = "dev"
	commit  = "unknown"
)

const serviceName = "ranking-scheduler"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("scheduler_failed", "error", err)
		os.Exit(1)
	}
	logger.Info("scheduler_stopped")
}

func run(logger *slog.Logger) error {
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// Redis carries both the enqueue target and the overlap lock, so without it
	// the scheduler could neither dispatch nor stay single-flight.
	if !configuration.Redis.Enabled() {
		return errors.New("REDIS_ADDR is required by the scheduler")
	}

	redisClient := redisstore.Open(configuration.Redis)
	defer redisClient.Close()

	transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
	if err != nil {
		return fmt.Errorf("queue transport: %w", err)
	}
	// The scheduler publishes outside any transaction because it owns no writes,
	// so Publish is the correct entry point here rather than WithinTransaction.
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		return fmt.Errorf("queue publisher: %w", err)
	}
	locker, err := redislock.New(redisClient, scheduling.LockKeyPrefix)
	if err != nil {
		return fmt.Errorf("scheduler lock: %w", err)
	}

	runner, err := scheduling.NewRunner(scheduling.RunnerOptions{
		Locker:     locker,
		Dispatcher: publisher,
		Logger:     logger,
		Location:   configuration.Scheduler.Timezone,
	})
	if err != nil {
		return fmt.Errorf("scheduler runner: %w", err)
	}

	all := scheduling.Entries()
	if err := scheduling.Validate(all); err != nil {
		return err
	}
	enabled, disabled, err := scheduling.Select(all, os.Getenv)
	if err != nil {
		return err
	}
	if err := runner.Register(enabled); err != nil {
		return err
	}

	var ready atomic.Bool
	healthServer := health.NewServer(health.Options{
		Addr:        configuration.Scheduler.HealthAddr,
		ServiceName: serviceName,
		Version:     version,
		Commit:      commit,
		Environment: configuration.Environment,
		Ready:       readiness(&ready, redisClient),
		Logger:      logger,
	})

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	healthErrors := make(chan error, 1)
	go func() {
		logger.Info("scheduler_health_starting", "address", configuration.Scheduler.HealthAddr)
		healthErrors <- healthServer.ListenAndServe()
	}()

	now := time.Now().In(configuration.Scheduler.Timezone)
	logger.Info("scheduler_starting",
		"environment", configuration.Environment,
		"version", version,
		// Logged explicitly because a wrong timezone silently shifts every entry;
		// the container default is UTC while the application runs on Asia/Taipei.
		"timezone", configuration.Scheduler.Timezone.String(),
		"local_time", now.Format(time.RFC3339),
		"registered_entries", runner.EntryCount(),
		// Both lists are logged so an operator can see at a glance which entries
		// this process owns and which are still Laravel's.
		"enabled", scheduling.Names(enabled),
		"disabled", scheduling.Names(disabled),
	)
	runner.Start()
	ready.Store(true)

	select {
	case err := <-healthErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server: %w", err)
		}
	case <-rootContext.Done():
		logger.Info("scheduler_stopping")
	}

	ready.Store(false)
	shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
	defer cancel()

	// Stop scheduling first, then let an in-flight dispatch finish, so shutdown
	// cannot strand a tick that has taken a lock but not yet published.
	if err := runner.Stop(shutdownContext); err != nil {
		logger.Warn("scheduler_drain_incomplete", "error", err)
	}
	dispatched, skipped, failed := runner.Stats()
	logger.Info("scheduler_totals", "dispatched", dispatched, "skipped", skipped, "failed", failed)

	if err := healthServer.Shutdown(shutdownContext); err != nil {
		_ = healthServer.Close()
		return fmt.Errorf("health shutdown: %w", err)
	}
	return nil
}

func readiness(ready *atomic.Bool, redisClient *redis.Client) health.ReadyFunc {
	return func(ctx context.Context) error {
		if !ready.Load() {
			return errors.New("scheduler is not started")
		}
		probeContext, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := redisstore.Ping(probeContext, redisClient); err != nil {
			return fmt.Errorf("redis unreachable: %w", err)
		}
		return nil
	}
}
