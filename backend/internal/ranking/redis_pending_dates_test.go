package ranking

import (
	"context"
	"testing"
	"time"
)

func newTestPendingDates(t *testing.T) *RedisPendingDates {
	t.Helper()
	client := testRedis(t)
	store, err := NewRedisPendingDates(client, "2pick:test:pending:")
	if err != nil {
		t.Fatalf("NewRedisPendingDates() error = %v", err)
	}
	cleanup := func() {
		keys, _ := client.Keys(context.Background(), "2pick:test:pending:*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		client.Close()
	})
	return store
}

func TestNewRedisPendingDatesRequiresClient(t *testing.T) {
	if _, err := NewRedisPendingDates(nil, ""); err == nil {
		t.Fatal("NewRedisPendingDates(nil) should fail")
	}
}

func TestPendingDatesAddThenPull(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	if err := store.Add(ctx, 46, HistoryRangeAll, []string{"2026-07-05", "2026-07-06"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	dates, err := store.Pull(ctx, 46, HistoryRangeAll)
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if len(dates) != 2 {
		t.Fatalf("Pull() = %#v, want two dates", dates)
	}

	// Pull clears, so a second pull is empty.
	again, err := store.Pull(ctx, 46, HistoryRangeAll)
	if err != nil {
		t.Fatalf("second Pull() error = %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second Pull() = %#v, want empty", again)
	}
}

// A date added twice must appear once. The PHP version relies on array_unique
// after a read-modify-write; a SET gives this atomically.
func TestPendingDatesDeduplicates(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	for run := 0; run < 3; run++ {
		if err := store.Add(ctx, 46, HistoryRangeAll, []string{"2026-07-05"}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}
	dates, err := store.Pull(ctx, 46, HistoryRangeAll)
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-07-05" {
		t.Fatalf("Pull() = %#v, want one entry", dates)
	}
}

// Concurrent producers must not lose each other's dates. This is the case the
// PHP pull-then-put loses.
func TestPendingDatesSurvivesConcurrentAdds(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	done := make(chan error, 10)
	for index := 0; index < 10; index++ {
		date := time.Date(2026, 7, index+1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
		go func(date string) {
			done <- store.Add(ctx, 46, HistoryRangeAll, []string{date})
		}(date)
	}
	for index := 0; index < 10; index++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent Add() error = %v", err)
		}
	}

	dates, err := store.Pull(ctx, 46, HistoryRangeAll)
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if len(dates) != 10 {
		t.Fatalf("Pull() returned %d dates, want all 10", len(dates))
	}
}

func TestPendingDatesAreScopedPerPostAndRange(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	if err := store.Add(ctx, 46, HistoryRangeAll, []string{"2026-07-05"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.Add(ctx, 46, HistoryRangeThousandVotes, []string{"2026-07-06"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.Add(ctx, 99, HistoryRangeAll, []string{"2026-07-07"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	all, _ := store.Pull(ctx, 46, HistoryRangeAll)
	thousand, _ := store.Pull(ctx, 46, HistoryRangeThousandVotes)
	other, _ := store.Pull(ctx, 99, HistoryRangeAll)

	if len(all) != 1 || all[0] != "2026-07-05" {
		t.Fatalf("all = %#v", all)
	}
	if len(thousand) != 1 || thousand[0] != "2026-07-06" {
		t.Fatalf("thousand_votes = %#v", thousand)
	}
	if len(other) != 1 || other[0] != "2026-07-07" {
		t.Fatalf("post 99 = %#v", other)
	}
}

func TestPendingDatesSetsATTL(t *testing.T) {
	store := newTestPendingDates(t)
	client := testRedis(t)
	defer client.Close()
	ctx := context.Background()

	if err := store.Add(ctx, 46, HistoryRangeAll, []string{"2026-07-05"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	ttl, err := client.TTL(ctx, "2pick:test:pending:46:all").Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	// Without a TTL a post that stops producing would leak its set forever.
	if ttl <= 29*24*time.Hour || ttl > PendingDatesTTL {
		t.Fatalf("TTL = %s, want just under %s", ttl, PendingDatesTTL)
	}
}

func TestPendingDatesIgnoresEmptyInput(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	if err := store.Add(ctx, 46, HistoryRangeAll, nil); err != nil {
		t.Fatalf("Add(nil) error = %v", err)
	}
	if err := store.Add(ctx, 46, HistoryRangeAll, []string{"", ""}); err != nil {
		t.Fatalf("Add(blanks) error = %v", err)
	}
	dates, _ := store.Pull(ctx, 46, HistoryRangeAll)
	if len(dates) != 0 {
		t.Fatalf("Pull() = %#v, want empty", dates)
	}
}

func TestPendingDatesRejectsUnknownRange(t *testing.T) {
	store := newTestPendingDates(t)
	ctx := context.Background()

	if err := store.Add(ctx, 46, HistoryTimeRange("week"), []string{"2026-07-05"}); err == nil {
		t.Fatal("Add() should reject a range with no build path")
	}
	if _, err := store.Pull(ctx, 46, HistoryTimeRange("week")); err == nil {
		t.Fatal("Pull() should reject a range with no build path")
	}
}
