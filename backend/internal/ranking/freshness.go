package ranking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// FreshnessStore reports which posts have had their ranks changed since the last
// history build.
//
// SHARED WITH LARAVEL. Both sides write it now — Go from the vote path when a game
// completes, Laravel from App\Listeners\UpdatePostRank on the same event — so the
// key has to be the one Laravel uses, under Laravel's own cache prefix. That is why
// the prefix is configurable rather than owned by this package, and it is also the
// reason both processes must point at the same Redis instance *and* the same
// database index: Laravel's cache connection is not necessarily index 0.
//
// The full Redis key Laravel produces is
//
//	{slug(APP_NAME)}_database_{slug(APP_NAME)}_cache:need_fresh_post_rank_{postID}
//
// so the prefix differs per environment and must be supplied. Observed locally:
// "2pick_test_database_2pick_test_cache:" in the test environment and
// "_database__cache:" with APP_NAME unset.
type FreshnessStore interface {
	// NeedsRebuild reports whether the post is flagged.
	NeedsRebuild(ctx context.Context, postID int64) (bool, error)
	// Clear removes the flag, which the build does once it has fanned out.
	Clear(ctx context.Context, postID int64) error
	// Set flags the post as needing a rebuild.
	//
	// This closes a real dependency on Laravel rather than a coexistence detail.
	// Only App\Listeners\UpdatePostRank sets this flag, on the GameComplete event,
	// so until Go's own vote path set it the rank history sweep could only ever see
	// posts that Laravel had finished a game for. With Laravel switched off the
	// sweep would have found nothing at all, every day, silently.
	Set(ctx context.Context, postID int64) error
}

// FreshnessTTL matches CacheService::setNeedFreshPostRank, which stores the flag
// for three days. The window has to outlast a sweep that fails or is skipped;
// three days covers a long weekend.
const FreshnessTTL = 3 * 24 * time.Hour

// serializedTrue is PHP's serialize(true), the exact bytes Laravel's cache stores
// for `true`.
//
// Written rather than any placeholder because Laravel still reads this key while
// both sides run: CacheService::getNeedFreshPostRank goes through Cache::get,
// which unserializes, and bytes PHP cannot unserialize make it warn on every read.
// The Go side only checks for presence, so it does not care either way.
const serializedTrue = "b:1;"

// FreshnessKey is the un-prefixed key Laravel uses.
func FreshnessKey(postID int64) string {
	return fmt.Sprintf("need_fresh_post_rank_%d", postID)
}

// RedisFreshness reads the flag from Redis.
type RedisFreshness struct {
	client    redis.Cmdable
	keyPrefix string
}

// NewRedisFreshness builds the store. An empty prefix is allowed but means the Go
// side will not see any flag Laravel wrote, so the caller should log that.
func NewRedisFreshness(client redis.Cmdable, keyPrefix string) (*RedisFreshness, error) {
	if client == nil {
		return nil, errors.New("ranking: redis client is required")
	}
	return &RedisFreshness{client: client, keyPrefix: keyPrefix}, nil
}

func (store *RedisFreshness) key(postID int64) string {
	return store.keyPrefix + FreshnessKey(postID)
}

func (store *RedisFreshness) NeedsRebuild(ctx context.Context, postID int64) (bool, error) {
	// Laravel serialises `true` rather than storing a bare flag, so only presence
	// is meaningful here; the value is deliberately not interpreted.
	count, err := store.client.Exists(ctx, store.key(postID)).Result()
	if err != nil {
		return false, fmt.Errorf("ranking: read freshness flag for post %d: %w", postID, err)
	}
	return count > 0, nil
}

func (store *RedisFreshness) Clear(ctx context.Context, postID int64) error {
	if err := store.client.Del(ctx, store.key(postID)).Err(); err != nil {
		return fmt.Errorf("ranking: clear freshness flag for post %d: %w", postID, err)
	}
	return nil
}

// Set flags the post, refreshing the TTL if it was already flagged.
//
// A plain SET rather than SET NX on purpose: a post played twice in three days
// should keep the full window from the second game, not expire on the first
// game's clock.
func (store *RedisFreshness) Set(ctx context.Context, postID int64) error {
	if postID <= 0 {
		return fmt.Errorf("ranking: cannot flag post id %d", postID)
	}
	if err := store.client.Set(ctx, store.key(postID), serializedTrue, FreshnessTTL).Err(); err != nil {
		return fmt.Errorf("ranking: set freshness flag for post %d: %w", postID, err)
	}
	return nil
}
