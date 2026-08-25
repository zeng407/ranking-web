package gameroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/realtime"
)

// Handler contracts.
//
// Laravel declared none of this, so both values are stated rather than inherited.
// A refresh in the largest observed room, 1,088 players, is three statements; the
// timeout is generous enough to absorb a slow replica without being long enough to
// hold a worker slot through an outage.
const (
	SettleTimeout   = 15 * time.Second
	RefreshTimeout  = 30 * time.Second
	SettleAttempts  = 5
	RefreshAttempts = 5
	// A round broadcast is one indexed read and one HTTP call to Soketi. Short, and
	// retried less: the pairing it carries is worth having within a second or two and
	// worthless a minute later, by which time the host has moved on and the room's own
	// poll has corrected itself anyway.
	RoundTimeout  = 10 * time.Second
	RoundAttempts = 3
)

// Lock namespaces for the per-room serialization.
//
// Deliberately separate: settlement writes wagers, a refresh reads them and writes
// standings, and the version counter already makes the pair safe to interleave — a
// settlement bumps the version only after its commit, so a refresh that ran during
// it records the older version and the next refresh picks up the difference. Sharing
// one key would make every vote in a busy room wait behind the refresh it triggered.
const (
	SettleLockPrefix  = "gameroom:settle:"
	RefreshLockPrefix = "gameroom:refresh:"
	// Its own namespace so a round broadcast never waits behind the recompute a
	// settlement triggered: the pairing is what the participants are looking at, and it
	// must arrive while the leaderboard is still being tallied.
	RoundLockPrefix = "gameroom:round:"
)

// SettlePayload is the message the vote path publishes. Port of the array
// NotifyVoted passes to UpdateGameBet.
type SettlePayload struct {
	RoomSerial     string `json:"room_serial"`
	WinnerID       int64  `json:"winner_id"`
	LoserID        int64  `json:"loser_id"`
	CurrentRound   int    `json:"current_round"`
	OfRound        int    `json:"of_round"`
	RemainElements int    `json:"remain_elements"`
}

func (payload SettlePayload) validate() error {
	if payload.RoomSerial == "" {
		return errors.New("room_serial is required")
	}
	if payload.WinnerID <= 0 || payload.LoserID <= 0 {
		return errors.New("winner_id and loser_id are required")
	}
	if payload.WinnerID == payload.LoserID {
		return errors.New("winner_id and loser_id must differ")
	}
	return nil
}

// RefreshPayload asks for a leaderboard rebuild.
type RefreshPayload struct {
	RoomSerial string `json:"room_serial"`
}

// RoundPayload asks for the room's current pairing to be broadcast.
//
// IT CARRIES NO TALLY, DELIBERATELY. The publisher is the API, which has just written the
// pair to games.candidates, and the handler reads the tally back through the same call the
// room's REST endpoint uses. So the event and the poll cannot disagree: they are the same
// query. A payload built at publish time would instead have to derive the round numbers a
// second way, and any drift between the two would show up as a pairing that jumps when the
// poll catches up.
type RoundPayload struct {
	RoomSerial string `json:"room_serial"`
	GameSerial string `json:"game_serial"`
}

func (payload RoundPayload) validate() error {
	if payload.RoomSerial == "" {
		return errors.New("room_serial is required")
	}
	if payload.GameSerial == "" {
		return errors.New("game_serial is required")
	}
	return nil
}

// RoundBroadcast is what a subscriber receives.
//
// GameSerial is not padding: a host who restarts points the room at a new game, and a
// participant has to reload that game's elements before the ids in the tally mean
// anything. Votes is null between rounds, which is a normal answer — the client keeps
// what it has and waits for the next event rather than blanking the board.
//
// Voting rides along because a new round is also a new deadline: the countdown every client
// is showing has just been re-armed, and this frame is what tells them so. Null only when
// the settings could not be read, which leaves the client on the last ones it had.
type RoundBroadcast struct {
	GameSerial string          `json:"game_serial"`
	Votes      *VoteTally      `json:"votes"`
	Voting     *VotingSettings `json:"voting"`
}

// VoteReader reads the tally for the pairing a room's host has on screen, and the settings
// the room is voting under.
//
// Satisfied by MySQLParticipation, which the API also reads through: one query, one
// meaning of "the match in progress".
type VoteReader interface {
	CurrentVotes(ctx context.Context, roomID int64, gameSerial string) (VoteTally, bool, error)
	Voting(ctx context.Context, roomID int64) (VotingSettings, error)
}

// Options wires the service.
type Options struct {
	Repository  Repository
	Tracker     RefreshTracker
	Legacy      LegacyCache
	Broadcaster Broadcaster
	Votes       VoteReader
	Publisher   *queue.Publisher
	Scoring     Scoring
	Logger      *slog.Logger
}

// Service owns the two handlers.
type Service struct {
	repository  Repository
	tracker     RefreshTracker
	legacy      LegacyCache
	broadcaster Broadcaster
	votes       VoteReader
	publisher   *queue.Publisher
	scoring     Scoring
	logger      *slog.Logger
}

func NewService(options Options) (*Service, error) {
	if options.Repository == nil {
		return nil, errors.New("gameroom: repository is required")
	}
	if options.Tracker == nil {
		return nil, errors.New("gameroom: refresh tracker is required")
	}
	if options.Broadcaster == nil {
		return nil, errors.New("gameroom: broadcaster is required")
	}
	if options.Votes == nil {
		return nil, errors.New("gameroom: vote reader is required")
	}
	if options.Publisher == nil {
		return nil, errors.New("gameroom: publisher is required")
	}
	legacy := options.Legacy
	if legacy == nil {
		legacy = NoLegacyCache{}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	scoring := options.Scoring
	if scoring == (Scoring{}) {
		scoring = DefaultScoring()
	}
	return &Service{
		repository:  options.Repository,
		tracker:     options.Tracker,
		legacy:      legacy,
		broadcaster: options.Broadcaster,
		votes:       options.Votes,
		publisher:   options.Publisher,
		scoring:     scoring,
		logger:      logger,
	}, nil
}

// SettleRegistration replaces UpdateGameBet.
func (service *Service) SettleRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeBetSettled,
		Handler:     jobs.HandlerFunc(service.handleSettle),
		Timeout:     SettleTimeout,
		MaxAttempts: SettleAttempts,
		SerialKey:   serialKeyFor[SettlePayload](SettleLockPrefix),
		LaravelJob:  "App\\Jobs\\UpdateGameBet",
	}
}

// RefreshRegistration replaces UpdateGameRoomRank and ReUpdateGameRoomRank.
func (service *Service) RefreshRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeRankRefresh,
		Handler:     jobs.HandlerFunc(service.handleRefresh),
		Timeout:     RefreshTimeout,
		MaxAttempts: RefreshAttempts,
		SerialKey:   serialKeyFor[RefreshPayload](RefreshLockPrefix),
		LaravelJob:  "App\\Jobs\\UpdateGameRoomRank",
	}
}

// RoundRegistration broadcasts the pairing the host moved to. Replaces
// BroadcastGameRoomRefresh.
func (service *Service) RoundRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeRoundChanged,
		Handler:     jobs.HandlerFunc(service.handleRound),
		Timeout:     RoundTimeout,
		MaxAttempts: RoundAttempts,
		SerialKey:   serialKeyFor[RoundPayload](RoundLockPrefix),
		LaravelJob:  "App\\Jobs\\BroadcastGameRoomRefresh",
	}
}

// roomSerialed is satisfied by every payload, so one serial-key helper covers them.
type roomSerialed interface {
	serial() string
}

func (payload SettlePayload) serial() string  { return payload.RoomSerial }
func (payload RefreshPayload) serial() string { return payload.RoomSerial }
func (payload RoundPayload) serial() string   { return payload.RoomSerial }

func serialKeyFor[T roomSerialed](prefix string) jobs.SerialKeyFunc {
	return func(message queue.Message) (string, error) {
		var payload T
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return "", fmt.Errorf("gameroom: decode payload for serial key: %w", err)
		}
		if payload.serial() == "" {
			return "", errors.New("gameroom: payload has no room serial")
		}
		return prefix + payload.serial(), nil
	}
}

// handleSettle applies one round's outcome and asks for a refresh.
//
// Order matters and is the reason this is not one step. The version must be
// incremented only after the settlement has committed: a refresh that ran between
// the increment and the commit would tally the room without this round's wagers and
// then record that version as applied, and since nothing would raise the version
// again, the round's scores would be lost until the next vote.
func (service *Service) handleSettle(ctx context.Context, message queue.Message) error {
	var payload SettlePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return jobs.Permanent(fmt.Errorf("gameroom: decode settle payload: %w", err))
	}
	if err := payload.validate(); err != nil {
		return jobs.Permanent(fmt.Errorf("gameroom: settle payload: %w", err))
	}

	room, err := service.repository.RoomBySerial(ctx, payload.RoomSerial)
	if errors.Is(err, ErrNotFound) {
		// The serial is wrong and will stay wrong, so retrying cannot help.
		return jobs.Permanent(err)
	}
	if err != nil {
		return err
	}

	settled, err := service.repository.SettleBets(ctx, BetOutcome{
		RoomID:         room.ID,
		WinnerID:       payload.WinnerID,
		LoserID:        payload.LoserID,
		CurrentRound:   payload.CurrentRound,
		OfRound:        payload.OfRound,
		RemainElements: payload.RemainElements,
	})
	if err != nil {
		return err
	}

	version, err := service.tracker.MarkChanged(ctx, room.Serial)
	if err != nil {
		return err
	}

	refresh, err := RefreshMessage(room.Serial)
	if err != nil {
		return err
	}
	if err := service.publisher.Publish(ctx, refresh); err != nil {
		return err
	}

	service.logger.Info("game_room_bet_settled",
		"room_serial", room.Serial,
		"won", settled.Won,
		"lost", settled.Lost,
		"discarded", settled.Discarded,
		"version", version,
	)
	return nil
}

// handleRefresh rebuilds the leaderboard and broadcasts it.
func (service *Service) handleRefresh(ctx context.Context, message queue.Message) error {
	var payload RefreshPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return jobs.Permanent(fmt.Errorf("gameroom: decode refresh payload: %w", err))
	}
	if payload.RoomSerial == "" {
		return jobs.Permanent(errors.New("gameroom: refresh payload has no room serial"))
	}

	outstanding, err := service.tracker.Outstanding(ctx, payload.RoomSerial)
	if err != nil {
		return err
	}
	if !outstanding.Pending() {
		// Coalesced. Another delivery already tallied every settled wager up to this
		// point, which is the common case in a busy room: one refresh does the work
		// and the rest of the burst acknowledges for the cost of one Redis read. This
		// is what ReUpdateGameRoomRank and the waiting-job flag existed to arrange.
		service.logger.Debug("game_room_refresh_coalesced",
			"room_serial", payload.RoomSerial, "applied", outstanding.Applied)
		return nil
	}

	room, err := service.repository.RoomBySerial(ctx, payload.RoomSerial)
	if errors.Is(err, ErrNotFound) {
		return jobs.Permanent(err)
	}
	if err != nil {
		return err
	}

	// Captured before the work: any wager settled while the recompute runs raises the
	// version past this, so the refresh those settlements published still has work to
	// do rather than finding itself already covered.
	target := outstanding.Version

	started := time.Now()
	if _, err := service.repository.RecomputeTotals(ctx, room.ID); err != nil {
		return err
	}
	moved, err := service.repository.AssignRanks(ctx, room.ID)
	if err != nil {
		return err
	}
	board, err := service.repository.Leaderboard(ctx, room.ID)
	if err != nil {
		return err
	}

	// Invalidated before the broadcast: a client that reacts to the event by calling
	// the PHP endpoint must not be served the entry this refresh just made stale.
	if err := service.legacy.InvalidateLeaderboard(ctx, room.Serial); err != nil {
		// The live path is the broadcast, so a stale legacy cache is worth a warning
		// rather than a retry that would repeat the whole recompute.
		service.logger.Warn("game_room_legacy_cache_not_invalidated",
			"room_serial", room.Serial, "error", err)
	}

	// Broadcast BEFORE recording the version. If the order were reversed and the
	// broadcast failed, the retry would find version == applied, acknowledge without
	// broadcasting, and the room would sit on a stale leaderboard until the next vote.
	// A repeated broadcast, by contrast, only makes clients redraw what they have.
	if err := service.broadcaster.Publish(ctx,
		realtime.GameRoomChannel(room.Serial), BroadcastEvent, board); err != nil {
		return err
	}

	if err := service.tracker.MarkApplied(ctx, room.Serial, target); err != nil {
		return err
	}

	// The PHP job cleared its processing flag only when it was not re-dispatching.
	// The equivalent is clearing it once nothing is outstanding; a vote that landed
	// during the recompute leaves it set, and the refresh that vote published clears
	// it when it finishes.
	if final, err := service.tracker.Outstanding(ctx, room.Serial); err == nil && !final.Pending() {
		if err := service.legacy.ClearUpdatingFlag(ctx, room.Serial); err != nil {
			service.logger.Warn("game_room_legacy_flag_not_cleared",
				"room_serial", room.Serial, "error", err)
		}
	}

	service.logger.Info("game_room_rank_refreshed",
		"room_serial", room.Serial,
		"total_users", board.TotalUsers,
		"players_moved", moved,
		"applied_version", target,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

// handleRound broadcasts the pairing the room is now voting on.
//
// NO COALESCING, unlike the refresh. A refresh recomputes a whole room and two of them in a
// row produce the same standings, so skipping the second is free. A round event is the
// opposite: every one of them is a different match, and dropping one leaves the room
// looking at a pairing the host has already left. The work is one read and one publish, so
// there is nothing to coalesce for.
func (service *Service) handleRound(ctx context.Context, message queue.Message) error {
	var payload RoundPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return jobs.Permanent(fmt.Errorf("gameroom: decode round payload: %w", err))
	}
	if err := payload.validate(); err != nil {
		return jobs.Permanent(fmt.Errorf("gameroom: round payload: %w", err))
	}

	room, err := service.repository.RoomBySerial(ctx, payload.RoomSerial)
	if errors.Is(err, ErrNotFound) {
		return jobs.Permanent(err)
	}
	if err != nil {
		return err
	}

	broadcast := RoundBroadcast{GameSerial: payload.GameSerial}
	tally, inProgress, err := service.votes.CurrentVotes(ctx, room.ID, payload.GameSerial)
	if err != nil {
		return err
	}
	if inProgress {
		broadcast.Votes = &tally
	}
	voting, err := service.votes.Voting(ctx, room.ID)
	if err != nil {
		return err
	}
	broadcast.Voting = &voting

	if err := service.broadcaster.Publish(ctx,
		realtime.GameRoomChannel(room.Serial), RoundEvent, broadcast); err != nil {
		return err
	}

	service.logger.Info("game_room_round_broadcast",
		"room_serial", room.Serial,
		"game_serial", payload.GameSerial,
		"in_progress", inProgress,
	)
	return nil
}

// SettleMessage builds the message the vote path publishes once a round is decided.
// Exported because the publisher is the API, not this package.
func SettleMessage(payload SettlePayload) (queue.Message, error) {
	if err := payload.validate(); err != nil {
		return queue.Message{}, fmt.Errorf("gameroom: settle message: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return queue.Message{}, fmt.Errorf("gameroom: encode settle payload: %w", err)
	}
	return queue.Message{
		Queue:   Queue,
		Type:    TypeBetSettled,
		Payload: body,
		// One round of one room settles once. A redelivery re-runs the same UPDATEs,
		// which are idempotent, so the key is for tracing rather than suppression.
		IdempotencyKey: fmt.Sprintf("%s:%d:%d:%d:%d", payload.RoomSerial,
			payload.CurrentRound, payload.OfRound, payload.WinnerID, payload.LoserID),
	}, nil
}

// RefreshMessage builds a leaderboard rebuild request.
func RefreshMessage(roomSerial string) (queue.Message, error) {
	if roomSerial == "" {
		return queue.Message{}, errors.New("gameroom: refresh message needs a room serial")
	}
	body, err := json.Marshal(RefreshPayload{RoomSerial: roomSerial})
	if err != nil {
		return queue.Message{}, fmt.Errorf("gameroom: encode refresh payload: %w", err)
	}
	return queue.Message{Queue: Queue, Type: TypeRankRefresh, Payload: body}, nil
}

// RoundMessage builds the message that broadcasts the pairing now on screen.
//
// No idempotency key. The same pairing broadcast twice only makes clients redraw what they
// already have, and suppressing a redelivery would risk dropping the one event a room
// needed to leave a match it has already lost.
func RoundMessage(payload RoundPayload) (queue.Message, error) {
	if err := payload.validate(); err != nil {
		return queue.Message{}, fmt.Errorf("gameroom: round message: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return queue.Message{}, fmt.Errorf("gameroom: encode round payload: %w", err)
	}
	return queue.Message{Queue: Queue, Type: TypeRoundChanged, Payload: body}, nil
}
