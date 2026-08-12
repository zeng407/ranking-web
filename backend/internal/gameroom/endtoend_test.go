package gameroom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/realtime"
	"2pick.app/backend/internal/realtime/pushertest"
)

// The end-to-end check: a settled round goes through MySQL, Redis and Soketi, and a
// subscribed client receives the leaderboard.
//
// Every other test in this package stubs at least one edge. This one stubs nothing,
// which is what makes it able to catch the failures that only appear at the seams:
// a channel name that does not match what the browser subscribes to, an event name
// that does not match broadcastAs, or a payload whose field names or types the
// frontend cannot read.
//
// It needs the whole local stack:
//
//	MYSQL_TEST_HOST, MYSQL_TEST_PASSWORD, REDIS_TEST_ADDR,
//	SOKETI_WS, SOKETI_HTTP, SOKETI_APP_ID, SOKETI_APP_KEY, SOKETI_APP_SECRET
func TestSettledRoundReachesASubscribedClient(t *testing.T) {
	wsURL := os.Getenv("SOKETI_WS")
	httpHost := os.Getenv("SOKETI_HTTP")
	appID := os.Getenv("SOKETI_APP_ID")
	appKey := os.Getenv("SOKETI_APP_KEY")
	appSecret := os.Getenv("SOKETI_APP_SECRET")
	if wsURL == "" || httpHost == "" || appID == "" || appKey == "" || appSecret == "" {
		t.Skip("SOKETI_WS, SOKETI_HTTP and the SOKETI_APP_* variables are not set; skipping the end-to-end test")
	}

	database := testDatabase(t)
	redisClient := testRedis(t)
	ctx := context.Background()

	host, port := splitHostPort(t, httpHost)
	broadcaster, err := realtime.NewPusherPublisher(realtime.Config{
		AppID: appID, Key: appKey, Secret: appSecret, Host: host, Port: port,
	})
	if err != nil {
		t.Fatalf("NewPusherPublisher() error = %v", err)
	}

	tracker, err := NewRedisTracker(redisClient)
	if err != nil {
		t.Fatalf("NewRedisTracker() error = %v", err)
	}
	transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	legacyCache, err := NewRedisLegacyCache(redisClient, "e2e-test-cache:")
	if err != nil {
		t.Fatalf("NewRedisLegacyCache() error = %v", err)
	}

	repository := NewMySQLRepository(database, DefaultScoring())
	service, err := NewService(Options{
		Repository:  repository,
		Tracker:     tracker,
		Legacy:      legacyCache,
		Broadcaster: broadcaster,
		Publisher:   publisher,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	room := newSyntheticRoom(t, database)
	t.Cleanup(func() {
		redisClient.Del(context.Background(), TrackerKeyPrefix+room.serial)
	})

	const (
		currentRound = 3
		ofRound      = 4
		placedAt     = 8
		afterVote    = 7
		winnerCombo  = 2
		loserCombo   = 5
	)
	room.placeBet(t, database, "winner", room.winnerID, room.loserID,
		currentRound, ofRound, placedAt, winnerCombo)
	room.placeBet(t, database, "loser", room.loserID, room.winnerID,
		currentRound, ofRound, placedAt, loserCombo)

	// A leftover message from a failed earlier run would be reserved below instead of
	// this run's, so the queue starts empty.
	drainQueue(t, transport)

	// Subscribed and confirmed before anything is published, so a missing event is a
	// real failure rather than a race.
	channel := realtime.GameRoomChannel(room.serial)
	client := pushertest.Dial(t, wsURL, appKey)
	client.Subscribe(t, channel)

	settleMessage, err := SettleMessage(SettlePayload{
		RoomSerial:     room.serial,
		WinnerID:       room.winnerID,
		LoserID:        room.loserID,
		CurrentRound:   currentRound,
		OfRound:        ofRound,
		RemainElements: afterVote,
	})
	if err != nil {
		t.Fatalf("SettleMessage() error = %v", err)
	}
	if err := service.handleSettle(ctx, settleMessage); err != nil {
		t.Fatalf("handleSettle() error = %v", err)
	}

	// handleSettle publishes the refresh onto the real queue. Take it from there
	// rather than building one, so the message the worker would consume is the
	// message under test.
	reservation, err := transport.Reserve(ctx, []string{Queue}, 5*time.Second)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if reservation == nil {
		t.Fatal("handleSettle published no refresh message")
	}
	if reservation.Message.Type != TypeRankRefresh {
		t.Fatalf("reserved a %q message, want %q", reservation.Message.Type, TypeRankRefresh)
	}
	if err := service.handleRefresh(ctx, reservation.Message); err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if err := reservation.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}

	frame, ok := client.AwaitEvent(t, channel, BroadcastEvent, 10*time.Second)
	if !ok {
		t.Fatal("the subscribed client never received the leaderboard")
	}

	// Decoded from the wire, not from the struct that produced it: this is what the
	// browser actually parses.
	var received Leaderboard
	if err := json.Unmarshal([]byte(frame.Data), &received); err != nil {
		t.Fatalf("decode broadcast payload %q: %v", frame.Data, err)
	}
	if received.TotalUsers != 3 {
		t.Errorf("total_users = %d, want 3", received.TotalUsers)
	}
	if len(received.Top10) != 3 {
		t.Fatalf("top_10 has %d entries, want 3", len(received.Top10))
	}
	if received.Top10[0].Name != "winner" {
		t.Errorf("top_10[0] = %q, want winner", received.Top10[0].Name)
	}
	// 1000 + (2 * 10 + 10).
	if received.Top10[0].Score != 1030 {
		t.Errorf("winner score = %d, want 1030", received.Top10[0].Score)
	}
	if received.Top10[0].Combo != winnerCombo+1 {
		t.Errorf("winner combo = %d, want %d", received.Top10[0].Combo, winnerCombo+1)
	}

	// The field the legacy UI interpolates directly. A number here would render
	// "100%" where the PHP rendered "100.00%".
	assertAccuracyIsATwoDecimalString(t, frame.Data)

	// Nothing outstanding once the refresh has run, which is what the PHP endpoint
	// reports as rank_updating being false.
	outstanding, err := tracker.Outstanding(ctx, room.serial)
	if err != nil {
		t.Fatalf("Outstanding() error = %v", err)
	}
	if outstanding.Pending() {
		t.Errorf("work is still outstanding after the refresh: %+v", outstanding)
	}
}

// assertAccuracyIsATwoDecimalString inspects the raw JSON, because unmarshalling
// into Leaderboard would hide whether the wire form was a string or a number.
func assertAccuracyIsATwoDecimalString(t *testing.T, payload string) {
	t.Helper()

	var raw struct {
		Top10 []map[string]json.RawMessage `json:"top_10"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(raw.Top10) == 0 {
		t.Fatal("top_10 is empty")
	}

	accuracy, ok := raw.Top10[0]["accuracy"]
	if !ok {
		t.Fatal("top_10[0] has no accuracy field")
	}
	var text string
	if err := json.Unmarshal(accuracy, &text); err != nil {
		t.Fatalf("accuracy is not a JSON string but %s; Game.vue interpolates it directly", accuracy)
	}
	if text != "100.00" {
		t.Errorf("accuracy = %q, want \"100.00\"", text)
	}
}

func splitHostPort(t *testing.T, value string) (string, string) {
	t.Helper()
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] == ':' {
			return value[:index], value[index+1:]
		}
	}
	t.Fatalf("SOKETI_HTTP must be host:port, got %q", value)
	return "", ""
}

// A leftover queue entry from a failed run would be consumed by the next one and
// confuse it, so the queue is drained before the end-to-end test publishes.
func drainQueue(t *testing.T, transport *queue.RedisTransport) {
	t.Helper()
	ctx := context.Background()
	for index := 0; index < 100; index++ {
		// One second is the floor: BRPOPLPUSH takes whole seconds, and go-redis
		// truncates anything smaller while logging a warning.
		reservation, err := transport.Reserve(ctx, []string{Queue}, time.Second)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if reservation == nil {
			return
		}
		if err := reservation.Ack(ctx); err != nil {
			t.Fatalf("drain ack: %v", err)
		}
	}
	t.Fatal(fmt.Sprintf("the %s queue did not drain", Queue))
}
