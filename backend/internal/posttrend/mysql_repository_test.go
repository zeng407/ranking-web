package posttrend

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MYSQL_TEST_HOST is not set; skipping MySQL integration test")
	}
	port, err := strconv.Atoi(envOr("MYSQL_TEST_PORT", "3306"))
	if err != nil {
		t.Fatalf("MYSQL_TEST_PORT is not a number: %v", err)
	}

	database, err := mysqlstore.Open(config.DatabaseConfig{
		Host:            host,
		Port:            port,
		Name:            envOr("MYSQL_TEST_DATABASE", "rk_db_restore_20260729"),
		User:            envOr("MYSQL_TEST_USERNAME", "root"),
		Password:        os.Getenv("MYSQL_TEST_PASSWORD"),
		MaxOpenConns:    4,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	}, mysqlstore.WithStatementTimeouts(2*time.Minute, 2*time.Minute))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("database unreachable: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// syntheticRange gives a test its own time_range namespace.
//
// The real ranges are live data on a copy of production, and the reset statement
// rewrites every position in its group, so writing to "today" would destroy the
// rankings this database is also used to verify. A range no schedule uses is
// isolated by construction, and the cleanup removes it.
func syntheticRange(t *testing.T, database *sql.DB) TimeRange {
	t.Helper()
	value := TimeRange(fmt.Sprintf("probe-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := database.ExecContext(ctx,
			"DELETE FROM post_trends WHERE time_range = ?", string(value)); err != nil {
			t.Errorf("clean up post_trends for %q: %v", value, err)
		}
		if _, err := database.ExecContext(ctx,
			"DELETE FROM post_statistics WHERE time_range = ?", string(value)); err != nil {
			t.Errorf("clean up post_statistics for %q: %v", value, err)
		}
	})
	return value
}

// The queries must run against the real schema. Both statements go through columns
// the migrations only just added, and the reset matches NULL with <=>, which is easy
// to write as `=` and then silently match nothing.
func TestRepositoryQueriesRunAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	probe := syntheticRange(t, database)

	window := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	for _, start := range []*time.Time{&window, nil} {
		if _, err := repository.ResetPositions(ctx, probe, start); err != nil {
			t.Fatalf("ResetPositions(window=%v) error = %v", start, err)
		}
		if _, err := repository.RankedPosts(ctx, probe, start, 10); err != nil {
			t.Fatalf("RankedPosts(window=%v) error = %v", start, err)
		}
	}
}

// The counting query must agree with an independent count written the other way
// round. This is the load-bearing check on the LEFT JOIN and on the vote filter
// living in the join condition rather than the WHERE clause.
func TestPlayCountsMatchAnIndependentCount(t *testing.T) {
	database := testDatabase(t)
	// Held because this counts every live post, and other packages' fixtures create
	// them. See sharedlock_test.go.
	lockSharedPosts(t, database)
	repository := NewMySQLRepository(database)
	ctx := context.Background()

	// A window with real activity in it.
	window := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	counts, err := repository.PlayCounts(ctx, RangeMonth, &window)
	if err != nil {
		t.Fatalf("PlayCounts() error = %v", err)
	}
	if len(counts) == 0 {
		t.Skip("no live posts in this database")
	}

	// Every live post must appear, whether or not it was played.
	var livePosts int
	if err := database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL").Scan(&livePosts); err != nil {
		t.Fatalf("count live posts: %v", err)
	}
	if len(counts) != livePosts {
		t.Fatalf("counted %d posts, want every live post (%d): a post with no games must still get a row",
			len(counts), livePosts)
	}

	byPost := make(map[int64]int64, len(counts))
	zeroes := 0
	total := int64(0)
	for _, entry := range counts {
		byPost[entry.PostID] = entry.Count
		total += entry.Count
		if entry.Count == 0 {
			zeroes++
		}
		if got := entry.StartDate.Format("2006-01-02"); got != "2026-07-01" {
			t.Fatalf("post %d keyed at %s, want the window start", entry.PostID, got)
		}
	}
	if zeroes == 0 {
		t.Error("no post had a zero count; the LEFT JOIN may have collapsed to an inner join")
	}

	// The same total, computed from games rather than from posts.
	var independent int64
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM games g
		  JOIN posts p ON p.id = g.post_id AND p.deleted_at IS NULL
		 WHERE g.vote_count >= ? AND g.created_at >= ?`,
		MinimumVoteCount, window.Format("2006-01-02")).Scan(&independent)
	if err != nil {
		t.Fatalf("independent count: %v", err)
	}
	if total != independent {
		t.Fatalf("play counts sum to %d, independent count is %d", total, independent)
	}

	// Spot-check one busy post on its own.
	var busiest int64
	var busiestCount int64
	for postID, count := range byPost {
		if count > busiestCount {
			busiest, busiestCount = postID, count
		}
	}
	if busiestCount > 0 {
		var single int64
		err = database.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM games WHERE post_id = ? AND vote_count >= ? AND created_at >= ?",
			busiest, MinimumVoteCount, window.Format("2006-01-02")).Scan(&single)
		if err != nil {
			t.Fatalf("single post count: %v", err)
		}
		if single != busiestCount {
			t.Fatalf("post %d: counted %d, want %d", busiest, busiestCount, single)
		}
		t.Logf("%d live posts, %d plays in the window, busiest post %d with %d",
			len(counts), total, busiest, busiestCount)
	}
}

// The all-time range keys each row by the post's own creation date, which is what
// `$startDate ?: $post->created_at->toDateString()` evaluates to. Getting this wrong
// would pile every post onto one date and collide on the unique index.
func TestPlayCountsForTheAllRangeKeyByEachPostsCreationDate(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()

	counts, err := repository.PlayCounts(ctx, RangeAll, nil)
	if err != nil {
		t.Fatalf("PlayCounts() error = %v", err)
	}
	if len(counts) < 2 {
		t.Skip("need at least two live posts")
	}

	distinctDates := make(map[string]struct{}, len(counts))
	for _, entry := range counts {
		distinctDates[entry.StartDate.Format("2006-01-02")] = struct{}{}
		if entry.StartDate.IsZero() {
			t.Fatalf("post %d has a zero start date", entry.PostID)
		}
	}
	if len(distinctDates) < 2 {
		t.Fatalf("every post keyed to the same date (%v); the all-time range must use each post's own creation date",
			distinctDates)
	}

	// Confirm one row against the post row it came from.
	sample := counts[0]
	var createdAt time.Time
	if err := database.QueryRowContext(ctx,
		"SELECT created_at FROM posts WHERE id = ?", sample.PostID).Scan(&createdAt); err != nil {
		t.Fatalf("read post %d: %v", sample.PostID, err)
	}
	if got, want := sample.StartDate.Format("2006-01-02"), createdAt.Format("2006-01-02"); got != want {
		t.Fatalf("post %d keyed at %s, want its creation date %s", sample.PostID, got, want)
	}
	t.Logf("%d posts across %d distinct creation dates", len(counts), len(distinctDates))
}

// The whole cycle on a range of its own: write counts, reset, rank, then check the
// positions are a dense 1..N in play-count order, and that a second run is stable.
func TestUpsertResetAndRankCycle(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	probe := syntheticRange(t, database)

	postIDs := livePostIDs(t, database, 5)
	window := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)

	// Descending counts, so the expected order is exactly the input order.
	counts := make([]PlayCount, 0, len(postIDs))
	for index, postID := range postIDs {
		counts = append(counts, PlayCount{
			PostID:    postID,
			StartDate: window,
			Count:     int64((len(postIDs) - index) * 10),
		})
	}
	if _, err := repository.UpsertPlayCounts(ctx, probe, counts); err != nil {
		t.Fatalf("UpsertPlayCounts() error = %v", err)
	}

	ranked, err := repository.RankedPosts(ctx, probe, &window, RankedLimit)
	if err != nil {
		t.Fatalf("RankedPosts() error = %v", err)
	}
	if len(ranked) != len(postIDs) {
		t.Fatalf("ranked %d posts, want %d", len(ranked), len(postIDs))
	}
	for index, postID := range postIDs {
		if ranked[index] != postID {
			t.Fatalf("rank %d = post %d, want %d (ordering must be play_count desc)",
				index+1, ranked[index], postID)
		}
	}

	positions := make([]TrendPosition, 0, len(ranked))
	for index, postID := range ranked {
		positions = append(positions, TrendPosition{PostID: postID, Position: index + 1})
	}
	if _, err := repository.UpsertPositions(ctx, probe, &window, positions); err != nil {
		t.Fatalf("UpsertPositions() error = %v", err)
	}

	stored := storedPositions(t, database, probe, &window)
	for _, entry := range positions {
		if stored[entry.PostID] != entry.Position {
			t.Errorf("post %d stored at %d, want %d", entry.PostID, stored[entry.PostID], entry.Position)
		}
	}

	// A second identical run must not create duplicates. Before migration 00005 this
	// was the race that produced 36 duplicate groups.
	if _, err := repository.UpsertPositions(ctx, probe, &window, positions); err != nil {
		t.Fatalf("second UpsertPositions() error = %v", err)
	}
	if again := storedPositions(t, database, probe, &window); len(again) != len(stored) {
		t.Fatalf("row count changed on the second run: %d then %d", len(stored), len(again))
	}

	// The reset must reach every row in the group.
	reset, err := repository.ResetPositions(ctx, probe, &window)
	if err != nil {
		t.Fatalf("ResetPositions() error = %v", err)
	}
	if reset != int64(len(positions)) {
		t.Errorf("reset %d rows, want %d", reset, len(positions))
	}
	for postID, position := range storedPositions(t, database, probe, &window) {
		if position != UnrankedPosition {
			t.Errorf("post %d is still at %d after the reset", postID, position)
		}
	}
}

// The all-time range stores start_date NULL. The reset matches it with <=>; a plain
// `=` would match nothing and leave every stale position in place, which is the
// failure the generated column in migration 00005 was added to make impossible to
// miss.
func TestResetAndUpsertHandleTheNullWindow(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	probe := syntheticRange(t, database)

	postIDs := livePostIDs(t, database, 3)
	positions := make([]TrendPosition, 0, len(postIDs))
	for index, postID := range postIDs {
		positions = append(positions, TrendPosition{PostID: postID, Position: index + 1})
	}
	if _, err := repository.UpsertPositions(ctx, probe, nil, positions); err != nil {
		t.Fatalf("UpsertPositions(nil) error = %v", err)
	}

	stored := storedPositions(t, database, probe, nil)
	if len(stored) != len(positions) {
		t.Fatalf("stored %d rows, want %d", len(stored), len(positions))
	}

	// Idempotent with a NULL window too: this is exactly the group a plain unique
	// index would not have protected.
	if _, err := repository.UpsertPositions(ctx, probe, nil, positions); err != nil {
		t.Fatalf("second UpsertPositions(nil) error = %v", err)
	}
	if again := storedPositions(t, database, probe, nil); len(again) != len(stored) {
		t.Fatalf("NULL-window rows duplicated: %d then %d", len(stored), len(again))
	}

	reset, err := repository.ResetPositions(ctx, probe, nil)
	if err != nil {
		t.Fatalf("ResetPositions(nil) error = %v", err)
	}
	if reset != int64(len(positions)) {
		t.Fatalf("reset %d rows, want %d: <=> is required to match a NULL start_date", reset, len(positions))
	}
}

func livePostIDs(t *testing.T, database *sql.DB, limit int) []int64 {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		"SELECT id FROM posts WHERE deleted_at IS NULL ORDER BY id LIMIT ?", limit)
	if err != nil {
		t.Fatalf("read posts: %v", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan post id: %v", err)
		}
		ids = append(ids, id)
	}
	if len(ids) < limit {
		t.Skipf("need %d live posts, found %d", limit, len(ids))
	}
	return ids
}

func storedPositions(t *testing.T, database *sql.DB, rangeValue TimeRange, window *time.Time) map[int64]int {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		"SELECT post_id, position FROM post_trends WHERE trend_type = ? AND time_range = ? AND start_date <=> ?",
		TrendTypeHot, string(rangeValue), nullableDate(window))
	if err != nil {
		t.Fatalf("read positions: %v", err)
	}
	defer rows.Close()

	stored := make(map[int64]int)
	for rows.Next() {
		var (
			postID   int64
			position int
		)
		if err := rows.Scan(&postID, &position); err != nil {
			t.Fatalf("scan position: %v", err)
		}
		stored[postID] = position
	}
	return stored
}
