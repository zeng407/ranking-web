package gameroom

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// These run against the real tables because the behaviour being ported is the behaviour
// of three unique indexes. A fake would only reproduce what I already believe.

type participationFixture struct {
	repository *MySQLParticipation
	database   *sql.DB
	// namespace prefixes every row this test writes, so cleanup is exact.
	namespace string
	postID    int64
	gameID    int64
	gameSer   string
}

func newParticipationFixture(t *testing.T) (*participationFixture, context.Context) {
	t.Helper()
	database := testDatabase(t)
	ctx := context.Background()

	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate a namespace: %v", err)
	}
	namespace := fmt.Sprintf("gt%x", raw)

	fixture := &participationFixture{
		repository: NewMySQLParticipation(database),
		database:   database,
		namespace:  namespace,
		gameSer:    namespace + "game",
	}

	// A game of its own rather than one borrowed from production: these tests create
	// rooms and wagers, and doing that on a real game would corrupt somebody's history.
	var postID int64
	if err := database.QueryRowContext(ctx,
		`SELECT id FROM posts WHERE deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&postID); err != nil {
		t.Fatalf("find a post to attach a game to: %v", err)
	}
	fixture.postID = postID

	result, err := database.ExecContext(ctx, `
		INSERT INTO games (post_id, serial, element_count, candidates, created_at, updated_at)
		VALUES (?, ?, 8, NULL, NOW(), NOW())`, postID, fixture.gameSer)
	if err != nil {
		t.Fatalf("create a test game: %v", err)
	}
	if fixture.gameID, err = result.LastInsertId(); err != nil {
		t.Fatalf("test game id: %v", err)
	}

	t.Cleanup(func() {
		// Rooms, participants and wagers all cascade from the game.
		if _, err := database.ExecContext(context.Background(),
			`DELETE FROM games WHERE serial LIKE ?`, namespace+"%"); err != nil {
			t.Errorf("clean up the test game: %v", err)
		}
	})
	return fixture, ctx
}

func (fixture *participationFixture) elementIDs(t *testing.T, ctx context.Context, count int) []int64 {
	t.Helper()
	rows, err := fixture.database.QueryContext(ctx,
		`SELECT id FROM elements WHERE deleted_at IS NULL ORDER BY id LIMIT ?`, count)
	if err != nil {
		t.Fatalf("read elements: %v", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan element: %v", err)
		}
		ids = append(ids, id)
	}
	if len(ids) < count {
		t.Fatalf("need %d elements, found %d", count, len(ids))
	}
	return ids
}

func (fixture *participationFixture) room(t *testing.T, ctx context.Context) Room {
	t.Helper()
	serial, err := NewRoomSerial()
	if err != nil {
		t.Fatalf("generate a serial: %v", err)
	}
	room, created, err := fixture.repository.EnsureRoom(ctx, fixture.gameSer, serial)
	if err != nil {
		t.Fatalf("EnsureRoom() error = %v", err)
	}
	if !created {
		t.Fatalf("the test game already had a room")
	}
	return room
}

// game creates another game the tests can move a room onto. The cleanup deletes every
// game in the namespace, so anything created here cascades away with the fixture.
func (fixture *participationFixture) game(t *testing.T, ctx context.Context, suffix string, postID int64) string {
	t.Helper()
	serial := fixture.namespace + suffix
	if _, err := fixture.database.ExecContext(ctx, `
		INSERT INTO games (post_id, serial, element_count, candidates, created_at, updated_at)
		VALUES (?, ?, 8, NULL, NOW(), NOW())`, postID, serial); err != nil {
		t.Fatalf("create the game %q: %v", serial, err)
	}
	return serial
}

// otherPostID finds a post that is not the fixture's, for the cross-post refusal.
func (fixture *participationFixture) otherPostID(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	var postID int64
	err := fixture.database.QueryRowContext(ctx, `
		SELECT id FROM posts WHERE deleted_at IS NULL AND id <> ? ORDER BY id LIMIT 1`,
		fixture.postID).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		t.Skip("the restore has only one post; skipping the cross-post case")
	}
	if err != nil {
		t.Fatalf("find a second post: %v", err)
	}
	return postID
}

func TestEnsureRoomCreatesOnceAndThenReturnsTheSameRoom(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)

	first := fixture.room(t, ctx)
	if first.Serial == "" || first.ID == 0 {
		t.Fatalf("incomplete room: %+v", first)
	}

	// A different serial is offered; the existing room must win. This is the host's page
	// reloading, which Laravel's firstOrCreate also had to survive.
	otherSerial, err := NewRoomSerial()
	if err != nil {
		t.Fatalf("generate a serial: %v", err)
	}
	second, created, err := fixture.repository.EnsureRoom(ctx, fixture.gameSer, otherSerial)
	if err != nil {
		t.Fatalf("second EnsureRoom() error = %v", err)
	}
	if created {
		t.Error("a second room was reported as created")
	}
	if second.ID != first.ID || second.Serial != first.Serial {
		t.Errorf("second call returned %+v, want %+v", second, first)
	}
}

func TestEnsureRoomRejectsAnUnknownGame(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)

	_, _, err := fixture.repository.EnsureRoom(ctx, fixture.namespace+"nope", "abcdefgh")
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("error = %v, want ErrGameNotFound", err)
	}
}

// THE RACE MIGRATION 00010 EXISTS FOR. Two hosts opening the same game at once must end
// up in ONE room; production has two games where they did not, and the extra room was
// unreachable because every later lookup goes through the game.
func TestConcurrentEnsureRoomProducesOneRoom(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)

	const callers = 8
	var (
		start    sync.WaitGroup
		done     sync.WaitGroup
		mutex    sync.Mutex
		rooms    = map[int64]int{}
		creates  int
		failures []error
	)
	start.Add(1)
	for range callers {
		done.Add(1)
		go func() {
			defer done.Done()
			serial, err := NewRoomSerial()
			if err != nil {
				mutex.Lock()
				failures = append(failures, err)
				mutex.Unlock()
				return
			}
			start.Wait()
			room, created, err := fixture.repository.EnsureRoom(ctx, fixture.gameSer, serial)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			rooms[room.ID]++
			if created {
				creates++
			}
		}()
	}
	start.Done()
	done.Wait()

	if len(failures) > 0 {
		t.Fatalf("callers failed: %v", failures)
	}
	if len(rooms) != 1 {
		t.Errorf("%d distinct rooms were returned, want 1: %v", len(rooms), rooms)
	}
	if creates != 1 {
		t.Errorf("%d callers reported creating the room, want 1", creates)
	}

	var stored int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_rooms WHERE game_id = ?`, fixture.gameID).Scan(&stored); err != nil {
		t.Fatalf("count rooms: %v", err)
	}
	if stored != 1 {
		t.Errorf("%d rooms exist for the game, want 1", stored)
	}
}

func TestEnsureParticipantCreatesOncePerBrowser(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	first, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "起始暱稱", 1000)
	if err != nil {
		t.Fatalf("EnsureParticipant() error = %v", err)
	}
	if first.Score != 1000 {
		t.Errorf("starting score = %d, want 1000", first.Score)
	}
	if first.Nickname != "起始暱稱" {
		t.Errorf("nickname = %q", first.Nickname)
	}
	if first.PlayerID() == "" {
		t.Error("no player digest")
	}

	// The same browser resumes rather than starting over. A second row here would reset
	// the player's score mid-game.
	again, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "別的暱稱", 1000)
	if err != nil {
		t.Fatalf("second EnsureParticipant() error = %v", err)
	}
	if again.ID != first.ID {
		t.Errorf("a second row was created: %d then %d", first.ID, again.ID)
	}
	if again.Nickname != "起始暱稱" {
		t.Errorf("the existing nickname was overwritten with %q", again.Nickname)
	}

	// A different browser is a different player.
	other, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-b", nil, "第二人", 1000)
	if err != nil {
		t.Fatalf("EnsureParticipant() for a second browser: %v", err)
	}
	if other.ID == first.ID {
		t.Error("two browsers resolved to one participant")
	}
}

// THE RACE MIGRATION 00012 EXISTS FOR. A room page fires its initial calls together, so
// two find-or-creates for one browser overlap on every join.
func TestConcurrentEnsureParticipantProducesOneRow(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	const callers = 8
	var (
		start        sync.WaitGroup
		done         sync.WaitGroup
		mutex        sync.Mutex
		participants = map[int64]int{}
		failures     []error
	)
	start.Add(1)
	for range callers {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-race", nil, "路人", 1000)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			participants[participant.ID]++
		}()
	}
	start.Done()
	done.Wait()

	if len(failures) > 0 {
		t.Fatalf("callers failed: %v", failures)
	}
	if len(participants) != 1 {
		t.Errorf("%d distinct participants were returned, want 1: %v", len(participants), participants)
	}
}

// user_id is filled in when a player signs in mid-room, and never overwritten. Laravel
// wrote it unconditionally, so on a shared browser the row would move to whoever logged
// in last.
func TestEnsureParticipantLinksAnAccountWithoutStealingTheRow(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	var firstUser, secondUser int64
	rows, err := fixture.database.QueryContext(ctx, `SELECT id FROM users ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatalf("read users: %v", err)
	}
	ids := []*int64{&firstUser, &secondUser}
	index := 0
	for rows.Next() && index < 2 {
		if err := rows.Scan(ids[index]); err != nil {
			rows.Close()
			t.Fatalf("scan user: %v", err)
		}
		index++
	}
	rows.Close()
	if index < 2 {
		t.Skip("need two accounts in this database")
	}

	anonymous, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-shared", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("anonymous join: %v", err)
	}
	if anonymous.UserID != nil {
		t.Fatalf("user id = %v on an anonymous join", *anonymous.UserID)
	}

	linked, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-shared", &firstUser, "路人", 1000)
	if err != nil {
		t.Fatalf("signed-in join: %v", err)
	}
	if linked.ID != anonymous.ID {
		t.Errorf("signing in created a new participant")
	}
	if linked.UserID == nil || *linked.UserID != firstUser {
		t.Fatalf("user id = %v, want %d", linked.UserID, firstUser)
	}

	// Somebody else signs in on the same browser. The row must stay with the first
	// account: it holds that person's score.
	stolen, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-shared", &secondUser, "路人", 1000)
	if err != nil {
		t.Fatalf("second signed-in join: %v", err)
	}
	if stolen.UserID == nil || *stolen.UserID != firstUser {
		t.Errorf("the participant was reassigned to user %v", stolen.UserID)
	}
}

// THE RACE MIGRATION 00011 EXISTS FOR, and the one that cost real scores: a double-clicked
// vote inserted twice, both copies settled, and the round counted twice.
func TestBettingTwiceOnOneRoundKeepsOneRow(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elements := fixture.elementIDs(t, ctx, 2)
	participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	bet := PlacedBet{WinnerID: elements[0], LoserID: elements[1], CurrentRound: 1, OfRound: 4, RemainElements: 8}
	for range 3 {
		if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID, bet, 0); err != nil {
			t.Fatalf("UpsertBet() error = %v", err)
		}
	}

	var stored int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_room_user_bets WHERE game_room_user_id = ?`, participant.ID).Scan(&stored); err != nil {
		t.Fatalf("count wagers: %v", err)
	}
	if stored != 1 {
		t.Errorf("%d wagers stored for one round, want 1", stored)
	}
}

func TestConcurrentBetsOnOneRoundKeepOneRow(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elements := fixture.elementIDs(t, ctx, 2)
	participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	bet := PlacedBet{WinnerID: elements[0], LoserID: elements[1], CurrentRound: 1, OfRound: 4, RemainElements: 8}
	var (
		start    sync.WaitGroup
		done     sync.WaitGroup
		mutex    sync.Mutex
		failures []error
	)
	start.Add(1)
	for range 8 {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID, bet, 0); err != nil {
				mutex.Lock()
				failures = append(failures, err)
				mutex.Unlock()
			}
		}()
	}
	start.Done()
	done.Wait()

	if len(failures) > 0 {
		t.Fatalf("wagers failed: %v", failures)
	}
	var stored int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_room_user_bets WHERE game_room_user_id = ?`, participant.ID).Scan(&stored); err != nil {
		t.Fatalf("count wagers: %v", err)
	}
	if stored != 1 {
		t.Errorf("%d wagers stored for one round under concurrency, want 1", stored)
	}
}

// Changing a vote must un-settle it. Leaving won_at set would score the new pick with the
// old outcome — a player could switch to the loser and keep the win.
func TestChangingAVoteClearsItsOutcome(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elements := fixture.elementIDs(t, ctx, 2)
	participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	bet := PlacedBet{WinnerID: elements[0], LoserID: elements[1], CurrentRound: 1, OfRound: 4, RemainElements: 8}
	if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID, bet, 2); err != nil {
		t.Fatalf("first wager: %v", err)
	}
	// Settle it as a win, as the worker would.
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE game_room_user_bets SET won_at = NOW(), score = 30 WHERE game_room_user_id = ?`,
		participant.ID); err != nil {
		t.Fatalf("settle the wager: %v", err)
	}

	// The player changes their mind before the host advances the round.
	bet.WinnerID, bet.LoserID = elements[1], elements[0]
	if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID, bet, 2); err != nil {
		t.Fatalf("changed wager: %v", err)
	}

	var (
		winner int64
		wonAt  sql.NullTime
		lostAt sql.NullTime
		score  int
	)
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT winner_id, won_at, lost_at, score FROM game_room_user_bets WHERE game_room_user_id = ?`,
		participant.ID).Scan(&winner, &wonAt, &lostAt, &score); err != nil {
		t.Fatalf("read the wager: %v", err)
	}
	if winner != elements[1] {
		t.Errorf("winner = %d, want the new pick %d", winner, elements[1])
	}
	if wonAt.Valid || lostAt.Valid {
		t.Error("the changed wager kept its old outcome")
	}
	if score != 0 {
		t.Errorf("score = %d, want it reset to 0", score)
	}
}

func TestPreviousBetStreakReportsTheLastOutcome(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elements := fixture.elementIDs(t, ctx, 2)
	participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	// Nothing wagered yet.
	if _, _, found, err := fixture.repository.PreviousBetStreak(ctx, participant.ID); err != nil || found {
		t.Fatalf("found = %v, err = %v, want no previous wager", found, err)
	}

	if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID,
		PlacedBet{WinnerID: elements[0], LoserID: elements[1], CurrentRound: 1, OfRound: 4, RemainElements: 8},
		3); err != nil {
		t.Fatalf("wager: %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE game_room_user_bets SET won_at = NOW() WHERE game_room_user_id = ?`, participant.ID); err != nil {
		t.Fatalf("settle: %v", err)
	}

	lastCombo, won, found, err := fixture.repository.PreviousBetStreak(ctx, participant.ID)
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if lastCombo != 3 {
		t.Errorf("last combo = %d, want 3", lastCombo)
	}
	if !won {
		t.Error("won = false for a wager with won_at set")
	}
}

func TestCurrentVotesTalliesBothDirections(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elements := fixture.elementIDs(t, ctx, 2)

	// No pairing set yet: the tally is absent rather than an error, matching what
	// GameRoomVoteResource returned for a game with no candidates.
	if _, present, err := fixture.repository.CurrentVotes(ctx, room.ID, fixture.gameSer); err != nil || present {
		t.Fatalf("present = %v, err = %v, want no tally before a pairing exists", present, err)
	}

	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE games SET candidates = ? WHERE id = ?`,
		fmt.Sprintf("%d,%d", elements[0], elements[1]), fixture.gameID); err != nil {
		t.Fatalf("set candidates: %v", err)
	}

	// Three players: two for the first candidate, one for the second.
	for index, winner := range []int64{elements[0], elements[0], elements[1]} {
		loser := elements[1]
		if winner == elements[1] {
			loser = elements[0]
		}
		participant, err := fixture.repository.EnsureParticipant(
			ctx, room.ID, fmt.Sprintf("browser-%d", index), nil, "路人", 1000)
		if err != nil {
			t.Fatalf("join %d: %v", index, err)
		}
		if err := fixture.repository.UpsertBet(ctx, room.ID, participant.ID,
			PlacedBet{WinnerID: winner, LoserID: loser, CurrentRound: 1, OfRound: 4, RemainElements: 8},
			0); err != nil {
			t.Fatalf("wager %d: %v", index, err)
		}
	}

	tally, present, err := fixture.repository.CurrentVotes(ctx, room.ID, fixture.gameSer)
	if err != nil || !present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
	if tally.FirstCandidate != elements[0] || tally.SecondCandidate != elements[1] {
		t.Errorf("candidates = %d/%d, want %d/%d",
			tally.FirstCandidate, tally.SecondCandidate, elements[0], elements[1])
	}
	if tally.FirstCandidateVotes != 2 || tally.SecondCandidateVote != 1 {
		t.Errorf("votes = %d/%d, want 2/1", tally.FirstCandidateVotes, tally.SecondCandidateVote)
	}
	if tally.TotalVotes != 3 {
		t.Errorf("total = %d, want 3", tally.TotalVotes)
	}
	// element_count is 8 on the test game and no round has been played, so the tally
	// falls back to it — the same fallback GameRoomVoteResource used.
	if tally.RemainElements != 8 {
		t.Errorf("remain elements = %d, want the game's element count 8", tally.RemainElements)
	}
}

func TestRenameWritesTheNewName(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	participant, err := fixture.repository.EnsureParticipant(ctx, room.ID, "browser-a", nil, "路人", 1000)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := fixture.repository.Rename(ctx, participant.ID, "新名字"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	reread, found, err := fixture.repository.participant(ctx, room.ID, "browser-a")
	if err != nil || !found {
		t.Fatalf("re-read: found = %v, err = %v", found, err)
	}
	if reread.Nickname != "新名字" {
		t.Errorf("nickname = %q", reread.Nickname)
	}
}

func TestRoomBySerialWithGameResolvesBoth(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	found, gameSerial, ok, err := fixture.repository.RoomBySerialWithGame(ctx, room.Serial)
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if found.ID != room.ID {
		t.Errorf("room id = %d, want %d", found.ID, room.ID)
	}
	if gameSerial != fixture.gameSer {
		t.Errorf("game serial = %q, want %q", gameSerial, fixture.gameSer)
	}

	if _, _, ok, err := fixture.repository.RoomBySerialWithGame(ctx, "nosuchroom"); err != nil || ok {
		t.Errorf("ok = %v, err = %v for an unknown serial", ok, err)
	}
}

func TestRoomSerialsAreLowercaseAlphanumericAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for range 500 {
		serial, err := NewRoomSerial()
		if err != nil {
			t.Fatalf("NewRoomSerial() error = %v", err)
		}
		if len(serial) != RoomSerialLength {
			t.Fatalf("serial %q is %d characters, want %d", serial, len(serial), RoomSerialLength)
		}
		if strings.ToLower(serial) != serial {
			t.Errorf("serial %q is not lowercase; the column is case-insensitive and the old ones were folded", serial)
		}
		for _, character := range serial {
			if !strings.ContainsRune(serialAlphabet, character) {
				t.Errorf("serial %q contains %q, which is outside the alphabet", serial, character)
			}
		}
		if seen[serial] {
			t.Errorf("serial %q was generated twice in 500 draws", serial)
		}
		seen[serial] = true
	}
}

/**
 * The restart. A restart mints a new game, and the room has to follow it while keeping the
 * serial already on invite links and QR codes — re-opening would hand out a new one and
 * strand everybody holding the old.
 */
func TestRebindRoomFollowsTheHostToANewGame(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	restarted := fixture.game(t, ctx, "restart", fixture.postID)

	moved, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, restarted)
	if err != nil {
		t.Fatalf("RebindRoom() error = %v", err)
	}
	if moved.ID != room.ID || moved.Serial != room.Serial {
		t.Errorf("room = %+v, want the same room as %+v", moved, room)
	}

	found, hosting, err := fixture.repository.RoomByGameSerial(ctx, restarted)
	if err != nil || !hosting {
		t.Fatalf("RoomByGameSerial(restarted) = %+v, %v, %v; want the room", found, hosting, err)
	}
	if found.ID != room.ID {
		t.Errorf("the new game holds room %d, want %d", found.ID, room.ID)
	}
	// And nothing is left behind on the game the host abandoned.
	if _, stillHosting, err := fixture.repository.RoomByGameSerial(ctx, fixture.gameSer); err != nil || stillHosting {
		t.Errorf("the old game still hosts a room (hosting = %v, err = %v)", stillHosting, err)
	}
}

// The reply can be lost — the host retries. The second attempt arrives naming a source game
// the room has already left, and must be read as "already done", not as a mismatch.
func TestRebindRoomIsIdempotent(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	restarted := fixture.game(t, ctx, "restart", fixture.postID)

	if _, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, restarted); err != nil {
		t.Fatalf("first RebindRoom() error = %v", err)
	}
	again, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, restarted)
	if err != nil {
		t.Fatalf("second RebindRoom() error = %v", err)
	}
	if again.ID != room.ID {
		t.Errorf("room = %+v, want the same room", again)
	}
}

// The source serial is the only proof of hosting this stack has: no column records an owner
// and every room route is optional-auth. A caller that cannot name the game the room is on
// is not the host, and must not be able to drag the room anywhere.
func TestRebindRoomRefusesAStaleSourceGame(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	elsewhere := fixture.game(t, ctx, "elsewhere", fixture.postID)
	target := fixture.game(t, ctx, "target", fixture.postID)

	if _, err := fixture.repository.RebindRoom(ctx, room.Serial, elsewhere, target); !errors.Is(err, ErrRoomMismatch) {
		t.Fatalf("error = %v, want ErrRoomMismatch", err)
	}
	if _, hosting, err := fixture.repository.RoomByGameSerial(ctx, fixture.gameSer); err != nil || !hosting {
		t.Errorf("the room left its game after a refused move (hosting = %v, err = %v)", hosting, err)
	}
}

// The invariant that protects seated participants: a room may only follow its host within
// the same post. Otherwise a rebind swaps the content under everyone in the room.
func TestRebindRoomRefusesAGameOnAnotherPost(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	foreign := fixture.game(t, ctx, "foreign", fixture.otherPostID(t, ctx))

	if _, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, foreign); !errors.Is(err, ErrRoomNotRebindable) {
		t.Fatalf("error = %v, want ErrRoomNotRebindable", err)
	}
}

// One room per game — the unique index migration 00010 added. A game that already has its
// own room cannot take another, and the duplicate key must surface as a refusal rather than
// a raw driver error.
func TestRebindRoomRefusesAGameThatAlreadyHasARoom(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	occupied := fixture.game(t, ctx, "occupied", fixture.postID)

	serial, err := NewRoomSerial()
	if err != nil {
		t.Fatalf("generate a serial: %v", err)
	}
	if _, _, err := fixture.repository.EnsureRoom(ctx, occupied, serial); err != nil {
		t.Fatalf("EnsureRoom() error = %v", err)
	}

	if _, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, occupied); !errors.Is(err, ErrRoomNotRebindable) {
		t.Fatalf("error = %v, want ErrRoomNotRebindable", err)
	}
}

func TestRebindRoomReportsWhatIsMissing(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)
	target := fixture.game(t, ctx, "target", fixture.postID)

	if _, err := fixture.repository.RebindRoom(ctx, "nosuchro", fixture.gameSer, target); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown room: error = %v, want ErrNotFound", err)
	}
	if _, err := fixture.repository.RebindRoom(ctx, room.Serial, fixture.gameSer, fixture.namespace+"nope"); !errors.Is(err, ErrGameNotFound) {
		t.Errorf("unknown game: error = %v, want ErrGameNotFound", err)
	}
}

/**
 * A new room decides its rounds the way every room did before this existed, so opening one
 * cannot change a host's game under them.
 */
func TestNewRoomsAreHostDecided(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	settings, err := fixture.repository.Voting(ctx, room.ID)
	if err != nil {
		t.Fatalf("Voting() error = %v", err)
	}
	if settings.Mode != VoteModeHost || settings.RoundSeconds != 0 {
		t.Errorf("settings = %+v, want the host deciding with no clock", settings)
	}
	if settings.SecondsLeft != nil {
		t.Errorf("seconds left = %v, want none: nothing is counting down", *settings.SecondsLeft)
	}
}

/**
 * The clock is the one part of this the server has to own, because the host and everybody
 * watching have to be counting down to the same instant and their device clocks are not
 * comparable. So what a read returns is time remaining, measured by the same clock that
 * wrote the deadline.
 */
func TestArmRoundDeadlineStartsTheClockForMajorityRooms(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	// Host mode has nothing to count down, so arming is a no-op for it.
	if err := fixture.repository.ArmRoundDeadline(ctx, room.ID); err != nil {
		t.Fatalf("ArmRoundDeadline() error = %v", err)
	}
	settings, err := fixture.repository.Voting(ctx, room.ID)
	if err != nil {
		t.Fatalf("Voting() error = %v", err)
	}
	if settings.SecondsLeft != nil {
		t.Errorf("a host-decided room was given a deadline: %v", *settings.SecondsLeft)
	}

	if err := fixture.repository.SetVoting(ctx, room.ID, VoteModeMajority, 30); err != nil {
		t.Fatalf("SetVoting() error = %v", err)
	}
	// Writing the settings clears the deadline: the previous round's clock has nothing to
	// do with the mode that has just been chosen.
	if settings, err = fixture.repository.Voting(ctx, room.ID); err != nil {
		t.Fatalf("Voting() error = %v", err)
	}
	if settings.Mode != VoteModeMajority || settings.RoundSeconds != 30 {
		t.Fatalf("settings = %+v, want majority at 30 seconds", settings)
	}
	if settings.SecondsLeft != nil {
		t.Errorf("seconds left = %v, want none until a round is armed", *settings.SecondsLeft)
	}

	if err := fixture.repository.ArmRoundDeadline(ctx, room.ID); err != nil {
		t.Fatalf("ArmRoundDeadline() error = %v", err)
	}
	if settings, err = fixture.repository.Voting(ctx, room.ID); err != nil {
		t.Fatalf("Voting() error = %v", err)
	}
	if settings.SecondsLeft == nil {
		t.Fatalf("seconds left = nil, want a running clock")
	}
	// A round of 30 seconds read back immediately: allow for the round trip, but it must
	// not have been armed to some other length.
	if *settings.SecondsLeft > 30 || *settings.SecondsLeft < 25 {
		t.Errorf("seconds left = %v, want just under 30", *settings.SecondsLeft)
	}
}

/**
 * Manual rounds are the other half of what the host can ask for. The mode is majority, so
 * the room decides the winner, but nothing expires on its own — the host says when.
 */
func TestManualMajorityRoundsHaveNoDeadline(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)
	room := fixture.room(t, ctx)

	if err := fixture.repository.SetVoting(ctx, room.ID, VoteModeMajority, 0); err != nil {
		t.Fatalf("SetVoting() error = %v", err)
	}
	if err := fixture.repository.ArmRoundDeadline(ctx, room.ID); err != nil {
		t.Fatalf("ArmRoundDeadline() error = %v", err)
	}

	settings, err := fixture.repository.Voting(ctx, room.ID)
	if err != nil {
		t.Fatalf("Voting() error = %v", err)
	}
	if !settings.Majority() || settings.RoundSeconds != 0 {
		t.Fatalf("settings = %+v, want majority with no round length", settings)
	}
	if settings.SecondsLeft != nil {
		t.Errorf("seconds left = %v, want none: the host ends this round by hand", *settings.SecondsLeft)
	}
}

func TestVotingReportsAMissingRoom(t *testing.T) {
	fixture, ctx := newParticipationFixture(t)

	if _, err := fixture.repository.Voting(ctx, -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Voting() error = %v, want ErrNotFound", err)
	}
}

// ---------- the vote history ----------
//
// These borrow syntheticRoom from the settlement tests rather than the fixture above,
// because the history is an aggregate over what a real settlement wrote and asserting it
// against hand-written won_at columns would only prove the test agrees with itself.

// historyOf is the read under test, for a room and a caller.
func historyOf(t *testing.T, room syntheticRoom, database *sql.DB, caller string, limit int) []RoundVotes {
	t.Helper()
	anonymousID := ""
	if caller != "" {
		anonymousID = "anon-" + caller + "-" + room.serial
	}
	history, err := NewMySQLParticipation(database).
		RoundHistory(context.Background(), room.roomID, anonymousID, limit)
	if err != nil {
		t.Fatalf("RoundHistory() error = %v", err)
	}
	return history
}

// settleRound decides one round the way the worker does.
func settleRound(
	t *testing.T, database *sql.DB, room Room,
	winnerID, loserID int64, currentRound, ofRound, remainElements int,
) {
	t.Helper()
	if _, err := NewMySQLRepository(database, DefaultScoring()).SettleBets(
		context.Background(), BetOutcome{
			RoomID: room.ID, WinnerID: winnerID, LoserID: loserID,
			CurrentRound: currentRound, OfRound: ofRound, RemainElements: remainElements,
			VoteMode: room.VoteMode,
		}); err != nil {
		t.Fatalf("SettleBets() error = %v", err)
	}
}

func TestRoundHistoryReportsTheSplitThatDecidedTheRound(t *testing.T) {
	database := testDatabase(t)
	room := newSyntheticRoom(t, database)
	settled := room.majorityRoom(t, database)

	// Seven for the winner, three for the loser: the 70/30 split the taste boards pay
	// ±17 for, and the one a player wants to see again afterwards.
	majority := []string{"winner", "maj1", "maj2", "maj3", "maj4", "maj5", "maj6"}
	minority := []string{"loser", "min1", "min2"}
	for _, name := range append(append([]string{}, majority[1:]...), minority[1:]...) {
		room.seat(t, database, name)
	}
	for _, name := range majority {
		room.placeBet(t, database, name, room.winnerID, room.loserID, 1, 4, 8, 0)
	}
	for _, name := range minority {
		room.placeBet(t, database, name, room.loserID, room.winnerID, 1, 4, 8, 0)
	}
	settleRound(t, database, settled, room.winnerID, room.loserID, 1, 4, 7)

	history := historyOf(t, room, database, "min1", DefaultHistoryRounds)
	if len(history) != 1 {
		t.Fatalf("history = %+v, want one round", history)
	}
	round := history[0]
	if round.WinnerID != room.winnerID || round.LoserID != room.loserID {
		t.Errorf("pairing = %d/%d, want %d/%d",
			round.WinnerID, round.LoserID, room.winnerID, room.loserID)
	}
	if round.WinnerVotes != 7 || round.LoserVotes != 3 {
		t.Errorf("votes = %d/%d, want 7/3", round.WinnerVotes, round.LoserVotes)
	}
	// The bracket numbers are the ones the wagers were placed under, which is why
	// remain_elements reads 8 for a round settled down to 7.
	if round.CurrentRound != 1 || round.OfRound != 4 || round.RemainElements != 8 {
		t.Errorf("round = %d of %d with %d remaining, want 1 of 4 with 8",
			round.CurrentRound, round.OfRound, round.RemainElements)
	}
	// A minority voter's own pick is the element that lost, not the one that won.
	if round.YourPick != room.loserID {
		t.Errorf("your pick = %d, want the minority element %d", round.YourPick, room.loserID)
	}

	if majorityView := historyOf(t, room, database, "maj3", DefaultHistoryRounds); majorityView[0].YourPick != room.winnerID {
		t.Errorf("majority voter's pick = %d, want %d", majorityView[0].YourPick, room.winnerID)
	}
	// Somebody reading a room they never played sees the split and no pick of their own.
	stranger := historyOf(t, room, database, "", DefaultHistoryRounds)
	if len(stranger) != 1 || stranger[0].YourPick != 0 || stranger[0].WinnerVotes != 7 {
		t.Errorf("stranger's view = %+v, want the same split with no pick", stranger)
	}
}

// The fallback that only a host-decided room needs: with no winning wager to read the
// winning element from, the losers' rows still name it.
func TestRoundHistoryNamesTheWinnerWhenEveryVoterWasWrong(t *testing.T) {
	database := testDatabase(t)
	room := newSyntheticRoom(t, database)

	for _, name := range []string{"winner", "loser"} {
		room.placeBet(t, database, name, room.loserID, room.winnerID, 1, 4, 8, 0)
	}
	settleRound(t, database, room.hostRoom(), room.winnerID, room.loserID, 1, 4, 7)

	history := historyOf(t, room, database, "loser", DefaultHistoryRounds)
	if len(history) != 1 {
		t.Fatalf("history = %+v, want one round", history)
	}
	round := history[0]
	if round.WinnerID != room.winnerID || round.LoserID != room.loserID {
		t.Errorf("pairing = %d/%d, want %d/%d",
			round.WinnerID, round.LoserID, room.winnerID, room.loserID)
	}
	if round.WinnerVotes != 0 || round.LoserVotes != 2 {
		t.Errorf("votes = %d/%d, want 0/2", round.WinnerVotes, round.LoserVotes)
	}
}

func TestRoundHistoryIsNewestFirstAndOnlyDecidedRounds(t *testing.T) {
	database := testDatabase(t)
	room := newSyntheticRoom(t, database)

	room.placeBet(t, database, "winner", room.winnerID, room.loserID, 1, 4, 8, 0)
	settleRound(t, database, room.hostRoom(), room.winnerID, room.loserID, 1, 4, 7)

	room.placeBet(t, database, "winner", room.winnerID, room.otherID, 2, 4, 7, 0)
	settleRound(t, database, room.hostRoom(), room.winnerID, room.otherID, 2, 4, 6)

	// A round still being voted on. It has no split yet, so it belongs to the tally
	// rather than to the history.
	room.placeBet(t, database, "winner", room.loserID, room.otherID, 3, 4, 6, 0)

	history := historyOf(t, room, database, "winner", DefaultHistoryRounds)
	if len(history) != 2 {
		t.Fatalf("history = %+v, want the two decided rounds", history)
	}
	if history[0].CurrentRound != 2 || history[1].CurrentRound != 1 {
		t.Errorf("rounds = %d then %d, want 2 then 1",
			history[0].CurrentRound, history[1].CurrentRound)
	}

	if limited := historyOf(t, room, database, "winner", 1); len(limited) != 1 || limited[0].CurrentRound != 2 {
		t.Errorf("limited history = %+v, want only the newest round", limited)
	}
}

// A restarted bracket repeats the round numbers, and two different matches carrying the
// same ones must stay two rounds rather than merge into one impossible pairing.
//
// Two players, because the unique index allows a player only one wager per round key: a
// player who votes again after the restart has their pre-restart wager overwritten, so the
// rounds that survive a replay are held by whoever did not vote in it.
func TestRoundHistoryKeepsARepeatedRoundNumberApart(t *testing.T) {
	database := testDatabase(t)
	room := newSyntheticRoom(t, database)

	room.placeBet(t, database, "winner", room.winnerID, room.loserID, 1, 4, 8, 0)
	settleRound(t, database, room.hostRoom(), room.winnerID, room.loserID, 1, 4, 7)

	room.placeBet(t, database, "loser", room.winnerID, room.otherID, 1, 4, 8, 0)
	settleRound(t, database, room.hostRoom(), room.winnerID, room.otherID, 1, 4, 7)

	history := historyOf(t, room, database, "winner", DefaultHistoryRounds)
	if len(history) != 2 {
		t.Fatalf("history = %+v, want the two matches kept apart", history)
	}
	if history[0].LoserID != room.otherID || history[1].LoserID != room.loserID {
		t.Errorf("losers = %d then %d, want %d then %d",
			history[0].LoserID, history[1].LoserID, room.otherID, room.loserID)
	}
	// The reader wagered only on the first of them.
	if history[0].YourPick != 0 || history[1].YourPick != room.winnerID {
		t.Errorf("picks = %d then %d, want none then %d",
			history[0].YourPick, history[1].YourPick, room.winnerID)
	}
}
