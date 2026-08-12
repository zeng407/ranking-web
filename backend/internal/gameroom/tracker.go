package gameroom

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// TrackerKeyPrefix namespaces the refresh state. Owned by Go, so it does not need
// Laravel's cache prefix.
const TrackerKeyPrefix = "gameroom:rank:"

// TrackerTTL bounds how long a room's refresh state lives.
//
// One key holds both counters as hash fields so they expire together. If they
// expired independently, version could vanish while applied survived, leaving
// version < applied — read as "nothing to do" — and the room would stop updating
// for the rest of the game. Every write refreshes the TTL, so the state lives as
// long as the room is active and is reclaimed a day after it goes quiet.
const TrackerTTL = 24 * time.Hour

const (
	versionField = "version"
	appliedField = "applied"
)

// markAppliedScript advances applied without ever moving it backwards.
//
// Two refreshes for the same room can overlap: the per-room serialization lock has
// a TTL, so a refresh that outruns its lease can still be running when the next one
// starts. If the slower one finished last with an older version, a plain SET would
// lower applied and make work that was already done look outstanding again — the
// room would then recompute forever, since every refresh would find itself behind.
var markAppliedScript = redis.NewScript(`
local applied = tonumber(redis.call('HGET', KEYS[1], ARGV[1]) or '0')
local target = tonumber(ARGV[2])
if target > applied then
	redis.call('HSET', KEYS[1], ARGV[1], target)
	applied = target
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
return applied
`)

// RedisTracker stores the refresh state in Redis.
type RedisTracker struct {
	// Cmdable rather than Scripter: its method set covers Scripter, so the same
	// field serves both the plain commands and markAppliedScript.Run.
	client redis.Cmdable
	prefix string
	ttl    time.Duration
}

func NewRedisTracker(client redis.Cmdable) (*RedisTracker, error) {
	if client == nil {
		return nil, errors.New("gameroom: redis client is required")
	}
	return &RedisTracker{client: client, prefix: TrackerKeyPrefix, ttl: TrackerTTL}, nil
}

func (tracker *RedisTracker) key(roomSerial string) string {
	return tracker.prefix + roomSerial
}

func (tracker *RedisTracker) MarkChanged(ctx context.Context, roomSerial string) (int64, error) {
	if roomSerial == "" {
		return 0, errors.New("gameroom: room serial is required")
	}

	key := tracker.key(roomSerial)
	pipeline := tracker.client.Pipeline()
	incremented := pipeline.HIncrBy(ctx, key, versionField, 1)
	pipeline.Expire(ctx, key, tracker.ttl)
	if _, err := pipeline.Exec(ctx); err != nil {
		return 0, fmt.Errorf("gameroom: mark room %q changed: %w", roomSerial, err)
	}
	return incremented.Val(), nil
}

func (tracker *RedisTracker) Outstanding(ctx context.Context, roomSerial string) (Outstanding, error) {
	if roomSerial == "" {
		return Outstanding{}, errors.New("gameroom: room serial is required")
	}

	values, err := tracker.client.HMGet(ctx, tracker.key(roomSerial), versionField, appliedField).Result()
	if err != nil {
		return Outstanding{}, fmt.Errorf("gameroom: read refresh state for %q: %w", roomSerial, err)
	}
	if len(values) != 2 {
		return Outstanding{}, fmt.Errorf("gameroom: refresh state for %q returned %d fields", roomSerial, len(values))
	}

	version, err := parseCounter(values[0])
	if err != nil {
		return Outstanding{}, fmt.Errorf("gameroom: refresh version for %q: %w", roomSerial, err)
	}
	applied, err := parseCounter(values[1])
	if err != nil {
		return Outstanding{}, fmt.Errorf("gameroom: applied version for %q: %w", roomSerial, err)
	}
	return Outstanding{Version: version, Applied: applied}, nil
}

func (tracker *RedisTracker) MarkApplied(ctx context.Context, roomSerial string, version int64) error {
	if roomSerial == "" {
		return errors.New("gameroom: room serial is required")
	}
	if version <= 0 {
		// Nothing was tallied, so there is nothing to record. Writing 0 would be
		// harmless but hides a caller bug.
		return fmt.Errorf("gameroom: applied version must be positive, got %d", version)
	}

	seconds := strconv.FormatInt(int64(tracker.ttl.Seconds()), 10)
	err := markAppliedScript.Run(ctx, tracker.client,
		[]string{tracker.key(roomSerial)}, appliedField, version, seconds).Err()
	if err != nil {
		return fmt.Errorf("gameroom: mark room %q applied at %d: %w", roomSerial, version, err)
	}
	return nil
}

// parseCounter reads a hash field that may be absent. A missing key means the room
// has never been touched, which is version 0 rather than an error.
func parseCounter(value any) (int64, error) {
	switch typed := value.(type) {
	case nil:
		return 0, nil
	case int64:
		return typed, nil
	case string:
		if typed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("not an integer: %q", typed)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}

// Laravel cache keys, from App\Helper\CacheService. Note there is no separator
// between the key and the serial: the PHP concatenates them directly.
const (
	legacyLeaderboardKey  = "game_bet_rank"
	legacyUpdatingFlagKey = "processing_job:update_game_room_rank"
)

// RedisLegacyCache deletes the Laravel cache entries the PHP API still reads.
type RedisLegacyCache struct {
	client redis.Cmdable
	// prefix is Laravel's full cache key prefix, which differs per environment and
	// so must be supplied rather than derived. Same value as the ranking package's
	// LARAVEL_CACHE_PREFIX.
	prefix string
}

func NewRedisLegacyCache(client redis.Cmdable, laravelCachePrefix string) (*RedisLegacyCache, error) {
	if client == nil {
		return nil, errors.New("gameroom: redis client is required")
	}
	return &RedisLegacyCache{client: client, prefix: laravelCachePrefix}, nil
}

func (cache *RedisLegacyCache) InvalidateLeaderboard(ctx context.Context, roomSerial string) error {
	key := cache.prefix + legacyLeaderboardKey + roomSerial
	if err := cache.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("gameroom: invalidate legacy leaderboard for %q: %w", roomSerial, err)
	}
	return nil
}

func (cache *RedisLegacyCache) ClearUpdatingFlag(ctx context.Context, roomSerial string) error {
	key := cache.prefix + legacyUpdatingFlagKey + roomSerial
	if err := cache.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("gameroom: clear legacy updating flag for %q: %w", roomSerial, err)
	}
	return nil
}

// NoLegacyCache is used once GameController no longer serves the room endpoints.
type NoLegacyCache struct{}

func (NoLegacyCache) InvalidateLeaderboard(context.Context, string) error { return nil }
func (NoLegacyCache) ClearUpdatingFlag(context.Context, string) error     { return nil }
