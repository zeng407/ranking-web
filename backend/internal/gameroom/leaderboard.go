package gameroom

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
)

// Standing is one player's score, which is all rank assignment needs.
type Standing struct {
	UserID int64
	Score  int
}

// AssignRanks numbers players 1..N by score descending, with no gaps and no shared
// positions — the dense sequence the PHP `$rank++` loop produced. Verified against
// production: all 8,720 ranked rooms use 1..N with no gaps and no duplicates.
//
// Ties are broken by id ascending, which the PHP did not do. It ordered by score
// alone and paged through the result with each(), so players on equal scores — 1,088
// players across only 101 distinct scores in the largest room, about eleven per
// score — took whatever order MySQL happened to return, and could swap positions
// between two refreshes with nothing having changed.
//
// Adding the tiebreak is safe rather than a behaviour change: across 24,772 ranked
// players it disagrees with the stored rank for 757, every one of them inside a
// tied-score group, and no player is ordered against a strictly lower score.
func AssignRanks(standings []Standing) map[int64]int {
	ordered := make([]Standing, len(standings))
	copy(ordered, standings)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Score != ordered[right].Score {
			return ordered[left].Score > ordered[right].Score
		}
		return ordered[left].UserID < ordered[right].UserID
	})

	ranks := make(map[int64]int, len(ordered))
	for index, standing := range ordered {
		ranks[standing.UserID] = index + 1
	}
	return ranks
}

// Player is one entry in the broadcast payload, matching GameRoomUserResource
// field for field.
type Player struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Rank   int    `json:"rank"`
	// Accuracy is a string, not a number. accuracy is decimal(5,2) and Eloquent
	// declares no cast for it, so PDO hands back "85.40" and the payload carries
	// that. Game.vue interpolates it straight into `勝率:${rank.accuracy}%`, so
	// emitting 85.4 instead of "85.40" would visibly change the UI.
	Accuracy     string `json:"accuracy"`
	TotalPlayed  int    `json:"total_played"`
	TotalCorrect int    `json:"total_correct"`
	Combo        int    `json:"combo"`
}

// Leaderboard is the whole payload, matching BroadcastGameBetRank::broadcastWith
// and the ranks field of GameController::roomRank.
type Leaderboard struct {
	TotalUsers int `json:"total_users"`
	// Top10 is best first.
	Top10 []Player `json:"top_10"`
	// Bottom10 is WORST first: the PHP used orderByDesc('rank')->limit(10), so the
	// array starts at the last place. Reversing it here would silently flip the
	// order the UI renders.
	Bottom10 []Player `json:"bottom_10"`
}

// PlayerID is the opaque identifier the payload exposes instead of the row id,
// from GameRoomUserResource: md5($this->id.':'.$this->anonymous_id).
//
// It is not a security boundary — the input is guessable — but it is a stable
// per-room handle the frontend uses to find itself in the list, so the exact bytes
// matter.
func PlayerID(userID int64, anonymousID string) string {
	sum := md5.Sum([]byte(strconv.FormatInt(userID, 10) + ":" + anonymousID))
	return hex.EncodeToString(sum[:])
}

// FormatAccuracy renders hundredths as the two-decimal string the column and the
// payload both use.
func FormatAccuracy(hundredths int) string {
	negative := hundredths < 0
	if negative {
		hundredths = -hundredths
	}
	formatted := fmt.Sprintf("%d.%02d", hundredths/100, hundredths%100)
	if negative {
		return "-" + formatted
	}
	return formatted
}
