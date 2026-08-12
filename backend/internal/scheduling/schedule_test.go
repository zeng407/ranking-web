package scheduling

import (
	"encoding/json"
	"testing"
	"time"

	"2pick.app/backend/internal/queue"
	"github.com/robfig/cron/v3"
)

func entryByName(t *testing.T, name string) Entry {
	t.Helper()
	for _, entry := range Entries() {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("no entry named %q", name)
	return Entry{}
}

func TestEntriesAreValid(t *testing.T) {
	if err := Validate(Entries()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// Guards the Laravel defect where two entries shared one mutex name and blocked
// each other.
func TestValidateRejectsDuplicateNameAndFlag(t *testing.T) {
	base := Entry{
		Name: "a", Spec: "* * * * *", LockTTL: time.Minute, Flag: "SCHEDULE_A",
		Message: sampleEntryMessage(),
	}
	duplicateName := base
	duplicateName.Flag = "SCHEDULE_B"
	if err := Validate([]Entry{base, duplicateName}); err == nil {
		t.Fatal("Validate() should reject a duplicate entry name")
	}

	duplicateFlag := base
	duplicateFlag.Name = "b"
	if err := Validate([]Entry{base, duplicateFlag}); err == nil {
		t.Fatal("Validate() should reject a duplicate feature flag")
	}
}

func TestValidateRejectsIncompleteEntry(t *testing.T) {
	cases := map[string]Entry{
		"no name":    {Spec: "* * * * *", LockTTL: time.Minute, Flag: "F", Message: sampleEntryMessage()},
		"no flag":    {Name: "a", Spec: "* * * * *", LockTTL: time.Minute, Message: sampleEntryMessage()},
		"no ttl":     {Name: "a", Spec: "* * * * *", Flag: "F", Message: sampleEntryMessage()},
		"no spec":    {Name: "a", LockTTL: time.Minute, Flag: "F", Message: sampleEntryMessage()},
		"no message": {Name: "a", Spec: "* * * * *", LockTTL: time.Minute, Flag: "F"},
	}
	for name, entry := range cases {
		if err := Validate([]Entry{entry}); err == nil {
			t.Errorf("Validate() should reject the %s case", name)
		}
	}
}

// Every spec must parse with the same parser the runner uses, or the entry would
// fail only at startup in production.
func TestEntrySpecsParseWithTheStandardParser(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for _, entry := range Entries() {
		if _, err := parser.Parse(entry.Spec); err != nil {
			t.Errorf("entry %q spec %q does not parse: %v", entry.Name, entry.Spec, err)
		}
	}
}

// Transcribed from app/Console/Kernel.php; a drift here silently changes when
// work runs.
func TestEntriesMatchTheLaravelKernel(t *testing.T) {
	expected := map[string]struct {
		spec    string
		lockTTL time.Duration
	}{
		"post-trend-all":                     {"15 * * * *", 120 * time.Minute},
		"post-trend-month":                   {"25 * * * *", 120 * time.Minute},
		"post-trend-week":                    {"35 * * * *", 120 * time.Minute},
		"post-trend-day":                     {"45 * * * *", 120 * time.Minute},
		"update-public-posts":                {"* * * * *", 60 * time.Minute},
		"make-thumbnails":                    {"0 * * * *", 120 * time.Minute},
		"remove-unused-images":               {"0 * * * *", 60 * time.Minute},
		"generate-sitemap":                   {"20 5 * * *", 120 * time.Minute},
		"make-rank-report-history":           {"15 6 * * *", 120 * time.Minute},
		"remove-outdate-rank-report-history": {"30 5 * * *", 120 * time.Minute},
	}

	entries := Entries()
	if len(entries) != len(expected) {
		t.Fatalf("got %d entries, want %d", len(entries), len(expected))
	}
	for _, entry := range entries {
		want, ok := expected[entry.Name]
		if !ok {
			t.Errorf("unexpected entry %q", entry.Name)
			continue
		}
		if entry.Spec != want.spec {
			t.Errorf("%s spec = %q, want %q", entry.Name, entry.Spec, want.spec)
		}
		if entry.LockTTL != want.lockTTL {
			t.Errorf("%s lock ttl = %s, want %s", entry.Name, entry.LockTTL, want.lockTTL)
		}
	}
}

// The Kernel runs `make:post-trend day`, and CreatePostTrend's switch maps "day"
// to createTodayPostTrends. The wire value must stay "day", not the
// TrendTimeRange constant "today".
func TestPostTrendRangeUsesTheCommandArgumentValue(t *testing.T) {
	entry := entryByName(t, "post-trend-day")

	var payload map[string]string
	if err := json.Unmarshal(entry.Message.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload["range"] != "day" {
		t.Fatalf("range = %q, want \"day\"", payload["range"])
	}
}

// cachePosts is deliberately not ported; see the note in schedule.go. This asserts
// the absence, so re-adding it silently is not possible: a warm-up handler that
// nothing can warm would look harmless and quietly burn a worker slot every five
// minutes.
func TestCachePostsIsNotScheduled(t *testing.T) {
	for _, entry := range Entries() {
		if entry.Name == "cache-posts" || entry.Flag == "SCHEDULE_CACHE_POSTS" {
			t.Fatalf("cache-posts is scheduled again: %+v", entry)
		}
		if entry.Message.Type == "public_post.warm_cache" {
			t.Fatalf("a warm_cache message is scheduled: %+v", entry)
		}
	}
}

// The safety property for the whole cutover: an entry with no flag set must not
// run, because Laravel is still running it.
func TestSelectDisablesEverythingByDefault(t *testing.T) {
	enabled, disabled, err := Select(Entries(), func(string) string { return "" })
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(enabled) != 0 {
		t.Fatalf("enabled = %v, want none", Names(enabled))
	}
	if len(disabled) != len(Entries()) {
		t.Fatalf("disabled %d entries, want %d", len(disabled), len(Entries()))
	}
}

func TestSelectEnablesOnlyTheFlaggedEntry(t *testing.T) {
	enabled, disabled, err := Select(Entries(), func(flag string) string {
		if flag == "SCHEDULE_POST_TREND_ALL" {
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(enabled) != 1 || enabled[0].Name != "post-trend-all" {
		t.Fatalf("enabled = %v", Names(enabled))
	}
	if len(disabled) != len(Entries())-1 {
		t.Fatalf("disabled = %v", Names(disabled))
	}
}

func TestSelectAcceptsTheDocumentedFlagValues(t *testing.T) {
	for _, value := range []string{"true", "TRUE", "1", "on", " on "} {
		enabled, _, err := Select([]Entry{entryByName(t, "generate-sitemap")}, func(string) string { return value })
		if err != nil {
			t.Fatalf("Select(%q) error = %v", value, err)
		}
		if len(enabled) != 1 {
			t.Errorf("value %q should enable the entry", value)
		}
	}
	for _, value := range []string{"", "false", "0", "off"} {
		enabled, _, err := Select([]Entry{entryByName(t, "generate-sitemap")}, func(string) string { return value })
		if err != nil {
			t.Fatalf("Select(%q) error = %v", value, err)
		}
		if len(enabled) != 0 {
			t.Errorf("value %q should leave the entry disabled", value)
		}
	}
}

// A typo must be loud. Silently treating "ture" as off looks identical to a
// deliberate disable, and the operator would think the cutover had happened.
func TestSelectRejectsAnUnrecognisedFlagValue(t *testing.T) {
	if _, _, err := Select(Entries(), func(string) string { return "ture" }); err == nil {
		t.Fatal("Select() should reject an unrecognised flag value")
	}
}

func TestIdempotencyKeyIsStablePerTickAndDistinctAcrossTicks(t *testing.T) {
	firedAt := time.Date(2026, 8, 5, 1, 15, 0, 0, time.UTC)

	first := IdempotencyKey("post-trend-all", firedAt)
	if first != IdempotencyKey("post-trend-all", firedAt) {
		t.Fatal("the same tick must produce the same key")
	}
	if first == IdempotencyKey("post-trend-all", firedAt.Add(time.Hour)) {
		t.Fatal("a later tick must produce a different key")
	}
	if first == IdempotencyKey("post-trend-week", firedAt) {
		t.Fatal("a different entry must produce a different key")
	}

	// Ticks are compared in UTC so a timezone offset cannot make one tick look
	// like two.
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	if first != IdempotencyKey("post-trend-all", firedAt.In(taipei)) {
		t.Fatal("the same instant in another zone must produce the same key")
	}
}

func sampleEntryMessage() queue.Message {
	return queue.Message{Queue: "default", Type: "test.entry"}
}
