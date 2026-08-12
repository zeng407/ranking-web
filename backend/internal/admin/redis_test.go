package admin

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const testCachePrefix = "2pick:test:admin:"

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set; skipping Redis integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis at %s is unreachable: %v", addr, err)
	}
	t.Cleanup(func() {
		keys, _ := client.Keys(context.Background(), testCachePrefix+"*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
		_ = client.Close()
	})
	return client
}

// The key has to be the one Laravel's rememberUserRole wrote, prefix included: a delete
// under the wrong key leaves the banned account looking unbanned to every Blade page.
func TestForgettingRolesDeletesLaravelsKey(t *testing.T) {
	client := testRedis(t)
	cache, err := NewRedisRoleCache(client, testCachePrefix)
	if err != nil {
		t.Fatalf("NewRedisRoleCache() error = %v", err)
	}
	key := testCachePrefix + UserRoleKeyPrefix + "77"
	if err := client.Set(context.Background(), key, `a:0:{}`, time.Hour).Err(); err != nil {
		t.Fatalf("seed error = %v", err)
	}

	if err := cache.ForgetUserRoles(context.Background(), 77); err != nil {
		t.Fatalf("ForgetUserRoles() error = %v", err)
	}
	if count, err := client.Exists(context.Background(), key).Result(); err != nil || count != 0 {
		t.Errorf("key exists = %d (error %v), want it gone", count, err)
	}
}

// Deleting a key that is not there is not a failure: the cache may simply have expired.
func TestForgettingAnAbsentCacheEntrySucceeds(t *testing.T) {
	client := testRedis(t)
	roles, err := NewRedisRoleCache(client, testCachePrefix)
	if err != nil {
		t.Fatalf("NewRedisRoleCache() error = %v", err)
	}
	carousels, err := NewRedisCarouselCache(client, testCachePrefix)
	if err != nil {
		t.Fatalf("NewRedisCarouselCache() error = %v", err)
	}

	if err := roles.ForgetUserRoles(context.Background(), 999999); err != nil {
		t.Errorf("ForgetUserRoles() error = %v", err)
	}
	if err := carousels.ForgetCarousels(context.Background()); err != nil {
		t.Errorf("ForgetCarousels() error = %v", err)
	}
}

func TestForgettingTheCarouselDeletesLaravelsKey(t *testing.T) {
	client := testRedis(t)
	cache, err := NewRedisCarouselCache(client, testCachePrefix)
	if err != nil {
		t.Fatalf("NewRedisCarouselCache() error = %v", err)
	}
	key := testCachePrefix + CarouselsKey
	if err := client.Set(context.Background(), key, `a:0:{}`, time.Hour).Err(); err != nil {
		t.Fatalf("seed error = %v", err)
	}

	if err := cache.ForgetCarousels(context.Background()); err != nil {
		t.Fatalf("ForgetCarousels() error = %v", err)
	}
	if count, err := client.Exists(context.Background(), key).Result(); err != nil || count != 0 {
		t.Errorf("key exists = %d (error %v), want it gone", count, err)
	}
}

func newTestAnnouncements(t *testing.T) (*RedisAnnouncements, *redis.Client) {
	t.Helper()
	client := testRedis(t)
	store, err := NewRedisAnnouncements(client, testCachePrefix)
	if err != nil {
		t.Fatalf("NewRedisAnnouncements() error = %v", err)
	}
	return store, client
}

// What Go writes, Go reads back — and what it writes is what PHP's unserialize() takes,
// which the format test below pins byte for byte.
func TestAnAnnouncementRoundTripsThroughTheSharedCache(t *testing.T) {
	store, _ := newTestAnnouncements(t)
	published := Announcement{
		ID:          "9f1c",
		Content:     "維護公告：晚上十點停機",
		ImageURL:    "https://img.example/notice.png",
		CreatedAt:   "2026-08-12 22:00:00",
		KeepMinutes: 30,
	}

	if err := store.PutAnnouncement(context.Background(), published); err != nil {
		t.Fatalf("PutAnnouncement() error = %v", err)
	}
	read, found, err := store.Announcement(context.Background())
	if err != nil {
		t.Fatalf("Announcement() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want the announcement")
	}
	if read != published {
		t.Errorf("read = %+v, want %+v", read, published)
	}
}

// The stored bytes are PHP's serialize() output, with byte lengths rather than rune
// counts: a rune count would make unserialize() read past the end of a Chinese string.
func TestTheStoredAnnouncementIsPHPSerialized(t *testing.T) {
	store, client := newTestAnnouncements(t)

	if err := store.PutAnnouncement(context.Background(), Announcement{
		ID: "abc", Content: "嗨", CreatedAt: "2026-08-12 22:00:00", KeepMinutes: 60,
	}); err != nil {
		t.Fatalf("PutAnnouncement() error = %v", err)
	}

	payload, err := client.Get(context.Background(), testCachePrefix+AnnouncementKey).Result()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// content is one 3-byte rune, and the length prefix says 3, not 1.
	want := `a:5:{s:2:"id";s:3:"abc";s:7:"content";s:3:"嗨";` +
		`s:9:"image_url";N;s:10:"created_at";s:19:"2026-08-12 22:00:00";` +
		`s:12:"keep_minutes";i:60;}`
	if payload != want {
		t.Errorf("payload = %s, want %s", payload, want)
	}
}

// The TTL is the announcement's own keep_minutes, which is also the client cookie's
// lifetime: a banner that outlives the cookie would be shown twice.
func TestAnAnnouncementExpiresAfterItsKeepMinutes(t *testing.T) {
	store, client := newTestAnnouncements(t)

	if err := store.PutAnnouncement(context.Background(), Announcement{
		ID: "abc", Content: "hello", KeepMinutes: 45,
	}); err != nil {
		t.Fatalf("PutAnnouncement() error = %v", err)
	}

	ttl, err := client.TTL(context.Background(), testCachePrefix+AnnouncementKey).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 44*time.Minute || ttl > 45*time.Minute {
		t.Errorf("ttl = %v, want about 45m", ttl)
	}
}

// Laravel's rememberAnnouncement caches the closure's result even when it is null, so the
// key can exist and hold nothing. That is "no announcement", not a decode failure.
func TestASerializedNullMeansNoAnnouncement(t *testing.T) {
	store, client := newTestAnnouncements(t)
	if err := client.Set(context.Background(), testCachePrefix+AnnouncementKey, "N;", time.Hour).Err(); err != nil {
		t.Fatalf("seed error = %v", err)
	}

	_, found, err := store.Announcement(context.Background())
	if err != nil {
		t.Fatalf("Announcement() error = %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

func TestAnAbsentAnnouncementIsNotAnError(t *testing.T) {
	store, _ := newTestAnnouncements(t)

	_, found, err := store.Announcement(context.Background())
	if err != nil {
		t.Fatalf("Announcement() error = %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

// A value in a shape this decoder does not cover is an error rather than a guess: a
// guessed announcement is a wrong one on every page of the site.
func TestAnUndecodableAnnouncementIsAnError(t *testing.T) {
	store, client := newTestAnnouncements(t)
	if err := client.Set(context.Background(), testCachePrefix+AnnouncementKey,
		`O:8:"stdClass":0:{}`, time.Hour).Err(); err != nil {
		t.Fatalf("seed error = %v", err)
	}

	if _, _, err := store.Announcement(context.Background()); err == nil {
		t.Error("Announcement() error = nil, want a decode error")
	} else if !strings.Contains(err.Error(), "read the announcement") {
		t.Errorf("error = %v, want it to name the operation", err)
	}
}

func TestTheRedisStoresNeedAClient(t *testing.T) {
	if _, err := NewRedisRoleCache(nil, ""); err == nil {
		t.Error("NewRedisRoleCache(nil) error = nil, want an error")
	}
	if _, err := NewRedisCarouselCache(nil, ""); err == nil {
		t.Error("NewRedisCarouselCache(nil) error = nil, want an error")
	}
	if _, err := NewRedisAnnouncements(nil, ""); err == nil {
		t.Error("NewRedisAnnouncements(nil) error = nil, want an error")
	}
}
