package ranking

import (
	"context"
	"testing"
	"time"
)

func newTestFreshness(t *testing.T) (*RedisFreshness, string) {
	t.Helper()
	client := testRedis(t)
	prefix := "2pick:test:freshness:"
	store, err := NewRedisFreshness(client, prefix)
	if err != nil {
		t.Fatalf("NewRedisFreshness() error = %v", err)
	}
	cleanup := func() {
		keys, _ := client.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		client.Close()
	})
	return store, prefix
}

func TestNewRedisFreshnessRequiresClient(t *testing.T) {
	if _, err := NewRedisFreshness(nil, ""); err == nil {
		t.Fatal("NewRedisFreshness(nil) should fail")
	}
}

// The key must match what Laravel writes, minus the prefix, because both sides now
// set this flag and both read it.
func TestFreshnessKeyMatchesLaravel(t *testing.T) {
	if got := FreshnessKey(123); got != "need_fresh_post_rank_123" {
		t.Fatalf("FreshnessKey(123) = %q", got)
	}
}

func TestFreshnessReadsAndClears(t *testing.T) {
	store, prefix := newTestFreshness(t)
	client := testRedis(t)
	defer client.Close()
	ctx := context.Background()

	flagged, err := store.NeedsRebuild(ctx, 42)
	if err != nil {
		t.Fatalf("NeedsRebuild() error = %v", err)
	}
	if flagged {
		t.Fatal("an unflagged post must not report as needing a rebuild")
	}

	// Laravel serialises the value; only presence is read, so any value works here.
	client.Set(ctx, prefix+FreshnessKey(42), "b:1;", 0)

	flagged, err = store.NeedsRebuild(ctx, 42)
	if err != nil {
		t.Fatalf("NeedsRebuild() error = %v", err)
	}
	if !flagged {
		t.Fatal("a flagged post must report as needing a rebuild")
	}

	if err := store.Clear(ctx, 42); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	flagged, _ = store.NeedsRebuild(ctx, 42)
	if flagged {
		t.Fatal("Clear() must remove the flag")
	}
}

// Clearing something that is not set is not an error: the sweep clears after
// dispatching, and a concurrent Laravel run may have cleared it first.
func TestFreshnessClearIsIdempotent(t *testing.T) {
	store, _ := newTestFreshness(t)

	if err := store.Clear(context.Background(), 999); err != nil {
		t.Fatalf("Clear() of an unset flag error = %v", err)
	}
}

// Go writing the flag is what removes a real dependency on Laravel: before this,
// App\Listeners\UpdatePostRank was the only thing that set it, so with Laravel off
// the daily sweep would have found nothing, every day, without any error.
func TestFreshnessSetWritesWhatLaravelCanRead(t *testing.T) {
	store, prefix := newTestFreshness(t)
	client := testRedis(t)
	defer client.Close()
	ctx := context.Background()
	const postID = int64(987654321)

	if err := store.Set(ctx, postID); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	key := prefix + FreshnessKey(postID)
	value, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("the key was not written at %q: %v", key, err)
	}
	// Laravel's Cache::get runs unserialize() over whatever is stored. Presence alone
	// would satisfy the Go reader, but bytes PHP cannot unserialize make every PHP
	// read emit a warning, so the value has to be serialize(true).
	if value != "b:1;" {
		t.Errorf("value = %q, want PHP serialize(true) %q", value, "b:1;")
	}

	if flagged, err := store.NeedsRebuild(ctx, postID); err != nil || !flagged {
		t.Fatalf("NeedsRebuild after Set = %v, %v; want true", flagged, err)
	}
}

// Three days, matching CacheService::setNeedFreshPostRank. The window has to outlast
// a sweep that fails or is skipped.
func TestFreshnessSetUsesTheLaravelWindow(t *testing.T) {
	store, prefix := newTestFreshness(t)
	client := testRedis(t)
	defer client.Close()
	ctx := context.Background()
	const postID = int64(4242)

	if FreshnessTTL != 72*time.Hour {
		t.Fatalf("FreshnessTTL = %v, want 72h to match the PHP", FreshnessTTL)
	}
	if err := store.Set(ctx, postID); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	ttl, err := client.TTL(ctx, prefix+FreshnessKey(postID)).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > FreshnessTTL {
		t.Fatalf("ttl = %v, want (0, %v]", ttl, FreshnessTTL)
	}
}

// A post played twice keeps the full window from the second game. SET NX would leave
// it expiring on the first game's clock, so the flag could vanish before any sweep
// ran.
func TestFreshnessSetRefreshesTheWindow(t *testing.T) {
	store, prefix := newTestFreshness(t)
	client := testRedis(t)
	defer client.Close()
	ctx := context.Background()
	const postID = int64(4343)
	key := prefix + FreshnessKey(postID)

	if err := store.Set(ctx, postID); err != nil {
		t.Fatalf("first Set() error = %v", err)
	}
	if err := client.Expire(ctx, key, 30*time.Second).Err(); err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if err := store.Set(ctx, postID); err != nil {
		t.Fatalf("second Set() error = %v", err)
	}

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= time.Hour {
		t.Fatalf("ttl = %v after the second Set; the window was not refreshed", ttl)
	}
}

func TestFreshnessSetRejectsABadPostID(t *testing.T) {
	store, _ := newTestFreshness(t)
	for _, postID := range []int64{0, -1} {
		if err := store.Set(context.Background(), postID); err == nil {
			t.Errorf("Set(%d) should be rejected", postID)
		}
	}
}
