package ingest

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter is the per-account upload budget: 30 MiB or 50 files a minute.
//
// THE ORIGINAL'S VERSION HAD TWO DEFECTS, AND THIS IS NOT A FAITHFUL PORT OF THEM.
//
// It compared before adding — `if ($value > $limit) throw` and only then
// `$value += $size` — so the budget was checked against the previous total. An account
// could send one 4 MiB file every time the counter sat at 29 MiB and be let through, and
// the refusal only arrived on the request after the one that broke the limit.
//
// It also re-put the key with a fresh one-minute expiry on every upload, so the window
// never ended for anyone still uploading: a steady stream of files kept extending the
// same counter until it tripped, rather than starting a new minute. That is a sliding
// penalty, not the fixed minute the setting names.
//
// Here both counters are incremented first and the expiry is set only when the counter is
// new, which makes it the fixed window the configuration describes. A refusal rolls its
// own increment back, so a rejected request does not spend budget the author never used.
type RedisRateLimiter struct {
	client redis.Cmdable
	prefix string
}

func NewRedisRateLimiter(client redis.Cmdable, prefix string) (*RedisRateLimiter, error) {
	if client == nil {
		return nil, fmt.Errorf("ingest: redis client is required")
	}
	if prefix == "" {
		prefix = "2pick:go:upload:"
	}
	return &RedisRateLimiter{client: client, prefix: prefix}, nil
}

func (limiter *RedisRateLimiter) Allow(ctx context.Context, userID int64, size int) (bool, error) {
	bytesKey := fmt.Sprintf("%sbytes:%d", limiter.prefix, userID)
	filesKey := fmt.Sprintf("%sfiles:%d", limiter.prefix, userID)

	usedBytes, err := limiter.bump(ctx, bytesKey, int64(size))
	if err != nil {
		return false, err
	}
	if usedBytes > RateLimitBytes {
		// Rolled back so the refused request does not count against the next minute.
		_ = limiter.client.DecrBy(ctx, bytesKey, int64(size)).Err()
		return false, nil
	}

	usedFiles, err := limiter.bump(ctx, filesKey, 1)
	if err != nil {
		return false, err
	}
	if usedFiles > RateLimitFiles {
		_ = limiter.client.DecrBy(ctx, filesKey, 1).Err()
		_ = limiter.client.DecrBy(ctx, bytesKey, int64(size)).Err()
		return false, nil
	}
	return true, nil
}

// bump adds to a counter and gives it an expiry only if it did not have one.
//
// EXPIRE NX rather than a plain EXPIRE: refreshing the deadline on every write is what
// turned the original's fixed minute into a window that never closed.
func (limiter *RedisRateLimiter) bump(ctx context.Context, key string, amount int64) (int64, error) {
	used, err := limiter.client.IncrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, fmt.Errorf("ingest: rate limit %q: %w", key, err)
	}
	if err := limiter.client.ExpireNX(ctx, key, RateLimitWindow).Err(); err != nil {
		return 0, fmt.Errorf("ingest: rate limit expiry %q: %w", key, err)
	}
	return used, nil
}
