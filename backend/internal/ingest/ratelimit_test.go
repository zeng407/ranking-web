package ingest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// The rate limiter against a real Redis, because what it is actually asserting is that
// EXPIRE NX leaves an existing deadline alone — which is the whole difference between the
// fixed minute the setting describes and the window the original never let close.

func testRedis(t *testing.T) redis.Cmdable {
	t.Helper()
	address := os.Getenv("REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("REDIS_TEST_ADDR is not set; skipping Redis integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis unreachable: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func newLimiter(t *testing.T) (*RedisRateLimiter, int64) {
	t.Helper()
	client := testRedis(t)
	prefix := fmt.Sprintf("2pick:go:test:upload:%d:", time.Now().UnixNano())
	limiter, err := NewRedisRateLimiter(client, prefix)
	if err != nil {
		t.Fatalf("NewRedisRateLimiter() error = %v", err)
	}
	const userID = 4242
	t.Cleanup(func() {
		client.Del(context.Background(),
			fmt.Sprintf("%sbytes:%d", prefix, userID), fmt.Sprintf("%sfiles:%d", prefix, userID))
	})
	return limiter, userID
}

func TestTheBudgetAllowsUpToTheLimitAndThenRefuses(t *testing.T) {
	limiter, userID := newLimiter(t)
	ctx := context.Background()

	// Three quarters of the byte budget, in two goes.
	for attempt := 0; attempt < 2; attempt++ {
		allowed, err := limiter.Allow(ctx, userID, RateLimitBytes/4)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !allowed {
			t.Fatalf("attempt %d was refused inside the budget", attempt)
		}
	}

	// The one that would cross it.
	allowed, err := limiter.Allow(ctx, userID, RateLimitBytes)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Error("an upload past the byte budget was allowed")
	}

	// AND THE REFUSAL DID NOT SPEND ANYTHING. The original added first and compared the
	// previous total, so the request that broke the limit was let through and the next
	// one paid for it. Here the refused request rolls its own increment back, which is
	// what lets this one still fit.
	allowed, err = limiter.Allow(ctx, userID, RateLimitBytes/4)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Error("a refused upload consumed part of the budget")
	}
}

func TestTheFileCountIsBudgetedSeparately(t *testing.T) {
	limiter, userID := newLimiter(t)
	ctx := context.Background()

	// Fifty one-byte files: nowhere near the byte budget, exactly at the file one.
	for attempt := 0; attempt < RateLimitFiles; attempt++ {
		allowed, err := limiter.Allow(ctx, userID, 1)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !allowed {
			t.Fatalf("file %d was refused inside the budget", attempt+1)
		}
	}

	allowed, err := limiter.Allow(ctx, userID, 1)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Errorf("a %dst file was allowed", RateLimitFiles+1)
	}
}

/*
THE FIXED WINDOW, WHICH IS THE DEFECT THIS REPLACES.

The original re-put its counter with a fresh one-minute expiry on every upload, so the
minute never ended for anyone still uploading: a steady stream kept extending the same
counter until it tripped. EXPIRE NX is what makes the deadline belong to the first write
of the window rather than the latest one.
*/
func TestTheWindowDoesNotSlideForwardOnEveryUpload(t *testing.T) {
	limiter, userID := newLimiter(t)
	ctx := context.Background()
	client := testRedis(t)

	if _, err := limiter.Allow(ctx, userID, 1); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	key := fmt.Sprintf("%sbytes:%d", limiter.prefix, userID)
	first, err := client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read the first deadline: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if _, err := limiter.Allow(ctx, userID, 1); err != nil {
		t.Fatalf("second Allow() error = %v", err)
	}
	second, err := client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read the second deadline: %v", err)
	}

	if second >= first {
		t.Errorf("the deadline moved from %v to %v; a later upload must not extend the window",
			first, second)
	}
}

func TestTheBudgetIsPerAccount(t *testing.T) {
	limiter, userID := newLimiter(t)
	ctx := context.Background()
	const other = 9999
	t.Cleanup(func() {
		testRedis(t).Del(ctx,
			fmt.Sprintf("%sbytes:%d", limiter.prefix, other),
			fmt.Sprintf("%sfiles:%d", limiter.prefix, other))
	})

	for attempt := 0; attempt <= RateLimitFiles; attempt++ {
		if _, err := limiter.Allow(ctx, userID, 1); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}

	allowed, err := limiter.Allow(ctx, other, 1)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Error("one account's uploads exhausted another's budget")
	}
}
