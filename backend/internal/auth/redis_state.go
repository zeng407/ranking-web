package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// oauthStateKeyPrefix namespaces the flow state in Redis.
//
// Not under Laravel's cache prefix, unlike the rank freshness flag: nothing in PHP
// reads or writes these, so there is no compatibility to preserve. Prefixed anyway so
// the keys are recognisable in a shared instance.
const oauthStateKeyPrefix = "go:oauth:state:"

// RedisOAuthStates stores in-flight OAuth flows.
type RedisOAuthStates struct {
	client *redis.Client
}

func NewRedisOAuthStates(client *redis.Client) *RedisOAuthStates {
	return &RedisOAuthStates{client: client}
}

// storedState is the wire form. Separate from OAuthState so a field rename in the
// struct cannot silently invalidate every flow already in flight.
type storedState struct {
	Verifier      string `json:"verifier"`
	ReturnTo      string `json:"return_to"`
	ConnectUserID int64  `json:"connect_user_id"`
	CreatedAt     int64  `json:"created_at"`
}

func (store *RedisOAuthStates) Put(
	ctx context.Context, key string, state OAuthState, ttl time.Duration,
) error {
	payload, err := json.Marshal(storedState{
		Verifier:      state.Verifier,
		ReturnTo:      state.ReturnTo,
		ConnectUserID: state.ConnectUserID,
		CreatedAt:     state.CreatedAt.Unix(),
	})
	if err != nil {
		return fmt.Errorf("auth: encode oauth state: %w", err)
	}
	// SetNX, not Set: the key is 32 random bytes, so a collision means something is
	// badly wrong with the entropy source, and overwriting would be the wrong
	// response to it.
	created, err := store.client.SetNX(ctx, oauthStateKeyPrefix+key, payload, ttl).Result()
	if err != nil {
		return fmt.Errorf("auth: store oauth state: %w", err)
	}
	if !created {
		return errors.New("auth: oauth state key already exists")
	}
	return nil
}

// Consume reads and deletes in one round trip.
//
// GETDEL rather than GET followed by DEL. Two commands would let two callbacks
// carrying the same state both read it before either deleted it, which is exactly the
// replay this is here to prevent. GETDEL is atomic and has been in Redis since 6.2.
func (store *RedisOAuthStates) Consume(ctx context.Context, key string) (OAuthState, error) {
	payload, err := store.client.GetDel(ctx, oauthStateKeyPrefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		// Unknown, expired, or already consumed. All three mean the same thing to the
		// caller and must not be distinguishable to whoever sent the callback.
		return OAuthState{}, ErrOAuthStateInvalid
	}
	if err != nil {
		return OAuthState{}, fmt.Errorf("auth: read oauth state: %w", err)
	}

	var stored storedState
	if err := json.Unmarshal(payload, &stored); err != nil {
		// A state that cannot be decoded is a state that cannot be trusted. It has
		// already been deleted, so there is nothing to clean up.
		return OAuthState{}, fmt.Errorf("%w: %v", ErrOAuthStateInvalid, err)
	}
	return OAuthState{
		Verifier:      stored.Verifier,
		ReturnTo:      stored.ReturnTo,
		ConnectUserID: stored.ConnectUserID,
		CreatedAt:     time.Unix(stored.CreatedAt, 0).UTC(),
	}, nil
}

// MemoryOAuthStates is an in-process implementation, for tests and for a
// single-instance deployment with no Redis.
//
// Not suitable behind a load balancer: the callback can land on a different instance
// than the one that started the flow, and every such login would fail with an invalid
// state. The API is configured to prefer Redis whenever it has one.
type MemoryOAuthStates struct {
	mutex  sync.Mutex
	states map[string]memoryState
	now    func() time.Time
}

type memoryState struct {
	state     OAuthState
	expiresAt time.Time
}

func NewMemoryOAuthStates() *MemoryOAuthStates {
	return &MemoryOAuthStates{states: make(map[string]memoryState), now: time.Now}
}

func (store *MemoryOAuthStates) Put(
	_ context.Context, key string, state OAuthState, ttl time.Duration,
) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.states[key] = memoryState{state: state, expiresAt: store.now().Add(ttl)}
	return nil
}

func (store *MemoryOAuthStates) Consume(_ context.Context, key string) (OAuthState, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	entry, found := store.states[key]
	// Deleted whether or not it turns out to be expired: a state is one-shot.
	delete(store.states, key)
	if !found || store.now().After(entry.expiresAt) {
		return OAuthState{}, ErrOAuthStateInvalid
	}
	return entry.state, nil
}
