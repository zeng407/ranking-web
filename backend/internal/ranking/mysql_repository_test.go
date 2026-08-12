package ranking

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

// testDatabase connects only when MYSQL_TEST_HOST is set. The release image runs
// `go test ./...` during the build with no database, and CI must not be pointed
// at a real one by accident, so this skips rather than fails.
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
		Host: host,
		Port: port,
		// The same default as every other package's integration test. It used to be
		// "rk-db", whose rank_reports has no hidden column, so these tests failed with
		// "Unknown column 'rr.hidden'" for anyone who ran go test ./... without setting
		// MYSQL_TEST_DATABASE — a failure about the environment, not the code.
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

// fixtureIDs names a post and element that have round data. It is overridable
// because the local database carries only a small slice of the game tables.
func fixtureIDs(t *testing.T) (postID, elementID int64) {
	t.Helper()
	post, err := strconv.ParseInt(envOr("MYSQL_TEST_POST_ID", "98"), 10, 64)
	if err != nil {
		t.Fatalf("MYSQL_TEST_POST_ID: %v", err)
	}
	element, err := strconv.ParseInt(envOr("MYSQL_TEST_ELEMENT_ID", "9715"), 10, 64)
	if err != nil {
		t.Fatalf("MYSQL_TEST_ELEMENT_ID: %v", err)
	}
	return post, element
}

// The four delta queries are checked against an independently written
// aggregation over the same rows, so a mistake in the join or the completed_at
// filter shows up as a mismatch rather than a plausible wrong number.
func TestDeltaQueriesMatchAnIndependentAggregation(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	var wantAllWin, wantAllLose, wantCompletedWin, wantCompletedLose, wantChampion int64
	err := database.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM game_1v1_rounds r
		    WHERE r.winner_id = ? AND r.game_id IN (SELECT id FROM games WHERE post_id = ?)),
		  (SELECT COUNT(*) FROM game_1v1_rounds r
		    WHERE r.loser_id = ? AND r.game_id IN (SELECT id FROM games WHERE post_id = ?)),
		  (SELECT COUNT(*) FROM game_1v1_rounds r
		    WHERE r.winner_id = ? AND r.game_id IN (SELECT id FROM games WHERE post_id = ? AND completed_at IS NOT NULL)),
		  (SELECT COUNT(*) FROM game_1v1_rounds r
		    WHERE r.loser_id = ? AND r.game_id IN (SELECT id FROM games WHERE post_id = ? AND completed_at IS NOT NULL)),
		  (SELECT COUNT(*) FROM game_1v1_rounds r
		    WHERE r.winner_id = ? AND r.remain_elements = 1
		      AND r.game_id IN (SELECT id FROM games WHERE post_id = ? AND completed_at IS NOT NULL))`,
		elementID, postID, elementID, postID, elementID, postID, elementID, postID, elementID, postID,
	).Scan(&wantAllWin, &wantAllLose, &wantCompletedWin, &wantCompletedLose, &wantChampion)
	if err != nil {
		t.Fatalf("independent aggregation: %v", err)
	}

	completedWin, err := repository.CompletedWinDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("CompletedWinDelta() error = %v", err)
	}
	if completedWin.Count != wantCompletedWin || completedWin.ChampionCount != wantChampion {
		t.Errorf("CompletedWinDelta() = %#v, want count %d champion %d",
			completedWin, wantCompletedWin, wantChampion)
	}

	completedLose, err := repository.CompletedLoseDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("CompletedLoseDelta() error = %v", err)
	}
	if completedLose.Count != wantCompletedLose {
		t.Errorf("CompletedLoseDelta() count = %d, want %d", completedLose.Count, wantCompletedLose)
	}

	allWin, err := repository.AllGamesWinDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("AllGamesWinDelta() error = %v", err)
	}
	if allWin.Count != wantAllWin {
		t.Errorf("AllGamesWinDelta() count = %d, want %d", allWin.Count, wantAllWin)
	}

	allLose, err := repository.AllGamesLoseDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("AllGamesLoseDelta() error = %v", err)
	}
	if allLose.Count != wantAllLose {
		t.Errorf("AllGamesLoseDelta() count = %d, want %d", allLose.Count, wantAllLose)
	}
}

// The watermark must actually restrict the scan, otherwise every run would
// re-aggregate the whole table and the memo would be pointless.
func TestDeltaQueriesRespectTheWatermark(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	full, err := repository.AllGamesWinDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("AllGamesWinDelta() error = %v", err)
	}
	if full.Count == 0 {
		t.Skipf("post %d element %d has no rounds in this database", postID, elementID)
	}

	// Above the highest id, nothing is new.
	empty, err := repository.AllGamesWinDelta(ctx, postID, elementID, full.MaxID)
	if err != nil {
		t.Fatalf("AllGamesWinDelta() error = %v", err)
	}
	if empty.Count != 0 {
		t.Fatalf("count above the watermark = %d, want 0", empty.Count)
	}
	if empty.MaxID != 0 {
		t.Fatalf("MaxID with no matches = %d, want 0 rather than NULL", empty.MaxID)
	}
}

// UpsertRank must be idempotent by natural key. This is what
// ranks_post_element_type_date_unique buys: the SELECT-then-write it replaces had
// already produced duplicate rows.
func TestUpsertRankIsIdempotentByNaturalKey(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	// A far-future record_date so this cannot collide with real rows.
	recordDate := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	cleanup := func() {
		_, _ = database.ExecContext(ctx,
			`DELETE FROM ranks WHERE post_id = ? AND element_id = ? AND record_date = ?`,
			postID, elementID, recordDate.Format("2006-01-02"))
	}
	cleanup()
	t.Cleanup(cleanup)

	rank := Rank{
		PostID: postID, ElementID: elementID, RankType: RankTypePKKing,
		RecordDate: recordDate, WinCount: 4, RoundCount: 4, WinRate: 100,
	}
	if err := repository.UpsertRank(ctx, rank); err != nil {
		t.Fatalf("first UpsertRank() error = %v", err)
	}

	// Same key, different values: must update in place, not insert a second row.
	rank.WinCount = 7
	rank.RoundCount = 10
	rank.WinRate = 70
	if err := repository.UpsertRank(ctx, rank); err != nil {
		t.Fatalf("second UpsertRank() error = %v", err)
	}

	var rows, winCount, roundCount int64
	var winRate float64
	err := database.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(win_count),0), COALESCE(MAX(round_count),0), COALESCE(MAX(win_rate),0)
		   FROM ranks WHERE post_id = ? AND element_id = ? AND rank_type = ? AND record_date = ?`,
		postID, elementID, string(RankTypePKKing), recordDate.Format("2006-01-02"),
	).Scan(&rows, &winCount, &roundCount, &winRate)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows = %d, want exactly 1: the unique key must collapse the upsert", rows)
	}
	if winCount != 7 || roundCount != 10 || winRate != 70 {
		t.Fatalf("row = (%d, %d, %v), want (7, 10, 70)", winCount, roundCount, winRate)
	}
}

func TestUpsertRankRejectsUnknownRankType(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)

	err := repository.UpsertRank(context.Background(), Rank{
		PostID: 1, ElementID: 1, RankType: RankType("king"), RecordDate: time.Now(),
	})
	if err == nil {
		t.Fatal("UpsertRank() should reject an unknown rank type before touching the database")
	}
}

// End to end against the real schema: the service reads the deltas, writes both
// rank rows, and a second run with a warm memo produces identical values.
func TestServiceWritesExpectedRanksAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	allWin, err := repository.AllGamesWinDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("AllGamesWinDelta() error = %v", err)
	}
	if allWin.Count == 0 {
		t.Skipf("post %d element %d has no rounds in this database", postID, elementID)
	}

	recordDate := time.Date(2099, 12, 30, 0, 0, 0, 0, time.UTC)
	cleanup := func() {
		_, _ = database.ExecContext(ctx,
			`DELETE FROM ranks WHERE post_id = ? AND element_id = ? AND record_date = ?`,
			postID, elementID, recordDate.Format("2006-01-02"))
	}
	cleanup()
	t.Cleanup(cleanup)

	stats := &fakeStats{}
	service, err := NewService(Options{
		Repository: repository,
		Stats:      stats,
		Location:   taipei(t),
		Now:        func() time.Time { return recordDate },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.UpdateElementRank(ctx, postID, elementID); err != nil {
		t.Fatalf("UpdateElementRank() error = %v", err)
	}

	type row struct {
		winCount   int64
		roundCount int64
		winRate    float64
	}
	read := func() map[string]row {
		t.Helper()
		rows, err := database.QueryContext(ctx,
			`SELECT rank_type, win_count, round_count, win_rate FROM ranks
			  WHERE post_id = ? AND element_id = ? AND record_date = ?`,
			postID, elementID, recordDate.Format("2006-01-02"))
		if err != nil {
			t.Fatalf("read ranks: %v", err)
		}
		defer rows.Close()
		out := make(map[string]row)
		for rows.Next() {
			var rankType string
			var value row
			if err := rows.Scan(&rankType, &value.winCount, &value.roundCount, &value.winRate); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out[rankType] = value
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("rows: %v", err)
		}
		return out
	}

	first := read()
	pk, ok := first[string(RankTypePKKing)]
	if !ok {
		t.Fatalf("no pk_king row was written, got %#v", first)
	}
	allLose, err := repository.AllGamesLoseDelta(ctx, postID, elementID, 0)
	if err != nil {
		t.Fatalf("AllGamesLoseDelta() error = %v", err)
	}
	wantRounds := allWin.Count + allLose.Count
	if pk.winCount != allWin.Count || pk.roundCount != wantRounds {
		t.Fatalf("pk_king = %#v, want wins %d over %d rounds", pk, allWin.Count, wantRounds)
	}
	if pk.winRate != WinRate(allWin.Count, wantRounds) {
		t.Fatalf("pk_king win rate = %v, want %v", pk.winRate, WinRate(allWin.Count, wantRounds))
	}

	// Second run, warm memo: no new rounds, so the absolute values must be
	// rewritten unchanged and no extra row created.
	if err := service.UpdateElementRank(ctx, postID, elementID); err != nil {
		t.Fatalf("second UpdateElementRank() error = %v", err)
	}
	second := read()
	if len(second) != len(first) {
		t.Fatalf("row count changed on the second run: %d then %d", len(first), len(second))
	}
	if second[string(RankTypePKKing)] != pk {
		t.Fatalf("pk_king drifted on the second run: %#v then %#v", pk, second[string(RankTypePKKing)])
	}
}
