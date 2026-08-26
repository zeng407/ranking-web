package gameroom

// Scoring mirrors config/setting.php. The values are constants there rather than
// database rows, so they are constants here too; a change in one place must be
// mirrored in the other during the cutover.
type Scoring struct {
	// DefaultScore is every player's starting score (default_bet_score).
	DefaultScore int
	// ComboScore multiplies the streak a winning wager was placed on
	// (bet_combo_score).
	ComboScore int
	// WonScore is the flat bonus for a correct wager (bet_won_score).
	WonScore int
	// LoseScore is applied to an incorrect wager and is negative (bet_lose_score).
	LoseScore int
}

// DefaultScoring returns the production values.
func DefaultScoring() Scoring {
	return Scoring{DefaultScore: 1000, ComboScore: 10, WonScore: 10, LoseScore: -10}
}

// Bet is one wager, as stored.
type Bet struct {
	ID int64
	// LastCombo and Score are what the settlement wrote.
	//
	// In a host-decided room they are an audit and nothing more: Score holds
	// last_combo*ComboScore+WonScore or LoseScore, and the tally ignores both because the
	// streak has to be derived from the outcomes rather than trusted from the row. See
	// Tally.
	//
	// In a room that decides its own rounds, Score is the answer rather than a record of
	// it. What a round pays there depends on how the whole room split, which no single
	// player's rows can show, so the settlement resolves it once and writes it here. See
	// MajorityPayout.
	LastCombo int
	Score     int
	// Won is won_at IS NOT NULL. It implies Settled.
	Won bool
	// Settled is won_at IS NOT NULL OR lost_at IS NOT NULL: the round this wager
	// was placed on has been decided. An unsettled wager is one the player has
	// placed on a round that has not resolved yet, and it does not count towards
	// anything until it does.
	Settled bool
}

// Totals is one player's standing.
type Totals struct {
	Score        int
	Combo        int
	TotalPlayed  int
	TotalCorrect int
	// AccuracyHundredths is the win rate in hundredths of a percent, so 85.42% is
	// 8542. Kept as an integer because the column is decimal(5,2) and the payload
	// renders two fixed decimals; a float64 here would introduce a rounding
	// decision at every boundary it crosses.
	AccuracyHundredths int
}

// Tally recomputes a player's standing from the settled wagers that exist right
// now. bets must be ordered by id ascending, which is how the repository reads them.
//
// Two deliberate differences from GameService::updateGameRoomUserBetScore.
//
// A FULL RECOMPUTE, not one capped step. The PHP read only the first
// total_played+1 wagers (`->limit($currentPlayed + 1)`), so one call advanced the
// player by at most one wager. That cap is incompatible with coalescing: when two
// votes' refreshes collapse into one — which is exactly what the waiting-job flag was
// built to do — the cursor advances once while two wagers settled, and the second
// wager's score is never counted. Nothing later recovers it, because every subsequent
// refresh is capped the same way. The production database shows 145 players two or
// more wagers behind their own bet rows, the worst by 46.
//
// SETTLED WAGERS ONLY. The PHP counted every row inside the cursor whether or not
// its round had been decided, so a pending wager scored zero and dragged the win rate
// down until it resolved. That is residue rather than intent: 749 of the 771 pending
// wagers still sitting inside a cursor were placed between August and December 2025,
// there are none at all from February to June 2026, and one in July. Counting only
// settled wagers also makes total_played mean what its name says.
//
// mode selects the room's scoring rules and must be the room's own vote_mode, because the
// two modes pay for different things. See the majority branch below.
func Tally(bets []Bet, scoring Scoring, mode string) Totals {
	totals := Totals{Score: scoring.DefaultScore}
	majority := mode == VoteModeMajority

	// THE STREAK IS DERIVED HERE, NOT READ FROM THE ROW.
	//
	// Each wager carries a last_combo written when it was placed, from the outcome of the
	// wager before it — and that is only correct if the previous wager had already been
	// settled at that moment. Settlement is asynchronous, so a player who bets faster than
	// the host votes was writing zero into every row and losing the whole bonus.
	//
	// Laravel avoided that by accident rather than by design: its room client played a two
	// second win/lose animation before showing the next pairing, and the settlement job was
	// dispatched immediately, so the previous wager was practically always settled first.
	// That is UI timing standing in for a data rule, and this port had already removed it —
	// the room reads the pairing straight from games.candidates, which the vote writes
	// synchronously.
	//
	// Deriving it here makes the bonus depend only on which wagers won, in round order, so
	// it cannot be changed by how quickly anyone clicks or by the order settlements arrive.
	streak := 0
	won := false
	for _, bet := range bets {
		if !bet.Settled {
			continue
		}
		totals.TotalPlayed++
		if bet.Won {
			totals.TotalCorrect++
		}

		// A ROOM THAT DECIDES ITS OWN ROUNDS PAYS FROM THE ROW, AND HAS NO COMBO.
		//
		// The magnitude of a majority round is MajorityPayout of how the room split, and
		// the split is a fact about the round rather than about any one player — nothing
		// in these wagers can reach it. So here the stored score is authoritative and the
		// streak is not scored at all: agreeing with the room six times running does not
		// make a player six times more mainstream than agreeing once, and the two taste
		// boards are read off this score alone.
		//
		// Trusting a written column is the opposite of what the branch below does, for a
		// reason that survives the comparison: last_combo is written when the wager is
		// PLACED, from whatever had settled by then, while this score is written by the
		// settlement itself from the rows it is settling. Re-deriving it could only
		// arrive at the same number, if it could see the other players at all.
		if majority {
			totals.Score += bet.Score
			continue
		}

		if bet.Won {
			totals.Score += streak*scoring.ComboScore + scoring.WonScore
			streak++
			won = true
			continue
		}
		totals.Score += scoring.LoseScore
		streak = 0
		won = false
	}

	// The displayed combo is the streak the NEXT wager would ride on, which is why a win
	// leaves it one higher than the bonus that win was paid.
	if won {
		totals.Combo = streak
	}

	totals.AccuracyHundredths = AccuracyHundredths(totals.TotalCorrect, totals.TotalPlayed)
	return totals
}

// AccuracyHundredths returns correct/played as a percentage in hundredths, rounded
// half away from zero.
//
// Integer arithmetic on purpose. The set-based statement computes
// ROUND(correct * 10000 / played) / 100, and MySQL evaluates that division as an
// exact DECIMAL and rounds half away from zero. Doing the same in float64 here
// would disagree at the exact halves that are representable in binary — 1/32 of a
// percent lands on 3.125 — so both paths use the same rational rounding instead.
func AccuracyHundredths(correct, played int) int {
	if played <= 0 {
		return 0
	}
	// Widened before multiplying: a long game in a large room reaches
	// correct*10000 in the billions, past a 32-bit int.
	numerator := int64(correct)*10000*2 + int64(played)
	return int(numerator / (int64(played) * 2))
}

// SettledScore is the score the settlement writes into a wager row in a host-decided
// room.
//
// Mirrors the two DB::raw expressions in GameService::updateGameBet: a correct
// wager earns last_combo * ComboScore + WonScore, an incorrect one takes the flat
// LoseScore regardless of streak.
func SettledScore(won bool, lastCombo int, scoring Scoring) int {
	if !won {
		return scoring.LoseScore
	}
	return lastCombo*scoring.ComboScore + scoring.WonScore
}

// MajorityPayout is what one round pays each side when the room decided it: the winning
// side gains it and the losing side loses the same amount.
//
//	WonScore * (1 + winnerVotes/totalVotes)
//
// So the usual WonScore, plus up to another WonScore for how one-sided the round was. A
// coin-tossed tie splits 50/50 and pays 15; the 70% majority pays 17 and the 30% minority
// loses 17; a unanimous round pays 20.
//
// Proportional because that is what the 大眾品味 board claims to measure. Siding with an
// overwhelming crowd is more mainstream than siding with a bare one, and a flat ±WonScore
// would score a 51/49 round and a 99/1 round identically.
//
// Integer arithmetic, rounded half away from zero, for the same reason as
// AccuracyHundredths: the number crosses into SQL as a parameter and must not depend on
// which side of the boundary computed it.
//
// totalVotes <= 0 pays nothing. A round nobody wagered on has no rows to write it to.
func MajorityPayout(winnerVotes, totalVotes int, scoring Scoring) int {
	if totalVotes <= 0 {
		return 0
	}
	numerator := int64(scoring.WonScore)*int64(totalVotes+winnerVotes)*2 + int64(totalVotes)
	return int(numerator / (int64(totalVotes) * 2))
}
