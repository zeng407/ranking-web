package admin

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// The site-wide announcement, from Admin\AnnouncementController.
//
// It lives in the shared cache rather than a table, which is the original's design and is
// kept: the Blade layout reads that key on every page, so a Go-only store would make an
// announcement published here invisible until Blade retires. See RedisAnnouncements for
// the format that sharing requires.

// DefaultAnnouncementMinutes is the original's default keep_minutes.
const DefaultAnnouncementMinutes = 60

// MaxAnnouncementMinutes bounds how long one can be published for.
//
// A month. The original accepted any integer, so a typo could pin a banner in the cache
// for years — and nothing but a cache flush or a replacement takes one down.
const MaxAnnouncementMinutes = 60 * 24 * 30

// MaxAnnouncementContentRunes bounds the body. The original validated only that it was a
// string; this is a dialog, not an article.
const MaxAnnouncementContentRunes = 2000

// ErrAnnouncementsUnavailable means this process has no shared cache to publish into.
var ErrAnnouncementsUnavailable = errors.New("admin: the announcement store is not configured")

// AnnouncementLocation is config/app.php's timezone, which is what Laravel's now()
// formats in — the cached created_at is read by clients that were written against it.
var AnnouncementLocation = time.FixedZone("Asia/Taipei", 8*60*60)

// Announcement is what every visitor is shown once.
type Announcement struct {
	// ID is what the client remembers in its "never show again" cookie, so a new
	// announcement needs a new one — see PublishAnnouncement.
	ID       string
	Content  string
	ImageURL string
	// CreatedAt is Laravel's `Y-m-d H:i:s` in the application's timezone, kept as a
	// string because that is what the cached value holds and what the client reads.
	CreatedAt string
	// KeepMinutes is both the cookie's lifetime on the client and the cache entry's TTL.
	KeepMinutes int
}

// AnnouncementDraft is what publishing accepts.
type AnnouncementDraft struct {
	Content  string
	ImageURL string
	// Minutes of 0 means the default.
	Minutes int
}

// Announcement reads the current one. The second result is false when there is none.
func (service *Service) Announcement(ctx context.Context) (Announcement, bool, error) {
	if service.announcements == nil {
		return Announcement{}, false, ErrAnnouncementsUnavailable
	}
	announcement, found, err := service.announcements.Announcement(ctx)
	return announcement, found, wrap("read the announcement", err)
}

// PublishAnnouncement replaces the announcement.
//
// The id is generated here rather than accepted: it is what the client's "never show
// again" cookie holds, so reusing one would leave the new announcement unseen by everyone
// who dismissed the old.
func (service *Service) PublishAnnouncement(
	ctx context.Context, draft AnnouncementDraft,
) (Announcement, error) {
	if service.announcements == nil {
		return Announcement{}, ErrAnnouncementsUnavailable
	}

	content := strings.TrimSpace(draft.Content)
	switch {
	case content == "":
		return Announcement{}, invalid("content", authoringRequired)
	case utf8.RuneCountInString(content) > MaxAnnouncementContentRunes:
		return Announcement{}, invalid("content", authoringTooLong)
	}

	minutes := draft.Minutes
	if minutes == 0 {
		minutes = DefaultAnnouncementMinutes
	}
	if minutes < 1 || minutes > MaxAnnouncementMinutes {
		return Announcement{}, invalid("minutes", CodeInvalidRange)
	}

	announcement := Announcement{
		ID:          uuid.NewString(),
		Content:     content,
		ImageURL:    strings.TrimSpace(draft.ImageURL),
		CreatedAt:   service.now().In(AnnouncementLocation).Format("2006-01-02 15:04:05"),
		KeepMinutes: minutes,
	}
	if err := service.announcements.PutAnnouncement(ctx, announcement); err != nil {
		return Announcement{}, wrap("publish the announcement", err)
	}
	return announcement, nil
}

// KeepFor is the cache TTL an announcement asks for.
func (announcement Announcement) KeepFor() time.Duration {
	return time.Duration(announcement.KeepMinutes) * time.Minute
}
