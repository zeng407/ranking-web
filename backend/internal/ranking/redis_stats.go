package ranking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStats memoises the aggregation watermarks in Redis.
//
// It reads and writes the same key and JSON shape as
// CacheService::getElementRankStats / putElementRankStats so the Go and Laravel
// implementations can share entries during the cutover. That only holds when both
// point at the same Redis instance; if they do not, the Go side simply starts
// from a cold memo, which is a full recount and therefore correct.
type RedisStats struct {
	client redis.Cmdable
	// keyPrefix is Laravel's cache prefix, needed because Laravel's Cache facade
	// prefixes every key. Leave it empty when the Go worker owns its own Redis.
	keyPrefix string
}

func NewRedisStats(client redis.Cmdable, keyPrefix string) (*RedisStats, error) {
	if client == nil {
		return nil, errors.New("ranking: redis client is required")
	}
	return &RedisStats{client: client, keyPrefix: keyPrefix}, nil
}

func (store *RedisStats) key(postID, elementID int64) string {
	return store.keyPrefix + StatsKey(postID, elementID)
}

// Get returns the memoised stats. A missing key yields the zero value with no
// error: that means "recount from the beginning", which produces the same
// absolute totals.
func (store *RedisStats) Get(ctx context.Context, postID, elementID int64) (Stats, error) {
	body, err := store.client.Get(ctx, store.key(postID, elementID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Stats{}, nil
	}
	if err != nil {
		return Stats{}, fmt.Errorf("ranking: read stats key: %w", err)
	}

	var stats Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		// A corrupt entry must not stall ranking. Discarding it costs one full
		// recount and restores a well-formed memo, which is strictly better than
		// failing this element forever.
		return Stats{}, nil
	}
	return stats, nil
}

func (store *RedisStats) Put(ctx context.Context, postID, elementID int64, stats Stats) error {
	body, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("ranking: encode stats: %w", err)
	}
	if err := store.client.Set(ctx, store.key(postID, elementID), body, StatsTTL).Err(); err != nil {
		return fmt.Errorf("ranking: write stats key: %w", err)
	}
	return nil
}
