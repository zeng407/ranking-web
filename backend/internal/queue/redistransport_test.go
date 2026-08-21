package queue

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisTransportKeyIsNamespacedAwayFromLaravel(t *testing.T) {
	transport, err := NewRedisTransport(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), "")
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}

	key := transport.Key("default")
	if key != "2pick:go:queue:default" {
		t.Fatalf("Key() = %q", key)
	}
	// Laravel's own list for the same logical queue is "queues:default"; sharing
	// one Redis during the migration means these must never coincide.
	if key == "queues:default" {
		t.Fatal("Go queue key collides with the Laravel queue key")
	}
}

func TestNewRedisTransportRequiresClient(t *testing.T) {
	if _, err := NewRedisTransport(nil, ""); err == nil {
		t.Fatal("NewRedisTransport(nil) should fail")
	}
}

func TestDecodeRoundTripsMessage(t *testing.T) {
	original := Message{
		Queue:          "game_room",
		Type:           "game_room.update_rank",
		Payload:        json.RawMessage(`{"game_room_id":7}`),
		IdempotencyKey: "room-7-rank-v1",
		EnqueuedAt:     time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
		Attempt:        2,
	}

	body, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	decoded, err := Decode(body)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if decoded.Queue != original.Queue || decoded.Type != original.Type {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.IdempotencyKey != original.IdempotencyKey || decoded.Attempt != 2 {
		t.Fatalf("decoded = %#v", decoded)
	}
	if !decoded.EnqueuedAt.Equal(original.EnqueuedAt) {
		t.Fatalf("EnqueuedAt = %s, want %s", decoded.EnqueuedAt, original.EnqueuedAt)
	}
	if string(decoded.Payload) != `{"game_room_id":7}` {
		t.Fatalf("Payload = %s", decoded.Payload)
	}
}

// A corrupt or foreign entry must not become a message with an empty queue that
// a worker then tries to process.
func TestDecodeRejectsUnusableEntries(t *testing.T) {
	if _, err := Decode([]byte(`not json`)); err == nil {
		t.Fatal("Decode() should reject malformed JSON")
	}
	if _, err := Decode([]byte(`{"type":"rank.update_element"}`)); err == nil {
		t.Fatal("Decode() should reject a message with no queue")
	}
	if _, err := Decode([]byte(`{"queue":"default"}`)); err == nil {
		t.Fatal("Decode() should reject a message with no type")
	}
}

// testRedis returns a client only when an address is supplied. The release image
// runs `go test ./...` during the build with no Redis available, so this must
// skip rather than fail there.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set; skipping Redis integration test")
	}

	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis at %s is unreachable: %v", addr, err)
	}
	return client
}

/**
 * The blocking reserve has to honour a sub-second window.
 *
 * Only the last queue in the priority order blocks; the rest are drained by one
 * non-blocking pop per loop, so this window is the delay a message on any of
 * them pays. The client's typed BRPopLPush helper floors anything under a second
 * to a second, which is why Reserve issues the command itself — and why this
 * test exists, because that floor is silent.
 */
func TestReserveHonoursASubSecondBlock(t *testing.T) {
	client := testRedis(t)
	defer client.Close()

	transport, err := NewRedisTransport(client, "2pick:test:queue:")
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}
	ctx := context.Background()
	queues := []string{"high", "low"}
	keys := []string{transport.Key("high"), transport.Key("low"),
		transport.Key("high") + ":processing", transport.Key("low") + ":processing"}
	client.Del(ctx, keys...)
	t.Cleanup(func() { client.Del(context.Background(), keys...) })

	start := time.Now()
	reservation, err := transport.Reserve(ctx, queues, 250*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if reservation != nil {
		t.Fatalf("Reserve() returned %v from empty queues", reservation.Message)
	}
	// Generous, because a loaded CI box is slow — but nowhere near the second the
	// typed helper would have imposed.
	if elapsed > 700*time.Millisecond {
		t.Errorf("an idle reserve blocked for %s, want about 250ms", elapsed)
	}
}

func TestRedisTransportPublishesFIFOPerQueue(t *testing.T) {
	client := testRedis(t)
	defer client.Close()

	transport, err := NewRedisTransport(client, "2pick:test:queue:")
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}
	ctx := context.Background()
	keys := []string{transport.Key("default"), transport.Key("high")}
	client.Del(ctx, keys...)
	t.Cleanup(func() { client.Del(context.Background(), keys...) })

	publisher, err := NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	err = publisher.Publish(ctx,
		Message{Queue: "default", Type: "first"},
		Message{Queue: "high", Type: "urgent"},
		Message{Queue: "default", Type: "second"},
	)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if got := client.LLen(ctx, transport.Key("default")).Val(); got != 2 {
		t.Fatalf("default queue length = %d, want 2", got)
	}
	if got := client.LLen(ctx, transport.Key("high")).Val(); got != 1 {
		t.Fatalf("high queue length = %d, want 1", got)
	}

	// BRPOP takes from the tail, so the first published message must come out
	// first.
	for _, want := range []string{"first", "second"} {
		body, err := client.RPop(ctx, transport.Key("default")).Bytes()
		if err != nil {
			t.Fatalf("RPop() error = %v", err)
		}
		message, err := Decode(body)
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if message.Type != want {
			t.Fatalf("popped %q, want %q", message.Type, want)
		}
		if message.Attempt != 1 || message.EnqueuedAt.IsZero() {
			t.Fatalf("message = %#v", message)
		}
	}
}
