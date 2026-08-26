package gameroom

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

// testDatabase connects only when MYSQL_TEST_HOST is set, matching the ranking
// package. The release image runs `go test ./...` with no database.
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

// ---------- parity against real rooms ----------

// snapshotRoom records the standings so the test can put them back. The recompute
// writes to the room, and this suite runs against a copy of the production database,
// so leaving it modified would quietly invalidate every later comparison.
func snapshotRoom(t *testing.T, database *sql.DB, roomID int64) {
	t.Helper()
	ctx := context.Background()

	type row struct {
		id                            int64
		score, combo, played, correct int
		accuracy                      string
		rank                          int
	}
	rows, err := database.QueryContext(ctx,
		"SELECT id, score, combo, total_played, total_correct, accuracy, `rank`"+
			" FROM game_room_users WHERE game_room_id = ?", roomID)
	if err != nil {
		t.Fatalf("snapshot room %d: %v", roomID, err)
	}
	var saved []row
	for rows.Next() {
		var entry row
		if err := rows.Scan(&entry.id, &entry.score, &entry.combo, &entry.played,
			&entry.correct, &entry.accuracy, &entry.rank); err != nil {
			rows.Close()
			t.Fatalf("scan snapshot: %v", err)
		}
		saved = append(saved, entry)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	t.Cleanup(func() {
		for _, entry := range saved {
			_, err := database.ExecContext(context.Background(),
				"UPDATE game_room_users SET score = ?, combo = ?, total_played = ?,"+
					" total_correct = ?, accuracy = ?, `rank` = ? WHERE id = ?",
				entry.score, entry.combo, entry.played, entry.correct,
				entry.accuracy, entry.rank, entry.id)
			if err != nil {
				t.Errorf("restore player %d: %v", entry.id, err)
			}
		}
	})
}

// parityRooms picks real rooms to check, largest first so the 1,088-player room is
// always included.
func parityRooms(t *testing.T, database *sql.DB, limit int) []int64 {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		`SELECT game_room_id FROM game_room_users
		  GROUP BY game_room_id ORDER BY COUNT(*) DESC LIMIT ?`, limit)
	if err != nil {
		t.Fatalf("pick parity rooms: %v", err)
	}
	defer rows.Close()

	var rooms []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan room id: %v", err)
		}
		rooms = append(rooms, id)
	}
	if len(rooms) == 0 {
		t.Skip("no game rooms with players in this database")
	}
	return rooms
}

// The set-based recompute must agree with Tally, the pure function that states the
// scoring rules. One is a single SQL statement over the whole room, the other a loop
// over one player's wagers; if they ever disagree, the SQL is wrong.
func TestRecomputeTotalsMatchesTheTally(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()

	for _, roomID := range parityRooms(t, database, 5) {
		snapshotRoom(t, database, roomID)

		if _, err := repository.RecomputeTotals(ctx, Room{ID: roomID, VoteMode: VoteModeHost}); err != nil {
			t.Fatalf("room %d: RecomputeTotals() error = %v", roomID, err)
		}

		stored, err := repository.StoredTotals(ctx, roomID)
		if err != nil {
			t.Fatalf("room %d: StoredTotals() error = %v", roomID, err)
		}
		wagers, err := repository.BetsByPlayer(ctx, roomID)
		if err != nil {
			t.Fatalf("room %d: BetsByPlayer() error = %v", roomID, err)
		}

		compared := 0
		for playerID, actual := range stored {
			want := Tally(wagers[playerID], DefaultScoring(), VoteModeHost)
			if actual != want {
				t.Fatalf("room %d player %d: stored %+v, Tally gives %+v", roomID, playerID, actual, want)
			}
			compared++
		}
		if compared == 0 {
			t.Fatalf("room %d: compared no players", roomID)
		}
		t.Logf("room %d: %d players agree with Tally", roomID, compared)
	}
}

// Running the recompute twice must land on the same numbers. This is what makes
// coalescing safe: a redelivered refresh, or one that absorbs several votes at once,
// cannot drift.
func TestRecomputeTotalsIsIdempotent(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()

	roomID := parityRooms(t, database, 1)[0]
	snapshotRoom(t, database, roomID)

	if _, err := repository.RecomputeTotals(ctx, Room{ID: roomID, VoteMode: VoteModeHost}); err != nil {
		t.Fatalf("first RecomputeTotals() error = %v", err)
	}
	first, err := repository.StoredTotals(ctx, roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	if _, err := repository.RecomputeTotals(ctx, Room{ID: roomID, VoteMode: VoteModeHost}); err != nil {
		t.Fatalf("second RecomputeTotals() error = %v", err)
	}
	second, err := repository.StoredTotals(ctx, roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("player count changed between runs: %d then %d", len(first), len(second))
	}
	for playerID, before := range first {
		if second[playerID] != before {
			t.Fatalf("player %d drifted: %+v then %+v", playerID, before, second[playerID])
		}
	}
	t.Logf("room %d: %d players stable across two recomputes", roomID, len(first))
}

// The rank statement must agree with AssignRanks, and the result must be a dense
// 1..N with no gaps or repeats.
func TestAssignRanksMatchesTheOracleAndStaysDense(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()

	for _, roomID := range parityRooms(t, database, 5) {
		snapshotRoom(t, database, roomID)

		if _, err := repository.AssignRanks(ctx, roomID); err != nil {
			t.Fatalf("room %d: AssignRanks() error = %v", roomID, err)
		}

		standings, err := repository.Standings(ctx, roomID)
		if err != nil {
			t.Fatalf("room %d: Standings() error = %v", roomID, err)
		}
		want := AssignRanks(standings)

		stored, err := storedRanks(ctx, database, roomID)
		if err != nil {
			t.Fatalf("room %d: read stored ranks: %v", roomID, err)
		}
		if len(stored) != len(want) {
			t.Fatalf("room %d: %d stored ranks for %d players", roomID, len(stored), len(want))
		}

		seen := make(map[int]bool, len(stored))
		for playerID, rank := range stored {
			if rank != want[playerID] {
				t.Fatalf("room %d player %d: stored rank %d, oracle says %d",
					roomID, playerID, rank, want[playerID])
			}
			if rank < 1 || rank > len(stored) {
				t.Fatalf("room %d player %d: rank %d outside 1..%d", roomID, playerID, rank, len(stored))
			}
			if seen[rank] {
				t.Fatalf("room %d: rank %d assigned twice", roomID, rank)
			}
			seen[rank] = true
		}
		t.Logf("room %d: %d ranks dense and matching the oracle", roomID, len(stored))
	}
}

func storedRanks(ctx context.Context, database *sql.DB, roomID int64) (map[int64]int, error) {
	rows, err := database.QueryContext(ctx,
		"SELECT id, `rank` FROM game_room_users WHERE game_room_id = ?", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ranks := make(map[int64]int)
	for rows.Next() {
		var (
			id   int64
			rank int
		)
		if err := rows.Scan(&id, &rank); err != nil {
			return nil, err
		}
		ranks[id] = rank
	}
	return ranks, rows.Err()
}

// ---------- end to end on a room built for the test ----------

// syntheticRoom builds a room with known wagers so the whole settle-then-refresh
// path can be checked against arithmetic rather than against existing data.
//
// The room hangs off a real game and real elements because of the foreign keys, and
// game_rooms cascades on delete, so removing the room removes its players and their
// wagers with it.
type syntheticRoom struct {
	roomID    int64
	serial    string
	playerIDs map[string]int64
	winnerID  int64
	loserID   int64
	// otherID is an element of the room that is not in the settled pairing.
	otherID int64
}

func newSyntheticRoom(t *testing.T, database *sql.DB) syntheticRoom {
	t.Helper()
	ctx := context.Background()

	var gameID int64
	if err := database.QueryRowContext(ctx, "SELECT id FROM games ORDER BY id LIMIT 1").Scan(&gameID); err != nil {
		t.Skipf("no games available to attach a test room to: %v", err)
	}

	elementRows, err := database.QueryContext(ctx,
		"SELECT id FROM elements WHERE deleted_at IS NULL ORDER BY id LIMIT 3")
	if err != nil {
		t.Fatalf("read elements: %v", err)
	}
	var elements []int64
	for elementRows.Next() {
		var id int64
		if err := elementRows.Scan(&id); err != nil {
			elementRows.Close()
			t.Fatalf("scan element: %v", err)
		}
		elements = append(elements, id)
	}
	elementRows.Close()
	if len(elements) < 3 {
		// Three, not two: a test needs an element outside the pairing to wager on a match
		// this round did not present.
		t.Skip("need at least three elements to build a test room")
	}

	serial := fmt.Sprintf("gr-test-%d", time.Now().UnixNano())
	result, err := database.ExecContext(ctx,
		"INSERT INTO game_rooms (game_id, serial, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
		gameID, serial)
	if err != nil {
		t.Fatalf("insert test room: %v", err)
	}
	roomID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("test room id: %v", err)
	}
	// CASCADE removes the players and their wagers.
	t.Cleanup(func() {
		if _, err := database.ExecContext(context.Background(),
			"DELETE FROM game_rooms WHERE id = ?", roomID); err != nil {
			t.Errorf("remove test room %d: %v", roomID, err)
		}
	})

	room := syntheticRoom{
		roomID:    roomID,
		serial:    serial,
		playerIDs: make(map[string]int64, 3),
		winnerID:  elements[0],
		loserID:   elements[1],
		otherID:   elements[2],
	}
	for _, name := range []string{"winner", "loser", "abstainer"} {
		room.seat(t, database, name)
	}
	return room
}

// seat adds one participant. Separate from newSyntheticRoom because a majority round is
// only interesting with more players than three.
func (room syntheticRoom) seat(t *testing.T, database *sql.DB, name string) int64 {
	t.Helper()
	result, err := database.ExecContext(context.Background(),
		`INSERT INTO game_room_users
		 (game_room_id, anonymous_id, nickname, score, `+"`rank`"+`, accuracy, combo,
		  total_played, total_correct, created_at, updated_at)
		 VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, NOW(), NOW())`,
		room.roomID, "anon-"+name+"-"+room.serial, name)
	if err != nil {
		t.Fatalf("insert player %s: %v", name, err)
	}
	playerID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("player id for %s: %v", name, err)
	}
	room.playerIDs[name] = playerID
	return playerID
}

// hostRoom is the fixture as the worker would have resolved it: the default rules.
func (room syntheticRoom) hostRoom() Room {
	return Room{ID: room.roomID, Serial: room.serial, VoteMode: VoteModeHost}
}

// majorityRoom switches the room to deciding its own rounds, in the database and in the
// value the worker would have read.
func (room syntheticRoom) majorityRoom(t *testing.T, database *sql.DB) Room {
	t.Helper()
	if _, err := database.ExecContext(context.Background(),
		"UPDATE game_rooms SET vote_mode = ? WHERE id = ?", VoteModeMajority, room.roomID); err != nil {
		t.Fatalf("switch room %d to majority: %v", room.roomID, err)
	}
	return Room{ID: room.roomID, Serial: room.serial, VoteMode: VoteModeMajority}
}

func (room syntheticRoom) placeBet(
	t *testing.T, database *sql.DB, player string, winnerID, loserID int64,
	currentRound, ofRound, remainElements, lastCombo int,
) {
	t.Helper()
	_, err := database.ExecContext(context.Background(),
		`INSERT INTO game_room_user_bets
		 (game_room_id, game_room_user_id, current_round, of_round, remain_elements,
		  winner_id, loser_id, last_combo, score, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, NOW(), NOW())`,
		room.roomID, room.playerIDs[player], currentRound, ofRound, remainElements,
		winnerID, loserID, lastCombo)
	if err != nil {
		t.Fatalf("place %s wager: %v", player, err)
	}
}

// The full path on known inputs: settle one round, recompute, rank, read the
// payload. Every expected number here is arithmetic from config/setting.php rather
// than a value copied out of the database.
func TestSettleThenRefreshProducesTheExpectedLeaderboard(t *testing.T) {
	database := testDatabase(t)
	scoring := DefaultScoring()
	repository := NewMySQLRepository(database, scoring)
	ctx := context.Background()

	room := newSyntheticRoom(t, database)

	// Round 3 of 4 with eight elements still in play. The vote leaves seven, so the
	// wagers were placed at remain_elements = 8.
	const (
		currentRound = 3
		ofRound      = 4
		placedAt     = 8
		afterVote    = 7
	)
	// last_combo is left at zero on both, because the recompute derives the streak from the
	// outcomes and ignores what the row carries. A first settled wager therefore rides a
	// streak of zero whatever was written when it was placed.
	room.placeBet(t, database, "winner", room.winnerID, room.loserID,
		currentRound, ofRound, placedAt, 0)
	room.placeBet(t, database, "loser", room.loserID, room.winnerID,
		currentRound, ofRound, placedAt, 0)
	// A wager on THIS round but on a pairing it did not present. That is the one a settle
	// discards: same round, no side to resolve it against.
	//
	// It used to be a wager on an older round (staleRemaining), which the wider discard
	// swept up as well. That scope had to go — settles are not ordered, so "older" could
	// mean "not settled yet" — and an unsettleable wager now simply lingers. It costs a row
	// and nothing else: the tally counts settled wagers only.
	room.placeBet(t, database, "abstainer", room.otherID, room.loserID,
		currentRound, ofRound, placedAt, 0)

	settled, err := repository.SettleBets(ctx, BetOutcome{
		RoomID:         room.roomID,
		WinnerID:       room.winnerID,
		LoserID:        room.loserID,
		CurrentRound:   currentRound,
		OfRound:        ofRound,
		RemainElements: afterVote,
	})
	if err != nil {
		t.Fatalf("SettleBets() error = %v", err)
	}
	if settled.Won != 1 || settled.Lost != 1 || settled.Discarded != 1 {
		t.Fatalf("settled %+v, want one won, one lost, one discarded", settled)
	}

	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}
	if _, err := repository.AssignRanks(ctx, room.roomID); err != nil {
		t.Fatalf("AssignRanks() error = %v", err)
	}

	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	wantTotals := map[string]Totals{
		// One win on a streak of zero pays WonScore alone: 1000 + 10. The displayed combo is
		// one, being the streak the next wager would ride.
		"winner": {Score: 1010, Combo: 1, TotalPlayed: 1, TotalCorrect: 1, AccuracyHundredths: 10000},
		// 1000 - 10 = 990, streak reset, 0 of 1 correct.
		"loser": {Score: 990, Combo: 0, TotalPlayed: 1, TotalCorrect: 0, AccuracyHundredths: 0},
		// Their wager was discarded, so they are back to the starting score.
		"abstainer": {Score: 1000, Combo: 0, TotalPlayed: 0, TotalCorrect: 0, AccuracyHundredths: 0},
	}
	for name, want := range wantTotals {
		got := stored[room.playerIDs[name]]
		if got != want {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}

	// 1010 > 1000 > 990.
	wantRanks := map[string]int{"winner": 1, "abstainer": 2, "loser": 3}
	ranks, err := storedRanks(ctx, database, room.roomID)
	if err != nil {
		t.Fatalf("read stored ranks: %v", err)
	}
	for name, want := range wantRanks {
		if got := ranks[room.playerIDs[name]]; got != want {
			t.Errorf("%s ranked %d, want %d", name, got, want)
		}
	}

	board, err := repository.Leaderboard(ctx, room.roomID)
	if err != nil {
		t.Fatalf("Leaderboard() error = %v", err)
	}
	if board.TotalUsers != 3 {
		t.Errorf("TotalUsers = %d, want 3", board.TotalUsers)
	}
	if len(board.Top10) != 3 || len(board.Bottom10) != 3 {
		t.Fatalf("top has %d and bottom has %d entries, want 3 each", len(board.Top10), len(board.Bottom10))
	}
	if board.Top10[0].Name != "winner" || board.Top10[2].Name != "loser" {
		t.Errorf("top_10 order = %s..%s, want winner..loser", board.Top10[0].Name, board.Top10[2].Name)
	}
	// Worst first, matching orderByDesc('rank').
	if board.Bottom10[0].Name != "loser" || board.Bottom10[2].Name != "winner" {
		t.Errorf("bottom_10 order = %s..%s, want loser..winner",
			board.Bottom10[0].Name, board.Bottom10[2].Name)
	}
	if board.Top10[0].Accuracy != "100.00" {
		t.Errorf("winner accuracy = %q, want \"100.00\"", board.Top10[0].Accuracy)
	}
	if board.Top10[0].UserID != PlayerID(room.playerIDs["winner"], "anon-winner-"+room.serial) {
		t.Errorf("user_id does not match the resource digest: %q", board.Top10[0].UserID)
	}
}

// Settling the same round twice must not double anyone's score. Redelivery is
// normal — the queue is at-least-once — so the statements have to be idempotent.
func TestSettleBetsIsIdempotent(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()

	room := newSyntheticRoom(t, database)
	room.placeBet(t, database, "winner", room.winnerID, room.loserID, 1, 4, 8, 0)

	outcome := BetOutcome{
		RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
		CurrentRound: 1, OfRound: 4, RemainElements: 7,
	}
	if _, err := repository.SettleBets(ctx, outcome); err != nil {
		t.Fatalf("first SettleBets() error = %v", err)
	}
	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("first RecomputeTotals() error = %v", err)
	}
	first, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	if _, err := repository.SettleBets(ctx, outcome); err != nil {
		t.Fatalf("second SettleBets() error = %v", err)
	}
	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("second RecomputeTotals() error = %v", err)
	}
	second, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	winner := room.playerIDs["winner"]
	if first[winner] != second[winner] {
		t.Fatalf("redelivery changed the winner: %+v then %+v", first[winner], second[winner])
	}
	// One win, no streak behind it: 1000 + 10.
	if first[winner].Score != 1010 {
		t.Fatalf("winner score = %d, want 1010", first[winner].Score)
	}
}

// A room nobody has bet in must still be written, or its players keep the column
// default of zero instead of the starting score. All 5,329 such players in the
// production data hold the starting score, so the PHP did touch them.
func TestRecomputeTotalsGivesNonBettorsTheStartingScore(t *testing.T) {
	database := testDatabase(t)
	scoring := DefaultScoring()
	repository := NewMySQLRepository(database, scoring)
	ctx := context.Background()

	room := newSyntheticRoom(t, database)
	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}

	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("wrote %d players, want 3", len(stored))
	}
	for playerID, totals := range stored {
		want := Totals{Score: scoring.DefaultScore}
		if totals != want {
			t.Errorf("player %d: got %+v, want %+v", playerID, totals, want)
		}
	}
}

func TestRoomBySerialReportsAnUnknownSerial(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())

	room := newSyntheticRoom(t, database)
	found, err := repository.RoomBySerial(context.Background(), room.serial)
	if err != nil {
		t.Fatalf("RoomBySerial() error = %v", err)
	}
	if found.ID != room.roomID || found.Serial != room.serial {
		t.Errorf("RoomBySerial() = %+v, want id %d serial %q", found, room.roomID, room.serial)
	}

	if _, err := repository.RoomBySerial(context.Background(), "no-such-room-serial"); err == nil {
		t.Error("an unknown serial must be an error")
	}
}

// The set-based statement must apply the settled-only rule too, not just Tally.
// This recomputes without settling anything, so the only wagers present are pending.
func TestRecomputeTotalsIgnoresUnsettledWagers(t *testing.T) {
	database := testDatabase(t)
	scoring := DefaultScoring()
	repository := NewMySQLRepository(database, scoring)
	ctx := context.Background()

	room := newSyntheticRoom(t, database)
	// Placed but never settled: no vote runs in this test.
	room.placeBet(t, database, "winner", room.winnerID, room.loserID, 1, 4, 8, 3)

	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}

	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}
	got := stored[room.playerIDs["winner"]]
	want := Totals{Score: scoring.DefaultScore}
	if got != want {
		t.Fatalf("a player with only a pending wager got %+v, want %+v", got, want)
	}

	// And the oracle agrees, which is what makes the statement and Tally one rule
	// rather than two that happen to match today.
	wagers, err := repository.BetsByPlayer(ctx, room.roomID)
	if err != nil {
		t.Fatalf("BetsByPlayer() error = %v", err)
	}
	if len(wagers[room.playerIDs["winner"]]) != 1 {
		t.Fatalf("the pending wager should still be readable, got %d rows",
			len(wagers[room.playerIDs["winner"]]))
	}
	if oracle := Tally(wagers[room.playerIDs["winner"]], scoring, VoteModeHost); oracle != want {
		t.Fatalf("Tally gives %+v, want %+v", oracle, want)
	}
}

// A pending wager placed after a win must not break the streak. The statement picks
// the newest row for the combo, so the settled-only filter has to apply before that
// choice rather than after it.
func TestRecomputeTotalsKeepsTheStreakAcrossAPendingWager(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()

	room := newSyntheticRoom(t, database)

	// Four rounds won in a row, so the streak is established by the outcomes rather than by
	// a last_combo handed to the fixture. remain_elements counts down, which is the order
	// the recompute reads them in.
	for round, remaining := 1, 8; round <= 4; round, remaining = round+1, remaining-1 {
		room.placeBet(t, database, "winner", room.winnerID, room.loserID, round, 4, remaining, 0)
		if _, err := repository.SettleBets(ctx, BetOutcome{
			RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
			CurrentRound: round, OfRound: 4, RemainElements: remaining - 1,
		}); err != nil {
			t.Fatalf("settle round %d: %v", round, err)
		}
	}
	// Then a wager on the next round, not yet decided.
	room.placeBet(t, database, "winner", room.winnerID, room.loserID, 5, 4, 4, 0)

	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}
	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	got := stored[room.playerIDs["winner"]]
	// 1000 + 10 + 20 + 30 + 40, four of four correct, and the pending wager leaves the
	// streak at four rather than resetting it.
	want := Totals{Score: 1100, Combo: 4, TotalPlayed: 4, TotalCorrect: 4, AccuracyHundredths: 10000}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// THE RULE THE COMBO NOW OBEYS: the bonus depends only on which wagers won, in round
// order — never on when they were placed or on the order their settlements arrived.
//
// Before this, last_combo was resolved when a wager was placed, from the wager before it,
// which is only right if that one had already settled. A player betting faster than the
// host votes wrote zero into every row and lost the whole bonus: measured on an 8-element
// room with four correct picks, paced betting scored 1100 and rapid betting 1050.
//
// Laravel appeared not to have the problem because its room client played a two second
// animation before showing the next pairing while the settlement job ran in milliseconds.
// That is UI timing standing in for a data rule, and this port had already removed it.
func TestTheComboBonusIgnoresWhenWagersWerePlaced(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()
	room := newSyntheticRoom(t, database)

	// Every wager placed BEFORE anything settles, with last_combo zero on all of them —
	// which is exactly what a fast better produces.
	for round, remaining := 1, 8; round <= 4; round, remaining = round+1, remaining-1 {
		room.placeBet(t, database, "winner", room.winnerID, room.loserID, round, 4, remaining, 0)
	}
	// And the settlements arrive in reverse order, to show the result does not depend on it.
	for round, remaining := 4, 5; round >= 1; round, remaining = round-1, remaining+1 {
		if _, err := repository.SettleBets(ctx, BetOutcome{
			RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
			CurrentRound: round, OfRound: 4, RemainElements: remaining - 1,
		}); err != nil {
			t.Fatalf("settle round %d: %v", round, err)
		}
	}

	if _, err := repository.RecomputeTotals(ctx, room.hostRoom()); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}
	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	got := stored[room.playerIDs["winner"]]
	// The same 1100 a paced player gets: 10 + 20 + 30 + 40.
	want := Totals{Score: 1100, Combo: 4, TotalPlayed: 4, TotalCorrect: 4, AccuracyHundredths: 10000}
	if got != want {
		t.Fatalf("got %+v, want %+v — the bonus must not depend on betting speed", got, want)
	}
}

// A wager on the round AFTER the one being settled must survive.
//
// THE WINDOW IS REAL, AND THIS WAS BROKEN. The host's vote writes the next pairing in the
// same transaction that records the round, so participants see the new pair and wager on it
// BEFORE the queued settlement of the previous round runs. The discard used to remove every
// unsettled wager in the room, so it deleted theirs and their vote silently never counted.
//
// Reproduced end to end before the fix: a wager placed between two host votes was deleted by
// the first vote's settlement, and the second settled nothing at all.
func TestSettlementLeavesTheNextRoundsWagersAlone(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()
	room := newSyntheticRoom(t, database)

	const (
		currentRound = 3
		ofRound      = 4
		placedAt     = 8
		afterVote    = 7
	)

	// This round: one wager that will be settled, and one on a pairing it did not present —
	// same round, so this settle discards it.
	room.placeBet(t, database, "winner", room.winnerID, room.loserID, currentRound, ofRound, placedAt, 0)
	room.placeBet(t, database, "abstainer", room.otherID, room.loserID, currentRound, ofRound, placedAt, 0)
	// The next round, already on screen. remain_elements counts DOWN, so a newer round has
	// a smaller value.
	room.placeBet(t, database, "loser", room.winnerID, room.loserID, currentRound+1, ofRound, afterVote, 0)

	settled, err := repository.SettleBets(ctx, BetOutcome{
		RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
		CurrentRound: currentRound, OfRound: ofRound, RemainElements: afterVote,
	})
	if err != nil {
		t.Fatalf("SettleBets() error = %v", err)
	}
	if settled.Won != 1 {
		t.Errorf("won = %d, want 1", settled.Won)
	}
	// Only the stale one. Two would mean the next round's wager was eaten as well.
	if settled.Discarded != 1 {
		t.Errorf("discarded = %d, want 1 — the next round's wager must not be discarded", settled.Discarded)
	}

	var surviving int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_room_user_bets
		 WHERE game_room_id = ? AND remain_elements = ? AND won_at IS NULL AND lost_at IS NULL`,
		room.roomID, afterVote).Scan(&surviving); err != nil {
		t.Fatalf("count surviving wagers: %v", err)
	}
	if surviving != 1 {
		t.Fatalf("%d wagers survive for the round not yet settled, want 1", surviving)
	}
}

/**
 * SETTLES ARE NOT ORDERED, AND THE DISCARD MUST SURVIVE THAT.
 *
 * The worker runs four jobs at once and serialises a room's settles with a lock, but the
 * order four queued settles acquire that lock is not the order they were published in. A
 * discard scoped to "this round and everything older" therefore deletes wagers whose own
 * settle has not run yet — and that settle then matches nothing.
 *
 * Reproduced end to end before the fix: on an 8-element room where the participant backed
 * the host's pick every time, pacing the votes settled all four (score 1100) while firing
 * them off settled three (1030), with "won=1 discarded=2" logged on the second settle.
 *
 * Here the later round settles FIRST, which is the case that used to lose the earlier
 * round's wager.
 */
func TestSettlingOutOfOrderDoesNotEatAnEarlierRoundsWager(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database, DefaultScoring())
	ctx := context.Background()
	room := newSyntheticRoom(t, database)

	// Two consecutive rounds, both wagered on before either settled — which is what happens
	// whenever a participant decides faster than the host advances.
	const (
		firstRound     = 3
		secondRound    = 4
		ofRound        = 4
		firstPlacedAt  = 8
		secondPlacedAt = 7
	)
	room.placeBet(t, database, "winner", room.winnerID, room.loserID, firstRound, ofRound, firstPlacedAt, 0)
	room.placeBet(t, database, "loser", room.winnerID, room.loserID, secondRound, ofRound, secondPlacedAt, 0)

	// The SECOND round settles first.
	second, err := repository.SettleBets(ctx, BetOutcome{
		RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
		CurrentRound: secondRound, OfRound: ofRound, RemainElements: secondPlacedAt - 1,
	})
	if err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if second.Won != 1 {
		t.Errorf("second settle won = %d, want 1", second.Won)
	}
	if second.Discarded != 0 {
		t.Errorf("second settle discarded %d wagers; the earlier round's wager must survive", second.Discarded)
	}

	// Now the first round's settle arrives. Its wager has to still be there.
	first, err := repository.SettleBets(ctx, BetOutcome{
		RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
		CurrentRound: firstRound, OfRound: ofRound, RemainElements: firstPlacedAt - 1,
	})
	if err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if first.Won != 1 {
		t.Fatalf("first settle won = %d, want 1 — its wager was deleted by the later round", first.Won)
	}

	var settled int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_room_user_bets
		 WHERE game_room_id = ? AND won_at IS NOT NULL`, room.roomID).Scan(&settled); err != nil {
		t.Fatalf("count settled wagers: %v", err)
	}
	if settled != 2 {
		t.Errorf("%d wagers settled, want both", settled)
	}
}

// A round the ROOM decided pays by how the room split, the same magnitude either way.
//
// Seven of ten back the winning side, so the round pays 17 — the rule as the feature was
// asked for. The majority ends on 1017 and the minority on 983, and the number is written
// into the wager rows because nothing downstream can re-derive it: the split is a fact
// about the round, not about any one player's wagers.
func TestSettleInAMajorityRoomPaysBothSidesBySplit(t *testing.T) {
	database := testDatabase(t)
	scoring := DefaultScoring()
	repository := NewMySQLRepository(database, scoring)
	ctx := context.Background()

	room := newSyntheticRoom(t, database)
	majority := room.majorityRoom(t, database)

	const (
		currentRound = 3
		ofRound      = 4
		placedAt     = 8
		afterVote    = 7
		payout       = 17
	)

	var backers, dissenters []string
	for index := 1; index <= 7; index++ {
		name := fmt.Sprintf("majority-%d", index)
		room.seat(t, database, name)
		room.placeBet(t, database, name, room.winnerID, room.loserID,
			currentRound, ofRound, placedAt, 0)
		backers = append(backers, name)
	}
	for index := 1; index <= 3; index++ {
		name := fmt.Sprintf("minority-%d", index)
		room.seat(t, database, name)
		room.placeBet(t, database, name, room.loserID, room.winnerID,
			currentRound, ofRound, placedAt, 0)
		dissenters = append(dissenters, name)
	}

	settled, err := repository.SettleBets(ctx, BetOutcome{
		RoomID:         room.roomID,
		WinnerID:       room.winnerID,
		LoserID:        room.loserID,
		CurrentRound:   currentRound,
		OfRound:        ofRound,
		RemainElements: afterVote,
		VoteMode:       VoteModeMajority,
	})
	if err != nil {
		t.Fatalf("SettleBets() error = %v", err)
	}
	if settled.Won != 7 || settled.Lost != 3 {
		t.Fatalf("settled %+v, want seven won and three lost", settled)
	}

	// The rows carry the payout, both signs of it.
	wagers, err := repository.BetsByPlayer(ctx, room.roomID)
	if err != nil {
		t.Fatalf("BetsByPlayer() error = %v", err)
	}
	for _, name := range backers {
		rows := wagers[room.playerIDs[name]]
		if len(rows) != 1 || rows[0].Score != payout {
			t.Fatalf("%s wager rows = %+v, want one row scoring %d", name, rows, payout)
		}
	}
	for _, name := range dissenters {
		rows := wagers[room.playerIDs[name]]
		if len(rows) != 1 || rows[0].Score != -payout {
			t.Fatalf("%s wager rows = %+v, want one row scoring %d", name, rows, -payout)
		}
	}

	if _, err := repository.RecomputeTotals(ctx, majority); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}
	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	wantBacker := Totals{
		Score: scoring.DefaultScore + payout, Combo: 0,
		TotalPlayed: 1, TotalCorrect: 1, AccuracyHundredths: 10000,
	}
	wantDissenter := Totals{
		Score: scoring.DefaultScore - payout, Combo: 0,
		TotalPlayed: 1, TotalCorrect: 0, AccuracyHundredths: 0,
	}
	for _, name := range backers {
		if got := stored[room.playerIDs[name]]; got != wantBacker {
			t.Fatalf("%s got %+v, want %+v", name, got, wantBacker)
		}
	}
	for _, name := range dissenters {
		if got := stored[room.playerIDs[name]]; got != wantDissenter {
			t.Fatalf("%s got %+v, want %+v", name, got, wantDissenter)
		}
	}
	// The three seated by the fixture never wagered, so they hold the starting score —
	// the LEFT JOIN survives in the majority statement too.
	for _, name := range []string{"winner", "loser", "abstainer"} {
		if got := stored[room.playerIDs[name]]; got.Score != scoring.DefaultScore {
			t.Fatalf("%s never bet but got %+v", name, got)
		}
	}

	// And the oracle agrees, which is what makes the statement and Tally one rule rather
	// than two that match today.
	for playerID, actual := range stored {
		if want := Tally(wagers[playerID], scoring, VoteModeMajority); actual != want {
			t.Fatalf("player %d: stored %+v, Tally gives %+v", playerID, actual, want)
		}
	}

	// A redelivered settle recounts the same rows and writes the same magnitude, so
	// nobody moves. The counts do not depend on won_at, which is what makes that true.
	if _, err := repository.SettleBets(ctx, BetOutcome{
		RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
		CurrentRound: currentRound, OfRound: ofRound, RemainElements: afterVote,
		VoteMode: VoteModeMajority,
	}); err != nil {
		t.Fatalf("second SettleBets() error = %v", err)
	}
	if _, err := repository.RecomputeTotals(ctx, majority); err != nil {
		t.Fatalf("second RecomputeTotals() error = %v", err)
	}
	again, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}
	for playerID, before := range stored {
		if again[playerID] != before {
			t.Fatalf("player %d drifted on redelivery: %+v then %+v",
				playerID, before, again[playerID])
		}
	}
}

// NO COMBO IN A MAJORITY ROOM. Two rounds on the winning side pay the same magnitude
// twice, where host rules would have paid the second one a streak bonus.
//
// Two of three back the winner each round, which pays 17, so the taste score lands on
// 1034 with the combo at zero. The same two outcomes under host rules give 1030 and a
// combo of two — the numbers differ, so this cannot pass by accident.
func TestMajorityRoomsPayNoComboBonus(t *testing.T) {
	database := testDatabase(t)
	scoring := DefaultScoring()
	repository := NewMySQLRepository(database, scoring)
	ctx := context.Background()

	room := newSyntheticRoom(t, database)
	majority := room.majorityRoom(t, database)
	room.seat(t, database, "ally")

	for round, remaining := 1, 8; round <= 2; round, remaining = round+1, remaining-1 {
		room.placeBet(t, database, "winner", room.winnerID, room.loserID, round, 4, remaining, 0)
		room.placeBet(t, database, "ally", room.winnerID, room.loserID, round, 4, remaining, 0)
		room.placeBet(t, database, "loser", room.loserID, room.winnerID, round, 4, remaining, 0)
		if _, err := repository.SettleBets(ctx, BetOutcome{
			RoomID: room.roomID, WinnerID: room.winnerID, LoserID: room.loserID,
			CurrentRound: round, OfRound: 4, RemainElements: remaining - 1,
			VoteMode: VoteModeMajority,
		}); err != nil {
			t.Fatalf("settle round %d: %v", round, err)
		}
	}

	if _, err := repository.RecomputeTotals(ctx, majority); err != nil {
		t.Fatalf("RecomputeTotals() error = %v", err)
	}
	stored, err := repository.StoredTotals(ctx, room.roomID)
	if err != nil {
		t.Fatalf("StoredTotals() error = %v", err)
	}

	// 1000 + 17 + 17, and no streak.
	want := Totals{Score: 1034, Combo: 0, TotalPlayed: 2, TotalCorrect: 2, AccuracyHundredths: 10000}
	if got := stored[room.playerIDs["winner"]]; got != want {
		t.Fatalf("got %+v, want %+v — a taste score pays no streak bonus", got, want)
	}
	// The minority pays the same magnitude twice: 1000 - 34.
	wantLoser := Totals{Score: 966, Combo: 0, TotalPlayed: 2, TotalCorrect: 0, AccuracyHundredths: 0}
	if got := stored[room.playerIDs["loser"]]; got != wantLoser {
		t.Fatalf("the minority got %+v, want %+v", got, wantLoser)
	}
}
