package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// The per-source cap on reset mails. See ResetRequestLimiter for why the per-account
// throttle does not cover this.

const (
	resetLimiterKeyPrefix = "go:password-reset:ip:"
	// ResetRequestsPerWindow and ResetRequestWindow: five mails an hour from one source.
	// A person who mistypes their address a few times stays under it; a script working
	// through a list of addresses does not.
	ResetRequestsPerWindow = 5
	ResetRequestWindow     = time.Hour
)

// RedisResetLimiter is a fixed-window counter per source address.
type RedisResetLimiter struct {
	client redis.Cmdable
}

func NewRedisResetLimiter(client redis.Cmdable) (*RedisResetLimiter, error) {
	if client == nil {
		return nil, fmt.Errorf("auth: redis client is required")
	}
	return &RedisResetLimiter{client: client}, nil
}

// AllowReset counts this request and reports whether it is within the window's budget.
//
// EXPIRE NX rather than a plain EXPIRE, the same as ingest.RedisRateLimiter: refreshing
// the deadline on every request would turn the fixed hour into a window that never ends
// for anyone still trying.
func (limiter *RedisResetLimiter) AllowReset(ctx context.Context, ip string) (bool, error) {
	key := resetLimiterKeyPrefix + ip
	used, err := limiter.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("auth: password reset rate limit %q: %w", key, err)
	}
	if err := limiter.client.ExpireNX(ctx, key, ResetRequestWindow).Err(); err != nil {
		return false, fmt.Errorf("auth: password reset rate limit expiry %q: %w", key, err)
	}
	// Not rolled back on refusal, unlike the upload limiter: there the budget is the
	// resource the author is spending, here the point is to stop the attempts
	// themselves, and letting a refused attempt cost nothing would let a script keep
	// probing for free.
	return used <= ResetRequestsPerWindow, nil
}
