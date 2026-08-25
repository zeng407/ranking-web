package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"2pick.app/backend/internal/gameplay"
	"2pick.app/backend/internal/gameroom"
)

const (
	maxGameRequestBytes = 64 << 10
	maxBatchVotes       = 128
)

type createGameRequest struct {
	PostSerial   string `json:"post_serial"`
	ElementCount int    `json:"element_count"`
}

type submitGameVotesRequest struct {
	ExpectedVoteCount *int            `json:"expected_vote_count"`
	Votes             []gameplay.Vote `json:"votes"`
	AnonymousID       string          `json:"anonymous_id"`
	// CurrentCandidates is the pair the client is showing after these votes when hosting a
	// room, and the final two in the order they were shown on the batch that finishes the
	// game. See gameplay.BatchInput.CurrentCandidates for why the server cannot work
	// either one out itself.
	CurrentCandidates []int64 `json:"current_candidates"`
}

func (a *api) gameDefinition(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameplay(w, r) {
		return
	}
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_post_serial", "post serial is required and must contain at most 255 characters")
		return
	}
	caller := a.callerFor(r)
	definition, err := a.gameplay.Definition(r.Context(), serial, caller)
	if err != nil {
		a.writeGameplayError(w, r, err)
		return
	}
	a.writeScopedJSON(w, r, caller, serial, definition)
}

func (a *api) createGame(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameplay(w, r) {
		return
	}
	var request createGameRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	request.PostSerial = strings.TrimSpace(request.PostSerial)
	if request.PostSerial == "" || utf8.RuneCountInString(request.PostSerial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_post_serial", "post_serial is required and must contain at most 255 characters")
		return
	}
	if request.ElementCount < 2 || request.ElementCount > 1024 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_element_count", "element_count must be between 2 and 1024")
		return
	}

	caller := a.callerFor(r)
	session, err := a.gameplay.Create(r.Context(), gameplay.CreateInput{
		PostSerial: request.PostSerial, ElementCount: request.ElementCount, Caller: caller,
	})
	if err != nil {
		a.writeGameplayError(w, r, err)
		return
	}
	a.refreshPostAccess(w, caller, request.PostSerial)
	writePrivateJSON(w, r, http.StatusCreated, session)
}

func (a *api) resumeGame(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameplay(w, r) {
		return
	}
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_game_serial", "game serial is required and must contain at most 255 characters")
		return
	}
	caller := a.callerFor(r)
	session, err := a.gameplay.Resume(r.Context(), serial, caller)
	if err != nil {
		a.writeGameplayError(w, r, err)
		return
	}
	// Keyed on the POST serial, not the game serial: the token proves a door code, and
	// the path here names the game.
	a.refreshPostAccess(w, caller, session.Definition.Serial)
	writePrivateJSON(w, r, http.StatusOK, session)
}

func (a *api) gameResult(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameplay(w, r) {
		return
	}
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_game_serial", "game serial is required and must contain at most 255 characters")
		return
	}
	caller := a.callerFor(r)
	result, err := a.gameplay.Result(r.Context(), serial, caller)
	if err != nil {
		a.writeGameplayError(w, r, err)
		return
	}
	// A game serial is a share capability. Do not let a shared user's personal
	// result leak into a broader CDN cache key or become stale as global ranks move.
	a.refreshPostAccess(w, caller, result.PostSerial)
	writePrivateJSON(w, r, http.StatusOK, result)
}

func (a *api) submitGameVotes(w http.ResponseWriter, r *http.Request) {
	if !a.requireGameplay(w, r) {
		return
	}
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" || utf8.RuneCountInString(serial) > 255 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_game_serial", "game serial is required and must contain at most 255 characters")
		return
	}
	var request submitGameVotesRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if request.ExpectedVoteCount == nil || *request.ExpectedVoteCount < 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_revision", "expected_vote_count is required and must be zero or greater")
		return
	}
	request.Votes = uniqueVotes(request.Votes)
	if len(request.Votes) == 0 || len(request.Votes) > maxBatchVotes {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_votes", fmt.Sprintf("votes must contain between 1 and %d unique votes", maxBatchVotes))
		return
	}
	for _, vote := range request.Votes {
		if vote.WinnerID <= 0 || vote.LoserID <= 0 || vote.WinnerID == vote.LoserID {
			writeError(w, r, http.StatusUnprocessableEntity, "invalid_vote", "winner_id and loser_id must be different positive integers")
			return
		}
	}
	request.AnonymousID = strings.TrimSpace(request.AnonymousID)
	if utf8.RuneCountInString(request.AnonymousID) > 128 {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_anonymous_id", "anonymous_id must contain at most 128 characters")
		return
	}

	result, err := a.gameplay.SubmitVotes(r.Context(), serial, gameplay.BatchInput{
		ExpectedVoteCount: *request.ExpectedVoteCount,
		Votes:             request.Votes,
		AnonymousID:       request.AnonymousID,
		CurrentCandidates: request.CurrentCandidates,
		Caller:            a.callerFor(r),
	})
	if err != nil {
		var conflict *gameplay.ConflictError
		if errors.As(err, &conflict) {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, r, http.StatusConflict, envelope{
				Data:  map[string]any{"reason": conflict.Reason, "server_vote_count": conflict.ServerVoteCount},
				Error: &apiErr{Code: "game_state_conflict", Message: "the server and local game branches differ"},
			})
			return
		}
		a.writeGameplayError(w, r, err)
		return
	}

	if result.JustCompleted {
		a.flagPostRanksStale(r, result.PostID)
	}
	a.announceGameRoomRounds(r, serial, result.SettledRounds)
	writePrivateJSON(w, r, http.StatusOK, result)
}

// announceGameRoomRounds tells the room about the matches this batch decided.
//
// THE PRODUCER THE WORKER WAS MISSING. Until this existed the Go vote path recorded rounds
// and stopped, so a host playing through Go settled nobody's wagers and every room's
// leaderboard sat still while the handler that would have moved it waited for a message
// nothing published.
//
// Best effort and detached, for the same reason flagPostRanksStale is: the votes are
// already committed by the time this runs, so failing the response would make the client
// retry a batch the server has accepted and collect a conflict for it. A lost announcement
// costs the room one stale round; the next vote publishes a refresh that tallies from the
// version counter and catches up.
//
// Logged at error level even so — if this starts failing steadily, rooms silently stop
// updating and nothing else reveals it.
func (a *api) announceGameRoomRounds(r *http.Request, gameSerial string, rounds []gameplay.SettledRound) {
	if len(rounds) == 0 {
		return
	}

	// The round the host has just moved onto is a new round, and in a majority room that
	// means a new countdown. Done here because this is the seam every settled round
	// already passes through, and before the announce below so the frame the worker
	// builds from it carries the deadline that was just armed. Ahead of the announcer
	// check because a room is playable by polling alone, and its clock has to run either
	// way.
	a.armGameRoomRound(r, gameSerial)

	if a.gameRoomAnnouncer == nil {
		return
	}

	decided := make([]gameroom.DecidedRound, 0, len(rounds))
	for _, round := range rounds {
		decided = append(decided, gameroom.DecidedRound{
			WinnerID: round.WinnerID, LoserID: round.LoserID,
			CurrentRound: round.CurrentRound, OfRound: round.OfRound,
			RemainElements: round.RemainElements,
		})
	}

	// Detached from the request context: the response is about to be written, and
	// cancelling the publish because the client hung up would drop the room's update.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), announceTimeout)
	defer cancel()

	published, err := a.gameRoomAnnouncer.AnnounceRounds(ctx, gameSerial, decided)
	if err != nil {
		a.logger.Error("game_room_announce_failed",
			"game_serial", gameSerial, "rounds", len(decided), "error", err)
		return
	}
	if published > 0 {
		a.logger.Info("game_room_rounds_announced", "game_serial", gameSerial, "rounds", published)
	}
}

// armGameRoomRound restarts the round clock for a room whose host has advanced the game.
//
// Best effort, like everything else on this path: the votes are committed, and failing the
// response over a clock would make the client retry a batch the server has accepted. What a
// lost arming costs is one round whose countdown reads as already expired, which the host's
// client settles at once — the room moves on rather than stalling.
func (a *api) armGameRoomRound(r *http.Request, gameSerial string) {
	if a.gameRooms == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), announceTimeout)
	defer cancel()

	if err := a.gameRooms.ArmRound(ctx, gameSerial); err != nil {
		a.logger.Error("game_room_arm_round_failed", "game_serial", gameSerial, "error", err)
	}
}

// announceRoomPairing tells a room to redraw the pairing its host now has up.
//
// Same best-effort treatment as announceGameRoomRounds and for the same reason: the move it
// follows is already committed, and failing the response would have the client retry
// something the server has accepted. What it costs to lose is one round of staleness — the
// room's own poll re-reads the whole state on a timer and corrects itself.
func (a *api) announceRoomPairing(r *http.Request, gameSerial string) {
	if a.gameRoomAnnouncer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), announceTimeout)
	defer cancel()

	published, err := a.gameRoomAnnouncer.AnnounceRoom(ctx, gameSerial)
	if err != nil {
		a.logger.Error("game_room_pairing_announce_failed", "game_serial", gameSerial, "error", err)
		return
	}
	if published {
		a.logger.Info("game_room_pairing_announced", "game_serial", gameSerial)
	}
}

// announceTimeout bounds the publish. Short: it is two or three Redis writes, and a slow
// Redis must not hold the response open.
const announceTimeout = 3 * time.Second

// flagPostRanksStale marks the post as needing a rank history rebuild, which is
// what App\Listeners\UpdatePostRank does on the GameComplete event.
//
// Best effort, deliberately. The votes are already committed by the time this runs,
// so returning an error here would make the client retry a batch the server has
// accepted and get a conflict for its trouble. A lost flag costs one post one day
// of history freshness, and the next completed game on that post sets it again —
// which is a far smaller failure than rejecting a completed game.
//
// It is logged at error level even so: if this starts failing steadily the history
// build quietly stops seeing new posts, and nothing else would reveal that.
func (a *api) flagPostRanksStale(r *http.Request, postID int64) {
	if a.rankFreshness == nil || postID <= 0 {
		return
	}
	// Detached from the request context: the response is about to be written, and a
	// client that disconnects must not cancel the flag for a game that did finish.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), rankFreshnessTimeout)
	defer cancel()

	if err := a.rankFreshness.Set(ctx, postID); err != nil {
		a.logger.Error("rank_freshness_flag_failed", "post_id", postID, "error", err)
	}
}

// rankFreshnessTimeout bounds the one Redis write. It is short because the client is
// waiting on the response behind it.
const rankFreshnessTimeout = 2 * time.Second

func (a *api) requireGameplay(w http.ResponseWriter, r *http.Request) bool {
	if a.gameplay == nil {
		writeError(w, r, http.StatusServiceUnavailable, "gameplay_not_configured", "gameplay database is not configured")
		return false
	}
	return true
}

func (a *api) writeGameplayError(w http.ResponseWriter, r *http.Request, err error) {
	if wroteSignInRequired(w, r, err) {
		return
	}
	if errors.Is(err, gameplay.ErrInvalidElementCount) {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_element_count", err.Error())
		return
	}
	if errors.Is(err, gameplay.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "game or public post was not found")
		return
	}
	a.logger.Error("gameplay_request_failed",
		"request_id", requestIDFromContext(r.Context()), "path", r.URL.Path, "error", err,
	)
	writeError(w, r, http.StatusServiceUnavailable, "gameplay_unavailable", "gameplay is temporarily unavailable")
}

func writePrivateJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Cloudflare-CDN-Cache-Control", "no-store")
	writeJSON(w, r, status, envelope{Data: data})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxGameRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("request body must contain valid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func uniqueVotes(votes []gameplay.Vote) []gameplay.Vote {
	unique := make([]gameplay.Vote, 0, len(votes))
	seen := make(map[string]struct{}, len(votes))
	for _, vote := range votes {
		key := fmt.Sprintf("%d:%d", vote.WinnerID, vote.LoserID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, vote)
	}
	return unique
}
