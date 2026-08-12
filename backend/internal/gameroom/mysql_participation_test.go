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
