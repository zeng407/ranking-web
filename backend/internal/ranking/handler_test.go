package ranking

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

func testService(t *testing.T, repository Repository) *Service {
	t.Helper()
	service, err := NewService(Options{
		Repository: repository,
		Stats:      &fakeStats{},
		Location:   taipei(t),
		Now:        func() time.Time { return time.Date(2026, 8, 5, 9, 0, 0, 0, taipei(t)) },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestNewUpdateElementRankMessage(t *testing.T) {
	message, err := NewUpdateElementRankMessage(46, 2759)
	if err != nil {
		t.Fatalf("NewUpdateElementRankMessage() error = %v", err)
	}
	if message.Queue != QueueDefault || message.Type != MessageTypeUpdateElementRank {
		t.Fatalf("message = %#v", message)
	}

	var payload UpdateElementRankPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.PostID != 46 || payload.ElementID != 2759 {
		t.Fatalf("payload = %#v", payload)
	}
	// The natural key, not a timestamp: two dispatches for the same element are
	// the same work.
	if message.IdempotencyKey != "rank.update_element:46:2759" {
		t.Fatalf("IdempotencyKey = %q", message.IdempotencyKey)
	}
}

func TestNewUpdateElementRankMessageRejectsMissingIdentifiers(t *testing.T) {
	if _, err := NewUpdateElementRankMessage(0, 2759); err == nil {
		t.Error("a zero post id must be rejected")
	}
	if _, err := NewUpdateElementRankMessage(46, 0); err == nil {
		t.Error("a zero element id must be rejected")
	}
}

func TestRegistrationIsAcceptedByTheRegistry(t *testing.T) {
	service := testService(t, &fakeRepository{})
	registry := jobs.NewRegistry()

	if err := registry.Register(service.Registration()); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registration, err := registry.Lookup(MessageTypeUpdateElementRank)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if registration.SerialKey == nil {
		t.Fatal("element rank updates must be serialized by key")
	}
	if registration.LaravelJob == "" {
		t.Fatal("the replaced Laravel job must be recorded for cutover traceability")
	}
	// Must stay within the worker ceiling, which mirrors Laravel's redis
	// retry_after of 90s.
	if registration.Timeout > 90*time.Second {
		t.Fatalf("Timeout = %s, must stay under the 90s redelivery window", registration.Timeout)
	}
}

func TestSerialKeyIsPerElement(t *testing.T) {
	registration := testService(t, &fakeRepository{}).Registration()

	first, err := NewUpdateElementRankMessage(46, 2759)
	if err != nil {
		t.Fatalf("NewUpdateElementRankMessage() error = %v", err)
	}
	second, err := NewUpdateElementRankMessage(46, 9999)
	if err != nil {
		t.Fatalf("NewUpdateElementRankMessage() error = %v", err)
	}

	firstKey, err := registration.SerialKey(first)
	if err != nil {
		t.Fatalf("SerialKey() error = %v", err)
	}
	secondKey, err := registration.SerialKey(second)
	if err != nil {
		t.Fatalf("SerialKey() error = %v", err)
	}
	if firstKey == secondKey {
		t.Fatal("different elements must not serialize against each other")
	}

	// The same element must always produce the same key, or the lock would not
	// serialize anything.
	repeat, _ := NewUpdateElementRankMessage(46, 2759)
	repeatKey, _ := registration.SerialKey(repeat)
	if repeatKey != firstKey {
		t.Fatalf("SerialKey is unstable: %q then %q", firstKey, repeatKey)
	}
}

func TestHandlerRunsTheComputation(t *testing.T) {
	repository := &fakeRepository{
		allWin:  map[int64]RoundDelta{0: {Count: 4, MaxID: 151}},
		allLose: map[int64]RoundDelta{0: {}},
	}
	service := testService(t, repository)

	message, err := NewUpdateElementRankMessage(98, 9715)
	if err != nil {
		t.Fatalf("NewUpdateElementRankMessage() error = %v", err)
	}
	if err := service.Registration().Handler.Handle(context.Background(), message); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	pk := repository.ranksOfType(RankTypePKKing)
	if len(pk) != 1 || pk[0].WinCount != 4 || pk[0].RoundCount != 4 {
		t.Fatalf("pk ranks = %#v", pk)
	}
}

// A malformed payload must be dead-lettered rather than retried five times.
func TestHandlerMarksMalformedPayloadPermanent(t *testing.T) {
	service := testService(t, &fakeRepository{})
	registration := service.Registration()

	cases := map[string]queue.Message{
		"not json":    {Queue: QueueDefault, Type: MessageTypeUpdateElementRank, Payload: json.RawMessage(`{`)},
		"no ids":      {Queue: QueueDefault, Type: MessageTypeUpdateElementRank, Payload: json.RawMessage(`{}`)},
		"zero ids":    {Queue: QueueDefault, Type: MessageTypeUpdateElementRank, Payload: json.RawMessage(`{"post_id":0,"element_id":0}`)},
		"negative id": {Queue: QueueDefault, Type: MessageTypeUpdateElementRank, Payload: json.RawMessage(`{"post_id":-1,"element_id":5}`)},
	}
	for name, message := range cases {
		err := registration.Handler.Handle(context.Background(), message)
		if err == nil {
			t.Errorf("%s: Handle() should fail", name)
			continue
		}
		if !jobs.IsPermanent(err) {
			t.Errorf("%s: error must be permanent, got %v", name, err)
		}
	}
}

// A key that cannot be derived is also a payload problem, so the runner
// dead-letters it through the same path.
func TestSerialKeyFailsOnMalformedPayload(t *testing.T) {
	registration := testService(t, &fakeRepository{}).Registration()

	_, err := registration.SerialKey(queue.Message{
		Queue: QueueDefault, Type: MessageTypeUpdateElementRank, Payload: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("SerialKey() should reject a payload with no identifiers")
	}
}
