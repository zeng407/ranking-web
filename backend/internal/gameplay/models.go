package gameplay

import (
	"context"
	"errors"
	"fmt"

	"2pick.app/backend/internal/postaccess"
)

var ErrNotFound = errors.New("gameplay resource not found")
var ErrInvalidElementCount = errors.New("invalid game element count")

/*
Repository reads and writes games.

EVERY METHOD TAKES A CALLER, AND THAT IS ON PURPOSE.

Laravel gated each of these separately — GamePolicy::play on resume, submit and result,
PostPolicy::newGame on creation — so a password post's game could not be played by someone
who never entered the door code, not merely started. Passing the caller as an argument
rather than reading it from the context means a new method cannot quietly default to
"whoever asks": it does not compile until the caller is decided.

The caller reaches the SQL rather than being checked before it, so the same statement that
reads the row applies the rule. See postaccess.VisibilityClause.
*/
type Repository interface {
	Definition(ctx context.Context, postSerial string, caller postaccess.Caller) (Definition, error)
	Create(ctx context.Context, input CreateInput) (Session, error)
	Resume(ctx context.Context, gameSerial string, caller postaccess.Caller) (Session, error)
	SubmitVotes(ctx context.Context, gameSerial string, input BatchInput) (BatchResult, error)
	Result(ctx context.Context, gameSerial string, caller postaccess.Caller) (GameResult, error)
}

// PreviewElement is one of the two options shown before a game starts. It
// mirrors the shape the legacy post payload already uses, so the same
// pre-computed preview backs both the home cards and the game page.
type PreviewElement struct {
	ID          *int64  `json:"id"`
	URL         *string `json:"url"`
	URL2        *string `json:"url2"`
	Title       *string `json:"title"`
	Type        *string `json:"type"`
	VideoSource *string `json:"video_source"`
	Previewable bool    `json:"previewable"`
}

type Definition struct {
	Title         string `json:"title"`
	Serial        string `json:"serial"`
	Description   string `json:"description"`
	IsCensored    bool   `json:"is_censored"`
	ElementsCount int    `json:"elements_count"`
	MaxElements   int    `json:"max_elements"`
	// Preview options, so the page can render something on first paint instead
	// of waiting for the player to press start. Null when no preview is
	// available; the page must stay usable without it.
	Element1 *PreviewElement `json:"element1"`
	Element2 *PreviewElement `json:"element2"`
}

type Element struct {
	ID                  int64   `json:"id"`
	SourceURL           *string `json:"source_url"`
	ThumbURL            *string `json:"thumb_url"`
	MediumThumbURL      *string `json:"mediumthumb_url"`
	LowThumbURL         *string `json:"lowthumb_url"`
	Title               string  `json:"title"`
	Type                string  `json:"type"`
	VideoStartSecond    *int64  `json:"video_start_second"`
	VideoEndSecond      *int64  `json:"video_end_second"`
	VideoSource         *string `json:"video_source"`
	VideoID             *string `json:"video_id"`
	VideoDurationSecond *int64  `json:"video_duration_second"`
}

type CreateInput struct {
	PostSerial   string
	ElementCount int
	Caller       postaccess.Caller
}

type Session struct {
	GameSerial      string     `json:"game_serial"`
	ServerVoteCount int        `json:"server_vote_count"`
	Definition      Definition `json:"post"`
	Elements        []Element  `json:"elements"`
}

type Vote struct {
	WinnerID int64 `json:"winner_id"`
	LoserID  int64 `json:"loser_id"`
}

type BatchInput struct {
	ExpectedVoteCount int
	Votes             []Vote
	AnonymousID       string
	// CurrentCandidates is the pair the client is displaying AFTER these votes, when it
	// is hosting a game room.
	//
	// WHAT games.candidates MEANS, AND WHY THIS EXISTS. In Laravel the column holds the
	// pair currently on screen: the host's client asked the server for each next pair
	// (getNextElements), so the server knew what it had handed out. The Go client plays
	// its bracket locally and batch-submits the results, so the server no longer knows —
	// and a room reads that column to show its participants what to wager on.
	//
	// Without this the column ends up holding the pair just ELIMINATED, and a room shows
	// a match that is already decided. Two elements, or empty to leave the column alone.
	CurrentCandidates []int64
	Caller            postaccess.Caller
}

type BatchResult struct {
	Status          string `json:"status"`
	ServerVoteCount int    `json:"server_vote_count"`
	// Complete reports whether the game is finished, which is true for every later
	// call about the same game too.
	Complete bool `json:"complete"`
	// JustCompleted is true only for the call that finished the game, which is the
	// moment Laravel raises GameComplete. Distinct from Complete on purpose: a
	// client retrying against an already-finished game must not re-trigger the
	// completion side effects.
	//
	// Not serialised: it exists so the transport layer can act once, and the browser
	// has no use for it.
	JustCompleted bool `json:"-"`
	// PostID identifies the post whose ranks the finished game changed. Needed by
	// the completion side effects, which are keyed by post rather than by game.
	PostID int64 `json:"-"`
	// SettledRounds is every match this batch decided, in the order it decided them.
	//
	// Needed because a game may be hosting a room, and each decided match settles the
	// wagers everyone in that room placed on it. The round numbers have to be the ones
	// actually written to game_1v1_rounds — a wager is matched on remain_elements, so a
	// recomputed guess here would settle the wrong round or none at all.
	//
	// Not serialised: the browser plays its own local game and has no use for it.
	SettledRounds []SettledRound `json:"-"`
}

// SettledRound is one match this batch decided, as recorded.
type SettledRound struct {
	WinnerID       int64
	LoserID        int64
	CurrentRound   int
	OfRound        int
	RemainElements int
}

type GameResultItem struct {
	Rank       int     `json:"rank"`
	WinCount   int     `json:"win_count"`
	GlobalRank *int64  `json:"global_rank"`
	Element    Element `json:"element"`
}

type GameResult struct {
	GameSerial string           `json:"game_serial"`
	PostSerial string           `json:"post_serial"`
	Items      []GameResultItem `json:"items"`
}

type ConflictError struct {
	Reason          string
	ServerVoteCount int
}

func (err *ConflictError) Error() string {
	return fmt.Sprintf("game state conflict: %s at revision %d", err.Reason, err.ServerVoteCount)
}
