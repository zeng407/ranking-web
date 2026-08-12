package ranking

import (
	"context"
	"testing"
	"time"
)

// The queries must run against the real schema: `rank` is reserved in MySQL 8 and
// the soft-delete filter is easy to omit, and neither shows up in a unit test.
func TestHistoryQueriesRunAgainstTheRealSchema(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	var rankReportID int64
	err := database.QueryRowContext(ctx,
		`SELECT id FROM rank_reports WHERE post_id = ? AND element_id = ? LIMIT 1`,
		postID, elementID).Scan(&rankReportID)
	if err != nil {
		t.Skipf("no rank report for post %d element %d: %v", postID, elementID, err)
	}

	for _, timeRange := range []HistoryTimeRange{HistoryRangeAll, HistoryRangeThousandVotes} {
		if _, err := repository.LatestHistoryStartDate(ctx, rankReportID, timeRange); err != nil {
			t.Errorf("LatestHistoryStartDate(%s) error = %v", timeRange, err)
		}
		if _, err := repository.HistoryDatesPresent(ctx, rankReportID, timeRange); err != nil {
			t.Errorf("HistoryDatesPresent(%s) error = %v", timeRange, err)
		}
	}
	if _, err := repository.RanksOnOrAfter(ctx, postID, elementID, day(2026, time.July, 1)); err != nil {
		t.Errorf("RanksOnOrAfter() error = %v", err)
	}
	for _, rankType := range []RankType{RankTypePKKing, RankTypeChampion} {
		if _, err := repository.FirstRankOnOrAfter(ctx, postID, elementID, day(2026, time.July, 1), rankType); err != nil {
			t.Errorf("FirstRankOnOrAfter(%s) error = %v", rankType, err)
		}
	}
}

func TestFirstRankOnOrAfterRejectsUnknownType(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)

	_, err := repository.FirstRankOnOrAfter(context.Background(), 1, 1, time.Now(), RankType("king"))
	if err == nil {
		t.Fatal("FirstRankOnOrAfter() should reject an unknown rank type")
	}
}

// FirstRankOnOrAfter must return the EARLIEST row at or after the date. The
// original method is named getLastRankRecord but orders ascending and takes the
// first, so the name is misleading and a "last" implementation would be wrong.
func TestFirstRankOnOrAfterReturnsTheEarliestRow(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	var earliest time.Time
	err := database.QueryRowContext(ctx,
		`SELECT MIN(record_date) FROM ranks WHERE post_id = ? AND element_id = ? AND rank_type = ?`,
		postID, elementID, string(RankTypePKKing)).Scan(&earliest)
	if err != nil || earliest.IsZero() {
		t.Skipf("no pk_king ranks for post %d element %d", postID, elementID)
	}

	got, err := repository.FirstRankOnOrAfter(ctx, postID, elementID, earliest, RankTypePKKing)
	if err != nil {
		t.Fatalf("FirstRankOnOrAfter() error = %v", err)
	}
	if got == nil {
		t.Fatal("FirstRankOnOrAfter() = nil, want the earliest row")
	}
	if !got.RecordDate.Equal(earliest) {
		t.Fatalf("RecordDate = %s, want the earliest %s",
			got.RecordDate.Format(dateLayout), earliest.Format(dateLayout))
	}
}

// RecentVotes must return the newest rounds first and classify win/lose correctly.
func TestRecentVotesReturnsNewestFirstWithCorrectOutcome(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	votes, err := repository.RecentVotes(ctx, postID, elementID, 50)
	if err != nil {
		t.Fatalf("RecentVotes() error = %v", err)
	}
	if len(votes) == 0 {
		t.Skipf("post %d element %d has no rounds", postID, elementID)
	}

	for index := 1; index < len(votes); index++ {
		if votes[index-1].RoundID <= votes[index].RoundID {
			t.Fatalf("votes are not newest-first at %d: %d then %d",
				index, votes[index-1].RoundID, votes[index].RoundID)
		}
	}

	// Cross-check the win flag against the round itself.
	var wins int64
	for _, vote := range votes {
		if vote.Won {
			wins++
		}
	}
	var expectedWins int64
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_1v1_rounds rounds
		 WHERE rounds.id IN (`+placeholderList(len(votes))+`) AND rounds.winner_id = ?`,
		append(voteIDArgs(votes), elementID)...).Scan(&expectedWins)
	if err != nil {
		t.Fatalf("cross-check wins: %v", err)
	}
	if wins != expectedWins {
		t.Fatalf("wins = %d, want %d", wins, expectedWins)
	}
}

func TestRecentVotesRejectsNonPositiveLimit(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)

	if _, err := repository.RecentVotes(context.Background(), 1, 1, 0); err == nil {
		t.Fatal("RecentVotes() should reject a zero limit")
	}
}

// The window must be capped at exactly ThousandVotesWindow even when far more
// rounds exist.
func TestRecentVotesRespectsTheWindowCap(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLHistoryRepository(database)
	postID, elementID := fixtureIDs(t)
	ctx := context.Background()

	var total int64
	err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_1v1_rounds rounds
		  JOIN games ON games.id = rounds.game_id
		 WHERE games.post_id = ? AND (rounds.winner_id = ? OR rounds.loser_id = ?)`,
		postID, elementID, elementID).Scan(&total)
	if err != nil {
		t.Fatalf("count rounds: %v", err)
	}
	if total <= ThousandVotesWindow {
		t.Skipf("post %d element %d has only %d rounds, below the %d cap", postID, elementID, total, ThousandVotesWindow)
	}

	votes, err := repository.RecentVotes(ctx, postID, elementID, ThousandVotesWindow)
	if err != nil {
		t.Fatalf("RecentVotes() error = %v", err)
	}
	if len(votes) != ThousandVotesWindow {
		t.Fatalf("got %d votes, want exactly %d", len(votes), ThousandVotesWindow)
	}
}

func placeholderList(count int) string {
	if count == 0 {
		return "NULL"
	}
	out := make([]byte, 0, count*3)
	for index := 0; index < count; index++ {
		if index > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, '?')
	}
	return string(out)
}

func voteIDArgs(votes []VoteOutcome) []any {
	args := make([]any, 0, len(votes)+1)
	for _, vote := range votes {
		args = append(args, vote.RoundID)
	}
	return args
}
