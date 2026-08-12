package queue

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const testPrefix = "2pick:test:consume:"

func newConsumeFixture(t *testing.T) (*RedisTransport, *redis.Client, context.Context) {
	t.Helper()
	client := testRedis(t)
	transport, err := NewRedisTransport(client, testPrefix)
	if err != nil {
		t.Fatalf("NewRedisTransport() error = %v", err)
	}
	ctx := context.Background()

	cleanup := func() {
		keys, _ := client.Keys(context.Background(), testPrefix+"*").Result()
		if len(keys) > 0 {
			client.Del(context.Background(), keys...)
		}
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		client.Close()
	})
	return transport, client, ctx
}

func publishOne(t *testing.T, transport *RedisTransport, message Message) {
	t.Helper()
	publisher, err := NewPublisher(transport)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	if err := publisher.Publish(context.Background(), message); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestRetryDelayGrowsAndIsCapped(t *testing.T) {
	if got := RetryDelay(1); got != BaseRetryDelay {
		t.Fatalf("RetryDelay(1) = %s, want %s", got, BaseRetryDelay)
	}
	if got := RetryDelay(2); got != 2*BaseRetryDelay {
		t.Fatalf("RetryDelay(2) = %s, want %s", got, 2*BaseRetryDelay)
	}
	if got := RetryDelay(3); got != 4*BaseRetryDelay {
		t.Fatalf("RetryDelay(3) = %s", got)
	}
	// Must not overflow into a negative or absurd delay at high attempt counts.
	for _, attempt := range []int{20, 64, 1000} {
		if got := RetryDelay(attempt); got != MaxRetryDelay {
			t.Errorf("RetryDelay(%d) = %s, want %s", attempt, got, MaxRetryDelay)
		}
	}
	if got := RetryDelay(0); got != BaseRetryDelay {
		t.Fatalf("RetryDelay(0) = %s, want %s", got, BaseRetryDelay)
	}
}

func TestReserveRequiresAQueue(t *testing.T) {
	transport, _, ctx := newConsumeFixture(t)
	if _, err := transport.Reserve(ctx, nil, time.Millisecond); err == nil {
		t.Fatal("Reserve() should reject an empty queue list")
	}
}

func TestReserveReturnsNilWhenIdle(t *testing.T) {
	transport, _, ctx := newConsumeFixture(t)

	reservation, err := transport.Reserve(ctx, []string{"default"}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if reservation != nil {
		t.Fatalf("Reserve() = %#v, want nil when idle", reservation)
	}
}

// Reserving must move the message to the processing list rather than deleting
// it, so a crash before the handler finishes cannot lose the work.
func TestReserveHoldsMessageInProcessingUntilAcked(t *testing.T) {
	transport, client, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "default", Type: "rank.update_element"})

	reservation, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}

	if got := client.LLen(ctx, transport.Key("default")).Val(); got != 0 {
		t.Fatalf("queue length = %d, want 0 after reserve", got)
	}
	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 1 {
		t.Fatalf("processing length = %d, want 1 while in flight", got)
	}

	if err := reservation.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0 after ack", got)
	}
}

// Queue order in the config is consumption priority.
func TestReserveRespectsQueuePriority(t *testing.T) {
	transport, _, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "low", Type: "warm"})
	publishOne(t, transport, Message{Queue: "high", Type: "urgent"})

	reservation, err := transport.Reserve(ctx, []string{"high", "default", "low"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}
	if reservation.Message.Type != "urgent" {
		t.Fatalf("reserved %q, want the high-priority message", reservation.Message.Type)
	}
	if err := reservation.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

func TestRetryRequeuesWithIncrementedAttempt(t *testing.T) {
	transport, client, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "default", Type: "rank.update_element"})

	reservation, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}
	if reservation.Attempt() != 1 {
		t.Fatalf("Attempt() = %d, want 1", reservation.Attempt())
	}
	if err := reservation.Retry(ctx); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	// Nothing may be left in flight after a retry.
	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0 after retry", got)
	}

	again, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || again == nil {
		t.Fatalf("second Reserve() = (%v, %v)", again, err)
	}
	if again.Attempt() != 2 {
		t.Fatalf("Attempt() = %d, want 2 on redelivery", again.Attempt())
	}
	if err := again.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

func TestDeadLetterMovesMessageOffTheQueue(t *testing.T) {
	transport, client, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "default", Type: "rank.update_element"})

	reservation, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}
	if err := reservation.DeadLetter(ctx); err != nil {
		t.Fatalf("DeadLetter() error = %v", err)
	}

	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0", got)
	}
	if got := client.LLen(ctx, transport.Key("default")).Val(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
	length, err := transport.DeadLetterLength(ctx, "default")
	if err != nil {
		t.Fatalf("DeadLetterLength() error = %v", err)
	}
	if length != 1 {
		t.Fatalf("dead-letter length = %d, want 1", length)
	}
}

// An entry that cannot be decoded can never succeed, so it must not sit on the
// processing list blocking recovery forever.
func TestReserveDeadLettersAnUndecodableEntry(t *testing.T) {
	transport, client, ctx := newConsumeFixture(t)
	client.LPush(ctx, transport.Key("default"), []byte(`{"queue":`))

	if _, err := transport.Reserve(ctx, []string{"default"}, time.Second); err == nil {
		t.Fatal("Reserve() should report the decode failure")
	}
	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0", got)
	}
	length, _ := transport.DeadLetterLength(ctx, "default")
	if length != 1 {
		t.Fatalf("dead-letter length = %d, want 1", length)
	}
}

// The other half of crash safety: without recovery, a message stranded in flight
// by a killed worker would never be picked up again.
func TestRecoverProcessingReturnsStrandedMessages(t *testing.T) {
	transport, client, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "default", Type: "rank.update_element"})

	reservation, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}
	// Simulate the worker being killed: neither Ack, Retry nor DeadLetter runs.

	recovered, err := transport.RecoverProcessing(ctx, []string{"default"})
	if err != nil {
		t.Fatalf("RecoverProcessing() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}
	if got := client.LLen(ctx, transport.ProcessingKey("default")).Val(); got != 0 {
		t.Fatalf("processing length = %d, want 0 after recovery", got)
	}

	again, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || again == nil {
		t.Fatalf("Reserve() after recovery = (%v, %v)", again, err)
	}
	if err := again.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

// Recovery must not reset the attempt count, or a message that keeps killing the
// worker would be retried forever instead of reaching the dead-letter queue.
func TestRecoverProcessingPreservesAttemptCount(t *testing.T) {
	transport, _, ctx := newConsumeFixture(t)
	publishOne(t, transport, Message{Queue: "default", Type: "poison", Attempt: 3})

	reservation, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || reservation == nil {
		t.Fatalf("Reserve() = (%v, %v)", reservation, err)
	}
	if _, err := transport.RecoverProcessing(ctx, []string{"default"}); err != nil {
		t.Fatalf("RecoverProcessing() error = %v", err)
	}

	again, err := transport.Reserve(ctx, []string{"default"}, time.Second)
	if err != nil || again == nil {
		t.Fatalf("Reserve() = (%v, %v)", again, err)
	}
	if again.Attempt() != 3 {
		t.Fatalf("Attempt() = %d, want 3 preserved through recovery", again.Attempt())
	}
	if err := again.Ack(ctx); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

func TestRecoverProcessingIsIdleWhenNothingStranded(t *testing.T) {
	transport, _, ctx := newConsumeFixture(t)

	recovered, err := transport.RecoverProcessing(ctx, []string{"default", "high", "low"})
	if err != nil {
		t.Fatalf("RecoverProcessing() error = %v", err)
	}
	if recovered != 0 {
		t.Fatalf("recovered = %d, want 0", recovered)
	}
}
