package ranking

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

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

func newTestStats(t *testing.T) (*RedisStats, *redis.Client) {
	t.Helper()
	client := testRedis(t)
	store, err := NewRedisStats(client, "2pick:test:rank:")
	if err != nil {
		t.Fatalf("NewRedisStats() error = %v", err)
	}
	cleanup := func() {
		keys, _ := client.Keys(context.Background(), "2pick:test:rank:*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		client.Close()
	})
	return store, client
}

func TestNewRedisStatsRequiresClient(t *testing.T) {
	if _, err := NewRedisStats(nil, ""); err == nil {
		t.Fatal("NewRedisStats(nil) should fail")
	}
}

// A cold memo must read as the zero value, not an error: that is the signal to
// recount from the beginning.
func TestGetReturnsZeroStatsWhenAbsent(t *testing.T) {
	store, _ := newTestStats(t)

	stats, err := store.Get(context.Background(), 46, 2759)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stats != (Stats{}) {
		t.Fatalf("Get() = %#v, want the zero value", stats)
	}
}

func TestPutThenGetRoundTrips(t *testing.T) {
	store, _ := newTestStats(t)
	ctx := context.Background()
	want := Stats{
		ChampionMaxWinID: 110, ChampionMaxLoseID: 108,
		ChampionRoundWins: 10, ChampionRoundLoses: 5, ChampionGameWins: 2,
		PKMaxWinID: 200, PKMaxLoseID: 205, PKWinCount: 20, PKLoseCount: 30,
	}

	if err := store.Put(ctx, 46, 2759, want); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := store.Get(ctx, 46, 2759)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != want {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}
}

// The JSON field names must match CacheService's payload so the Laravel and Go
// implementations can read each other's entries during the cutover.
func TestStoredPayloadMatchesTheLaravelFieldNames(t *testing.T) {
	store, client := newTestStats(t)
	ctx := context.Background()

	if err := store.Put(ctx, 46, 2759, Stats{ChampionMaxWinID: 110, PKWinCount: 20}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	body, err := client.Get(ctx, "2pick:test:rank:element_rank_stats:46:2759").Bytes()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, field := range []string{
		"champion_max_win_id", "champion_max_lose_id", "champion_round_wins",
		"champion_round_loses", "champion_game_wins",
		"pk_max_win_id", "pk_max_lose_id", "pk_win_count", "pk_lose_count",
	} {
		if _, ok := raw[field]; !ok {
			t.Errorf("stored payload is missing %q", field)
		}
	}
}

func TestPutSetsTheSevenDayTTL(t *testing.T) {
	store, client := newTestStats(t)
	ctx := context.Background()

	if err := store.Put(ctx, 46, 2759, Stats{PKWinCount: 1}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	ttl, err := client.TTL(ctx, "2pick:test:rank:element_rank_stats:46:2759").Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	// Matches CacheService's 7 day cache; a missing TTL would let a stale memo
	// live forever.
	if ttl <= 6*24*time.Hour || ttl > StatsTTL {
		t.Fatalf("TTL = %s, want just under %s", ttl, StatsTTL)
	}
}

// A corrupt entry must cost one recount, not stall the element permanently.
func TestGetTreatsCorruptEntryAsCold(t *testing.T) {
	store, client := newTestStats(t)
	ctx := context.Background()
	client.Set(ctx, "2pick:test:rank:element_rank_stats:46:2759", "not json", time.Minute)

	stats, err := store.Get(ctx, 46, 2759)
	if err != nil {
		t.Fatalf("Get() error = %v, want a cold read", err)
	}
	if stats != (Stats{}) {
		t.Fatalf("Get() = %#v, want the zero value", stats)
	}
}

func TestKeysAreScopedPerElement(t *testing.T) {
	store, _ := newTestStats(t)
	ctx := context.Background()

	if err := store.Put(ctx, 46, 2759, Stats{PKWinCount: 20}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := store.Put(ctx, 46, 9999, Stats{PKWinCount: 7}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	first, _ := store.Get(ctx, 46, 2759)
	second, _ := store.Get(ctx, 46, 9999)
	if first.PKWinCount != 20 || second.PKWinCount != 7 {
		t.Fatalf("entries leaked across elements: %#v / %#v", first, second)
	}
}
