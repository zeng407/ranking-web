package ranking

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisPendingDates records which history dates still need their rank assigned.
//
// The builder writes rank = 0 and a later pass fills it in, so the set of dates
// touched has to survive between the two jobs.
//
// It uses a Redis SET rather than the PHP version's serialised array. Laravel does
// read-modify-write on a cache entry:
//
//	$previous = Cache::pull(...); $dates = array_unique(array_merge(...)); Cache::put(...)
//
// A pull followed by a put is not atomic, so two builders finishing at the same
// moment can lose one another's dates, and a crash between the two loses the whole
// set. SADD is atomic and gives uniqueness for free.
//
// The trade-off is that the key shape differs from Laravel's, so the two
// implementations do not share state. That is acceptable because the producing and
// consuming jobs are cut over together.
type RedisPendingDates struct {
	client    redis.Cmdable
	keyPrefix string
}

// PendingDatesKeyPrefix namespaces the set away from Laravel's own cache entry,
// which is "RankHistoryNeededUpdateDatesCache:<post>_<range>" under its cache
// prefix.
const PendingDatesKeyPrefix = "2pick:go:rank-history-pending:"

func NewRedisPendingDates(client redis.Cmdable, keyPrefix string) (*RedisPendingDates, error) {
	if client == nil {
		return nil, errors.New("ranking: redis client is required")
	}
	if keyPrefix == "" {
		keyPrefix = PendingDatesKeyPrefix
	}
	return &RedisPendingDates{client: client, keyPrefix: keyPrefix}, nil
}

func (store *RedisPendingDates) key(postID int64, timeRange HistoryTimeRange) string {
	return fmt.Sprintf("%s%d:%s", store.keyPrefix, postID, timeRange)
}

func (store *RedisPendingDates) Add(
	ctx context.Context, postID int64, timeRange HistoryTimeRange, dates []string,
) error {
	if len(dates) == 0 {
		return nil
	}
	if !timeRange.Valid() {
		return fmt.Errorf("ranking: unknown history time range %q", timeRange)
	}

	members := make([]any, 0, len(dates))
	for _, date := range dates {
		if date == "" {
			continue
		}
		members = append(members, date)
	}
	if len(members) == 0 {
		return nil
	}

	key := store.key(postID, timeRange)
	pipeline := store.client.Pipeline()
	pipeline.SAdd(ctx, key, members...)
	// Refreshed on every add so a post that keeps producing dates keeps its set,
	// while one that goes quiet expires instead of leaking.
	pipeline.Expire(ctx, key, PendingDatesTTL)
	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("ranking: record pending dates: %w", err)
	}
	return nil
}

// Pull returns the recorded dates and clears them in one atomic step.
//
// Reading and deleting separately would drop any date added in between, which is
// exactly the window the reorder pass runs in.
func (store *RedisPendingDates) Pull(
	ctx context.Context, postID int64, timeRange HistoryTimeRange,
) ([]string, error) {
	if !timeRange.Valid() {
		return nil, fmt.Errorf("ranking: unknown history time range %q", timeRange)
	}

	key := store.key(postID, timeRange)
	pipeline := store.client.Pipeline()
	members := pipeline.SMembers(ctx, key)
	pipeline.Del(ctx, key)
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("ranking: pull pending dates: %w", err)
	}

	dates, err := members.Result()
	if err != nil {
		return nil, fmt.Errorf("ranking: read pending dates: %w", err)
	}
	return dates, nil
}
