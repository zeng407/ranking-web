package gameroom

import (
	"fmt"
	"testing"
)

func TestTallySumsEverySettledWager(t *testing.T) {
	scoring := DefaultScoring()
	totals := Tally([]Bet{
		{ID: 1, LastCombo: 0, Score: 10, Won: true, Settled: true},
		{ID: 2, LastCombo: 1, Score: 20, Won: true, Settled: true},
		{ID: 3, LastCombo: 2, Score: -10, Won: false, Settled: true},
		{ID: 4, LastCombo: 0, Score: 10, Won: true, Settled: true},
	}, scoring)

	if totals.Score != 1030 {
		t.Errorf("Score = %d, want 1030 (1000 + 10 + 20 - 10 + 10)", totals.Score)
	}
	if totals.TotalPlayed != 4 {
		t.Errorf("TotalPlayed = %d, want 4", totals.TotalPlayed)
	}
	if totals.TotalCorrect != 3 {
		t.Errorf("TotalCorrect = %d, want 3", totals.TotalCorrect)
	}
	// 3/4 = 75.00%
	if totals.AccuracyHundredths != 7500 {
		t.Errorf("AccuracyHundredths = %d, want 7500", totals.AccuracyHundredths)
	}
}

// The whole point of the rewrite: a recompute must not be capped at one wager per
// call, because coalescing means one call has to absorb several settled wagers.
// The PHP limit($total_played + 1) is what left 145 production players stranded
// behind their own bet rows.
func TestTallyAbsorbsEveryWagerSettledSinceTheLastRun(t *testing.T) {
	scoring := DefaultScoring()
	bets := make([]Bet, 0, 12)
	for index := 0; index < 12; index++ {
		bets = append(bets, Bet{ID: int64(index + 1), Won: true, Settled: true})
	}

	// Simulates twelve votes whose refreshes all collapsed into one run.
	totals := Tally(bets, scoring)
	if totals.TotalPlayed != 12 {
		t.Fatalf("TotalPlayed = %d, want 12; the recompute must not stop after one wager", totals.TotalPlayed)
	}
	// Twelve consecutive wins, so the bonus grows: 10, 20, 30 ... 120. The row's stored
	// score is deliberately left at zero here — the tally derives the payout from the
	// outcomes, and a test that supplied it would not be testing that.
	wantScore := scoring.DefaultScore
	for streak := 0; streak < 12; streak++ {
		wantScore += streak*scoring.ComboScore + scoring.WonScore
	}
	if totals.Score != wantScore {
		t.Fatalf("Score = %d, want %d", totals.Score, wantScore)
	}
}

// Idempotence is what makes the coalescing safe: running the same recompute twice
// must land on the same numbers, so a redelivered message is harmless.
func TestTallyIsIdempotent(t *testing.T) {
	scoring := DefaultScoring()
	bets := []Bet{
		{ID: 1, Won: true, Settled: true},
		{ID: 2, Won: false, Settled: true},
	}
	first := Tally(bets, scoring)
	second := Tally(bets, scoring)
	if first != second {
		t.Fatalf("Tally is not idempotent: %+v then %+v", first, second)
	}
}

func TestTallyComboFollowsTheNewestWager(t *testing.T) {
	scoring := DefaultScoring()

	// Four wins after a loss, so the fifth wager would ride a streak of four and the
	// displayed combo is four. Stated as a run of outcomes, because that is the only input
	// the streak is derived from now.
	won := Tally([]Bet{
		{ID: 1, Won: false, Settled: true},
		{ID: 2, Won: true, Settled: true},
		{ID: 3, Won: true, Settled: true},
		{ID: 4, Won: true, Settled: true},
		{ID: 5, Won: true, Settled: true},
	}, scoring)
	if won.Combo != 4 {
		t.Errorf("Combo after four consecutive wins = %d, want 4", won.Combo)
	}
	// 10 + 20 + 30 + 40 for the wins, minus 10 for the loss that opened the run.
	if want := scoring.DefaultScore - 10 + 10 + 20 + 30 + 40; won.Score != want {
		t.Errorf("Score = %d, want %d", won.Score, want)
	}

	lost := Tally([]Bet{
		{ID: 1, Won: true, Settled: true},
		{ID: 2, Won: true, Settled: true},
		{ID: 3, Won: false, Settled: true},
	}, scoring)
	if lost.Combo != 0 {
		t.Errorf("Combo after a loss = %d, want 0", lost.Combo)
	}
}

func TestTallyWithNoWagersReturnsTheStartingScore(t *testing.T) {
	scoring := DefaultScoring()
	totals := Tally(nil, scoring)
	if totals.Score != scoring.DefaultScore {
		t.Errorf("Score = %d, want %d", totals.Score, scoring.DefaultScore)
	}
	if totals.AccuracyHundredths != 0 || totals.Combo != 0 || totals.TotalPlayed != 0 {
		t.Errorf("a player with no wagers should be all zeros, got %+v", totals)
	}
}

func TestAccuracyHundredthsRoundsHalfAwayFromZero(t *testing.T) {
	tests := []struct {
		correct, played, want int
		note                  string
	}{
		{correct: 0, played: 0, want: 0, note: "no games"},
		{correct: 1, played: 1, want: 10000, note: "100%"},
		{correct: 0, played: 7, want: 0, note: "0%"},
		{correct: 3, played: 4, want: 7500, note: "exact"},
		{correct: 1, played: 3, want: 3333, note: "repeating, rounds down"},
		{correct: 2, played: 3, want: 6667, note: "repeating, rounds up"},
		// 1/32 = 3.125%, an exact binary half at the third decimal. This is the case
		// a float64 path would resolve differently from MySQL's decimal rounding.
		{correct: 1, played: 32, want: 313, note: "exact half rounds away from zero"},
		{correct: 41, played: 48, want: 8542, note: "from production room 11731"},
		{correct: 39, played: 49, want: 7959, note: "from production room 11731"},
		{correct: 42, played: 46, want: 9130, note: "from production room 11731"},
	}
	for _, test := range tests {
		got := AccuracyHundredths(test.correct, test.played)
		if got != test.want {
			t.Errorf("AccuracyHundredths(%d, %d) = %d, want %d (%s)",
				test.correct, test.played, got, test.want, test.note)
		}
	}
}

// A long game in a large room multiplies past a 32-bit int, so the widening in
// AccuracyHundredths is load-bearing rather than defensive.
func TestAccuracyHundredthsDoesNotOverflow(t *testing.T) {
	const played = 1_000_000
	if got := AccuracyHundredths(played, played); got != 10000 {
		t.Fatalf("AccuracyHundredths(%d, %d) = %d, want 10000", played, played, got)
	}
}

func TestSettledScoreMirrorsTheRawExpressions(t *testing.T) {
	scoring := DefaultScoring()
	// last_combo * bet_combo_score + bet_won_score
	if got := SettledScore(true, 0, scoring); got != 10 {
		t.Errorf("first win = %d, want 10", got)
	}
	if got := SettledScore(true, 7, scoring); got != 80 {
		t.Errorf("win on a streak of 7 = %d, want 80", got)
	}
	// A loss is the flat penalty, streak ignored.
	if got := SettledScore(false, 7, scoring); got != -10 {
		t.Errorf("loss = %d, want -10", got)
	}
}

func TestFormatAccuracyAlwaysUsesTwoDecimals(t *testing.T) {
	tests := map[int]string{
		0:     "0.00",
		313:   "3.13",
		7500:  "75.00",
		8542:  "85.42",
		10000: "100.00",
		5:     "0.05",
		-250:  "-2.50",
	}
	for hundredths, want := range tests {
		if got := FormatAccuracy(hundredths); got != want {
			t.Errorf("FormatAccuracy(%d) = %q, want %q", hundredths, got, want)
		}
	}
}

// The payload carries accuracy as text, so the two representations must agree for
// every value the column can hold.
func TestFormatAccuracyRoundTripsThroughTheColumnFormat(t *testing.T) {
	for hundredths := 0; hundredths <= 10000; hundredths++ {
		text := FormatAccuracy(hundredths)
		parsed, err := parseAccuracyHundredths(text)
		if err != nil {
			t.Fatalf("parseAccuracyHundredths(%q): %v", text, err)
		}
		if parsed != hundredths {
			t.Fatalf("round trip of %d gave %q then %d", hundredths, text, parsed)
		}
	}
}

func ExampleFormatAccuracy() {
	fmt.Println(FormatAccuracy(AccuracyHundredths(41, 48)))
	// Output: 85.42
}

// A wager on a round that has not been decided counts towards nothing. The PHP
// counted it — score 0, not correct — which dragged the win rate down until the round
// resolved. The pending rows still in production stop in early 2026, so that was
// residue rather than intent.
func TestTallyIgnoresUnsettledWagers(t *testing.T) {
	scoring := DefaultScoring()
	totals := Tally([]Bet{
		{ID: 1, LastCombo: 0, Score: 10, Won: true, Settled: true},
		// Placed, not yet decided.
		{ID: 2, LastCombo: 1, Score: 0, Won: false, Settled: false},
	}, scoring)

	if totals.TotalPlayed != 1 {
		t.Errorf("TotalPlayed = %d, want 1: a pending wager has not been played", totals.TotalPlayed)
	}
	if totals.TotalCorrect != 1 {
		t.Errorf("TotalCorrect = %d, want 1", totals.TotalCorrect)
	}
	if totals.Score != 1010 {
		t.Errorf("Score = %d, want 1010", totals.Score)
	}
	// 1 of 1, not 1 of 2.
	if totals.AccuracyHundredths != 10000 {
		t.Errorf("AccuracyHundredths = %d, want 10000; a pending wager must not lower the win rate",
			totals.AccuracyHundredths)
	}
}

// The streak follows the newest *settled* wager. A pending wager placed after a win
// must not reset the combo, which is what would happen if the newest row of any kind
// decided it.
func TestTallyStreakIgnoresATrailingUnsettledWager(t *testing.T) {
	scoring := DefaultScoring()
	// Four wins, then a wager whose round has not been decided. The pending one is skipped
	// entirely, so it neither breaks the run nor counts as a loss.
	totals := Tally([]Bet{
		{ID: 1, Won: true, Settled: true},
		{ID: 2, Won: true, Settled: true},
		{ID: 3, Won: true, Settled: true},
		{ID: 4, Won: true, Settled: true},
		{ID: 5, Won: false, Settled: false},
	}, scoring)

	if totals.Combo != 4 {
		t.Errorf("Combo = %d, want 4: a pending wager must not break the streak", totals.Combo)
	}
	if totals.TotalPlayed != 4 {
		t.Errorf("TotalPlayed = %d, want 4", totals.TotalPlayed)
	}
}

func TestTallyWithOnlyUnsettledWagersIsAFreshPlayer(t *testing.T) {
	scoring := DefaultScoring()
	totals := Tally([]Bet{
		{ID: 1, LastCombo: 0, Score: 0, Won: false, Settled: false},
		{ID: 2, LastCombo: 0, Score: 0, Won: false, Settled: false},
	}, scoring)

	want := Totals{Score: scoring.DefaultScore}
	if totals != want {
		t.Errorf("got %+v, want %+v", totals, want)
	}
}
