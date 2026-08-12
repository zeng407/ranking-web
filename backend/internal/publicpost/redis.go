package publicpost

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// serializedTrue is PHP's serialize(true).
//
// The flag is shared with Laravel, which stores cache values as serialize() output and
// reads them back through unserialize(). Cache::has() only checks for a non-null
// result, so any bytes would keep the flag working — but a value PHP cannot
// unserialize makes it emit a warning on every read. These four bytes are the whole of
// what this package needs from PHP's serialisation format; anything structured is
// deleted rather than written. See gameroom.LegacyCache for the same choice.
const serializedTrue = "b:1;"

// RedisFreshness is the shared debounce flag.
type RedisFreshness struct {
	client redis.Cmdable
	// prefix is Laravel's full cache key prefix, which differs per environment and so
	// must be supplied rather than derived.
	prefix string
	ttl    time.Duration
}

func NewRedisFreshness(client redis.Cmdable, laravelCachePrefix string) (*RedisFreshness, error) {
	if client == nil {
		return nil, errors.New("publicpost: redis client is required")
	}
	return &RedisFreshness{client: client, prefix: laravelCachePrefix, ttl: FreshnessTTL}, nil
}

func (store *RedisFreshness) key() string {
	return store.prefix + FreshnessKey
}

func (store *RedisFreshness) IsFresh(ctx context.Context) (bool, error) {
	count, err := store.client.Exists(ctx, store.key()).Result()
	if err != nil {
		return false, fmt.Errorf("publicpost: read the freshness flag: %w", err)
	}
	return count > 0, nil
}

func (store *RedisFreshness) MarkFresh(ctx context.Context) error {
	if err := store.client.Set(ctx, store.key(), serializedTrue, store.ttl).Err(); err != nil {
		return fmt.Errorf("publicpost: set the freshness flag: %w", err)
	}
	return nil
}

// AlwaysStale disables the debounce, so a refresh starts again as soon as the previous
// one finishes and the listing is never more than one pass out of date.
//
// This is what "run continuously" means in practice, and it is a real choice rather
// than a debug switch. The shared flag makes the rebuild happen at most every
// FreshnessTTL unless a post changes, and posts change about ten times a day in the
// production data — so with the flag the listing is rebuilt roughly every ten minutes,
// not continuously. Without it the only thing pacing the work is the overlap lock.
//
// Selected by PUBLIC_POST_CONTINUOUS. It costs a full pass per tick, which is why it
// is not the default.
type AlwaysStale struct{}

func (AlwaysStale) IsFresh(context.Context) (bool, error) { return false, nil }
func (AlwaysStale) MarkFresh(context.Context) error       { return nil }

// RedisResourceCache deletes Laravel's per-post resource entries.
type RedisResourceCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisResourceCache(client redis.Cmdable, laravelCachePrefix string) (*RedisResourceCache, error) {
	if client == nil {
		return nil, errors.New("publicpost: redis client is required")
	}
	return &RedisResourceCache{client: client, prefix: laravelCachePrefix}, nil
}

// ClearBatchSize bounds one DEL. Deleting a few thousand keys in one command would
// block Redis for the whole operation, and Redis is on the request path for the API.
const ClearBatchSize = 200

func (cache *RedisResourceCache) Clear(ctx context.Context, postIDs []int64) error {
	if len(postIDs) == 0 {
		return nil
	}
	for start := 0; start < len(postIDs); start += ClearBatchSize {
		end := start + ClearBatchSize
		if end > len(postIDs) {
			end = len(postIDs)
		}

		keys := make([]string, 0, end-start)
		for _, postID := range postIDs[start:end] {
			keys = append(keys, cache.prefix+PostResourceKeyPrefix+strconv.FormatInt(postID, 10))
		}
		if err := cache.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("publicpost: clear %d resource cache keys: %w", len(keys), err)
		}
	}
	return nil
}
