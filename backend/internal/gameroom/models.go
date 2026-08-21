// Package gameroom keeps a multiplayer room's betting leaderboard up to date.
//
// It replaces three Laravel jobs. Two of them survive as one message type here,
// and the third disappears entirely:
//
//	UpdateGameBet         -> game_room.bet_settled
//	UpdateGameRoomRank    -> game_room.rank_refresh
//	ReUpdateGameRoomRank  -> deleted
//
// ReUpdateGameRoomRank existed only to re-dispatch UpdateGameRoomRank after a five
// second delay, because Laravel's ShouldBeUnique would otherwise drop a refresh
// requested while one was already queued. Around that sat two cache flags
// ("waiting_job:" and "processing_job:") whose job was to remember that a dropped
// refresh still needed to happen.
//
// A version counter replaces all of it: every settled bet increments the room's
// version, and a refresh records the version it tallied. "A refresh is needed"
// becomes version > applied, which is derived state that cannot leak, and the
// re-dispatch is unnecessary because every vote already publishes its own refresh
// message. See RefreshTracker.
package gameroom

import (
	"context"
	"errors"
)

// ErrNotFound means the room serial does not resolve to a room. Rooms are created
// with the game and never deleted, so this is a malformed payload rather than a
// transient condition.
var ErrNotFound = errors.New("gameroom: room not found")

// Queue is the queue these messages travel on, matching the name the Laravel jobs
// already declare through onQueue('game_room').
const Queue = "game_room"

// Message types.
const (
	// TypeBetSettled applies the outcome of one round to everyone who wagered on
	// it. Port of UpdateGameBet.
	TypeBetSettled = "game_room.bet_settled"
	// TypeRankRefresh recomputes the room's leaderboard and broadcasts it. Port of
	// UpdateGameRoomRank, with ReUpdateGameRoomRank folded in.
	TypeRankRefresh = "game_room.rank_refresh"
	// TypeRoundChanged broadcasts the pairing the host now has on screen. Laravel did
	// this with BroadcastGameRoomRefresh, dispatched from the RefreshGameCandidates
	// listener; the Go stack had no equivalent, so a participant learned about a new
	// match only by re-reading the room on a timer.
	TypeRoundChanged = "game_room.round_changed"
)

// Room is the minimum a handler needs: the id for queries, the serial for the
// broadcast channel and the lock key.
type Room struct {
	ID     int64
	Serial string
}

// BetOutcome is the round a vote just decided.
//
// RemainElements is the count *after* the vote, which is what GameElementVoted
// carries. The wagers being settled were placed when one more element was still in
// play, so the repository matches on RemainElements+1. Preserved from
// GameService::updateGameBet, where the +1 is applied inline.
type BetOutcome struct {
	RoomID         int64
	WinnerID       int64
	LoserID        int64
	CurrentRound   int
	OfRound        int
	RemainElements int
}

// SettleResult reports what one settlement changed, for the log.
type SettleResult struct {
	Won int64
	// Lost counts wagers on the losing element.
	Lost int64
	// Discarded counts wagers left unresolved after this round and removed, which
	// is what the PHP delete of every won_at/lost_at NULL row does.
	Discarded int64
}

// Repository is the database surface. Every method is scoped to one room so a
// mistake cannot spill into another room's leaderboard.
type Repository interface {
	// RoomBySerial resolves the serial carried in the message payload.
	RoomBySerial(ctx context.Context, serial string) (Room, error)
	// SettleBets marks the winning and losing wagers for one round and removes the
	// wagers that round left unresolved.
	SettleBets(ctx context.Context, outcome BetOutcome) (SettleResult, error)
	// RecomputeTotals rebuilds score, combo, accuracy and the played counters for
	// every player in the room.
	RecomputeTotals(ctx context.Context, roomID int64) (int64, error)
	// AssignRanks numbers the room 1..N by score.
	AssignRanks(ctx context.Context, roomID int64) (int64, error)
	// Leaderboard reads the payload the room subscribes to.
	Leaderboard(ctx context.Context, roomID int64) (Leaderboard, error)
	// Standings and BetsByPlayer exist for the parity test, which checks the two
	// set-based statements against the pure functions in this package.
	Standings(ctx context.Context, roomID int64) ([]Standing, error)
	BetsByPlayer(ctx context.Context, roomID int64) (map[int64][]Bet, error)
}

// Outstanding is the room's refresh state.
type Outstanding struct {
	// Version counts settled bets. Monotonic for the life of the room.
	Version int64
	// Applied is the highest version a completed refresh has tallied.
	Applied int64
}

// Pending reports whether a refresh still has work to do.
func (outstanding Outstanding) Pending() bool {
	return outstanding.Version > outstanding.Applied
}

// RefreshTracker coalesces refreshes.
//
// The problem it solves: a busy room votes far faster than the leaderboard can be
// recomputed, and recomputing once per vote is pure waste when the votes arrive in
// bursts. Laravel coalesced with ShouldBeUnique plus two cache flags plus a
// delayed re-dispatch job. This does it with a counter, which has two advantages
// worth the change:
//
// Nothing leaks. "Updating" is version > applied, computed from state the work
// itself maintains, so there is no flag left set by a path that forgot to clear it.
//
// No timer. Every settled bet publishes its own refresh message, so the work is
// already queued; a refresh that finds version == applied simply acknowledges. The
// five second delay existed to give a dropped dispatch another chance, and nothing
// is dropped here.
type RefreshTracker interface {
	// MarkChanged records that a bet settled and returns the new version.
	MarkChanged(ctx context.Context, roomSerial string) (int64, error)
	// Outstanding reads the room's refresh state.
	Outstanding(ctx context.Context, roomSerial string) (Outstanding, error)
	// MarkApplied records that a refresh tallied everything up to version. It must
	// never move Applied backwards: two refreshes can overlap, and the later one
	// finishing first would otherwise resurrect work that was already done.
	MarkApplied(ctx context.Context, roomSerial string, version int64) error
}

// LegacyCache invalidates the Laravel cache entries the PHP API still reads.
//
// Needed only while GameController serves the room endpoints. Both operations are
// deletes, deliberately: Laravel stores cache values as PHP serialize() output, and
// writing that format from Go would couple this package to PHP's serialisation for
// no benefit. Deleting is format-agnostic and leaves Laravel to rebuild from the
// database, which is also fresher than the 60 second cache it would have served.
type LegacyCache interface {
	// InvalidateLeaderboard drops the cached leaderboard so GameController::roomRank
	// re-reads it.
	InvalidateLeaderboard(ctx context.Context, roomSerial string) error
	// ClearUpdatingFlag clears the flag GameController reports as rank_updating.
	// Laravel sets it on every vote and its own job clears it, so leaving it alone
	// would make the endpoint claim an update is in flight for the flag's whole
	// 24 minute lifetime.
	ClearUpdatingFlag(ctx context.Context, roomSerial string) error
}

// Broadcaster publishes an event to a channel. Satisfied by realtime.PusherPublisher.
type Broadcaster interface {
	Publish(ctx context.Context, channel, event string, payload any) error
}

// BroadcastEvent is the name the frontend listens for, from
// BroadcastGameBetRank::broadcastAs.
const BroadcastEvent = "GameBetRank"

// RoundEvent is the name the frontend listens for when the pairing changes.
//
// NOT the Laravel name. BroadcastGameRoomRefresh called itself GameRoomRefresh and sent a
// whole GameRoundResource — elements, images and all. This one sends the same tally the
// room's REST endpoint returns, because the client already knows the elements and the two
// paths agreeing by construction is worth more than the name matching.
const RoundEvent = "GameRoomRound"
