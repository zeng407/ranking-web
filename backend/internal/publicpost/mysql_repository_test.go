package publicpost

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
	"2pick.app/backend/internal/postaccess"
	"2pick.app/backend/internal/publiccontent"
	"2pick.app/backend/internal/queue"
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
	}, mysqlstore.WithStatementTimeouts(5*time.Minute, 5*time.Minute))
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

// snapshotPublicPosts saves and restores the whole table.
//
// A refresh rewrites every row, and this suite runs against a copy of production, so
// leaving the listing rebuilt would invalidate anything else that reads it. The table
// is small — around 2,200 rows — so a full snapshot is cheap.
func snapshotPublicPosts(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()

	type saved struct {
		postID                      int64
		newPos, dayPos              int
		weekPos, monthPos           int
		title, description, tagList string
		data                        []byte
		isDirty                     bool
	}

	rows, err := database.QueryContext(ctx,
		`SELECT post_id, new_position, day_position, week_position, month_position,
		        title, description, tags, data, is_dirty
		   FROM public_posts`)
	if err != nil {
		t.Fatalf("snapshot public_posts: %v", err)
	}
	var records []saved
	for rows.Next() {
		var record saved
		if err := rows.Scan(&record.postID, &record.newPos, &record.dayPos,
			&record.weekPos, &record.monthPos, &record.title, &record.description,
			&record.tagList, &record.data, &record.isDirty); err != nil {
			rows.Close()
			t.Fatalf("scan snapshot: %v", err)
		}
		records = append(records, record)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	t.Logf("snapshotted %d public_posts rows", len(records))

	t.Cleanup(func() {
		restoreCtx := context.Background()
		if _, err := database.ExecContext(restoreCtx, "DELETE FROM public_posts"); err != nil {
			t.Errorf("clear public_posts before restore: %v", err)
			return
		}
		for _, record := range records {
			_, err := database.ExecContext(restoreCtx,
				`INSERT INTO public_posts
				       (post_id, new_position, day_position, week_position, month_position,
				        title, description, tags, data, is_dirty, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
				record.postID, record.newPos, record.dayPos, record.weekPos, record.monthPos,
				record.title, record.description, record.tagList, record.data, record.isDirty)
			if err != nil {
				t.Errorf("restore post %d: %v", record.postID, err)
			}
		}
	})
}

// The queries must run against the real schema. Several go through columns and
// indexes the migrations only just added, and the joins carry the soft-delete filters
// Eloquent applied implicitly.
func TestRepositoryQueriesRunAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()

	listed, err := repository.ListedPostIDs(ctx)
	if err != nil {
		t.Fatalf("ListedPostIDs() error = %v", err)
	}
	if len(listed) == 0 {
		t.Skip("no listable posts in this database")
	}
	t.Logf("%d posts qualify for the listing", len(listed))

	// Ordered newest first, and distinct.
	seen := make(map[int64]struct{}, len(listed))
	for index, postID := range listed {
		if _, exists := seen[postID]; exists {
			t.Fatalf("post %d appears twice in the source list", postID)
		}
		seen[postID] = struct{}{}
		if index > 0 && listed[index-1] <= postID {
			t.Fatalf("source list is not descending at %d: %d then %d",
				index, listed[index-1], postID)
		}
	}

	for _, pass := range []Pass{PassToday, PassWeek, PassMonth} {
		window, err := TrendWindowStart(pass, time.Now())
		if err != nil {
			t.Fatalf("TrendWindowStart(%q) error = %v", pass, err)
		}
		if _, err := repository.TrendedPostIDs(ctx, pass.TrendRange(), window); err != nil {
			t.Fatalf("TrendedPostIDs(%q) error = %v", pass, err)
		}
	}

	if _, err := repository.PublicPostIDs(ctx); err != nil {
		t.Fatalf("PublicPostIDs() error = %v", err)
	}
}

// The batch cap removal is the point of this port, so the source list has to reach
// past where the PHP stopped.
func TestListedPostIDsIsNotCappedAtTheOldBatchSize(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)

	listed, err := repository.ListedPostIDs(context.Background())
	if err != nil {
		t.Fatalf("ListedPostIDs() error = %v", err)
	}

	// An independent count of the same set.
	var expected int
	err = database.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM (
			SELECT p.id
			  FROM posts AS p
			  JOIN post_policies AS pol ON pol.post_id = p.id AND pol.access_policy = 'public'
			  JOIN post_elements AS pe ON pe.post_id = p.id
			  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
			 WHERE p.deleted_at IS NULL
			 GROUP BY p.id
			HAVING COUNT(*) >= ?
		) AS qualifying`, MinimumElementCount).Scan(&expected)
	if err != nil {
		t.Fatalf("independent count: %v", err)
	}
	if len(listed) != expected {
		t.Fatalf("listed %d posts, independent count says %d", len(listed), expected)
	}
	t.Logf("the source list covers all %d qualifying posts (the PHP stopped at 2000)", expected)
}

// LoadChunk must fill every field the payload needs, from real rows.
func TestLoadChunkAssemblesRealPosts(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()

	listed, err := repository.ListedPostIDs(ctx)
	if err != nil {
		t.Fatalf("ListedPostIDs() error = %v", err)
	}
	if len(listed) < 5 {
		t.Skip("need at least five listable posts")
	}
	chunk := listed[:5]

	rows, err := repository.LoadChunk(ctx, chunk)
	if err != nil {
		t.Fatalf("LoadChunk() error = %v", err)
	}
	if len(rows) != len(chunk) {
		t.Fatalf("loaded %d rows for %d ids", len(rows), len(chunk))
	}

	for _, row := range rows {
		if row.Resource.Serial == "" {
			t.Errorf("post %d has no serial", row.PostID)
		}
		// The listing requires at least MinimumElementCount elements, so the count must
		// reflect that rather than being left at zero.
		if row.Resource.ElementsCount < MinimumElementCount {
			t.Errorf("post %d reports %d elements, want at least %d",
				row.PostID, row.Resource.ElementsCount, MinimumElementCount)
		}
		if row.Resource.CreatedAt == "" || row.Resource.UpdatedAt == "" {
			t.Errorf("post %d has no timestamps: %+v", row.PostID, row.Resource)
		}
		if _, err := time.Parse(DateTimeLayout, row.Resource.CreatedAt); err != nil {
			t.Errorf("post %d created_at %q is not in Carbon's format: %v",
				row.PostID, row.Resource.CreatedAt, err)
		}

		ranked, fallback := row.Candidates()
		if len(ranked) == 0 && len(fallback) == 0 {
			t.Errorf("post %d has no preview candidates at all", row.PostID)
		}
		if len(ranked) > PreviewCandidateLimit {
			t.Errorf("post %d returned %d ranked candidates, want at most %d",
				row.PostID, len(ranked), PreviewCandidateLimit)
		}
		if len(fallback) > FallbackCandidateLimit {
			t.Errorf("post %d returned %d fallback candidates, want at most %d",
				row.PostID, len(fallback), FallbackCandidateLimit)
		}
		// The fallback is only loaded for posts the ranked set cannot cover.
		if len(ranked) >= 2 && len(fallback) != 0 {
			t.Errorf("post %d loaded the fallback despite %d ranked candidates",
				row.PostID, len(ranked))
		}
	}
}

// THE CHECK THAT MATTERS MOST. The write side and the read side are separate packages
// working on the same json column, and nothing but this proves they agree: a renamed
// field or a changed type would leave the listing decoding into zero values, which no
// unit test on either side would catch.
func TestWrittenPayloadDecodesOnTheReadSide(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	snapshotPublicPosts(t, database)

	service, err := NewService(Options{
		Repository: repository,
		// The debounce is irrelevant here; the point is to force one full pass.
		Freshness: AlwaysStale{},
		Cache:     NoResourceCache{},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:  time.UTC,
		Shuffle:   noShuffle,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	started := time.Now()
	if err := service.handleRefresh(ctx, mustRefreshMessage(t)); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	t.Logf("full refresh took %s", time.Since(started).Round(time.Millisecond))

	// Read what was written through the read side's own decoder.
	reader := publiccontent.NewMySQLRepository(database, postaccess.AdultPolicy{})
	page, err := reader.Posts(ctx, publiccontent.PostsQuery{
		Sort: "new", Page: 1, PerPage: 15,
	})
	if err != nil {
		t.Fatalf("read back through publiccontent: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatal("the listing is empty after a full refresh")
	}

	for _, item := range page.Items {
		if item.Title == "" || item.Serial == "" {
			t.Fatalf("a listing item decoded with empty scalars: %+v", item)
		}
		if item.ElementsCount < MinimumElementCount {
			t.Errorf("item %q decoded elements_count %d", item.Serial, item.ElementsCount)
		}
		if item.CreatedAt == "" {
			t.Errorf("item %q decoded no created_at", item.Serial)
		}
	}
	t.Logf("%d of %d listing items decoded cleanly on the read side",
		len(page.Items), page.Total)
}

// A full refresh must leave every position column dense from 1 for the posts it wrote,
// and every other row on the sentinel.
//
// WHY THIS SEEDS post_trends. Three of the four passes read post_trends for the CURRENT
// window — today, this week, this month — and a restore of production stops at whatever day
// it was dumped. On this database the newest "today" rows are eight days old, so those three
// passes find nothing, log "no qualifying posts" and skip; their position columns then hold
// frozen values from the dump that nothing maintains, and asserting density over those is
// asserting a property of the dump rather than of the code. It passed for a while and then
// stopped, for reasons that had nothing to do with the refresh.
//
// Seeding the current window makes the passes actually run, which is the behaviour worth
// testing. The rows are removed afterwards.
func TestFullRefreshProducesDensePositions(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	snapshotPublicPosts(t, database)
	seedCurrentTrendWindows(t, database)

	service, err := NewService(Options{
		Repository: repository,
		Freshness:  AlwaysStale{},
		Cache:      NoResourceCache{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:   time.UTC,
		Shuffle:    noShuffle,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.handleRefresh(ctx, mustRefreshMessage(t)); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	for _, pass := range Ordered() {
		column, err := pass.PositionColumn()
		if err != nil {
			t.Fatalf("PositionColumn(%q) error = %v", pass, err)
		}

		rows, err := database.QueryContext(ctx,
			"SELECT "+column+" FROM public_posts WHERE "+column+" < ? ORDER BY "+column,
			UnlistedPosition)
		if err != nil {
			t.Fatalf("read %s: %v", column, err)
		}

		expected := 1
		for rows.Next() {
			var position int
			if err := rows.Scan(&position); err != nil {
				rows.Close()
				t.Fatalf("scan %s: %v", column, err)
			}
			if position != expected {
				rows.Close()
				t.Fatalf("%s is not dense: expected %d, got %d", column, expected, position)
			}
			expected++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("read %s: %v", column, err)
		}
		t.Logf("%s: %d listed positions, dense from 1", column, expected-1)
	}

	// WHAT is_dirty MEANS AFTER A REFRESH, AND WHY "NOTHING IS DIRTY" IS THE WRONG CHECK.
	//
	// Each pass marks every row dirty, clears the flag on the rows it upserts, then gives
	// the rest the sentinel — WITHOUT clearing their flag. That is deliberate: RemoveDirty
	// scopes on is_dirty = 1, so the leftovers are precisely the candidate set it evaluates
	// for "this post no longer qualifies". Clearing the flag in the sentinel push would
	// leave RemoveDirty with nothing to consider and stale listings would never be removed.
	//
	// So after a full refresh the flag reflects the LAST pass only, and the rows outside
	// that pass's trend are legitimately still dirty. This assertion used to demand zero,
	// and it held only because the trend passes were skipping for want of current
	// post_trends rows — with them seeded and running, it is simply false.
	//
	// What is guaranteed is that a row the last pass wrote is clean, and every row has a
	// position for every pass. The density loop above covers the second; this covers the
	// first.
	lastPass := Ordered()[len(Ordered())-1]
	column, err := lastPass.PositionColumn()
	if err != nil {
		t.Fatalf("PositionColumn(%q): %v", lastPass, err)
	}
	var dirtyButListed int
	if err := database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM public_posts WHERE is_dirty = 1 AND "+column+" < ?",
		UnlistedPosition).Scan(&dirtyButListed); err != nil {
		t.Fatalf("count dirty rows the last pass listed: %v", err)
	}
	if dirtyButListed != 0 {
		t.Errorf("%d rows carry a %s yet are still dirty; the pass that wrote them must clear the flag",
			dirtyButListed, column)
	}

	// And nothing is left without a position: a row missing from a pass has the sentinel,
	// never a value below 1.
	for _, pass := range Ordered() {
		positionColumn, err := pass.PositionColumn()
		if err != nil {
			t.Fatalf("PositionColumn(%q): %v", pass, err)
		}
		var unpositioned int
		if err := database.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM public_posts WHERE "+positionColumn+" < 1").Scan(&unpositioned); err != nil {
			t.Fatalf("count unpositioned rows for %s: %v", positionColumn, err)
		}
		if unpositioned != 0 {
			t.Errorf("%d rows have no %s", unpositioned, positionColumn)
		}
	}
}

// Running twice must not duplicate a listing. Before migration 00007 this was a
// SELECT-then-write and the race had already produced 3 duplicate rows.
func TestFullRefreshIsIdempotent(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	snapshotPublicPosts(t, database)

	service, err := NewService(Options{
		Repository: repository,
		Freshness:  AlwaysStale{},
		Cache:      NoResourceCache{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:   time.UTC,
		Shuffle:    noShuffle,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.handleRefresh(ctx, mustRefreshMessage(t)); err != nil {
		t.Fatalf("first handleRefresh() error = %v", err)
	}
	first := listingSnapshot(t, database)

	if err := service.handleRefresh(ctx, mustRefreshMessage(t)); err != nil {
		t.Fatalf("second handleRefresh() error = %v", err)
	}
	second := listingSnapshot(t, database)

	if len(first) != len(second) {
		t.Fatalf("row count changed between runs: %d then %d", len(first), len(second))
	}
	for postID, positions := range first {
		if second[postID] != positions {
			t.Fatalf("post %d moved between two identical runs: %+v then %+v",
				postID, positions, second[postID])
		}
	}

	var duplicates int
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT post_id FROM public_posts GROUP BY post_id HAVING COUNT(*) > 1
		) AS t`).Scan(&duplicates)
	if err != nil {
		t.Fatalf("count duplicates: %v", err)
	}
	if duplicates != 0 {
		t.Fatalf("%d posts are listed more than once", duplicates)
	}
	t.Logf("%d listings stable across two refreshes", len(first))
}

type listingPositions struct {
	newPos, dayPos, weekPos, monthPos int
}

func listingSnapshot(t *testing.T, database *sql.DB) map[int64]listingPositions {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		"SELECT post_id, new_position, day_position, week_position, month_position FROM public_posts")
	if err != nil {
		t.Fatalf("read listing: %v", err)
	}
	defer rows.Close()

	listing := make(map[int64]listingPositions)
	for rows.Next() {
		var (
			postID    int64
			positions listingPositions
		)
		if err := rows.Scan(&postID, &positions.newPos, &positions.dayPos,
			&positions.weekPos, &positions.monthPos); err != nil {
			t.Fatalf("scan listing: %v", err)
		}
		listing[postID] = positions
	}
	return listing
}

// The tags column and the payload must carry the same list, or the listing filters on
// one thing and renders another.
func TestStoredTagsMatchThePayload(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	ctx := context.Background()
	snapshotPublicPosts(t, database)

	service, err := NewService(Options{
		Repository: repository,
		Freshness:  AlwaysStale{},
		Cache:      NoResourceCache{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location:   time.UTC,
		Shuffle:    noShuffle,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.handleRefresh(ctx, mustRefreshMessage(t)); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}

	rows, err := database.QueryContext(ctx,
		"SELECT post_id, tags, data FROM public_posts WHERE data IS NOT NULL LIMIT 200")
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var (
			postID  int64
			tagList string
			data    []byte
		)
		if err := rows.Scan(&postID, &tagList, &data); err != nil {
			t.Fatalf("scan row: %v", err)
		}

		var columnTags []string
		if err := json.Unmarshal([]byte(tagList), &columnTags); err != nil {
			t.Fatalf("post %d tags column %q is not JSON: %v", postID, tagList, err)
		}
		var resource Resource
		if err := json.Unmarshal(data, &resource); err != nil {
			t.Fatalf("post %d data is not a Resource: %v", postID, err)
		}
		if len(columnTags) != len(resource.Tags) {
			t.Fatalf("post %d: tags column has %d entries, payload has %d",
				postID, len(columnTags), len(resource.Tags))
		}
		for index := range columnTags {
			if columnTags[index] != resource.Tags[index] {
				t.Fatalf("post %d tag %d: column %q, payload %q",
					postID, index, columnTags[index], resource.Tags[index])
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("checked no rows")
	}
	t.Logf("%d rows have matching tags in the column and the payload", checked)
}

func mustRefreshMessage(t *testing.T) queue.Message {
	t.Helper()
	built, err := RefreshMessage()
	if err != nil {
		t.Fatalf("RefreshMessage() error = %v", err)
	}
	return built
}

// seedCurrentTrendWindows gives the trend passes something to read for today's windows.
//
// Only a handful of posts, all of which qualify: public policy, not deleted, and at least
// MinimumElementCount live elements — the same conditions TrendedPostIDs applies. If the id
// query accepts a post that the detail query then drops, the position it consumed is left
// unwritten and the density assertion catches it, which is exactly the failure worth having
// a test for.
func seedCurrentTrendWindows(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()

	rows, err := database.QueryContext(ctx, `
		SELECT p.id
		  FROM posts AS p
		  JOIN post_policies AS pol ON pol.post_id = p.id AND pol.access_policy = ?
		  JOIN post_elements AS pe ON pe.post_id = p.id
		  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
		 WHERE p.deleted_at IS NULL
		 GROUP BY p.id
		HAVING COUNT(*) >= ?
		 ORDER BY p.id
		 LIMIT 12`, accessPolicyPublic, MinimumElementCount)
	if err != nil {
		t.Fatalf("find posts to seed the trend with: %v", err)
	}
	var postIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("scan seed post: %v", err)
		}
		postIDs = append(postIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("read seed posts: %v", err)
	}
	if len(postIDs) < 2 {
		t.Skip("need at least two qualifying posts to seed a trend window")
	}

	now := time.Now().UTC()
	windows := make(map[Pass]time.Time, 3)
	for _, pass := range Ordered() {
		if pass.TrendRange() == "" {
			continue
		}
		start, err := TrendWindowStart(pass, now)
		if err != nil {
			t.Fatalf("TrendWindowStart(%q): %v", pass, err)
		}
		windows[pass] = start
	}

	// Recorded so the cleanup can remove exactly what was added rather than guessing.
	type seeded struct {
		rangeValue string
		start      string
	}
	added := make([]seeded, 0, len(windows))

	for pass, start := range windows {
		rangeValue := pass.TrendRange()
		startDate := start.Format(dateLayout)
		for index, postID := range postIDs {
			if _, err := database.ExecContext(ctx, `
				INSERT INTO post_trends (post_id, trend_type, time_range, start_date, position, created_at, updated_at)
				VALUES (?, 'hot', ?, ?, ?, NOW(), NOW())
				ON DUPLICATE KEY UPDATE position = VALUES(position), updated_at = NOW()`,
				postID, rangeValue, startDate, index+1); err != nil {
				t.Fatalf("seed %s trend for post %d: %v", rangeValue, postID, err)
			}
		}
		added = append(added, seeded{rangeValue: rangeValue, start: startDate})
		t.Logf("seeded %d %s trend rows for %s", len(postIDs), rangeValue, startDate)
	}

	t.Cleanup(func() {
		for _, entry := range added {
			if _, err := database.ExecContext(context.Background(),
				`DELETE FROM post_trends WHERE trend_type = 'hot' AND time_range = ? AND start_date = ?`,
				entry.rangeValue, entry.start); err != nil {
				t.Errorf("clean up seeded %s trend rows: %v", entry.rangeValue, err)
			}
		}
	})
}
