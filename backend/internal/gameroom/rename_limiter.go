package gameroom

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// renameKeyPrefix namespaces the cooldown in Redis.
//
// Not under Laravel's cache prefix. CacheService::putUpdateGameUserNameThreashold writes
// its own key, but nothing needs the two sides to agree: a rename is a single request
// against a single stack, and during the transition the worst case is one extra rename
// allowed at the moment traffic moves.
const renameKeyPrefix = "go:gameroom:rename:"

// RedisRenameLimiter enforces the rename cooldown across API instances.
type RedisRenameLimiter struct {
	client redis.Cmdable
}

func NewRedisRenameLimiter(client redis.Cmdable) *RedisRenameLimiter {
	return &RedisRenameLimiter{client: client}
}

// Allow claims the cooldown for one participant.
//
// SET NX with an expiry, which is one round trip and atomic. A read followed by a write
// would let two simultaneous renames both see no key and both proceed — and since a
// rename is broadcast to the room, that is the case worth preventing rather than the
// patient user who waits thirty seconds.
func (limiter *RedisRenameLimiter) Allow(
	ctx context.Context, participantID int64, cooldown time.Duration,
) (bool, error) {
	key := renameKeyPrefix + strconv.FormatInt(participantID, 10)
	claimed, err := limiter.client.SetNX(ctx, key, "1", cooldown).Result()
	if err != nil {
		return false, fmt.Errorf("gameroom: claim the rename cooldown: %w", err)
	}
	return claimed, nil
}

// MemoryRenameLimiter is the single-instance fallback, and the one tests use.
//
// Not suitable behind a load balancer: each instance would hold its own view of the
// cooldown, so N instances allow N renames per window. The API prefers Redis whenever it
// has one.
type MemoryRenameLimiter struct {
	mutex sync.Mutex
	until map[int64]time.Time
	now   func() time.Time
}

func NewMemoryRenameLimiter() *MemoryRenameLimiter {
	return &MemoryRenameLimiter{until: make(map[int64]time.Time), now: time.Now}
}

func (limiter *MemoryRenameLimiter) Allow(
	_ context.Context, participantID int64, cooldown time.Duration,
) (bool, error) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	if blockedUntil, found := limiter.until[participantID]; found && now.Before(blockedUntil) {
		return false, nil
	}
	limiter.until[participantID] = now.Add(cooldown)
	return true, nil
}
