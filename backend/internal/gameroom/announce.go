package gameroom

import (
	"context"
	"fmt"
	"log/slog"

	"2pick.app/backend/internal/queue"
)

// Announcer tells the room about matches the host's game just decided.
//
// THIS IS THE MISSING PRODUCER. The worker has consumed game_room.bet_settled since D6,
// but nothing published it: the Go vote path recorded rounds and stopped there, so a host
// playing through Go settled nobody's wagers and every room's leaderboard sat still. The
// Laravel equivalent is the NotifyVoted listener, which dispatched UpdateGameBet per
// decided round.
type Announcer struct {
	rooms     RoomLookup
	publisher *queue.Publisher
	logger    *slog.Logger
}

// RoomLookup is the slice of the participation repository this needs.
type RoomLookup interface {
	RoomByGameSerial(ctx context.Context, gameSerial string) (Room, bool, error)
}

func NewAnnouncer(rooms RoomLookup, publisher *queue.Publisher, logger *slog.Logger) (*Announcer, error) {
	if rooms == nil {
		return nil, fmt.Errorf("gameroom: room lookup is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("gameroom: publisher is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Announcer{rooms: rooms, publisher: publisher, logger: logger}, nil
}

// DecidedRound is one match a host's vote resolved.
//
// Deliberately its own type rather than gameplay.SettledRound: this package must not
// depend on gameplay, and the caller translating between them is one line at the seam
// instead of a dependency in both directions.
type DecidedRound struct {
	WinnerID       int64
	LoserID        int64
	CurrentRound   int
	OfRound        int
	RemainElements int
}

// AnnounceRounds publishes one settle message per decided match, and one refresh.
//
// Returns the number of settle messages published, so a caller can log the difference
// between "no room" and "nothing decided".
//
// A GAME WITH NO ROOM COSTS ONE QUERY AND NOTHING ELSE. Most games are solo, and this
// runs on every vote batch, so the lookup comes first and the common case exits there.
func (announcer *Announcer) AnnounceRounds(
	ctx context.Context, gameSerial string, rounds []DecidedRound,
) (int, error) {
	if len(rounds) == 0 {
		return 0, nil
	}

	room, hosting, err := announcer.rooms.RoomByGameSerial(ctx, gameSerial)
	if err != nil {
		return 0, err
	}
	if !hosting {
		return 0, nil
	}

	messages := make([]queue.Message, 0, len(rounds)+1)
	for _, round := range rounds {
		payload := SettlePayload{
			RoomSerial:     room.Serial,
			WinnerID:       round.WinnerID,
			LoserID:        round.LoserID,
			CurrentRound:   round.CurrentRound,
			OfRound:        round.OfRound,
			RemainElements: round.RemainElements,
		}
		message, err := SettleMessage(payload)
		if err != nil {
			return 0, err
		}
		messages = append(messages, message)
	}

	// One refresh for the whole batch rather than one per round. Each settle already
	// bumps the room's version, and the refresh handler tallies whatever the version has
	// reached — so N refreshes would recompute the same standings N times and broadcast
	// the same payload N times.
	refresh, err := RefreshMessage(room.Serial)
	if err != nil {
		return 0, err
	}
	messages = append(messages, refresh)

	if err := announcer.publisher.Publish(ctx, messages...); err != nil {
		return 0, err
	}
	return len(rounds), nil
}
