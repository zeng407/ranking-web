package gameroom

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// MySQLRepository implements Repository.
type MySQLRepository struct {
	database *sql.DB
	scoring  Scoring
}

func NewMySQLRepository(database *sql.DB, scoring Scoring) *MySQLRepository {
	return &MySQLRepository{database: database, scoring: scoring}
}

// vote_mode comes along because it decides what a round pays: the worker resolves the
// room once per message and both the settlement and the recompute need the rules.
const roomBySerialQuery = `SELECT id, serial, vote_mode FROM game_rooms WHERE serial = ?`

func (repository *MySQLRepository) RoomBySerial(ctx context.Context, serial string) (Room, error) {
	var room Room
	err := repository.database.QueryRowContext(ctx, roomBySerialQuery, serial).
		Scan(&room.ID, &room.Serial, &room.VoteMode)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, fmt.Errorf("%w: serial %q", ErrNotFound, serial)
	}
	if err != nil {
		return Room{}, fmt.Errorf("gameroom: load room %q: %w", serial, err)
	}
	return room, nil
}

// The three settlement statements, from GameService::updateGameBet.
//
// remain_elements is matched against the value passed +1 because the wagers were
// placed while one more element was still in play. The score expressions are the
// scoring rules inlined: they must be parameters rather than literals so a change
// in config/setting.php does not silently diverge from this file.
const (
	settleWinnersStatement = `
		UPDATE game_room_user_bets
		   SET won_at = NOW(),
		       score  = last_combo * ? + ?,
		       updated_at = NOW()
		 WHERE game_room_id = ?
		   AND current_round = ?
		   AND of_round = ?
		   AND remain_elements = ?
		   AND winner_id = ?
		   AND loser_id = ?`

	// settleWinnersFlatStatement is the winning side of a round the ROOM decided. Same
	// shape as the losing side: one score for everyone who was on that side, resolved by
	// MajorityPayout before the statement runs. No last_combo term, because a majority
	// round pays for agreeing with the room rather than for a run of wins.
	settleWinnersFlatStatement = `
		UPDATE game_room_user_bets
		   SET won_at = NOW(),
		       score  = ?,
		       updated_at = NOW()
		 WHERE game_room_id = ?
		   AND current_round = ?
		   AND of_round = ?
		   AND remain_elements = ?
		   AND winner_id = ?
		   AND loser_id = ?`

	settleLosersStatement = `
		UPDATE game_room_user_bets
		   SET lost_at = NOW(),
		       score  = ?,
		       updated_at = NOW()
		 WHERE game_room_id = ?
		   AND current_round = ?
		   AND of_round = ?
		   AND remain_elements = ?
		   AND winner_id = ?
		   AND loser_id = ?`

	// A wager that matched neither side is a bet on a pairing this round did not present,
	// so it is discarded rather than carried into the next round.
	//
	// SCOPED TO THIS ROUND EXACTLY, BECAUSE SETTLES ARE NOT ORDERED.
	//
	// The obvious scope is "this round and everything older", since remain_elements counts
	// down and an older wager has a larger value. That is wrong, and reproducibly so: the
	// worker runs four jobs at once and serialises a room's settles with a lock, but the
	// order in which four queued settles acquire that lock is not the order they were
	// published in. When round 2 settles first, "everything older" includes round 1's
	// wagers — which have not been settled yet — so they are deleted and round 1's settle
	// then matches nothing.
	//
	// Measured on an 8-element room where the participant backed the host's pick every
	// time: paced, all four settled (score 1100). Unpaced, three did (1030), and the worker
	// logged "won=1 discarded=2" on the second settle.
	//
	// So each settle touches only its own round. Wagers from a round whose settle never
	// arrives do linger unsettled, which costs a row: the tally counts SETTLED wagers only,
	// so a lingering one changes nobody's score. Losing a valid wager silently, which is
	// what the wider scope did, is the worse failure.
	discardUnsettledStatement = `
		DELETE FROM game_room_user_bets
		 WHERE game_room_id = ?
		   AND won_at IS NULL
		   AND lost_at IS NULL
		   AND remain_elements = ?`
)

// majorityRoundVotesQuery counts how the room split on the round being settled.
//
// THE SAME PREDICATE AS THE TWO UPDATES, DELIBERATELY. The payout's denominator has to be
// exactly the set of rows the payout is written to — everyone counted is paid and everyone
// paid is counted — so this matches on the round and both orientations of the pair rather
// than on the looser scope voteTallyQuery uses to render the tally.
//
// No won_at filter either. A row already settled by an earlier delivery of the same
// message is still a vote that was cast, so a redelivery counts the same rows and
// recomputes the same magnitude.
const majorityRoundVotesQuery = `
	SELECT COALESCE(SUM(winner_id = ? AND loser_id = ?), 0), COUNT(*)
	  FROM game_room_user_bets
	 WHERE game_room_id = ?
	   AND current_round = ?
	   AND of_round = ?
	   AND remain_elements = ?
	   AND ((winner_id = ? AND loser_id = ?) OR (winner_id = ? AND loser_id = ?))`

func (repository *MySQLRepository) SettleBets(ctx context.Context, outcome BetOutcome) (SettleResult, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return SettleResult{}, fmt.Errorf("gameroom: begin settlement: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	// The wagers under settlement were placed with one more element in play.
	placedAtRemaining := outcome.RemainElements + 1

	// Host rules by default: the streak bonus on the winning side, the flat penalty on
	// the losing one.
	winners := settleWinnersStatement
	winnerArgs := []any{repository.scoring.ComboScore, repository.scoring.WonScore}
	loserScore := repository.scoring.LoseScore

	// A round the ROOM decided pays by how the room split, the same magnitude to both
	// sides. The split is read before either side is written and inside this transaction,
	// so a wager arriving mid-settle cannot land between the count and the update it
	// would have changed.
	if outcome.VoteMode == VoteModeMajority {
		var winnerVotes, totalVotes int
		if err := transaction.QueryRowContext(ctx, majorityRoundVotesQuery,
			outcome.WinnerID, outcome.LoserID,
			outcome.RoomID, outcome.CurrentRound, outcome.OfRound, placedAtRemaining,
			outcome.WinnerID, outcome.LoserID, outcome.LoserID, outcome.WinnerID,
		).Scan(&winnerVotes, &totalVotes); err != nil {
			return SettleResult{}, fmt.Errorf("gameroom: count the round's votes: %w", err)
		}
		payout := MajorityPayout(winnerVotes, totalVotes, repository.scoring)
		winners = settleWinnersFlatStatement
		winnerArgs = []any{payout}
		loserScore = -payout
	}

	winnerArgs = append(winnerArgs,
		outcome.RoomID, outcome.CurrentRound, outcome.OfRound, placedAtRemaining,
		outcome.WinnerID, outcome.LoserID)

	won, err := transaction.ExecContext(ctx, winners, winnerArgs...)
	if err != nil {
		return SettleResult{}, fmt.Errorf("gameroom: settle winning wagers: %w", err)
	}

	// The losing side is the same round with the pair reversed.
	lost, err := transaction.ExecContext(ctx, settleLosersStatement,
		loserScore,
		outcome.RoomID, outcome.CurrentRound, outcome.OfRound, placedAtRemaining,
		outcome.LoserID, outcome.WinnerID)
	if err != nil {
		return SettleResult{}, fmt.Errorf("gameroom: settle losing wagers: %w", err)
	}

	discarded, err := transaction.ExecContext(ctx, discardUnsettledStatement, outcome.RoomID, placedAtRemaining)
	if err != nil {
		return SettleResult{}, fmt.Errorf("gameroom: discard unsettled wagers: %w", err)
	}

	if err := transaction.Commit(); err != nil {
		return SettleResult{}, fmt.Errorf("gameroom: commit settlement: %w", err)
	}

	result := SettleResult{}
	result.Won, _ = won.RowsAffected()
	result.Lost, _ = lost.RowsAffected()
	result.Discarded, _ = discarded.RowsAffected()
	return result, nil
}

// recomputeTotalsStatement rebuilds every player's standing in one statement.
//
// This replaces a loop that ran two queries per player — a SELECT of their wagers
// and an UPDATE — plus a second loop of one UPDATE each for the rank. In the largest
// production room, 1,088 players, that was over 3,200 round trips per vote. It is
// now three statements regardless of room size.
//
// LEFT JOIN, not JOIN: the PHP iterated every player in the room rather than every
// player with a wager, so someone who has joined and not yet bet is still written —
// which is what gives them the starting score, since the column defaults to 0.
// All 5,329 such players in production hold the starting score, confirming this.
//
// The window functions carry the per-player totals onto the newest wager row, so one
// pass yields both the sums and the streak that the newest wager decides.
//
// Only settled wagers are counted, mirroring Tally. A wager whose round has not been
// decided contributes nothing until it is; the filter belongs in the innermost select
// so it also decides which row is "newest" for the streak.
const recomputeTotalsStatement = `
	UPDATE game_room_users AS target
	LEFT JOIN (
		SELECT streaked.game_room_user_id,
		       COUNT(*)                                                 AS played,
		       SUM(streaked.won)                                        AS correct,
		       SUM(IF(streaked.won, streaked.streak * ? + ?, ?))        AS bet_score,
		       MAX(IF(streaked.from_end = 1, streaked.won, 0))          AS last_won,
		       MAX(IF(streaked.from_end = 1, streaked.streak + 1, 0))   AS next_streak
		  FROM (
			SELECT marked.game_room_user_id,
			       marked.won,
			       -- The streak this wager rode on: its position within the run of wagers
			       -- since the last loss, zero-based.
			       ROW_NUMBER() OVER (PARTITION BY marked.game_room_user_id, marked.losses_before
			                              ORDER BY marked.remain_elements DESC, marked.id) - 1 AS streak,
			       ROW_NUMBER() OVER (PARTITION BY marked.game_room_user_id
			                              ORDER BY marked.remain_elements ASC, marked.id DESC) AS from_end
			  FROM (
				SELECT settled.id,
				       settled.game_room_user_id,
				       settled.remain_elements,
				       settled.won,
				       -- Losses strictly before this wager. Rows sharing a value form one
				       -- run since the last loss, which is what the streak counts within.
				       COALESCE(SUM(settled.lost) OVER (
				           PARTITION BY settled.game_room_user_id
				               ORDER BY settled.remain_elements DESC, settled.id
				               ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) AS losses_before
				  FROM (
					SELECT bet.id,
					       bet.game_room_user_id,
					       bet.remain_elements,
					       (bet.won_at IS NOT NULL)  AS won,
					       (bet.lost_at IS NOT NULL) AS lost
					  FROM game_room_user_bets AS bet
					 WHERE bet.game_room_id = ?
					   AND (bet.won_at IS NOT NULL OR bet.lost_at IS NOT NULL)
				  ) AS settled
			  ) AS marked
		  ) AS streaked
		 GROUP BY streaked.game_room_user_id
	) AS tally ON tally.game_room_user_id = target.id
	   SET target.total_played  = COALESCE(tally.played, 0),
	       target.total_correct = COALESCE(tally.correct, 0),
	       target.score         = ? + COALESCE(tally.bet_score, 0),
	       target.combo         = IF(tally.last_won, tally.next_streak, 0),
	       target.accuracy      = IF(COALESCE(tally.played, 0) = 0, 0,
	                                 ROUND(tally.correct * 10000 / tally.played) / 100),
	       target.updated_at    = NOW()
	 WHERE target.game_room_id = ?`

// recomputeMajorityTotalsStatement is the same rebuild for a room that decides its own
// rounds: no window functions, because there is no streak to find.
//
// The score is the SUM of what the settlements wrote. A majority round pays by how the
// whole room split, which is not visible from one player's wagers, so the row carries the
// answer instead of a record of it — see Tally and MajorityPayout. Combo is zeroed rather
// than left alone: a room switched into this mode would otherwise keep showing the streak
// it had under host rules, which nothing here pays for any more.
//
// Everything else matches the host statement exactly, including the LEFT JOIN that gives a
// player who has not bet the starting score, and counting settled wagers only.
const recomputeMajorityTotalsStatement = `
	UPDATE game_room_users AS target
	LEFT JOIN (
		SELECT bet.game_room_user_id,
		       COUNT(*)                       AS played,
		       SUM(bet.won_at IS NOT NULL)    AS correct,
		       SUM(bet.score)                 AS bet_score
		  FROM game_room_user_bets AS bet
		 WHERE bet.game_room_id = ?
		   AND (bet.won_at IS NOT NULL OR bet.lost_at IS NOT NULL)
		 GROUP BY bet.game_room_user_id
	) AS tally ON tally.game_room_user_id = target.id
	   SET target.total_played  = COALESCE(tally.played, 0),
	       target.total_correct = COALESCE(tally.correct, 0),
	       target.score         = ? + COALESCE(tally.bet_score, 0),
	       target.combo         = 0,
	       target.accuracy      = IF(COALESCE(tally.played, 0) = 0, 0,
	                                 ROUND(tally.correct * 10000 / tally.played) / 100),
	       target.updated_at    = NOW()
	 WHERE target.game_room_id = ?`

// RecomputeTotals rebuilds every player's standing in a room from their settled wagers,
// under the room's own rules.
//
// Under host rules the streak, and therefore the combo bonus in each wager's payout, is
// DERIVED here rather than read from the row the settlement wrote. See Tally for why:
// last_combo is resolved when a wager is placed, from the wager before it, and that is only
// right if the previous one had already settled — which a player betting faster than the
// host votes cannot rely on. Deriving it makes the bonus a function of which wagers won, in
// round order, and nothing else.
//
// A room that decides its own rounds sums the stored scores instead, for the opposite
// reason: what a round paid there depends on the other players, so the settlement is the
// only place that could resolve it.
//
// One consequence worth stating: switching a room's mode re-scores its whole history under
// the new rules, including rounds played under the old ones. That is the honest reading of
// "the room's rules changed" — the alternative is a leaderboard scored by two rules at once
// with no way to tell which row used which.
func (repository *MySQLRepository) RecomputeTotals(ctx context.Context, room Room) (int64, error) {
	statement := recomputeTotalsStatement
	// The payout expression, then the room, then the starting score, then the room again
	// for the outer WHERE.
	args := []any{
		repository.scoring.ComboScore, repository.scoring.WonScore, repository.scoring.LoseScore,
		room.ID, repository.scoring.DefaultScore, room.ID,
	}
	if room.VoteMode == VoteModeMajority {
		// No payout expression: the rows already hold what they paid.
		statement = recomputeMajorityTotalsStatement
		args = []any{room.ID, repository.scoring.DefaultScore, room.ID}
	}

	result, err := repository.database.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, fmt.Errorf("gameroom: recompute totals for room %d: %w", room.ID, err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// assignRanksStatement numbers the room 1..N by score.
//
// `rank` is a reserved word in MySQL 8 (it is a window function), so the column must
// be quoted. The tiebreak on id is what makes the order stable between refreshes;
// see AssignRanks.
const assignRanksStatement = "UPDATE game_room_users AS target\n" +
	"	JOIN (\n" +
	"		SELECT player.id,\n" +
	"		       ROW_NUMBER() OVER (ORDER BY player.score DESC, player.id) AS position\n" +
	"		  FROM game_room_users AS player\n" +
	"		 WHERE player.game_room_id = ?\n" +
	"	) AS ordered ON ordered.id = target.id\n" +
	"	   SET target.`rank` = ordered.position,\n" +
	"	       target.updated_at = NOW()"

func (repository *MySQLRepository) AssignRanks(ctx context.Context, roomID int64) (int64, error) {
	result, err := repository.database.ExecContext(ctx, assignRanksStatement, roomID)
	if err != nil {
		return 0, fmt.Errorf("gameroom: assign ranks for room %d: %w", roomID, err)
	}
	// MySQL counts only rows whose values changed, so this is the number of players
	// who actually moved, not the room size.
	affected, _ := result.RowsAffected()
	return affected, nil
}

const (
	totalUsersQuery = `SELECT COUNT(*) FROM game_room_users WHERE game_room_id = ?`

	// Only ranked players appear in the payload, matching where('rank', '>', 0).
	// A player with rank 0 has never been through a refresh.
	leaderboardQuery = "SELECT id, anonymous_id, nickname, score, `rank`, total_played, total_correct, combo, accuracy\n" +
		"	  FROM game_room_users\n" +
		"	 WHERE game_room_id = ? AND `rank` > 0\n" +
		"	 ORDER BY `rank` %s\n" +
		"	 LIMIT ?"
)

// LeaderboardSize is how many players each end of the payload carries, from the
// limit(10) in CacheService::rememberGameBetRank.
const LeaderboardSize = 10

func (repository *MySQLRepository) Leaderboard(ctx context.Context, roomID int64) (Leaderboard, error) {
	var board Leaderboard

	// Counts every player, ranked or not, which is what users()->count() did.
	if err := repository.database.QueryRowContext(ctx, totalUsersQuery, roomID).Scan(&board.TotalUsers); err != nil {
		return Leaderboard{}, fmt.Errorf("gameroom: count players in room %d: %w", roomID, err)
	}

	top, err := repository.players(ctx, roomID, "ASC")
	if err != nil {
		return Leaderboard{}, err
	}
	bottom, err := repository.players(ctx, roomID, "DESC")
	if err != nil {
		return Leaderboard{}, err
	}

	board.Top10 = top
	board.Bottom10 = bottom
	return board, nil
}

func (repository *MySQLRepository) players(ctx context.Context, roomID int64, direction string) ([]Player, error) {
	// direction is one of two literals chosen here, never caller input.
	query := fmt.Sprintf(leaderboardQuery, direction)
	rows, err := repository.database.QueryContext(ctx, query, roomID, LeaderboardSize)
	if err != nil {
		return nil, fmt.Errorf("gameroom: query leaderboard for room %d: %w", roomID, err)
	}
	defer rows.Close()

	players := make([]Player, 0, LeaderboardSize)
	for rows.Next() {
		var (
			id           int64
			anonymousID  string
			nickname     string
			score        int
			rank         int
			totalPlayed  int
			totalCorrect int
			combo        int
			// Scanned as a string: the column is decimal(5,2) and the payload carries
			// the two-decimal text, so parsing it into a float and formatting it back
			// would only add a rounding step.
			accuracy string
		)
		if err := rows.Scan(&id, &anonymousID, &nickname, &score, &rank,
			&totalPlayed, &totalCorrect, &combo, &accuracy); err != nil {
			return nil, fmt.Errorf("gameroom: scan leaderboard row: %w", err)
		}
		players = append(players, Player{
			UserID:       PlayerID(id, anonymousID),
			Name:         nickname,
			Score:        score,
			Rank:         rank,
			Accuracy:     accuracy,
			TotalPlayed:  totalPlayed,
			TotalCorrect: totalCorrect,
			Combo:        combo,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gameroom: read leaderboard: %w", err)
	}
	return players, nil
}

const standingsQuery = `SELECT id, score FROM game_room_users WHERE game_room_id = ?`

func (repository *MySQLRepository) Standings(ctx context.Context, roomID int64) ([]Standing, error) {
	rows, err := repository.database.QueryContext(ctx, standingsQuery, roomID)
	if err != nil {
		return nil, fmt.Errorf("gameroom: query standings for room %d: %w", roomID, err)
	}
	defer rows.Close()

	standings := make([]Standing, 0)
	for rows.Next() {
		var standing Standing
		if err := rows.Scan(&standing.UserID, &standing.Score); err != nil {
			return nil, fmt.Errorf("gameroom: scan standing: %w", err)
		}
		standings = append(standings, standing)
	}
	return standings, rows.Err()
}

// betsByPlayerQuery reads every wager in the room, ordered so Tally sees them in
// the same order the set-based statement aggregates them.
//
// Unsettled wagers are read too, deliberately. Tally is the oracle the parity test
// checks the statement against, so the "settled only" rule has to be exercised in the
// oracle rather than filtered out before it — otherwise the test could not tell
// whether the statement applies the rule at all.
const betsByPlayerQuery = `
	SELECT game_room_user_id, id, last_combo, score,
	       won_at IS NOT NULL,
	       (won_at IS NOT NULL OR lost_at IS NOT NULL)
	  FROM game_room_user_bets
	 WHERE game_room_id = ?
	 ORDER BY game_room_user_id, id`

func (repository *MySQLRepository) BetsByPlayer(ctx context.Context, roomID int64) (map[int64][]Bet, error) {
	rows, err := repository.database.QueryContext(ctx, betsByPlayerQuery, roomID)
	if err != nil {
		return nil, fmt.Errorf("gameroom: query wagers for room %d: %w", roomID, err)
	}
	defer rows.Close()

	byPlayer := make(map[int64][]Bet)
	for rows.Next() {
		var (
			playerID int64
			bet      Bet
		)
		if err := rows.Scan(&playerID, &bet.ID, &bet.LastCombo, &bet.Score, &bet.Won, &bet.Settled); err != nil {
			return nil, fmt.Errorf("gameroom: scan wager: %w", err)
		}
		byPlayer[playerID] = append(byPlayer[playerID], bet)
	}
	return byPlayer, rows.Err()
}

// storedTotalsQuery is used by the parity test to read back what the statement
// wrote.
const storedTotalsQuery = "SELECT id, score, combo, total_played, total_correct, accuracy\n" +
	"	  FROM game_room_users WHERE game_room_id = ?"

// StoredTotals reads the persisted standing for every player in a room, keyed by
// player id. Exported for the parity test rather than for production use.
func (repository *MySQLRepository) StoredTotals(ctx context.Context, roomID int64) (map[int64]Totals, error) {
	rows, err := repository.database.QueryContext(ctx, storedTotalsQuery, roomID)
	if err != nil {
		return nil, fmt.Errorf("gameroom: query stored totals for room %d: %w", roomID, err)
	}
	defer rows.Close()

	stored := make(map[int64]Totals)
	for rows.Next() {
		var (
			id       int64
			totals   Totals
			accuracy string
		)
		if err := rows.Scan(&id, &totals.Score, &totals.Combo,
			&totals.TotalPlayed, &totals.TotalCorrect, &accuracy); err != nil {
			return nil, fmt.Errorf("gameroom: scan stored totals: %w", err)
		}
		hundredths, err := parseAccuracyHundredths(accuracy)
		if err != nil {
			return nil, fmt.Errorf("gameroom: player %d accuracy %q: %w", id, accuracy, err)
		}
		totals.AccuracyHundredths = hundredths
		stored[id] = totals
	}
	return stored, rows.Err()
}

// parseAccuracyHundredths turns the decimal(5,2) text into hundredths without
// going through a float, so the comparison in the parity test is exact.
func parseAccuracyHundredths(value string) (int, error) {
	whole, fraction := value, "00"
	for index := 0; index < len(value); index++ {
		if value[index] == '.' {
			whole, fraction = value[:index], value[index+1:]
			break
		}
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("more than two decimal places")
	}
	units, err := strconv.Atoi(whole)
	if err != nil {
		return 0, err
	}
	hundredths, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, err
	}
	if units < 0 {
		return units*100 - hundredths, nil
	}
	return units*100 + hundredths, nil
}
