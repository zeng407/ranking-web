package gameroom

import (
	"testing"
)

func TestAssignRanksIsDenseAndOrderedByScore(t *testing.T) {
	ranks := AssignRanks([]Standing{
		{UserID: 5, Score: 900},
		{UserID: 2, Score: 1200},
		{UserID: 9, Score: 1000},
	})

	want := map[int64]int{2: 1, 9: 2, 5: 3}
	for userID, wantRank := range want {
		if ranks[userID] != wantRank {
			t.Errorf("player %d ranked %d, want %d", userID, ranks[userID], wantRank)
		}
	}
	if len(ranks) != 3 {
		t.Errorf("assigned %d ranks, want 3", len(ranks))
	}
}

// The PHP ordered by score alone, so equal scores could swap places between two
// refreshes and the leaderboard would appear to shuffle with nothing happening. In
// the largest production room 1,088 players shared only 101 distinct scores, so ties
// were the normal case rather than an edge one.
func TestAssignRanksBreaksTiesDeterministically(t *testing.T) {
	tied := []Standing{
		{UserID: 30, Score: 1000},
		{UserID: 10, Score: 1000},
		{UserID: 20, Score: 1000},
	}

	first := AssignRanks(tied)
	if first[10] != 1 || first[20] != 2 || first[30] != 3 {
		t.Fatalf("ties must order by id ascending, got %v", first)
	}

	// Feeding the same scores in a different order must not change the result.
	reordered := AssignRanks([]Standing{tied[1], tied[2], tied[0]})
	for userID, rank := range first {
		if reordered[userID] != rank {
			t.Fatalf("player %d ranked %d then %d; the order is not stable",
				userID, rank, reordered[userID])
		}
	}
}

func TestAssignRanksLeavesNoGapsOrDuplicates(t *testing.T) {
	// Production invariant: all 8,720 ranked rooms use 1..N with no gaps and no
	// repeats, which is what the PHP $rank++ loop produced.
	standings := make([]Standing, 0, 50)
	for index := 0; index < 50; index++ {
		// Heavy tying on purpose: three distinct scores across fifty players.
		standings = append(standings, Standing{UserID: int64(index + 1), Score: 1000 + (index%3)*10})
	}

	ranks := AssignRanks(standings)
	seen := make(map[int]bool, len(ranks))
	for _, rank := range ranks {
		if rank < 1 || rank > len(standings) {
			t.Fatalf("rank %d is outside 1..%d", rank, len(standings))
		}
		if seen[rank] {
			t.Fatalf("rank %d assigned twice", rank)
		}
		seen[rank] = true
	}
	if len(seen) != len(standings) {
		t.Fatalf("assigned %d distinct ranks for %d players", len(seen), len(standings))
	}
}

func TestAssignRanksDoesNotMutateItsInput(t *testing.T) {
	standings := []Standing{
		{UserID: 1, Score: 100},
		{UserID: 2, Score: 900},
	}
	AssignRanks(standings)
	if standings[0].UserID != 1 || standings[1].UserID != 2 {
		t.Fatalf("input was reordered: %+v", standings)
	}
}

func TestAssignRanksHandlesAnEmptyRoom(t *testing.T) {
	if ranks := AssignRanks(nil); len(ranks) != 0 {
		t.Fatalf("an empty room produced %d ranks", len(ranks))
	}
}

// The frontend uses this to find itself in the list, so the exact digest matters.
func TestPlayerIDMatchesTheResourceDigest(t *testing.T) {
	// Taken from PHP itself: `php -r 'echo md5(22382 . ":" . "abc-123");'`, which is
	// what GameRoomUserResource evaluates.
	const want = "0832d1c9e557b1ae562a18c8b982c771"
	got := PlayerID(22382, "abc-123")
	if got != want {
		t.Fatalf("PlayerID(22382, %q) = %q, want %q", "abc-123", got, want)
	}

	if PlayerID(22383, "abc-123") == got {
		t.Error("PlayerID ignores the player id")
	}
	if PlayerID(22382, "abc-124") == got {
		t.Error("PlayerID ignores the anonymous id")
	}
	// The separator must be a colon, not concatenation: without it "1" + "23" and
	// "12" + "3" would collide.
	if PlayerID(1, "23") == PlayerID(12, "3") {
		t.Error("PlayerID collides when the id and anonymous id are concatenated")
	}
}
