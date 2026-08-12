// Package scheduling owns the recurring entries that Laravel's console Kernel
// currently runs.
//
// Two rules shape everything here:
//
// Entries only enqueue. Every entry's payload is a queue message; the work runs
// in the worker. That keeps this process stateless and free of a database
// handle, so the only thing a second replica can duplicate is a dispatch, which
// the lock already prevents.
//
// Every entry is gated by its own feature flag, defaulting to off. A schedule
// running in Laravel and in Go at the same time would double-count trends and
// double-write public posts, so cutover is one entry at a time: turn the Laravel
// entry off, turn the Go flag on, observe.
package scheduling

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"2pick.app/backend/internal/queue"
)

// Entry is one recurring schedule.
type Entry struct {
	// Name identifies the entry in logs and as the lock key. It must be stable:
	// changing it silently orphans a held lock.
	Name string
	// Spec is a standard 5-field cron expression, evaluated in the configured
	// timezone.
	Spec string
	// LockTTL mirrors the minutes passed to Laravel's withoutOverlapping.
	LockTTL time.Duration
	// Flag is the environment variable that enables this entry.
	Flag string
	// LaravelEntry is the Kernel entry this replaces, kept for traceability
	// during the cutover.
	LaravelEntry string
	// Message is the template enqueued when the entry fires. The runner stamps
	// the idempotency key per tick.
	Message queue.Message
}

func mustPayload(value any) json.RawMessage {
	body, err := json.Marshal(value)
	if err != nil {
		// The payloads are compile-time literals, so this is unreachable unless a
		// developer introduces an unmarshalable type.
		panic(fmt.Sprintf("scheduling: payload must marshal: %v", err))
	}
	return body
}

// Entries returns every schedule this process knows how to run, enabled or not.
//
// The cron specs and lock TTLs are transcribed from app/Console/Kernel.php. The
// post-trend range values are the command's own argument values: the Kernel runs
// `make:post-trend day`, which the command maps to createTodayPostTrends, so the
// wire value stays "day" rather than the TrendTimeRange constant "today".
func Entries() []Entry {
	postTrend := func(rangeValue, spec string) Entry {
		return Entry{
			Name:         "post-trend-" + rangeValue,
			Spec:         spec,
			LockTTL:      120 * time.Minute,
			Flag:         "SCHEDULE_POST_TREND_" + strings.ToUpper(rangeValue),
			LaravelEntry: "make:post-trend " + rangeValue,
			Message: queue.Message{
				Queue:   "default",
				Type:    "post_trend.create",
				Payload: mustPayload(map[string]string{"range": rangeValue}),
			},
		}
	}

	return []Entry{
		postTrend("all", "15 * * * *"),
		postTrend("month", "25 * * * *"),
		postTrend("week", "35 * * * *"),
		postTrend("day", "45 * * * *"),
		{
			Name:         "update-public-posts",
			Spec:         "* * * * *",
			LockTTL:      60 * time.Minute,
			Flag:         "SCHEDULE_UPDATE_PUBLIC_POSTS",
			LaravelEntry: "Update Public Posts",
			Message: queue.Message{
				Queue:   "default",
				Type:    "public_post.refresh",
				Payload: mustPayload(struct{}{}),
			},
		},
		{
			Name:    "generate-sitemap",
			Spec:    "20 5 * * *",
			LockTTL: 120 * time.Minute,
			Flag:    "SCHEDULE_GENERATE_SITEMAP",
			// Kernel: sitemap:generate dailyAt('05:20').
			//
			// Enabling this needs the frontend to be served from the bucket the
			// worker writes to, otherwise the file goes somewhere nothing reads.
			LaravelEntry: "Generate Sitemap",
			Message: queue.Message{
				// Nothing waits on a sitemap, so it shares the low queue.
				Queue:   "low",
				Type:    "sitemap.generate",
				Payload: mustPayload(struct{}{}),
			},
		},
		{
			Name:    "make-rank-report-history",
			Spec:    "15 6 * * *",
			LockTTL: 120 * time.Minute,
			Flag:    "SCHEDULE_MAKE_RANK_REPORT_HISTORY",
			// Kernel: Artisan::call('make:rank-report-history') at 06:15.
			LaravelEntry: "Make Rank Report History",
			Message: queue.Message{
				Queue: "rank_report_history",
				Type:  "rank.sweep_post_history",
				// No refresh: the daily run resumes rather than rebuilding, matching
				// the command being called without --refresh.
				Payload: mustPayload(struct{}{}),
			},
		},
		{
			Name:         "remove-outdate-rank-report-history",
			Spec:         "30 5 * * *",
			LockTTL:      120 * time.Minute,
			Flag:         "SCHEDULE_REMOVE_OUTDATE_RANK_REPORT_HISTORY",
			LaravelEntry: "Remove Outdate Rank Report History",
			Message: queue.Message{
				Queue:   "rank_report_history",
				Type:    "rank.sweep_purge_history",
				Payload: mustPayload(struct{}{}),
			},
		},
		{
			Name:    "make-thumbnails",
			Spec:    "0 * * * *",
			LockTTL: 120 * time.Minute,
			Flag:    "SCHEDULE_MAKE_THUMBNAILS",
			// ThumbnailExecutor::makeElementThumbnails(300) in the hourly entry.
			LaravelEntry: "Make Thumbnails",
			Message: queue.Message{
				// The media queue keeps ffmpeg work from sitting in front of the
				// ranking jobs.
				Queue: "media",
				Type:  "media.sweep_thumbnails",
				// An empty column means both the low and medium derivatives, which is
				// what the executor does.
				Payload: mustPayload(map[string]int{"limit": 300}),
			},
		},
		{
			Name:         "remove-unused-images",
			Spec:         "0 * * * *",
			LockTTL:      60 * time.Minute,
			Flag:         "SCHEDULE_REMOVE_UNUSED_IMAGES",
			LaravelEntry: "Remove Unused Images",
			Message: queue.Message{
				Queue:   "media",
				Type:    "media.remove_deleted_files",
				Payload: mustPayload(map[string]int{"limit": 1000}),
			},
		},
	}
}

// The cachePosts schedule is deliberately absent.
//
// Laravel warms its own response cache by issuing ten HTTP GETs to
// api.public-post.index, whose controller wraps the query in
// CacheService::rememberPosts. There is nothing equivalent to warm here: the Go
// endpoint reads the precomputed public_posts table directly and pushes caching to
// the edge with Cloudflare-CDN-Cache-Control, so it holds no server-side entry that
// a warm-up could fill. Porting the schedule would either be a no-op or ten HTTP
// calls the API makes to itself for no benefit.
//
// SCHEDULE_CACHE_POSTS therefore has no Go counterpart. Leave the Laravel entry
// running until the PHP endpoints stop serving traffic, then delete it there too.

// Validate checks the entry definitions are internally consistent. It guards
// against a copy-paste that would make two entries share a lock key, which is
// the defect Laravel has today: refresh:token twitch and refresh:token imgur
// both registered as "Refresh Twitch Token" and so blocked each other.
func Validate(entries []Entry) error {
	names := make(map[string]struct{}, len(entries))
	flags := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("scheduling: entry %q has no name", entry.LaravelEntry)
		}
		if _, exists := names[entry.Name]; exists {
			return fmt.Errorf("scheduling: duplicate entry name %q would share a lock key", entry.Name)
		}
		names[entry.Name] = struct{}{}

		if strings.TrimSpace(entry.Flag) == "" {
			return fmt.Errorf("scheduling: entry %q has no feature flag", entry.Name)
		}
		if _, exists := flags[entry.Flag]; exists {
			return fmt.Errorf("scheduling: duplicate feature flag %q", entry.Flag)
		}
		flags[entry.Flag] = struct{}{}

		if entry.LockTTL <= 0 {
			return fmt.Errorf("scheduling: entry %q needs a positive lock ttl", entry.Name)
		}
		if strings.TrimSpace(entry.Spec) == "" {
			return fmt.Errorf("scheduling: entry %q has no cron spec", entry.Name)
		}
		if entry.Message.Queue == "" || entry.Message.Type == "" {
			return fmt.Errorf("scheduling: entry %q has an incomplete message", entry.Name)
		}
	}
	return nil
}

// Select splits entries by their feature flag, using lookup to read the
// environment. An unrecognised flag value is an error rather than a silent
// "off", so a typo does not look like a deliberate disable.
func Select(entries []Entry, lookup func(string) string) (enabled, disabled []Entry, err error) {
	for _, entry := range entries {
		on, err := flagEnabled(lookup(entry.Flag))
		if err != nil {
			return nil, nil, fmt.Errorf("scheduling: %s: %w", entry.Flag, err)
		}
		if on {
			enabled = append(enabled, entry)
		} else {
			disabled = append(disabled, entry)
		}
	}
	return enabled, disabled, nil
}

func flagEnabled(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false", "0", "off":
		return false, nil
	case "true", "1", "on":
		return true, nil
	default:
		return false, fmt.Errorf("must be one of true, false, 1, 0, on, off; got %q", value)
	}
}

// Names returns the sorted entry names, for logging.
func Names(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

// IdempotencyKey identifies one scheduled tick, so a redelivery of the same
// dispatch is detectable by the handler. Minute precision matches the finest
// schedule granularity.
func IdempotencyKey(name string, firedAt time.Time) string {
	return name + ":" + firedAt.UTC().Format("2006-01-02T15:04Z")
}
