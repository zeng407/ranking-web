// Package redislock provides the single-flight lock shared by the scheduler and
// the worker.
//
// It lives here rather than in either caller so the compare-and-delete release
// exists in exactly one place. Getting that wrong is subtle and severe: a plain
// DEL would let a run that overran its TTL delete a lock another holder had
// since taken, allowing two holders at once.
//
// Each subsystem passes its own key prefix so the namespaces stay distinct, and
// distinct from Laravel's own scheduler mutexes ("framework/schedule-<hash>"
// under its cache prefix), because both applications may share one Redis during
// the migration.
package redislock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotHeld is returned when releasing a lock the caller no longer owns, which
// happens when a run overran its TTL and another holder took over.
var ErrNotHeld = errors.New("redislock: lock is no longer held by this owner")

// releaseScript deletes the key only when it still carries our token.
const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

// Releaser is a held lock. Keeping the release path behind an interface lets
// callers be tested without Redis, while the lock semantics themselves are
// covered against a real server here.
type Releaser interface {
	Release(ctx context.Context) error
}

// Lock is a held lock.
type Lock struct {
	key    string
	token  string
	client redis.Scripter
}

// Release gives the lock up early so the next attempt is not blocked for the
// whole TTL. It reports ErrNotHeld if the lock had already expired and been
// taken by someone else.
func (lock *Lock) Release(ctx context.Context) error {
	deleted, err := lock.client.Eval(ctx, releaseScript, []string{lock.key}, lock.token).Int64()
	if err != nil {
		return fmt.Errorf("redislock: release %q: %w", lock.key, err)
	}
	if deleted == 0 {
		return ErrNotHeld
	}
	return nil
}

// Locker acquires named locks under a key prefix.
type Locker struct {
	client    redis.UniversalClient
	keyPrefix string
}

func New(client redis.UniversalClient, keyPrefix string) (*Locker, error) {
	if client == nil {
		return nil, errors.New("redislock: redis client is required")
	}
	if keyPrefix == "" {
		return nil, errors.New("redislock: key prefix is required")
	}
	return &Locker{client: client, keyPrefix: keyPrefix}, nil
}

// Acquire takes the lock for name, holding it for at most ttl. It reports
// acquired=false when another holder has it, which the caller must treat as
// "skip" or "try later", not as an error.
func (locker *Locker) Acquire(ctx context.Context, name string, ttl time.Duration) (Releaser, bool, error) {
	if name == "" {
		return nil, false, errors.New("redislock: lock name is required")
	}
	if ttl <= 0 {
		return nil, false, fmt.Errorf("redislock: ttl for %q must be positive", name)
	}

	token, err := newToken()
	if err != nil {
		return nil, false, err
	}

	key := locker.keyPrefix + name
	// The TTL is the safety net: if this process dies while holding the lock it
	// expires instead of blocking forever.
	acquired, err := locker.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redislock: acquire %q: %w", key, err)
	}
	if !acquired {
		return nil, false, nil
	}
	return &Lock{key: key, token: token, client: locker.client}, true, nil
}

func newToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("redislock: generate token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
