package redislock

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// testRedis mirrors the queue package: the release image runs `go test ./...`
// during the build with no Redis, so these must skip rather than fail there.
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

func newTestLocker(t *testing.T) (*Locker, *redis.Client, string) {
	t.Helper()
	client := testRedis(t)
	prefix := "2pick:test:lock:"
	locker, err := New(client, prefix)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		keys, _ := client.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
		client.Close()
	})
	return locker, client, prefix
}

func TestNewRequiresClient(t *testing.T) {
	if _, err := New(nil, ""); err == nil {
		t.Fatal("New(nil) should fail")
	}
}

func TestNewRequiresKeyPrefix(t *testing.T) {
	// Each subsystem must namespace its own locks; an empty prefix would let the
	// scheduler and the worker collide, and could collide with Laravel's own
	// scheduler mutexes when they share a Redis.
	if _, err := New(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), ""); err == nil {
		t.Fatal("New() should reject an empty key prefix")
	}
}

func TestAcquireRejectsBadArguments(t *testing.T) {
	locker, _, _ := newTestLocker(t)
	ctx := context.Background()

	if _, _, err := locker.Acquire(ctx, "", time.Minute); err == nil {
		t.Fatal("Acquire() should reject an empty name")
	}
	if _, _, err := locker.Acquire(ctx, "entry", 0); err == nil {
		t.Fatal("Acquire() should reject a non-positive ttl")
	}
}

// The core withoutOverlapping behaviour: while one run holds the lock, the next
// tick must be told to skip rather than run concurrently.
func TestSecondAcquireIsRefusedWhileHeld(t *testing.T) {
	locker, _, _ := newTestLocker(t)
	ctx := context.Background()

	first, acquired, err := locker.Acquire(ctx, "post-trend-all", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("first Acquire() = (%v, %v)", acquired, err)
	}

	_, acquired, err = locker.Acquire(ctx, "post-trend-all", time.Minute)
	if err != nil {
		t.Fatalf("second Acquire() error = %v", err)
	}
	if acquired {
		t.Fatal("the second acquire must be refused while the first run holds the lock")
	}

	if err := first.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Once released, the next tick proceeds instead of waiting out the TTL.
	_, acquired, err = locker.Acquire(ctx, "post-trend-all", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("Acquire() after release = (%v, %v)", acquired, err)
	}
}

func TestDifferentEntriesDoNotBlockEachOther(t *testing.T) {
	locker, _, _ := newTestLocker(t)
	ctx := context.Background()

	if _, acquired, err := locker.Acquire(ctx, "post-trend-all", time.Minute); err != nil || !acquired {
		t.Fatalf("Acquire(all) = (%v, %v)", acquired, err)
	}
	// Regression guard for the Laravel defect where refresh:token twitch and
	// refresh:token imgur shared one mutex name and blocked each other.
	if _, acquired, err := locker.Acquire(ctx, "post-trend-week", time.Minute); err != nil || !acquired {
		t.Fatalf("Acquire(week) = (%v, %v)", acquired, err)
	}
}

// A lock must not outlive its TTL, otherwise a crashed run blocks the schedule
// permanently.
func TestLockExpiresAfterTTL(t *testing.T) {
	locker, _, _ := newTestLocker(t)
	ctx := context.Background()

	if _, acquired, err := locker.Acquire(ctx, "short-lived", 150*time.Millisecond); err != nil || !acquired {
		t.Fatalf("Acquire() = (%v, %v)", acquired, err)
	}
	time.Sleep(300 * time.Millisecond)

	_, acquired, err := locker.Acquire(ctx, "short-lived", time.Minute)
	if err != nil {
		t.Fatalf("Acquire() after expiry error = %v", err)
	}
	if !acquired {
		t.Fatal("the lock should have expired and become available")
	}
}

// The dangerous case: a run overruns its TTL, another replica takes the lock,
// and then the slow run finishes. Its release must not delete the new owner's
// lock, or a third run could start alongside the second.
func TestReleaseAfterExpiryDoesNotDeleteAnotherOwnersLock(t *testing.T) {
	locker, client, prefix := newTestLocker(t)
	ctx := context.Background()

	slowRun, acquired, err := locker.Acquire(ctx, "overrun", 150*time.Millisecond)
	if err != nil || !acquired {
		t.Fatalf("Acquire() = (%v, %v)", acquired, err)
	}
	time.Sleep(300 * time.Millisecond)

	newOwner, acquired, err := locker.Acquire(ctx, "overrun", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("second owner Acquire() = (%v, %v)", acquired, err)
	}

	if err := slowRun.Release(ctx); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("Release() error = %v, want %v", err, ErrNotHeld)
	}

	// The new owner's lock must still be in place.
	if exists := client.Exists(ctx, prefix+"overrun").Val(); exists != 1 {
		t.Fatal("the new owner's lock was deleted by the expired run's release")
	}
	if _, acquired, _ := locker.Acquire(ctx, "overrun", time.Minute); acquired {
		t.Fatal("a third run acquired the lock while the second owner still holds it")
	}
	if err := newOwner.Release(ctx); err != nil {
		t.Fatalf("new owner Release() error = %v", err)
	}
}

func TestReleaseIsNotRepeatable(t *testing.T) {
	locker, _, _ := newTestLocker(t)
	ctx := context.Background()

	lock, acquired, err := locker.Acquire(ctx, "single-release", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("Acquire() = (%v, %v)", acquired, err)
	}
	if err := lock.Release(ctx); err != nil {
		t.Fatalf("first Release() error = %v", err)
	}
	if err := lock.Release(ctx); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("second Release() error = %v, want %v", err, ErrNotHeld)
	}
}
