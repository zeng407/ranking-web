package postaccess

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// MySQLStore implements Store.
type MySQLStore struct {
	database *sql.DB
}

func NewMySQLStore(database *sql.DB) *MySQLStore {
	return &MySQLStore{database: database}
}

const postQuery = `
	SELECT p.id, p.serial, COALESCE(p.user_id, 0), pp.access_policy, COALESCE(pp.password, '')
	  FROM posts AS p
	  JOIN post_policies AS pp ON pp.post_id = p.id
	 WHERE p.serial = ? AND p.deleted_at IS NULL
	 LIMIT 1`

func (store *MySQLStore) Post(ctx context.Context, serial string) (Post, error) {
	var post Post
	err := store.database.QueryRowContext(ctx, postQuery, serial).Scan(
		&post.ID, &post.Serial, &post.OwnerID, &post.Policy, &post.PasswordDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return Post{}, ErrPostNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("postaccess: read post %q: %w", serial, err)
	}
	return post, nil
}

// RedisAttempts limits password guesses per post.
//
// Per post rather than per visitor, matching GameController::access, which keyed its
// limiter on 'access:' . $post->id. A door code is shared, so the thing worth protecting
// is the post, not any one guesser — and a per-visitor limit is defeated by changing
// address.
type RedisAttempts struct {
	client redis.Cmdable
	prefix string
}

func NewRedisAttempts(client redis.Cmdable, prefix string) (*RedisAttempts, error) {
	if client == nil {
		return nil, errors.New("postaccess: redis client is required")
	}
	if prefix == "" {
		prefix = "2pick:go:post-access:"
	}
	return &RedisAttempts{client: client, prefix: prefix}, nil
}

func (attempts *RedisAttempts) Allow(ctx context.Context, serial string) (bool, error) {
	key := attempts.prefix + serial

	used, err := attempts.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("postaccess: rate limit %q: %w", serial, err)
	}
	// EXPIRE NX so the window belongs to the first attempt of the minute rather than the
	// latest: refreshing it on every guess would let a steady guesser hold one window open
	// indefinitely.
	if err := attempts.client.ExpireNX(ctx, key, RateWindow).Err(); err != nil {
		return false, fmt.Errorf("postaccess: rate limit expiry %q: %w", serial, err)
	}
	return used <= RateLimit, nil
}
