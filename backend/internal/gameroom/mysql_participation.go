package gameroom

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"2pick.app/backend/internal/platform/mysqlstore"
)

// MySQLParticipation implements ParticipationRepository.
type MySQLParticipation struct {
	database *sql.DB
}

func NewMySQLParticipation(database *sql.DB) *MySQLParticipation {
	return &MySQLParticipation{database: database}
}

const gameIDBySerialQuery = `SELECT id FROM games WHERE serial = ? LIMIT 1`

const roomByGameIDQuery = `SELECT id, serial FROM game_rooms WHERE game_id = ? LIMIT 1`

const insertRoomStatement = `
	INSERT INTO game_rooms (game_id, serial, created_at, updated_at)
	VALUES (?, ?, NOW(), NOW())`

// EnsureRoom is a read, then an insert, then the read again on a duplicate.
//
// The final re-read is the whole point and is not defensive padding: the unique index on
// game_id added in 00010 means a concurrent second caller's insert fails, and that caller
// must end up with the winner's room rather than an error. Laravel's firstOrCreate had no
// index to lose against, which is how two games ended up with two rooms each.
func (repository *MySQLParticipation) EnsureRoom(
	ctx context.Context, gameSerial, newSerial string,
) (Room, bool, error) {
	var gameID int64
	err := repository.database.QueryRowContext(ctx, gameIDBySerialQuery, gameSerial).Scan(&gameID)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, false, ErrGameNotFound
	}
	if err != nil {
		return Room{}, false, fmt.Errorf("gameroom: look up game %q: %w", gameSerial, err)
	}

	existing, found, err := repository.roomByGameID(ctx, gameID)
	if err != nil {
		return Room{}, false, err
	}
	if found {
		return existing, false, nil
	}

	result, err := repository.database.ExecContext(ctx, insertRoomStatement, gameID, newSerial)
	if err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			// Either another caller created the room, or the serial collided. Re-reading
			// by game covers the first, which is the case that actually happens.
			winner, found, readErr := repository.roomByGameID(ctx, gameID)
			if readErr != nil {
				return Room{}, false, readErr
			}
			if found {
				return winner, false, nil
			}
		}
		return Room{}, false, fmt.Errorf("gameroom: create room for game %d: %w", gameID, err)
	}

	roomID, err := result.LastInsertId()
	if err != nil {
		return Room{}, false, fmt.Errorf("gameroom: new room id: %w", err)
	}
	return Room{ID: roomID, Serial: newSerial}, true, nil
}

func (repository *MySQLParticipation) roomByGameID(ctx context.Context, gameID int64) (Room, bool, error) {
	var room Room
	err := repository.database.QueryRowContext(ctx, roomByGameIDQuery, gameID).Scan(&room.ID, &room.Serial)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, false, nil
	}
	if err != nil {
		return Room{}, false, fmt.Errorf("gameroom: look up room for game %d: %w", gameID, err)
	}
	return room, true, nil
}

// Rebinding a room: read the pair under a lock, then move the row.
//
// FOR UPDATE is not decoration. Two restarts racing (a double-tapped button, or two tabs
// of the same game) would otherwise both read the room on its old game and both move it,
// and the second move would be to a game the host has already left.
const roomForRebindQuery = `
	SELECT r.id, r.serial, g.serial, g.post_id
	  FROM game_rooms AS r
	  JOIN games AS g ON g.id = r.game_id
	 WHERE r.serial = ?
	 LIMIT 1
	   FOR UPDATE`

const gameForRebindQuery = `SELECT id, post_id FROM games WHERE serial = ? LIMIT 1`

const rebindRoomStatement = `UPDATE game_rooms SET game_id = ?, updated_at = NOW() WHERE id = ?`

// RebindRoom points a room at another game of the same post.
//
// The three refusals are all deliberate and all cheap to state:
//
// The room is not on fromGameSerial — the caller's idea of the room is stale, so it must
// not move it. Already on toGameSerial is the harmless half of that and returns the room:
// a retried request must not fail.
//
// The games belong to different posts. This is the one rule protecting people sitting in
// the room from having the content swapped under them.
//
// The target game already has a room of its own, which the unique index on game_id
// reports as a duplicate key. Nothing to do here: the host already has a room for that
// game and should be using it.
func (repository *MySQLParticipation) RebindRoom(
	ctx context.Context, roomSerial, fromGameSerial, toGameSerial string,
) (Room, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, fmt.Errorf("gameroom: begin rebind: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	var (
		room        Room
		boundSerial string
		boundPostID int64
	)
	err = transaction.QueryRowContext(ctx, roomForRebindQuery, roomSerial).
		Scan(&room.ID, &room.Serial, &boundSerial, &boundPostID)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, fmt.Errorf("gameroom: look up room %q for rebind: %w", roomSerial, err)
	}

	var (
		targetID     int64
		targetPostID int64
	)
	err = transaction.QueryRowContext(ctx, gameForRebindQuery, toGameSerial).
		Scan(&targetID, &targetPostID)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, ErrGameNotFound
	}
	if err != nil {
		return Room{}, fmt.Errorf("gameroom: look up game %q for rebind: %w", toGameSerial, err)
	}

	if boundSerial == toGameSerial {
		// Already moved. Committing an empty transaction rather than returning early
		// keeps the lock's lifetime obvious.
		if err := transaction.Commit(); err != nil {
			return Room{}, fmt.Errorf("gameroom: commit rebind: %w", err)
		}
		return room, nil
	}
	if boundSerial != fromGameSerial {
		return Room{}, ErrRoomMismatch
	}
	if targetPostID != boundPostID {
		return Room{}, ErrRoomNotRebindable
	}

	if _, err := transaction.ExecContext(ctx, rebindRoomStatement, targetID, room.ID); err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			return Room{}, ErrRoomNotRebindable
		}
		return Room{}, fmt.Errorf("gameroom: rebind room %q: %w", roomSerial, err)
	}
	if err := transaction.Commit(); err != nil {
		return Room{}, fmt.Errorf("gameroom: commit rebind: %w", err)
	}
	return room, nil
}

const roomWithGameQuery = `
	SELECT r.id, r.serial, g.serial
	  FROM game_rooms AS r
	  JOIN games AS g ON g.id = r.game_id
	 WHERE r.serial = ?
	 LIMIT 1`

func (repository *MySQLParticipation) RoomBySerialWithGame(
	ctx context.Context, roomSerial string,
) (Room, string, bool, error) {
	var (
		room       Room
		gameSerial string
	)
	err := repository.database.QueryRowContext(ctx, roomWithGameQuery, roomSerial).
		Scan(&room.ID, &room.Serial, &gameSerial)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, "", false, nil
	}
	if err != nil {
		return Room{}, "", false, fmt.Errorf("gameroom: look up room %q: %w", roomSerial, err)
	}
	return room, gameSerial, true, nil
}

// setOnScreenPairStatement records the pair a host is displaying.
//
// The WHERE clause is the validation: both ids must be live elements of that game, so a
// stale or invented pair updates nothing rather than putting a room's participants on a
// match that can never be settled.
const setOnScreenPairStatement = `
	UPDATE games AS g
	   SET g.candidates = CONCAT(?, ',', ?), g.updated_at = NOW()
	 WHERE g.serial = ?
	   AND EXISTS (SELECT 1 FROM game_elements AS ge
	                WHERE ge.game_id = g.id AND ge.element_id = ? AND ge.is_eliminated = 0)
	   AND EXISTS (SELECT 1 FROM game_elements AS ge
	                WHERE ge.game_id = g.id AND ge.element_id = ? AND ge.is_eliminated = 0)`

const votingQuery = `
	SELECT vote_mode, round_seconds,
	       TIMESTAMPDIFF(MICROSECOND, NOW(3), round_ends_at)
	  FROM game_rooms
	 WHERE id = ?
	 LIMIT 1`

// Voting reads a room's settings and what is left of the round in progress.
//
// The remainder is computed by MySQL rather than in Go, so the subtraction happens against
// the same clock and the same session timezone the deadline was written with. Reading the
// deadline out and comparing it to time.Now() here would compare a database timestamp to an
// application host's clock, which are two different machines in every deployment that
// matters.
func (repository *MySQLParticipation) Voting(
	ctx context.Context, roomID int64,
) (VotingSettings, error) {
	var settings VotingSettings
	var remaining sql.NullInt64
	err := repository.database.QueryRowContext(ctx, votingQuery, roomID).
		Scan(&settings.Mode, &settings.RoundSeconds, &remaining)
	if errors.Is(err, sql.ErrNoRows) {
		return VotingSettings{}, ErrNotFound
	}
	if err != nil {
		return VotingSettings{}, fmt.Errorf("gameroom: read the voting settings for room %d: %w", roomID, err)
	}
	if remaining.Valid {
		// Clamped at zero: an expired deadline means "no time left", and a negative
		// number would have every client render its own idea of what that means.
		seconds := float64(remaining.Int64) / 1e6
		if seconds < 0 {
			seconds = 0
		}
		settings.SecondsLeft = &seconds
	}
	return settings, nil
}

const setVotingStatement = `
	UPDATE game_rooms
	   SET vote_mode = ?, round_seconds = ?, round_ends_at = NULL, updated_at = NOW()
	 WHERE id = ?`

// SetVoting stores the mode and round length, and clears whatever deadline the old
// settings had armed.
//
// Clearing is not tidiness. A room switched back to host mode with a deadline still set
// would go on counting down for its participants, towards a settlement that nothing is
// going to perform. The caller arms the new one immediately afterwards when the new mode
// calls for it.
func (repository *MySQLParticipation) SetVoting(
	ctx context.Context, roomID int64, mode string, seconds int,
) error {
	if _, err := repository.database.ExecContext(ctx, setVotingStatement, mode, seconds, roomID); err != nil {
		return fmt.Errorf("gameroom: store the voting settings for room %d: %w", roomID, err)
	}
	return nil
}

const armRoundDeadlineStatement = `
	UPDATE game_rooms
	   SET round_ends_at = CASE WHEN vote_mode = ? AND round_seconds > 0
	                            THEN NOW(3) + INTERVAL round_seconds SECOND END,
	       updated_at = NOW()
	 WHERE id = ?`

// ArmRoundDeadline starts the clock on the round now on screen.
//
// One statement with the mode inside it, rather than a read of the settings followed by a
// write of the deadline: this runs on every round change of every room, and the CASE also
// makes the clearing automatic — a room that is not on a countdown ends up with NULL,
// which is exactly what "no deadline" means everywhere else.
func (repository *MySQLParticipation) ArmRoundDeadline(ctx context.Context, roomID int64) error {
	if _, err := repository.database.ExecContext(ctx,
		armRoundDeadlineStatement, VoteModeMajority, roomID); err != nil {
		return fmt.Errorf("gameroom: arm the round deadline for room %d: %w", roomID, err)
	}
	return nil
}

func (repository *MySQLParticipation) SetOnScreenPair(
	ctx context.Context, gameSerial string, first, second int64,
) error {
	if first <= 0 || second <= 0 || first == second {
		return nil
	}
	if _, err := repository.database.ExecContext(ctx, setOnScreenPairStatement,
		first, second, gameSerial, first, second); err != nil {
		return fmt.Errorf("gameroom: record the on-screen pair for game %q: %w", gameSerial, err)
	}
	return nil
}

const roomByGameSerialQuery = `
	SELECT r.id, r.serial
	  FROM game_rooms AS r
	  JOIN games AS g ON g.id = r.game_id
	 WHERE g.serial = ?
	 LIMIT 1`

// RoomByGameSerial reports the room hosting a game, or that there is none.
//
// A miss is the common case and not an error: most games are played solo. Returning
// false rather than ErrNotFound keeps the vote path from having to distinguish "no room"
// from "something broke" on every single batch.
func (repository *MySQLParticipation) RoomByGameSerial(
	ctx context.Context, gameSerial string,
) (Room, bool, error) {
	var room Room
	err := repository.database.QueryRowContext(ctx, roomByGameSerialQuery, gameSerial).
		Scan(&room.ID, &room.Serial)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, false, nil
	}
	if err != nil {
		return Room{}, false, fmt.Errorf("gameroom: look up the room for game %q: %w", gameSerial, err)
	}
	return room, true, nil
}

// participantColumns is shared by the lookup and the post-insert read so the two cannot
// drift into scanning different things.
const participantColumns = "id, game_room_id, user_id, anonymous_id, nickname, score, `rank`, " +
	"CAST(ROUND(accuracy * 100) AS SIGNED), total_played, total_correct, combo"

const participantQuery = "SELECT " + participantColumns +
	" FROM game_room_users WHERE game_room_id = ? AND anonymous_id = ? LIMIT 1"

const insertParticipantStatement = `
	INSERT INTO game_room_users
	       (game_room_id, user_id, anonymous_id, nickname, score, ` + "`rank`" + `,
	        accuracy, combo, total_played, total_correct, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0, 0, NOW(), NOW())`

// linkParticipantStatement records the account behind an existing row.
//
// Only ever fills in a NULL: a row that already names a user is left alone. Laravel wrote
// `user_id` unconditionally on every lookup, which meant a shared browser could reassign
// someone else's participant row to whoever logged in last.
const linkParticipantStatement = `
	UPDATE game_room_users SET user_id = ?, updated_at = NOW()
	 WHERE id = ? AND user_id IS NULL`

func (repository *MySQLParticipation) EnsureParticipant(
	ctx context.Context, roomID int64, anonymousID string, userID *int64,
	nickname string, startingScore int,
) (Participant, error) {
	participant, found, err := repository.participant(ctx, roomID, anonymousID)
	if err != nil {
		return Participant{}, err
	}
	if found {
		if userID != nil && participant.UserID == nil {
			if _, err := repository.database.ExecContext(ctx,
				linkParticipantStatement, *userID, participant.ID); err != nil {
				return Participant{}, fmt.Errorf("gameroom: link participant %d: %w", participant.ID, err)
			}
			participant.UserID = userID
		}
		return participant, nil
	}

	_, err = repository.database.ExecContext(ctx, insertParticipantStatement,
		roomID, userID, anonymousID, nickname, startingScore)
	if err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			// The participant unique index from 00012: a simultaneous first request won.
			// Re-reading gives that request's row, which is the one both callers want.
			winner, found, readErr := repository.participant(ctx, roomID, anonymousID)
			if readErr != nil {
				return Participant{}, readErr
			}
			if found {
				return winner, nil
			}
		}
		return Participant{}, fmt.Errorf("gameroom: create participant in room %d: %w", roomID, err)
	}

	// Re-read rather than assembling the row from the arguments: the columns carry
	// defaults, and a hand-built struct would drift the moment one of them changes.
	created, found, err := repository.participant(ctx, roomID, anonymousID)
	if err != nil {
		return Participant{}, err
	}
	if !found {
		return Participant{}, fmt.Errorf("gameroom: the participant just created is not readable")
	}
	return created, nil
}

func (repository *MySQLParticipation) participant(
	ctx context.Context, roomID int64, anonymousID string,
) (Participant, bool, error) {
	var (
		participant Participant
		userID      sql.NullInt64
	)
	err := repository.database.QueryRowContext(ctx, participantQuery, roomID, anonymousID).Scan(
		&participant.ID, &participant.RoomID, &userID, &participant.AnonymousID,
		&participant.Nickname, &participant.Score, &participant.Rank,
		&participant.AccuracyHundredths, &participant.TotalPlayed,
		&participant.TotalCorrect, &participant.Combo)
	if errors.Is(err, sql.ErrNoRows) {
		return Participant{}, false, nil
	}
	if err != nil {
		return Participant{}, false, fmt.Errorf("gameroom: read participant: %w", err)
	}
	if userID.Valid {
		participant.UserID = &userID.Int64
	}
	return participant, true, nil
}

const previousBetQuery = `
	SELECT last_combo, won_at IS NOT NULL
	  FROM game_room_user_bets
	 WHERE game_room_user_id = ?
	 ORDER BY id DESC
	 LIMIT 1`

func (repository *MySQLParticipation) PreviousBetStreak(
	ctx context.Context, participantID int64,
) (int, bool, bool, error) {
	var (
		lastCombo int
		won       bool
	)
	err := repository.database.QueryRowContext(ctx, previousBetQuery, participantID).Scan(&lastCombo, &won)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, fmt.Errorf("gameroom: read the previous wager: %w", err)
	}
	return lastCombo, won, true, nil
}

// upsertBetStatement replaces the caller's wager on the same round.
//
// ON DUPLICATE KEY UPDATE against the unique index added in 00011, rather than the
// SELECT-then-write Laravel's updateOrCreate performed. That is what makes a
// double-clicked vote one row instead of two — the defect that double-counted thirteen
// rounds in production.
//
// won_at and lost_at are deliberately reset: changing a vote un-settles it, and the
// settlement pass will decide it again. Leaving a stale outcome would score the new vote
// with the old result.
const upsertBetStatement = `
	INSERT INTO game_room_user_bets
	       (game_room_id, game_room_user_id, current_round, of_round, remain_elements,
	        winner_id, loser_id, last_combo, score, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, NOW(), NOW())
	ON DUPLICATE KEY UPDATE
	        winner_id = VALUES(winner_id),
	        loser_id = VALUES(loser_id),
	        last_combo = VALUES(last_combo),
	        score = 0,
	        won_at = NULL,
	        lost_at = NULL,
	        updated_at = NOW()`

func (repository *MySQLParticipation) UpsertBet(
	ctx context.Context, roomID, participantID int64, bet PlacedBet, lastCombo int,
) error {
	_, err := repository.database.ExecContext(ctx, upsertBetStatement,
		roomID, participantID, bet.CurrentRound, bet.OfRound, bet.RemainElements,
		bet.WinnerID, bet.LoserID, lastCombo)
	if err != nil {
		return fmt.Errorf("gameroom: record wager for participant %d: %w", participantID, err)
	}
	return nil
}

const renameStatement = `UPDATE game_room_users SET nickname = ?, updated_at = NOW() WHERE id = ?`

func (repository *MySQLParticipation) Rename(ctx context.Context, participantID int64, nickname string) error {
	if _, err := repository.database.ExecContext(ctx, renameStatement, nickname, participantID); err != nil {
		return fmt.Errorf("gameroom: rename participant %d: %w", participantID, err)
	}
	return nil
}

const latestBetQuery = `
	SELECT winner_id, loser_id, current_round, of_round, remain_elements
	  FROM game_room_user_bets
	 WHERE game_room_user_id = ?
	 ORDER BY id DESC
	 LIMIT 1`

func (repository *MySQLParticipation) LatestBet(
	ctx context.Context, participantID int64,
) (PlacedBet, bool, error) {
	var bet PlacedBet
	err := repository.database.QueryRowContext(ctx, latestBetQuery, participantID).Scan(
		&bet.WinnerID, &bet.LoserID, &bet.CurrentRound, &bet.OfRound, &bet.RemainElements)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacedBet{}, false, nil
	}
	if err != nil {
		return PlacedBet{}, false, fmt.Errorf("gameroom: read the latest wager: %w", err)
	}
	return bet, true, nil
}

// roundStateQuery reads the pairing on screen and the last COMPLETED match.
//
// remain_elements comes from the newest round row, falling back to the game's element
// count when no match has been played — matching GameRoomVoteResource, which reads the
// same two places in the same order. current_round and of_round come from that same row
// and are advanced in Go by NextRound, because "the match in progress" is one past the
// last row and crossing a bracket boundary is not something SQL should be deciding.
//
// One subquery per column rather than a join: game_1v1_rounds is 13.6 GiB and the index
// on game_id makes each of these an index seek, while a LATERAL join would need MySQL 8.0
// and buys nothing here.
const roundStateQuery = `
	SELECT g.candidates,
	       g.element_count,
	       (SELECT r.remain_elements FROM game_1v1_rounds AS r
	         WHERE r.game_id = g.id ORDER BY r.id DESC LIMIT 1),
	       (SELECT r.current_round FROM game_1v1_rounds AS r
	         WHERE r.game_id = g.id ORDER BY r.id DESC LIMIT 1),
	       (SELECT r.of_round FROM game_1v1_rounds AS r
	         WHERE r.game_id = g.id ORDER BY r.id DESC LIMIT 1)
	  FROM games AS g
	 WHERE g.serial = ?
	 LIMIT 1`

// RoundInProgress reads the match the room is voting on.
func (repository *MySQLParticipation) RoundInProgress(
	ctx context.Context, gameSerial string,
) (RoundInProgress, error) {
	var (
		candidates       sql.NullString
		elementCount     int
		lastRemain       sql.NullInt64
		lastCurrentRound sql.NullInt64
		lastOfRound      sql.NullInt64
	)
	err := repository.database.QueryRowContext(ctx, roundStateQuery, gameSerial).
		Scan(&candidates, &elementCount, &lastRemain, &lastCurrentRound, &lastOfRound)
	if errors.Is(err, sql.ErrNoRows) {
		return RoundInProgress{}, ErrGameNotFound
	}
	if err != nil {
		return RoundInProgress{}, fmt.Errorf("gameroom: read the round state: %w", err)
	}

	remain := elementCount
	if lastRemain.Valid {
		remain = int(lastRemain.Int64)
	}
	currentRound, ofRound := NextRound(int(lastCurrentRound.Int64), int(lastOfRound.Int64), remain)

	round := RoundInProgress{
		RemainElements: remain,
		CurrentRound:   currentRound,
		OfRound:        ofRound,
	}
	if first, second, ok := parseCandidatePair(candidates.String); ok {
		round.FirstCandidate, round.SecondCandidate, round.HasPairing = first, second, true
	}
	return round, nil
}

// voteTallyQuery counts both directions of the current pairing in one pass.
//
// One query with conditional sums rather than the two COUNT queries the PHP resource
// runs: the numbers are shown side by side, and two queries can disagree if a vote lands
// between them.
const voteTallyQuery = `
	SELECT COALESCE(SUM(winner_id = ? AND loser_id = ?), 0),
	       COALESCE(SUM(winner_id = ? AND loser_id = ?), 0)
	  FROM game_room_user_bets
	 WHERE game_room_id = ?
	   AND remain_elements = ?`

func (repository *MySQLParticipation) CurrentVotes(
	ctx context.Context, roomID int64, gameSerial string,
) (VoteTally, bool, error) {
	round, err := repository.RoundInProgress(ctx, gameSerial)
	if err != nil {
		return VoteTally{}, false, err
	}
	if !round.HasPairing {
		// No pairing in progress. GameRoomVoteResource answers with an empty object here
		// rather than an error, and so does this.
		return VoteTally{}, false, nil
	}

	var firstVotes, secondVotes int
	if err := repository.database.QueryRowContext(ctx, voteTallyQuery,
		round.FirstCandidate, round.SecondCandidate,
		round.SecondCandidate, round.FirstCandidate,
		roomID, round.RemainElements,
	).Scan(&firstVotes, &secondVotes); err != nil {
		return VoteTally{}, false, fmt.Errorf("gameroom: tally the round: %w", err)
	}

	return VoteTally{
		FirstCandidate:      round.FirstCandidate,
		SecondCandidate:     round.SecondCandidate,
		FirstCandidateVotes: firstVotes,
		SecondCandidateVote: secondVotes,
		RemainElements:      round.RemainElements,
		TotalVotes:          firstVotes + secondVotes,
		CurrentRound:        round.CurrentRound,
		OfRound:             round.OfRound,
	}, true, nil
}

// roundHistoryQuery reads the rounds a room has already decided, newest first.
//
// THE ROUND IS RECONSTRUCTED FROM THE WAGERS, because that is the only place it was ever
// written: settling a round marks the wagers on the winning element won_at and flips the
// pair on the losing ones, so counting the two flags counts the two sides.
//
// GROUPED BY THE ROUND KEY *AND* THE PAIRING. current_round/of_round/remain_elements repeat
// when a host restarts and the room follows them onto a new bracket, and two different
// matches that happen to share those numbers must stay two rows rather than merge into one
// impossible round. LEAST/GREATEST rather than the pair itself, so the winners and the
// losers of the same match — who hold it in opposite orders — group together.
//
// THE WINNING ELEMENT HAS A FALLBACK for a host-decided room, where every voter can be
// wrong: with no won_at row to read it from, the losers' loser_id is the element that won.
//
// your_pick rides along on a join instead of a second query, and matching an anonymous id
// never creates a participant, so reading a room's history leaves the reader off its
// leaderboard.
const roundHistoryQuery = `
	SELECT COALESCE(MAX(CASE WHEN bet.won_at IS NOT NULL THEN bet.winner_id END),
	                MAX(CASE WHEN bet.lost_at IS NOT NULL THEN bet.loser_id END)),
	       COALESCE(MAX(CASE WHEN bet.won_at IS NOT NULL THEN bet.loser_id END),
	                MAX(CASE WHEN bet.lost_at IS NOT NULL THEN bet.winner_id END)),
	       COALESCE(SUM(bet.won_at IS NOT NULL), 0),
	       COALESCE(SUM(bet.lost_at IS NOT NULL), 0),
	       bet.current_round, bet.of_round, bet.remain_elements,
	       COALESCE(MAX(CASE WHEN player.anonymous_id = ? THEN bet.winner_id END), 0)
	  FROM game_room_user_bets bet
	  JOIN game_room_users player ON player.id = bet.game_room_user_id
	 WHERE bet.game_room_id = ?
	   AND (bet.won_at IS NOT NULL OR bet.lost_at IS NOT NULL)
	 GROUP BY bet.current_round, bet.of_round, bet.remain_elements,
	          LEAST(bet.winner_id, bet.loser_id), GREATEST(bet.winner_id, bet.loser_id)
	 ORDER BY MAX(bet.id) DESC
	 LIMIT ?`

func (repository *MySQLParticipation) RoundHistory(
	ctx context.Context, roomID int64, anonymousID string, limit int,
) ([]RoundVotes, error) {
	rows, err := repository.database.QueryContext(ctx, roundHistoryQuery, anonymousID, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("gameroom: read the round history: %w", err)
	}
	defer rows.Close()

	history := make([]RoundVotes, 0, limit)
	for rows.Next() {
		var round RoundVotes
		if err := rows.Scan(
			&round.WinnerID, &round.LoserID,
			&round.WinnerVotes, &round.LoserVotes,
			&round.CurrentRound, &round.OfRound, &round.RemainElements,
			&round.YourPick,
		); err != nil {
			return nil, fmt.Errorf("gameroom: read the round history: %w", err)
		}
		history = append(history, round)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gameroom: read the round history: %w", err)
	}
	return history, nil
}

// parseCandidatePair reads the comma-separated pair the games table stores.
func parseCandidatePair(value string) (first, second int64, ok bool) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	first, firstErr := parseElementID(parts[0])
	second, secondErr := parseElementID(parts[1])
	if firstErr != nil || secondErr != nil || first == second {
		return 0, 0, false
	}
	return first, second, true
}

func parseElementID(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("gameroom: empty element id")
	}
	var id int64
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil || id <= 0 {
		return 0, fmt.Errorf("gameroom: %q is not an element id", value)
	}
	return id, nil
}
