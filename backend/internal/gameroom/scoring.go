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

// Bet is one wager, as stored. Score is already resolved: the settlement step
// writes last_combo*ComboScore+WonScore or LoseScore into the row as an audit of what
// that round paid when it settled — but the tally no longer reads it. See Tally for why
// the streak has to be derived from the outcomes rather than trusted from the row.
type Bet struct {
	ID int64
	// LastCombo and Score are what the settlement wrote. Kept for audit and for the
	// parity test against SettledScore; the totals are derived from Won and Settled.
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
func Tally(bets []Bet, scoring Scoring) Totals {
	totals := Totals{Score: scoring.DefaultScore}

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

// SettledScore is the score the settlement writes into a wager row.
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
