package gameroom

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

// testRedis connects only when REDIS_TEST_ADDR is set, matching the other Redis
// integration tests in this repository.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set; skipping Redis integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis unreachable at %s: %v", addr, err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// uniqueSerial keeps parallel runs and repeat runs from sharing room state.
func uniqueSerial(t *testing.T) string {
	t.Helper()
	serial := fmt.Sprintf("test-%s-%d", t.Name(), os.Getpid())
	t.Cleanup(func() {
		client := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_TEST_ADDR")})
		defer client.Close()
		client.Del(context.Background(), TrackerKeyPrefix+serial)
	})
	return serial
}

func TestRedisTrackerReportsAnUntouchedRoomAsUpToDate(t *testing.T) {
	tracker, err := NewRedisTracker(testRedis(t))
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	serial := uniqueSerial(t)

	outstanding, err := tracker.Outstanding(context.Background(), serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Pending() {
		t.Fatalf("a room with no votes reports pending work: %+v", outstanding)
	}
}

func TestRedisTrackerTracksVotesAndProgress(t *testing.T) {
	tracker, err := NewRedisTracker(testRedis(t))
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()
	serial := uniqueSerial(t)

	for want := int64(1); want <= 3; want++ {
		version, err := tracker.MarkChanged(ctx, serial)
		if err != nil {
			t.Fatalf("MarkChanged() error = %v", err)
		}
		if version != want {
			t.Fatalf("MarkChanged() = %d, want %d", version, want)
		}
	}

	outstanding, err := tracker.Outstanding(ctx, serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Version != 3 || outstanding.Applied != 0 || !outstanding.Pending() {
		t.Fatalf("outstanding = %+v, want version 3 applied 0", outstanding)
	}

	if err := tracker.MarkApplied(ctx, serial, 3); err != nil {
		t.Fatalf("MarkApplied() error = %v", err)
	}
	outstanding, err = tracker.Outstanding(ctx, serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Pending() {
		t.Fatalf("still pending after applying every vote: %+v", outstanding)
	}
}

// The monotonic guarantee. Two refreshes for the same room can overlap once one
// outruns its lock lease; if the slower one finished last and lowered applied, every
// later refresh would find itself behind and the room would recompute forever.
func TestRedisTrackerNeverLowersTheAppliedVersion(t *testing.T) {
	tracker, err := NewRedisTracker(testRedis(t))
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()
	serial := uniqueSerial(t)

	for index := 0; index < 5; index++ {
		if _, err := tracker.MarkChanged(ctx, serial); err != nil {
			t.Fatalf("MarkChanged() error = %v", err)
		}
	}
	if err := tracker.MarkApplied(ctx, serial, 5); err != nil {
		t.Fatalf("MarkApplied(5) error = %v", err)
	}
	// The straggler reports an older version.
	if err := tracker.MarkApplied(ctx, serial, 2); err != nil {
		t.Fatalf("MarkApplied(2) error = %v", err)
	}

	outstanding, err := tracker.Outstanding(ctx, serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Applied != 5 {
		t.Fatalf("applied = %d after a late report of 2, want 5", outstanding.Applied)
	}
	if outstanding.Pending() {
		t.Fatal("the late report resurrected work that was already done")
	}
}

// Concurrent votes must all be counted: the version is what guarantees no vote is
// coalesced away, so a lost increment is a lost score.
func TestRedisTrackerCountsConcurrentVotes(t *testing.T) {
	tracker, err := NewRedisTracker(testRedis(t))
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()
	serial := uniqueSerial(t)

	const votes = 50
	var group sync.WaitGroup
	errs := make(chan error, votes)
	for index := 0; index < votes; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := tracker.MarkChanged(ctx, serial); err != nil {
				errs <- err
			}
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("MarkChanged() error = %v", err)
	}

	outstanding, err := tracker.Outstanding(ctx, serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Version != votes {
		t.Fatalf("version = %d after %d concurrent votes", outstanding.Version, votes)
	}
}

func TestRedisTrackerRefreshesTheTTLOnEveryWrite(t *testing.T) {
	client := testRedis(t)
	tracker, err := NewRedisTracker(client)
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()
	serial := uniqueSerial(t)

	if _, err := tracker.MarkChanged(ctx, serial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	afterChange, err := client.TTL(ctx, TrackerKeyPrefix+serial).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if afterChange <= 0 {
		t.Fatalf("TTL after MarkChanged = %v; the state would never be reclaimed", afterChange)
	}

	if err := tracker.MarkApplied(ctx, serial, 1); err != nil {
		t.Fatalf("MarkApplied() error = %v", err)
	}
	afterApply, err := client.TTL(ctx, TrackerKeyPrefix+serial).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if afterApply <= 0 {
		t.Fatalf("TTL after MarkApplied = %v", afterApply)
	}
}

// Both counters live in one key so they expire together. Independent expiry could
// leave version below applied, which reads as "nothing to do" and would stop the
// room updating for the rest of the game.
func TestRedisTrackerKeepsBothCountersInOneKey(t *testing.T) {
	client := testRedis(t)
	tracker, err := NewRedisTracker(client)
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()
	serial := uniqueSerial(t)

	if _, err := tracker.MarkChanged(ctx, serial); err != nil {
		t.Fatalf("MarkChanged() error = %v", err)
	}
	if err := tracker.MarkApplied(ctx, serial, 1); err != nil {
		t.Fatalf("MarkApplied() error = %v", err)
	}

	keys, err := client.Keys(ctx, TrackerKeyPrefix+serial+"*").Result()
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("the refresh state occupies %d keys (%v), want 1", len(keys), keys)
	}
	fields, err := client.HGetAll(ctx, keys[0]).Result()
	if err != nil {
		t.Fatalf("HGetAll() error = %v", err)
	}
	if fields[versionField] != "1" || fields[appliedField] != "1" {
		t.Fatalf("fields = %v, want version 1 and applied 1", fields)
	}
}

func TestRedisTrackerRejectsBadArguments(t *testing.T) {
	tracker, err := NewRedisTracker(testRedis(t))
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	ctx := context.Background()

	if _, err := tracker.MarkChanged(ctx, ""); err == nil {
		t.Error("MarkChanged() accepted an empty serial")
	}
	if _, err := tracker.Outstanding(ctx, ""); err == nil {
		t.Error("Outstanding() accepted an empty serial")
	}
	if err := tracker.MarkApplied(ctx, "room", 0); err == nil {
		t.Error("MarkApplied() accepted version 0")
	}
}

func TestNewRedisTrackerRejectsANilClient(t *testing.T) {
	if _, err := NewRedisTracker(nil); err == nil {
		t.Error("NewRedisTracker(nil) should fail")
	}
}

// The legacy cache keys must match CacheService exactly, including the missing
// separator before the serial, or the deletes hit nothing and the PHP endpoint keeps
// serving stale data.
func TestRedisLegacyCacheDeletesTheKeysLaravelWrites(t *testing.T) {
	client := testRedis(t)
	const prefix = "2pick_test_database_2pick_test_cache:"
	cache, err := NewRedisLegacyCache(client, prefix)
	if err != nil {
		t.Fatalf("NewRedisLegacyCache() error = %v", err)
	}
	ctx := context.Background()
	serial := "legacy-test-room"

	leaderboardKey := prefix + "game_bet_rank" + serial
	flagKey := prefix + "processing_job:update_game_room_rank" + serial
	t.Cleanup(func() { client.Del(ctx, leaderboardKey, flagKey) })

	if err := client.Set(ctx, leaderboardKey, "b:1;", 0).Err(); err != nil {
		t.Fatalf("seed leaderboard key: %v", err)
	}
	if err := client.Set(ctx, flagKey, "b:1;", 0).Err(); err != nil {
		t.Fatalf("seed flag key: %v", err)
	}

	if err := cache.InvalidateLeaderboard(ctx, serial); err != nil {
		t.Fatalf("InvalidateLeaderboard() error = %v", err)
	}
	if err := cache.ClearUpdatingFlag(ctx, serial); err != nil {
		t.Fatalf("ClearUpdatingFlag() error = %v", err)
	}

	for _, key := range []string{leaderboardKey, flagKey} {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil {
			t.Fatalf("Exists(%q) error = %v", key, err)
		}
		if exists != 0 {
			t.Errorf("%q still exists", key)
		}
	}
}

// Deleting a key that is not there is the normal case — Laravel only sets the flag
// on rooms that are being voted in — so it must not be an error.
func TestRedisLegacyCacheToleratesMissingKeys(t *testing.T) {
	cache, err := NewRedisLegacyCache(testRedis(t), "unused-prefix:")
	if err != nil {
		t.Fatalf("NewRedisLegacyCache() error = %v", err)
	}
	ctx := context.Background()
	if err := cache.InvalidateLeaderboard(ctx, "never-existed"); err != nil {
		t.Errorf("InvalidateLeaderboard() error = %v", err)
	}
	if err := cache.ClearUpdatingFlag(ctx, "never-existed"); err != nil {
		t.Errorf("ClearUpdatingFlag() error = %v", err)
	}
}
