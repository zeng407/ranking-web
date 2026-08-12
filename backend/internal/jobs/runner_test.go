package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

const testPrefix = "2pick:test:jobs:"

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set; skipping Redis integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis at %s is unreachable: %v", addr, err)
	}
	return client
}

type fixture struct {
	transport *queue.RedisTransport
	publisher *queue.Publisher
	locker    *redislock.Locker
	client    *redis.Client
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	client := testRedis(t)
	transport, err := queue.NewRedisTransport(client, testPrefix)
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	locker, err := redislock.New(client, testPrefix+"lock:")
	if err != nil {
		t.Fatalf("redislock.New() error = %v", err)
	}

	cleanup := func() {
		keys, _ := client.Keys(context.Background(), testPrefix+"*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		client.Close()
	})
	return &fixture{transport: transport, publisher: publisher, locker: locker, client: client}
}

func (f *fixture) runner(t *testing.T, registry *Registry, concurrency int) *Runner {
	t.Helper()
	runner, err := NewRunner(RunnerOptions{
		Reserver:    f.transport,
		Registry:    registry,
		Locker:      f.locker,
		Logger:      quietLogger(),
		Queues:      []string{"default"},
		Concurrency: concurrency,
		JobTimeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	return runner
}

// runUntil runs the worker until stop() reports true or the deadline passes.
func runUntil(t *testing.T, runner *Runner, stop func() bool, deadline time.Duration) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runner.Run(ctx)
	}()

	waited := time.Duration(0)
	for waited < deadline && !stop() {
		time.Sleep(20 * time.Millisecond)
		waited += 20 * time.Millisecond
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not return after the context was cancelled")
	}
}

func registryWith(t *testing.T, registration Registration) *Registry {
	t.Helper()
	registry := NewRegistry()
	if err := registry.Register(registration); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return registry
}

func TestRegistryRejectsIncompleteRegistration(t *testing.T) {
	handler := HandlerFunc(func(context.Context, queue.Message) error { return nil })
	cases := map[string]Registration{
		"no type":     {Handler: handler, Timeout: time.Second, MaxAttempts: 1},
		"no handler":  {Type: "a", Timeout: time.Second, MaxAttempts: 1},
		"no timeout":  {Type: "a", Handler: handler, MaxAttempts: 1},
		"no attempts": {Type: "a", Handler: handler, Timeout: time.Second},
	}
	for name, registration := range cases {
		if err := NewRegistry().Register(registration); err == nil {
			t.Errorf("Register() should reject the %s case", name)
		}
	}
}

func TestRegistryRejectsDuplicateType(t *testing.T) {
	registration := Registration{
		Type: "a", Handler: HandlerFunc(func(context.Context, queue.Message) error { return nil }),
		Timeout: time.Second, MaxAttempts: 1,
	}
	registry := registryWith(t, registration)
	if err := registry.Register(registration); err == nil {
		t.Fatal("Register() should reject a duplicate type")
	}
}

func TestLookupReportsUnknownType(t *testing.T) {
	if _, err := NewRegistry().Lookup("nope"); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("Lookup() error = %v, want %v", err, ErrUnknownType)
	}
}

func TestPermanentWrapping(t *testing.T) {
	if IsPermanent(errors.New("plain")) {
		t.Fatal("a plain error must be retryable")
	}
	sentinel := errors.New("bad payload")
	wrapped := Permanent(sentinel)
	if !IsPermanent(wrapped) {
		t.Fatal("Permanent() must mark the error as permanent")
	}
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("Permanent() must preserve the wrapped error")
	}
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil")
	}
}

func TestNewRunnerRequiresDependencies(t *testing.T) {
	f := newFixture(t)
	registry := NewRegistry()
	base := RunnerOptions{
		Reserver: f.transport, Registry: registry, Locker: f.locker,
		Queues: []string{"default"}, Concurrency: 1, JobTimeout: time.Second,
	}
	mutate := map[string]func(*RunnerOptions){
		"no reserver":    func(o *RunnerOptions) { o.Reserver = nil },
		"no registry":    func(o *RunnerOptions) { o.Registry = nil },
		"no locker":      func(o *RunnerOptions) { o.Locker = nil },
		"no queues":      func(o *RunnerOptions) { o.Queues = nil },
		"no concurrency": func(o *RunnerOptions) { o.Concurrency = 0 },
		"no timeout":     func(o *RunnerOptions) { o.JobTimeout = 0 },
	}
	for name, apply := range mutate {
		options := base
		apply(&options)
		if _, err := NewRunner(options); err == nil {
			t.Errorf("NewRunner() should reject the %s case", name)
		}
	}
}

func TestRunnerProcessesAndAcksAMessage(t *testing.T) {
	f := newFixture(t)
	var handled atomic.Int64
	registry := registryWith(t, Registration{
		Type: "rank.update_element", Timeout: 5 * time.Second, MaxAttempts: 3,
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			handled.Add(1)
			return nil
		}),
	})
	runner := f.runner(t, registry, 1)

	if err := f.publisher.Publish(context.Background(),
		queue.Message{Queue: "default", Type: "rank.update_element"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	runUntil(t, runner, func() bool { return handled.Load() >= 1 }, 5*time.Second)

	if handled.Load() != 1 {
		t.Fatalf("handled = %d, want 1", handled.Load())
	}
	processed, _, _, _, _ := runner.Stats()
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	ctx := context.Background()
	if got := f.client.LLen(ctx, f.transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0", got)
	}
	if got := f.client.LLen(ctx, f.transport.Key("default")).Val(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

// A message with no handler can never succeed, so it must be dead-lettered
// rather than retried or left in flight.
func TestRunnerDeadLettersUnknownType(t *testing.T) {
	f := newFixture(t)
	runner := f.runner(t, NewRegistry(), 1)

	if err := f.publisher.Publish(context.Background(),
		queue.Message{Queue: "default", Type: "nobody.handles.this"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	runUntil(t, runner, func() bool {
		_, _, _, dead, _ := runner.Stats()
		return dead >= 1
	}, 5*time.Second)

	length, err := f.transport.DeadLetterLength(context.Background(), "default")
	if err != nil {
		t.Fatalf("DeadLetterLength() error = %v", err)
	}
	if length != 1 {
		t.Fatalf("dead-letter length = %d, want 1", length)
	}
}

// A permanent failure must skip the retry budget entirely.
func TestRunnerDeadLettersPermanentFailureWithoutRetrying(t *testing.T) {
	f := newFixture(t)
	var attempts atomic.Int64
	registry := registryWith(t, Registration{
		Type: "bad.payload", Timeout: 5 * time.Second, MaxAttempts: 5,
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			attempts.Add(1)
			return Permanent(errors.New("element_id missing"))
		}),
	})
	runner := f.runner(t, registry, 1)

	if err := f.publisher.Publish(context.Background(),
		queue.Message{Queue: "default", Type: "bad.payload"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	runUntil(t, runner, func() bool {
		_, _, _, dead, _ := runner.Stats()
		return dead >= 1
	}, 5*time.Second)

	if attempts.Load() != 1 {
		t.Fatalf("handler ran %d times, want 1: a permanent failure must not retry", attempts.Load())
	}
	_, _, retried, dead, _ := runner.Stats()
	if retried != 0 {
		t.Fatalf("retried = %d, want 0", retried)
	}
	if dead != 1 {
		t.Fatalf("dead-lettered = %d, want 1", dead)
	}
}

// A retryable failure must consume attempts and end in the dead-letter queue,
// never loop forever. MaxAttempts is 2 with the base backoff, so this stays fast.
func TestRunnerRetriesThenDeadLettersAfterMaxAttempts(t *testing.T) {
	f := newFixture(t)
	var attempts atomic.Int64
	registry := registryWith(t, Registration{
		Type: "flaky", Timeout: 5 * time.Second, MaxAttempts: 2,
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			attempts.Add(1)
			return errors.New("deadlock found when trying to get lock")
		}),
	})
	runner := f.runner(t, registry, 1)

	if err := f.publisher.Publish(context.Background(),
		queue.Message{Queue: "default", Type: "flaky"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	runUntil(t, runner, func() bool {
		_, _, _, dead, _ := runner.Stats()
		return dead >= 1
	}, 4*queue.BaseRetryDelay+10*time.Second)

	if attempts.Load() != 2 {
		t.Fatalf("handler ran %d times, want 2 (MaxAttempts)", attempts.Load())
	}
	_, _, retried, dead, _ := runner.Stats()
	if retried != 1 {
		t.Fatalf("retried = %d, want 1", retried)
	}
	if dead != 1 {
		t.Fatalf("dead-lettered = %d, want 1", dead)
	}
	if got := f.client.LLen(context.Background(), f.transport.Key("default")).Val(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

// Two messages sharing a serialization key must not run concurrently. Without
// this, both deliveries would repeat the same aggregation over game_1v1_rounds.
func TestRunnerSerializesMessagesSharingAKey(t *testing.T) {
	f := newFixture(t)
	var (
		mu       sync.Mutex
		inFlight int
		maxSeen  int
		handled  atomic.Int64
	)
	registry := registryWith(t, Registration{
		Type: "rank.update_element", Timeout: 10 * time.Second, MaxAttempts: 5,
		SerialKey: func(queue.Message) (string, error) { return "post-1-element-2", nil },
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			mu.Lock()
			inFlight++
			if inFlight > maxSeen {
				maxSeen = inFlight
			}
			mu.Unlock()

			time.Sleep(300 * time.Millisecond)

			mu.Lock()
			inFlight--
			mu.Unlock()
			handled.Add(1)
			return nil
		}),
	})
	runner := f.runner(t, registry, 4)

	for index := 0; index < 3; index++ {
		if err := f.publisher.Publish(context.Background(),
			queue.Message{Queue: "default", Type: "rank.update_element"}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	runUntil(t, runner, func() bool { return handled.Load() >= 3 }, 20*time.Second)

	mu.Lock()
	peak := maxSeen
	mu.Unlock()
	if peak != 1 {
		t.Fatalf("peak concurrent handlers for one key = %d, want 1", peak)
	}
	if handled.Load() != 3 {
		t.Fatalf("handled = %d, want 3: a deferred message must still be processed", handled.Load())
	}
	// A serialization conflict is not a failure, so it must not consume attempts.
	_, _, _, dead, deferred := runner.Stats()
	if dead != 0 {
		t.Fatalf("dead-lettered = %d, want 0", dead)
	}
	if deferred == 0 {
		t.Fatal("expected at least one deferred delivery")
	}
}

// Distinct keys must still run in parallel, or serialization would silently
// become a global bottleneck.
func TestRunnerRunsDistinctKeysConcurrently(t *testing.T) {
	f := newFixture(t)
	var (
		mu      sync.Mutex
		peak    int
		running int
		handled atomic.Int64
	)
	registry := registryWith(t, Registration{
		Type: "rank.update_element", Timeout: 10 * time.Second, MaxAttempts: 5,
		SerialKey: func(message queue.Message) (string, error) {
			return message.IdempotencyKey, nil
		},
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			mu.Lock()
			running++
			if running > peak {
				peak = running
			}
			mu.Unlock()

			time.Sleep(300 * time.Millisecond)

			mu.Lock()
			running--
			mu.Unlock()
			handled.Add(1)
			return nil
		}),
	})
	runner := f.runner(t, registry, 4)

	for index := 0; index < 3; index++ {
		if err := f.publisher.Publish(context.Background(), queue.Message{
			Queue: "default", Type: "rank.update_element",
			IdempotencyKey: string(rune('a' + index)),
		}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	runUntil(t, runner, func() bool { return handled.Load() >= 3 }, 20*time.Second)

	mu.Lock()
	observed := peak
	mu.Unlock()
	if observed < 2 {
		t.Fatalf("peak concurrency across distinct keys = %d, want at least 2", observed)
	}
}

// A malformed payload that breaks key derivation cannot be retried into working.
func TestRunnerDeadLettersWhenSerialKeyFails(t *testing.T) {
	f := newFixture(t)
	registry := registryWith(t, Registration{
		Type: "rank.update_element", Timeout: 5 * time.Second, MaxAttempts: 5,
		SerialKey: func(queue.Message) (string, error) { return "", errors.New("no element_id in payload") },
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			t.Error("the handler must not run when the serial key cannot be derived")
			return nil
		}),
	})
	runner := f.runner(t, registry, 1)

	if err := f.publisher.Publish(context.Background(),
		queue.Message{Queue: "default", Type: "rank.update_element"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	runUntil(t, runner, func() bool {
		_, _, _, dead, _ := runner.Stats()
		return dead >= 1
	}, 5*time.Second)

	length, _ := f.transport.DeadLetterLength(context.Background(), "default")
	if length != 1 {
		t.Fatalf("dead-letter length = %d, want 1", length)
	}
	// Nothing may be stranded in flight.
	if got := f.client.LLen(context.Background(), f.transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0", got)
	}
}

// Startup recovery must return work stranded by a previously killed worker.
func TestRunRecoversStrandedMessagesOnStartup(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.publisher.Publish(ctx, queue.Message{Queue: "default", Type: "rank.update_element"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	// Reserve and abandon, as a killed worker would.
	if _, err := f.transport.Reserve(ctx, []string{"default"}, time.Second); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}

	var handled atomic.Int64
	registry := registryWith(t, Registration{
		Type: "rank.update_element", Timeout: 5 * time.Second, MaxAttempts: 3,
		Handler: HandlerFunc(func(context.Context, queue.Message) error {
			handled.Add(1)
			return nil
		}),
	})
	runner := f.runner(t, registry, 1)

	runUntil(t, runner, func() bool { return handled.Load() >= 1 }, 5*time.Second)

	if handled.Load() != 1 {
		t.Fatalf("handled = %d, want 1: the stranded message was not recovered", handled.Load())
	}
}
