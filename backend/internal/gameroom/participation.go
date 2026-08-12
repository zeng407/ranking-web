package gameroom

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// This file is the request side of a game room: joining one, placing a wager, renaming
// yourself. The rest of the package is the settlement side, which the worker drives.
//
// The split matters because the two have opposite shapes. Settlement is set-based and
// idempotent — recompute everything, assign ranks, broadcast. Participation is one row
// at a time and has to be safe against a browser that fires the same request twice,
// which is what the unique indexes added in migrations 00010 to 00012 are for.

// Participation errors.
var (
	// ErrGameNotFound means the game serial does not resolve.
	ErrGameNotFound = errors.New("gameroom: game not found")
	// ErrRoomMismatch means the caller named a game that does not own this room. Kept
	// distinct from ErrNotFound because it is the signal of a stale link rather than a
	// missing row, and Laravel answered it 403.
	ErrRoomMismatch = errors.New("gameroom: the room does not belong to that game")
	// ErrNicknameTooSoon is the rename rate limit. Renaming is broadcast to everyone in
	// the room, so it is the one participation action worth throttling.
	ErrNicknameTooSoon = errors.New("gameroom: the nickname was changed too recently")
	// ErrInvalidNickname covers empty and over-long names.
	ErrInvalidNickname = errors.New("gameroom: the nickname is not valid")
	// ErrNoRoundInProgress means the host has not put a pairing on screen yet, so there
	// is nothing to wager on.
	ErrNoRoundInProgress = errors.New("gameroom: no round is in progress")
	// ErrNotTheCurrentPairing means the wager names elements that are not the two on
	// screen — a stale page, or a client inventing a matchup the settlement would never
	// resolve.
	ErrNotTheCurrentPairing = errors.New("gameroom: that is not the current pairing")
)

// MaxNicknameRunes is the game_room_users.nickname column width, counted in runes
// because the column is utf8mb4 and these names are mostly Chinese. Laravel validates
// max:10; the column holds 20. The narrower rule is kept.
const MaxNicknameRunes = 10

// NicknameCooldown is how long a player must wait between renames. Matches
// CacheService::putUpdateGameUserNameThreashold.
const NicknameCooldown = 30 * time.Second

// RoomSerialLength is the length of a generated room serial, matching
// SerialGenerator::genGameRoomSerial.
const RoomSerialLength = 8

// serialAlphabet is lowercase alphanumeric. Laravel produces Str::random(8) lowercased,
// which folds the two cases of every letter together; generating from a lowercase
// alphabet directly gives the same shape without the uneven distribution that folding
// introduces.
const serialAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// Participant is one player in a room.
type Participant struct {
	ID          int64
	RoomID      int64
	UserID      *int64
	AnonymousID string
	Nickname    string
	Score       int
	Rank        int
	// AccuracyHundredths is the stored accuracy times 100, so 63.49 is 6349. Kept as an
	// integer for the same reason the tally does: the column is DECIMAL(5,2) and a float
	// round-trip is the one way to make two equal accuracies compare unequal.
	AccuracyHundredths int
	TotalPlayed        int
	TotalCorrect       int
	Combo              int
}

// PlayerID is the digest the client sees instead of the row id, matching
// GameRoomUserResource. Defined on Participant for convenience.
func (participant Participant) PlayerID() string {
	return PlayerID(participant.ID, participant.AnonymousID)
}

// PlacedBet is a wager as the client submitted it.
type PlacedBet struct {
	WinnerID       int64
	LoserID        int64
	CurrentRound   int
	OfRound        int
	RemainElements int
}

// VoteTally is how the room voted on the round in progress.
type VoteTally struct {
	FirstCandidate      int64 `json:"first_candidate"`
	SecondCandidate     int64 `json:"second_candidate"`
	FirstCandidateVotes int   `json:"first_candidate_votes"`
	SecondCandidateVote int   `json:"second_candidate_votes"`
	RemainElements      int   `json:"remain_elements"`
	TotalVotes          int   `json:"total_votes"`
	// CurrentRound and OfRound describe the match in progress, so a client can show
	// "31 of 32" without computing it. See RoundInProgress for why it cannot.
	CurrentRound int `json:"current_round"`
	OfRound      int `json:"of_round"`
}

// RoundInProgress is the match the room is voting on right now.
//
// WHY THE SERVER DERIVES THIS AND THE CLIENT DOES NOT.
//
// game_1v1_rounds records COMPLETED matches, so the match in progress is one past the
// last row, and "one past" crosses a bracket boundary: the observed sequence goes
// (64 of 64, 64 remaining) then (1 of 32, 63 remaining). of_round is a property of the
// bracket, fixed when the bracket starts at ceil(elements/2) — which is why a row can
// read "30 of 32" with 34 remaining (the bracket began with 64).
//
// A room participant is watching somebody else's game and has none of that state. Making
// them reconstruct it would put bracket arithmetic in the browser, where a wrong answer
// writes a wager whose round counters disagree with everyone else's.
type RoundInProgress struct {
	// FirstCandidate and SecondCandidate are zero when no pairing is set.
	FirstCandidate  int64
	SecondCandidate int64
	HasPairing      bool
	RemainElements  int
	CurrentRound    int
	OfRound         int
}

// NextRound advances a completed match to the one in progress.
//
// Exported and pure so the rule is testable without a database. Within a bracket the
// match number simply increments; on the last match of a bracket the next one starts a
// new bracket, whose size is half of what is left.
func NextRound(lastCurrentRound, lastOfRound, remainElements int) (currentRound, ofRound int) {
	if lastOfRound <= 0 || lastCurrentRound <= 0 {
		// No match has been played yet: the first one of the first bracket.
		return 1, matchesFor(remainElements)
	}
	if lastCurrentRound < lastOfRound {
		return lastCurrentRound + 1, lastOfRound
	}
	return 1, matchesFor(remainElements)
}

// matchesFor is how many matches a bracket of this many elements holds: ceil(n/2),
// because an odd element gets a bye rather than being dropped.
func matchesFor(elements int) int {
	if elements <= 1 {
		return 1
	}
	return (elements + 1) / 2
}

// ParticipationRepository is the request-side half of the store.
type ParticipationRepository interface {
	// EnsureRoom returns the game's room, creating it if the game has none. Must be
	// safe to call concurrently: the unique index on game_rooms.game_id decides, and a
	// loser must return the winner's row rather than an error.
	EnsureRoom(ctx context.Context, gameSerial string, serial string) (Room, bool, error)
	// RoomBySerialWithGame resolves a room and the game serial that owns it, so a
	// caller can be told its link is stale.
	RoomBySerialWithGame(ctx context.Context, roomSerial string) (Room, string, bool, error)
	// SetOnScreenPair records the pair a host is displaying, so the room's participants
	// wager on the match in play rather than the one just decided. Must ignore a pair
	// that is not two live elements of the game.
	SetOnScreenPair(ctx context.Context, gameSerial string, first, second int64) error
	// RoomByGameSerial finds the room hosting a game WITHOUT creating one. The vote
	// path calls it on every batch, and most games have no room at all — creating one
	// there would give every solo game a room nobody asked for.
	RoomByGameSerial(ctx context.Context, gameSerial string) (Room, bool, error)
	// EnsureParticipant finds or creates the row for one browser in one room.
	EnsureParticipant(ctx context.Context, roomID int64, anonymousID string, userID *int64, nickname string, startingScore int) (Participant, error)
	// UpsertBet records a wager, replacing the caller's previous wager on the same
	// round. lastCombo is the streak the wager rides on, resolved by the caller.
	UpsertBet(ctx context.Context, roomID int64, participantID int64, bet PlacedBet, lastCombo int) error
	// PreviousBetStreak reports the combo and outcome of the participant's most recent
	// wager, which is what the next wager's last_combo is built from.
	PreviousBetStreak(ctx context.Context, participantID int64) (lastCombo int, won bool, found bool, err error)
	// Rename writes a new nickname.
	Rename(ctx context.Context, participantID int64, nickname string) error
	// CurrentVotes tallies the room's wagers on the round in progress.
	CurrentVotes(ctx context.Context, roomID int64, gameSerial string) (VoteTally, bool, error)
	// RoundInProgress reports the match the room is voting on, so a wager can be
	// recorded against the same round numbers everyone else's is.
	RoundInProgress(ctx context.Context, gameSerial string) (RoundInProgress, error)
	// LatestBet returns the caller's most recent wager, for rehydrating a reloaded page.
	LatestBet(ctx context.Context, participantID int64) (PlacedBet, bool, error)
}

// RenameLimiter remembers when a player last renamed themselves.
type RenameLimiter interface {
	// Allow reports whether a rename may proceed and records it when it may. One call,
	// not a check followed by a set: two simultaneous renames must not both pass.
	Allow(ctx context.Context, participantID int64, cooldown time.Duration) (bool, error)
}

// Participation serves the request side of a room.
type Participation struct {
	repository ParticipationRepository
	limiter    RenameLimiter
	scoring    Scoring
	// nicknames supplies a starting name for a new participant.
	nicknames func() string
}

// ParticipationOptions wires Participation.
type ParticipationOptions struct {
	Repository ParticipationRepository
	Limiter    RenameLimiter
	Scoring    Scoring
	Nicknames  func() string
}

func NewParticipation(options ParticipationOptions) (*Participation, error) {
	if options.Repository == nil {
		return nil, errors.New("gameroom: participation repository is required")
	}
	scoring := options.Scoring
	if scoring.DefaultScore == 0 {
		scoring = DefaultScoring()
	}
	nicknames := options.Nicknames
	if nicknames == nil {
		nicknames = RandomNickname
	}
	return &Participation{
		repository: options.Repository,
		limiter:    options.Limiter,
		scoring:    scoring,
		nicknames:  nicknames,
	}, nil
}

// EnsureRoom returns the room hosting a game, creating it on first request.
//
// Idempotent by design: the host's page calls this every time it loads, and Laravel's
// firstOrCreate had the same contract. The difference is that the unique index added in
// 00010 now makes a concurrent second call return the first call's room instead of
// creating a second one nobody can reach.
func (participation *Participation) EnsureRoom(
	ctx context.Context, gameSerial string, onScreen []int64,
) (Room, bool, error) {
	gameSerial = strings.TrimSpace(gameSerial)
	if gameSerial == "" {
		return Room{}, false, ErrGameNotFound
	}
	serial, err := NewRoomSerial()
	if err != nil {
		return Room{}, false, err
	}
	room, created, err := participation.repository.EnsureRoom(ctx, gameSerial, serial)
	if err != nil {
		return Room{}, false, err
	}

	// A host opens the room mid-game, so the pair already on screen has to be recorded or
	// the first participants are shown the match that was just decided. Failing this is
	// not worth failing the room: the next vote records the pair anyway.
	if len(onScreen) == 2 {
		if err := participation.repository.SetOnScreenPair(ctx, gameSerial, onScreen[0], onScreen[1]); err != nil {
			return room, created, err
		}
	}
	return room, created, nil
}

// Join returns the caller's participant row in a room, creating it on first visit.
//
// THE ANONYMOUS ID IS NOW CLIENT-SUPPLIED. Laravel read it from the session
// (`$request->session()->get('anonymous_id')`), which made it unforgeable but also tied
// participation to a PHP session cookie. The Go API has no session, so the client sends
// the same id it already sends for comments. The consequence is honest: someone who
// learns another player's anonymous id can act as them in a room. The stakes are a party
// game's score, the comments endpoint already made the same trade, and the alternative
// is keeping a Laravel session alive purely to identify anonymous players.
func (participation *Participation) Join(
	ctx context.Context, roomID int64, anonymousID string, userID *int64,
) (Participant, error) {
	anonymousID = strings.TrimSpace(anonymousID)
	if anonymousID == "" {
		// Laravel defaulted to the literal "unknown", which collapses every visitor
		// with no session into one shared participant. That is worse than refusing:
		// they would see each other's score and overwrite each other's wagers.
		return Participant{}, fmt.Errorf("%w: an anonymous id is required", ErrInvalidNickname)
	}
	return participation.repository.EnsureParticipant(
		ctx, roomID, anonymousID, userID, participation.nicknames(), participation.scoring.DefaultScore)
}

// Bet records a wager on the round in progress.
//
// The streak written onto the row is resolved here, from the outcome of the previous
// wager: a win continues it, anything else resets it.
//
// NOTHING IN THIS PACKAGE SCORES FROM IT ANY MORE. It cannot be trusted to: it is
// resolved when the wager is placed, and the previous wager has only been settled by
// then if the host happened to vote first. Tally and RecomputeTotals derive the streak
// from the outcomes instead, so the bonus does not depend on how fast anyone clicks.
//
// The column is still written because Laravel still reads it — its
// updateGameRoomUserBetScore sums the per-wager score that the settlement computes from
// last_combo — and the old Blade UI has not been retired yet. When it is, both this and
// the score column can go.
func (participation *Participation) Bet(
	ctx context.Context, roomID int64, participant Participant, bet PlacedBet,
) error {
	if bet.WinnerID <= 0 || bet.LoserID <= 0 || bet.WinnerID == bet.LoserID {
		return fmt.Errorf("gameroom: winner and loser must be different elements")
	}
	if bet.CurrentRound <= 0 || bet.OfRound <= 0 || bet.RemainElements < 0 {
		return fmt.Errorf("gameroom: the round is not valid")
	}

	lastCombo, won, found, err := participation.repository.PreviousBetStreak(ctx, participant.ID)
	if err != nil {
		return err
	}
	combo := 0
	if found && won {
		combo = lastCombo + 1
	}

	return participation.repository.UpsertBet(ctx, roomID, participant.ID, bet, combo)
}

// BetOnCurrentRound records a wager using the server's own view of the round.
//
// The caller sends only which element it picked. The round numbers come from
// RoundInProgress, so a participant cannot record a wager against a round that does not
// exist — and does not need the bracket arithmetic to know which one is in play.
func (participation *Participation) BetOnCurrentRound(
	ctx context.Context, roomID int64, participant Participant, gameSerial string, winnerID, loserID int64,
) error {
	round, err := participation.repository.RoundInProgress(ctx, gameSerial)
	if err != nil {
		return err
	}
	if !round.HasPairing {
		return ErrNoRoundInProgress
	}
	// The pick has to be the pairing actually on screen. Without this a client could
	// wager on any two elements in the post, which the settlement would never resolve.
	matchesPairing := (winnerID == round.FirstCandidate && loserID == round.SecondCandidate) ||
		(winnerID == round.SecondCandidate && loserID == round.FirstCandidate)
	if !matchesPairing {
		return ErrNotTheCurrentPairing
	}

	return participation.Bet(ctx, roomID, participant, PlacedBet{
		WinnerID:       winnerID,
		LoserID:        loserID,
		CurrentRound:   round.CurrentRound,
		OfRound:        round.OfRound,
		RemainElements: round.RemainElements,
	})
}

// Rename changes a player's display name, subject to the cooldown.
func (participation *Participation) Rename(
	ctx context.Context, participant Participant, nickname string,
) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > MaxNicknameRunes {
		return ErrInvalidNickname
	}

	if participation.limiter != nil {
		allowed, err := participation.limiter.Allow(ctx, participant.ID, NicknameCooldown)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrNicknameTooSoon
		}
	}

	return participation.repository.Rename(ctx, participant.ID, nickname)
}

// NewRoomSerial generates a room serial.
//
// Eight lowercase alphanumerics, matching what SerialGenerator produced. It does NOT
// check the table for a collision the way the PHP does: 36^8 is 2.8e12 against 15,209
// existing rooms, so a retry loop would be dead code, and the unique index on serial is
// the real guarantee either way.
func NewRoomSerial() (string, error) {
	raw := make([]byte, RoomSerialLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("gameroom: read random bytes: %w", err)
	}
	serial := make([]byte, RoomSerialLength)
	for index, value := range raw {
		// Modulo bias over a 36-letter alphabet from 256 values is under 2%, which for a
		// room serial is not worth a rejection loop.
		serial[index] = serialAlphabet[int(value)%len(serialAlphabet)]
	}
	return string(serial), nil
}

// nicknameAdjectives and nicknameNouns build a starting name, replacing PHP's
// random_nickname() helper.
var (
	nicknameAdjectives = []string{"快樂", "神秘", "勇敢", "冷靜", "熱血", "悠閒", "認真", "隨性"}
	nicknameNouns      = []string{"路人", "玩家", "觀眾", "選手", "旅人", "食客", "貓咪", "阿宅"}
)

// RandomNickname produces a starting display name.
//
// Two words rather than a number, so a room full of new players is readable rather than
// a list of "Player 4193". Collisions inside one room are possible and harmless: the
// leaderboard is keyed on the player digest, not the name.
func RandomNickname() string {
	raw := make([]byte, 2)
	if _, err := rand.Read(raw); err != nil {
		// A fixed name is better than a failed join. The player can rename themselves.
		return "路人"
	}
	return nicknameAdjectives[int(raw[0])%len(nicknameAdjectives)] +
		nicknameNouns[int(raw[1])%len(nicknameNouns)]
}
