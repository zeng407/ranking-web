package gameroom

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"2pick.app/backend/internal/queue"
)

// recordingTransport captures what the announcer published without a Redis.
type recordingTransport struct {
	published []queue.Message
	err       error
}

func (transport *recordingTransport) Publish(_ context.Context, messages []queue.Message) error {
	if transport.err != nil {
		return transport.err
	}
	transport.published = append(transport.published, messages...)
	return nil
}

type fakeRoomLookup struct {
	room    Room
	hosting bool
	err     error
	calls   int
}

func (lookup *fakeRoomLookup) RoomByGameSerial(_ context.Context, _ string) (Room, bool, error) {
	lookup.calls++
	return lookup.room, lookup.hosting, lookup.err
}

func newTestAnnouncer(t *testing.T, lookup *fakeRoomLookup) (*Announcer, *recordingTransport) {
	t.Helper()
	transport := &recordingTransport{}
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	announcer, err := NewAnnouncer(lookup, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAnnouncer() error = %v", err)
	}
	return announcer, transport
}

func decidedRounds() []DecidedRound {
	return []DecidedRound{
		{WinnerID: 11, LoserID: 22, CurrentRound: 31, OfRound: 32, RemainElements: 34},
		{WinnerID: 33, LoserID: 44, CurrentRound: 32, OfRound: 32, RemainElements: 33},
	}
}

/**
 * The whole point of this type. Until it existed the vote path recorded rounds and stopped,
 * so a host playing through Go settled nobody's wagers — the worker sat waiting for a
 * message nothing published.
 */
func TestAnnounceRoundsPublishesSettlesARefreshAndTheNewPairing(t *testing.T) {
	lookup := &fakeRoomLookup{room: Room{ID: 5, Serial: "abcdefgh"}, hosting: true}
	announcer, transport := newTestAnnouncer(t, lookup)

	published, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds())
	if err != nil {
		t.Fatalf("AnnounceRounds() error = %v", err)
	}
	if published != 2 {
		t.Errorf("published = %d, want 2", published)
	}

	if len(transport.published) != 4 {
		t.Fatalf("%d messages published, want 2 settles, 1 refresh and 1 round", len(transport.published))
	}

	settles := 0
	refreshes := 0
	rounds := 0
	for _, message := range transport.published {
		switch message.Type {
		case TypeBetSettled:
			settles++
		case TypeRankRefresh:
			refreshes++
		case TypeRoundChanged:
			rounds++
		default:
			t.Errorf("unexpected message type %q", message.Type)
		}
		if message.Queue != Queue {
			t.Errorf("message on queue %q, want %q", message.Queue, Queue)
		}
		// A settle is keyed per round, so a redelivery is traceable to the round it
		// belongs to. The refresh deliberately is NOT: it derives the standings from the
		// version counter, so there is nothing a key could suppress and a redelivery is
		// simply a second recompute of the same numbers.
		if message.Type == TypeBetSettled && message.IdempotencyKey == "" {
			t.Errorf("%s has no idempotency key", message.Type)
		}
	}
	if settles != 2 {
		t.Errorf("%d settle messages, want one per decided round", settles)
	}
	// ONE refresh for the batch, not one per round: each settle bumps the version and the
	// refresh tallies whatever it has reached, so N refreshes recompute and broadcast the
	// same standings N times.
	if refreshes != 1 {
		t.Errorf("%d refresh messages, want exactly 1 for the batch", refreshes)
	}
	// And one round message, naming the pairing the host moved on to. Without it a seated
	// participant keeps voting on the match that was just decided until its poll comes round.
	if rounds != 1 {
		t.Errorf("%d round messages, want exactly 1 for the batch", rounds)
	}
}

// Each settle carries a key naming its own round, so two rounds in one batch cannot be
// mistaken for a redelivery of each other.
func TestEachSettleIsKeyedToItsOwnRound(t *testing.T) {
	lookup := &fakeRoomLookup{room: Room{Serial: "abcdefgh"}, hosting: true}
	announcer, transport := newTestAnnouncer(t, lookup)

	if _, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds()); err != nil {
		t.Fatalf("AnnounceRounds() error = %v", err)
	}

	keys := map[string]bool{}
	for _, message := range transport.published {
		if message.Type != TypeBetSettled {
			continue
		}
		if keys[message.IdempotencyKey] {
			t.Errorf("two rounds share the key %q", message.IdempotencyKey)
		}
		keys[message.IdempotencyKey] = true
	}
	if len(keys) != 2 {
		t.Errorf("%d distinct keys, want one per round", len(keys))
	}
}

func TestAnnounceRoundsCarriesTheRoundNumbersThroughUnchanged(t *testing.T) {
	lookup := &fakeRoomLookup{room: Room{ID: 5, Serial: "abcdefgh"}, hosting: true}
	announcer, transport := newTestAnnouncer(t, lookup)

	if _, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds()); err != nil {
		t.Fatalf("AnnounceRounds() error = %v", err)
	}

	var first SettlePayload
	for _, message := range transport.published {
		if message.Type != TypeBetSettled {
			continue
		}
		if err := json.Unmarshal(message.Payload, &first); err != nil {
			t.Fatalf("decode settle payload: %v", err)
		}
		break
	}

	// The settlement matches wagers on remain_elements, so a wrong number here settles the
	// wrong round or nothing at all.
	want := decidedRounds()[0]
	if first.RoomSerial != "abcdefgh" || first.WinnerID != want.WinnerID || first.LoserID != want.LoserID ||
		first.CurrentRound != want.CurrentRound || first.OfRound != want.OfRound ||
		first.RemainElements != want.RemainElements {
		t.Errorf("payload = %+v, want it to match the recorded round %+v", first, want)
	}
}

// The common case: most games are solo. One lookup, no messages, no error.
func TestAnnounceRoundsIsSilentForAGameWithNoRoom(t *testing.T) {
	lookup := &fakeRoomLookup{hosting: false}
	announcer, transport := newTestAnnouncer(t, lookup)

	published, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds())
	if err != nil {
		t.Fatalf("AnnounceRounds() error = %v", err)
	}
	if published != 0 {
		t.Errorf("published = %d, want 0", published)
	}
	if len(transport.published) != 0 {
		t.Errorf("%d messages published for a game with no room", len(transport.published))
	}
	if lookup.calls != 1 {
		t.Errorf("the room was looked up %d times, want 1", lookup.calls)
	}
}

// No decided rounds means no lookup either: this runs on every vote batch, and a batch that
// changed nothing must not cost a query.
func TestAnnounceRoundsSkipsTheLookupWithNothingDecided(t *testing.T) {
	lookup := &fakeRoomLookup{hosting: true, room: Room{Serial: "abcdefgh"}}
	announcer, transport := newTestAnnouncer(t, lookup)

	published, err := announcer.AnnounceRounds(context.Background(), "game-serial", nil)
	if err != nil {
		t.Fatalf("AnnounceRounds() error = %v", err)
	}
	if published != 0 || lookup.calls != 0 || len(transport.published) != 0 {
		t.Errorf("published = %d, lookups = %d, messages = %d; want all zero",
			published, lookup.calls, len(transport.published))
	}
}

func TestAnnounceRoundsSurfacesALookupFailure(t *testing.T) {
	failure := errors.New("the database is down")
	announcer, transport := newTestAnnouncer(t, &fakeRoomLookup{err: failure})

	if _, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds()); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the lookup failure", err)
	}
	if len(transport.published) != 0 {
		t.Error("messages were published after the lookup failed")
	}
}

func TestAnnounceRoundsSurfacesAPublishFailure(t *testing.T) {
	lookup := &fakeRoomLookup{room: Room{Serial: "abcdefgh"}, hosting: true}
	transport := &recordingTransport{err: errors.New("redis is down")}
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	announcer, err := NewAnnouncer(lookup, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAnnouncer() error = %v", err)
	}

	if _, err := announcer.AnnounceRounds(context.Background(), "game-serial", decidedRounds()); err == nil {
		t.Fatal("AnnounceRounds() succeeded with a failing transport")
	}
}

/**
 * The restart path. Nothing was decided — the host simply reshuffled and is now on a new
 * game — so there is no settle to piggyback on and the pairing needs its own message.
 */
func TestAnnounceRoomPublishesJustThePairing(t *testing.T) {
	lookup := &fakeRoomLookup{room: Room{ID: 5, Serial: "abcdefgh"}, hosting: true}
	announcer, transport := newTestAnnouncer(t, lookup)

	published, err := announcer.AnnounceRoom(context.Background(), "game-serial")
	if err != nil {
		t.Fatalf("AnnounceRoom() error = %v", err)
	}
	if !published {
		t.Error("published = false, want the room's pairing announced")
	}
	if len(transport.published) != 1 {
		t.Fatalf("%d messages published, want exactly the round message", len(transport.published))
	}
	message := transport.published[0]
	if message.Type != TypeRoundChanged || message.Queue != Queue {
		t.Errorf("published %q on %q, want %q on %q", message.Type, message.Queue, TypeRoundChanged, Queue)
	}

	var payload RoundPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatalf("decode round payload: %v", err)
	}
	if payload.RoomSerial != "abcdefgh" || payload.GameSerial != "game-serial" {
		t.Errorf("payload = %+v, want the room and the game it now follows", payload)
	}
}

// Solo games are the common case, and a host who never opened a room must not cost a message.
func TestAnnounceRoomIsSilentForAGameWithNoRoom(t *testing.T) {
	lookup := &fakeRoomLookup{hosting: false}
	announcer, transport := newTestAnnouncer(t, lookup)

	published, err := announcer.AnnounceRoom(context.Background(), "game-serial")
	if err != nil {
		t.Fatalf("AnnounceRoom() error = %v", err)
	}
	if published || len(transport.published) != 0 {
		t.Errorf("published = %v with %d messages, want neither", published, len(transport.published))
	}
}

func TestAnnounceRoomSurfacesALookupFailure(t *testing.T) {
	failure := errors.New("the database is down")
	announcer, transport := newTestAnnouncer(t, &fakeRoomLookup{err: failure})

	if _, err := announcer.AnnounceRoom(context.Background(), "game-serial"); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the lookup failure", err)
	}
	if len(transport.published) != 0 {
		t.Error("messages were published after the lookup failed")
	}
}

func TestNewAnnouncerRejectsMissingDependencies(t *testing.T) {
	publisher, err := queue.NewPublisher(&recordingTransport{})
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	if _, err := NewAnnouncer(nil, publisher, nil); err == nil {
		t.Error("NewAnnouncer() accepted a nil lookup")
	}
	if _, err := NewAnnouncer(&fakeRoomLookup{}, nil, nil); err == nil {
		t.Error("NewAnnouncer() accepted a nil publisher")
	}
}
