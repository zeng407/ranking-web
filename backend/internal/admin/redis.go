package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// The caches this package shares with Laravel.
//
// All three keys are Laravel's, written and read by App\Helper\CacheService, so the full
// cache prefix has to be supplied rather than derived: the wrong prefix writes an entry
// nothing reads and deletes an entry that does not exist, neither of which fails loudly.

// Laravel's cache keys, from CacheService.
const (
	// AnnouncementKey is rememberAnnouncement's key.
	AnnouncementKey = "announcement"
	// UserRoleKeyPrefix is rememberUserRole's, with the user id appended.
	UserRoleKeyPrefix = "user_role_"
	// CarouselsKey is rememberCarousels', cleared by clearCarousels.
	CarouselsKey = "carousels"
)

// RedisRoleCache deletes the cached role list for one account.
type RedisRoleCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisRoleCache(client redis.Cmdable, laravelCachePrefix string) (*RedisRoleCache, error) {
	if client == nil {
		return nil, errors.New("admin: redis client is required")
	}
	return &RedisRoleCache{client: client, prefix: laravelCachePrefix}, nil
}

func (cache *RedisRoleCache) ForgetUserRoles(ctx context.Context, userID int64) error {
	key := cache.prefix + UserRoleKeyPrefix + strconv.FormatInt(userID, 10)
	if err := cache.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("admin: forget the cached roles of user %d: %w", userID, err)
	}
	return nil
}

// RedisCarouselCache deletes the cached home carousel.
type RedisCarouselCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisCarouselCache(client redis.Cmdable, laravelCachePrefix string) (*RedisCarouselCache, error) {
	if client == nil {
		return nil, errors.New("admin: redis client is required")
	}
	return &RedisCarouselCache{client: client, prefix: laravelCachePrefix}, nil
}

func (cache *RedisCarouselCache) ForgetCarousels(ctx context.Context) error {
	if err := cache.client.Del(ctx, cache.prefix+CarouselsKey).Err(); err != nil {
		return fmt.Errorf("admin: forget the cached carousel: %w", err)
	}
	return nil
}

// RedisAnnouncements is the announcement, stored where Laravel's layout reads it.
//
// THE VALUE IS PHP-SERIALISED, WHICH IS NOT THIS CODEBASE'S USUAL CHOICE. Everywhere else
// a shared cache entry is either a flag whose bytes nothing parses or a key Go only
// deletes. This one is a record that Blade renders on every page, so it has to be in the
// format Laravel's unserialize() reads — see phpserialize.go. When the Blade pages retire
// the storage can move without the API's callers noticing.
type RedisAnnouncements struct {
	client redis.Cmdable
	prefix string
}

func NewRedisAnnouncements(client redis.Cmdable, laravelCachePrefix string) (*RedisAnnouncements, error) {
	if client == nil {
		return nil, errors.New("admin: redis client is required")
	}
	return &RedisAnnouncements{client: client, prefix: laravelCachePrefix}, nil
}

func (store *RedisAnnouncements) key() string {
	return store.prefix + AnnouncementKey
}

// Field names of the cached array, which are the keys resources/js/components/Announcement.vue
// reads.
const (
	announcementFieldID          = "id"
	announcementFieldContent     = "content"
	announcementFieldImageURL    = "image_url"
	announcementFieldCreatedAt   = "created_at"
	announcementFieldKeepMinutes = "keep_minutes"
)

func (store *RedisAnnouncements) Announcement(ctx context.Context) (Announcement, bool, error) {
	payload, err := store.client.Get(ctx, store.key()).Result()
	if errors.Is(err, redis.Nil) {
		return Announcement{}, false, nil
	}
	if err != nil {
		return Announcement{}, false, fmt.Errorf("admin: read the announcement: %w", err)
	}
	// rememberAnnouncement caches the closure's result even when it is null, which is
	// stored as serialize(null). An entry that holds nothing is the same as no entry.
	if payload == "N;" {
		return Announcement{}, false, nil
	}

	array, err := decodePHPArray(payload)
	if err != nil {
		return Announcement{}, false, fmt.Errorf("admin: read the announcement: %w", err)
	}
	announcement := Announcement{
		ID:          array.string(announcementFieldID),
		Content:     array.string(announcementFieldContent),
		ImageURL:    array.string(announcementFieldImageURL),
		CreatedAt:   array.string(announcementFieldCreatedAt),
		KeepMinutes: array.int(announcementFieldKeepMinutes),
	}
	return announcement, true, nil
}

func (store *RedisAnnouncements) PutAnnouncement(ctx context.Context, announcement Announcement) error {
	array := newPHPArray()
	array.set(announcementFieldID, announcement.ID)
	array.set(announcementFieldContent, announcement.Content)
	// Absent rather than empty: the Blade component passes this straight to SweetAlert's
	// imageUrl, and an empty string there draws a broken image.
	if announcement.ImageURL == "" {
		array.set(announcementFieldImageURL, nil)
	} else {
		array.set(announcementFieldImageURL, announcement.ImageURL)
	}
	array.set(announcementFieldCreatedAt, announcement.CreatedAt)
	array.set(announcementFieldKeepMinutes, announcement.KeepMinutes)

	payload, err := array.encode()
	if err != nil {
		return fmt.Errorf("admin: encode the announcement: %w", err)
	}
	if err := store.client.Set(ctx, store.key(), payload, announcement.KeepFor()).Err(); err != nil {
		return fmt.Errorf("admin: publish the announcement: %w", err)
	}
	return nil
}
