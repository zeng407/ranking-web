package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/gameroom"
)

// GameRoomService is the slice of the game room this layer uses.
//
// Split from the settlement half deliberately: the worker owns that, and the API has no
// business recomputing a leaderboard inside a request.
type GameRoomService interface {
	EnsureRoom(ctx context.Context, gameSerial string, onScreen []int64) (gameroom.Room, bool, error)
	Join(ctx context.Context, roomID int64, anonymousID string, userID *int64) (gameroom.Participant, error)
	BetOnCurrentRound(ctx context.Context, roomID int64, participant gameroom.Participant, gameSerial string, winnerID, loserID int64) error
	Rename(ctx context.Context, participant gameroom.Participant, nickname string) error
}

// GameRoomReader is the read half: resolving a room and its current state.
type GameRoomReader interface {
	RoomBySerialWithGame(ctx context.Context, roomSerial string) (gameroom.Room, string, bool, error)
	CurrentVotes(ctx context.Context, roomID int64, gameSerial string) (gameroom.VoteTally, bool, error)
	LatestBet(ctx context.Context, participantID int64) (gameroom.PlacedBet, bool, error)
}

// GameRoomAnnouncer publishes the settlement work a host's votes create.
//
// Optional: without it the vote path records rounds and nothing settles, which is the
// state before this existed. New() warns when a room service is configured without one,
// because the symptom otherwise is a room whose leaderboard simply never moves.
type GameRoomAnnouncer interface {
	AnnounceRounds(ctx context.Context, gameSerial string, rounds []gameroom.DecidedRound) (int, error)
}

// GameRoomLeaderboard reads the standings.
//
// Separate from GameRoomReader because it is satisfied by the settlement repository, which
// already owns that query. Combining them would force the wiring to stitch two types into
// one value, and a method name added to either would then silently become ambiguous.
type GameRoomLeaderboard interface {
	Leaderboard(ctx context.Context, roomID int64) (gameroom.Leaderboard, error)
}

// maxAnonymousIDLength matches what the comments endpoints accept, so one browser id
// works everywhere.
const maxAnonymousIDLength = 255

// roomAnonymousID validates the caller's browser id, returning "" when it is unusable.
//
// DELIBERATELY NOT normalizedAnonymousID, which substitutes the literal "unknown" for a
// missing value. That default is harmless for comments — anonymous readers get grouped
// under one label and nothing is owned. In a room it would be a shared identity: every
// visitor with no id would land on ONE participant row, see each other's score, and
// overwrite each other's wagers. Laravel had exactly that behaviour
// (`session()->get('anonymous_id', 'unknown')`); refusing is the fix.
func roomAnonymousID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxAnonymousIDLength {
		return ""
	}
	return value
}

func (a *api) requireGameRoom(w http.ResponseWriter, r *http.Request) bool {
	if a.gameRooms == nil || a.gameRoomReader == nil || a.gameRoomBoard == nil {
		writeError(w, r, http.StatusServiceUnavailable, "game_rooms_unavailable",
			"game rooms are not configured on this server")
		return false
	}
	return true
}

// gameRoomResponse is what a client needs to draw the room.
type gameRoomResponse struct {
	Serial string `json:"serial"`
	// GameSerial lets a client that only has a room link find the game it belongs to,
	// which the old UI needed a separate request for.
	GameSerial  string                `json:"game_serial"`
	Player      *gameRoomPlayer       `json:"player"`
	Votes       *gameroom.VoteTally   `json:"votes"`
	LatestBet   *gameRoomBet          `json:"latest_bet"`
	Leaderboard *gameroom.Leaderboard `json:"leaderboard"`
}

// gameRoomPlayer mirrors GameRoomUserResource, including the digest instead of the row
// id: the id is a database key, and a room link is public.
type gameRoomPlayer struct {
	PlayerID     string `json:"player_id"`
	Name         string `json:"name"`
	Score        int    `json:"score"`
	Rank         int    `json:"rank"`
	Accuracy     string `json:"accuracy"`
	TotalPlayed  int    `json:"total_played"`
	TotalCorrect int    `json:"total_correct"`
	Combo        int    `json:"combo"`
}

type gameRoomBet struct {
	WinnerID       int64 `json:"winner_id"`
	LoserID        int64 `json:"loser_id"`
	CurrentRound   int   `json:"current_round"`
	OfRound        int   `json:"of_round"`
	RemainElements int   `json:"remain_elements"`
}

func playerFromParticipant(participant gameroom.Participant) *gameRoomPlayer {
	return &gameRoomPlayer{
		PlayerID: participant.PlayerID(),
		Name:     participant.Nickname,
		Score:    participant.Score,
		Rank:     participant.Rank,
		// A string, not a number, for the same reason the leaderboard uses one: the
		// client interpolates it straight into "勝率:{}%" and a float would render as
		// 63.489999999999995.
		Accuracy:     gameroom.FormatAccuracy(participant.AccuracyHundredths),
		TotalPlayed:  participant.TotalPlayed,
		TotalCorrect: participant.TotalCorrect,
		Combo:        participant.Combo,
	}
}

// createGameRoom opens the room for a game, or returns the one that already exists.
//
// Idempotent, which is why it can be called on every load of the host's page. The 200 vs
// 201 distinction is kept because it is the only way a host can tell "I just opened this"
// from "this was already running".
func (a *api) createGameRoom(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameRoom(w, r) {
		return
	}

	var request struct {
		GameSerial string `json:"game_serial"`
		// The pair already on screen when the room is opened. Laravel's createRoom took
		// the same parameter, for the same reason: a host opens the room mid-game, and
		// without this the room's first participants see whatever pair was last decided.
		CurrentCandidates []int64 `json:"current_candidates"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(request.GameSerial) == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_game_serial", "game_serial is required")
		return
	}

	room, created, err := a.gameRooms.EnsureRoom(r.Context(), request.GameSerial, request.CurrentCandidates)
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	// Private: a room serial is a capability, and this response hands one out.
	writePrivateJSON(w, r, status, gameRoomResponse{Serial: room.Serial, GameSerial: request.GameSerial})
}

// gameRoomState is the one call a joining client makes.
//
// Laravel spread this over three endpoints — getRoom, getRoomVotes, getRoomUser — which
// meant three round trips before the page could draw, each resolving the same room and
// each able to see a different moment. One call also means the player row is created
// once rather than by whichever of the three arrived first.
func (a *api) gameRoomState(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameRoom(w, r) {
		return
	}

	room, gameSerial, ok := a.resolveRoom(w, r)
	if !ok {
		return
	}

	// game_serial is optional here and checked when supplied: a client holding a stale
	// link would otherwise silently join a different game's room.
	if requested := strings.TrimSpace(r.URL.Query().Get("game_serial")); requested != "" && requested != gameSerial {
		writeError(w, r, http.StatusForbidden, "room_game_mismatch",
			"the room does not belong to that game")
		return
	}

	anonymousID := roomAnonymousID(r.URL.Query().Get("anonymous_id"))
	if anonymousID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_anonymous_id",
			"anonymous_id is required and must contain at most 255 characters")
		return
	}

	participant, err := a.gameRooms.Join(r.Context(), room.ID, anonymousID, a.optionalUserID(r))
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	response := gameRoomResponse{
		Serial:     room.Serial,
		GameSerial: gameSerial,
		Player:     playerFromParticipant(participant),
	}

	if votes, present, err := a.gameRoomReader.CurrentVotes(r.Context(), room.ID, gameSerial); err != nil {
		a.writeGameRoomError(w, r, err)
		return
	} else if present {
		response.Votes = &votes
	}

	if bet, found, err := a.gameRoomReader.LatestBet(r.Context(), participant.ID); err != nil {
		a.writeGameRoomError(w, r, err)
		return
	} else if found {
		response.LatestBet = &gameRoomBet{
			WinnerID: bet.WinnerID, LoserID: bet.LoserID, CurrentRound: bet.CurrentRound,
			OfRound: bet.OfRound, RemainElements: bet.RemainElements,
		}
	}

	board, err := a.gameRoomBoard.Leaderboard(r.Context(), room.ID)
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}
	response.Leaderboard = &board

	writePrivateJSON(w, r, http.StatusOK, response)
}

// gameRoomLeaderboard is the poll fallback for clients with no working websocket.
//
// The live path is the Soketi broadcast the worker publishes after each settlement; this
// exists because a blocked websocket must degrade to a stale-but-moving leaderboard
// rather than to a frozen one.
func (a *api) gameRoomLeaderboard(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameRoom(w, r) {
		return
	}
	room, _, ok := a.resolveRoom(w, r)
	if !ok {
		return
	}

	board, err := a.gameRoomBoard.Leaderboard(r.Context(), room.ID)
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}
	writePrivateJSON(w, r, http.StatusOK, board)
}

// placeGameRoomBet records a wager on the round in progress.
func (a *api) placeGameRoomBet(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameRoom(w, r) {
		return
	}

	// The pick, and nothing else.
	//
	// The round numbers are NOT taken from the client. game_1v1_rounds records completed
	// matches, so "the match in progress" is one past the last row and crossing a bracket
	// boundary is real arithmetic; a participant is watching somebody else's game and has
	// none of that state. The server derives them, and also checks that the pick is the
	// pairing actually on screen — otherwise a stale page could wager on a matchup the
	// settlement would never resolve.
	var request struct {
		AnonymousID string `json:"anonymous_id"`
		WinnerID    int64  `json:"winner_id"`
		LoserID     int64  `json:"loser_id"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	room, gameSerial, ok := a.resolveRoom(w, r)
	if !ok {
		return
	}

	anonymousID := roomAnonymousID(request.AnonymousID)
	if anonymousID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_anonymous_id",
			"anonymous_id is required and must contain at most 255 characters")
		return
	}

	participant, err := a.gameRooms.Join(r.Context(), room.ID, anonymousID, a.optionalUserID(r))
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	if err := a.gameRooms.BetOnCurrentRound(
		r.Context(), room.ID, participant, gameSerial, request.WinnerID, request.LoserID); err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	// 204: the wager is recorded but nothing is decided yet. The outcome arrives on the
	// room's channel when the host's vote settles the round, so returning a leaderboard
	// here would only hand back the one the caller already has.
	writeJSON(w, r, http.StatusNoContent, envelope{})
}

// renameGameRoomPlayer changes the caller's display name in a room.
func (a *api) renameGameRoomPlayer(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameRoom(w, r) {
		return
	}

	var request struct {
		AnonymousID string `json:"anonymous_id"`
		Nickname    string `json:"nickname"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	room, _, ok := a.resolveRoom(w, r)
	if !ok {
		return
	}

	anonymousID := roomAnonymousID(request.AnonymousID)
	if anonymousID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_anonymous_id",
			"anonymous_id is required and must contain at most 255 characters")
		return
	}

	participant, err := a.gameRooms.Join(r.Context(), room.ID, anonymousID, a.optionalUserID(r))
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	if err := a.gameRooms.Rename(r.Context(), participant, request.Nickname); err != nil {
		a.writeGameRoomError(w, r, err)
		return
	}

	participant.Nickname = strings.TrimSpace(request.Nickname)
	writePrivateJSON(w, r, http.StatusOK, playerFromParticipant(participant))
}

// resolveRoom reads the {serial} path value and answers 404 when it does not resolve.
func (a *api) resolveRoom(w http.ResponseWriter, r *http.Request) (gameroom.Room, string, bool) {
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_room_serial",
			"a room serial is required and must contain at most 255 characters")
		return gameroom.Room{}, "", false
	}

	room, gameSerial, found, err := a.gameRoomReader.RoomBySerialWithGame(r.Context(), serial)
	if err != nil {
		a.writeGameRoomError(w, r, err)
		return gameroom.Room{}, "", false
	}
	if !found {
		writeError(w, r, http.StatusNotFound, "room_not_found", "the room does not exist")
		return gameroom.Room{}, "", false
	}
	return room, gameSerial, true
}

// optionalUserID reads the caller's account when they happen to be signed in.
//
// A room is playable anonymously, so this never refuses. The id is recorded on the
// participant row for audit; identity in a room is the anonymous id, not the account.
func (a *api) optionalUserID(r *http.Request) *int64 {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		return nil
	}
	userID, err := auth.SubjectToUserID(identity.Subject)
	if err != nil {
		return nil
	}
	return &userID
}

func (a *api) writeGameRoomError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gameroom.ErrGameNotFound):
		writeError(w, r, http.StatusNotFound, "game_not_found", "the game does not exist")
	case errors.Is(err, gameroom.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "room_not_found", "the room does not exist")
	case errors.Is(err, gameroom.ErrRoomMismatch):
		writeError(w, r, http.StatusForbidden, "room_game_mismatch",
			"the room does not belong to that game")
	case errors.Is(err, gameroom.ErrNicknameTooSoon):
		// 429 with the same code Laravel's threshold produced, so a client that already
		// handles it does not need to change.
		writeError(w, r, http.StatusTooManyRequests, "rename_too_soon",
			"the nickname was changed too recently")
	case errors.Is(err, gameroom.ErrNoRoundInProgress):
		// 409: the request is well formed but the room is between matches. A retry after
		// the host advances will work, which 422 would not suggest.
		writeError(w, r, http.StatusConflict, "no_round_in_progress",
			"the host has not put a pairing on screen yet")
	case errors.Is(err, gameroom.ErrNotTheCurrentPairing):
		writeError(w, r, http.StatusConflict, "stale_pairing",
			"that is not the pairing currently on screen")
	case errors.Is(err, gameroom.ErrInvalidNickname):
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_nickname",
			"a nickname is required and must contain at most 10 characters")
	default:
		a.logger.Error("game_room_request_failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error",
			"the request could not be completed")
	}
}
