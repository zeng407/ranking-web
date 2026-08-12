package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type recordingTransport struct {
	batches [][]Message
	err     error
}

func (transport *recordingTransport) Publish(_ context.Context, messages []Message) error {
	if transport.err != nil {
		return transport.err
	}
	batch := make([]Message, len(messages))
	copy(batch, messages)
	transport.batches = append(transport.batches, batch)
	return nil
}

func (transport *recordingTransport) published() []Message {
	all := make([]Message, 0)
	for _, batch := range transport.batches {
		all = append(all, batch...)
	}
	return all
}

// fakeTx records the commit/rollback decision and can fail the commit, which is
// the case that distinguishes publish-after-commit from publish-before-commit.
type fakeTx struct {
	commitErr   error
	committed   bool
	rolledBack  bool
	rollbackErr error
}

func (transaction *fakeTx) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}

func (transaction *fakeTx) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func (transaction *fakeTx) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

func (transaction *fakeTx) Commit() error {
	if transaction.commitErr != nil {
		return transaction.commitErr
	}
	transaction.committed = true
	return nil
}

func (transaction *fakeTx) Rollback() error {
	transaction.rolledBack = true
	return transaction.rollbackErr
}

type fakeBeginner struct {
	transaction *fakeTx
	err         error
}

func (beginner *fakeBeginner) BeginTx(context.Context, *sql.TxOptions) (Tx, error) {
	if beginner.err != nil {
		return nil, beginner.err
	}
	return beginner.transaction, nil
}

func fixedClock() func() time.Time {
	moment := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return moment }
}

func newTestPublisher(t *testing.T, transport Transport) *Publisher {
	t.Helper()
	publisher, err := NewPublisher(transport, WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	return publisher
}

func sampleMessage() Message {
	return Message{
		Queue:          "rank_report_history",
		Type:           "rank.update_element",
		Payload:        json.RawMessage(`{"element_id":42}`),
		IdempotencyKey: "element-42-v1",
	}
}

// The core contract. Laravel's redis queue runs with after_commit: false, so a
// job could be handed a row that a later rollback removed. Nothing may reach the
// transport until the commit has actually succeeded.
func TestWithinTransactionPublishesOnlyAfterCommit(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{}

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, outbox *Outbox) error {
			if addErr := outbox.Add(sampleMessage()); addErr != nil {
				return addErr
			}
			// Still inside the transaction: the message must not be out yet.
			if len(transport.published()) != 0 {
				t.Fatal("message was published before the commit")
			}
			return nil
		})
	if err != nil {
		t.Fatalf("WithinTransaction() error = %v", err)
	}

	if !transaction.committed {
		t.Fatal("transaction should have been committed")
	}
	published := transport.published()
	if len(published) != 1 || published[0].Type != "rank.update_element" {
		t.Fatalf("published = %#v", published)
	}
}

// A failing commit means the work does not exist, so the follow-up job must not
// be published either.
func TestWithinTransactionPublishesNothingWhenCommitFails(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{commitErr: errors.New("deadlock found when trying to get lock")}

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, outbox *Outbox) error {
			return outbox.Add(sampleMessage())
		})
	if err == nil {
		t.Fatal("WithinTransaction() should return the commit error")
	}

	if got := len(transport.published()); got != 0 {
		t.Fatalf("published %d messages after a failed commit, want 0", got)
	}
}

func TestWithinTransactionRollsBackAndPublishesNothingOnError(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{}
	sentinel := errors.New("business rule rejected the write")

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, outbox *Outbox) error {
			if addErr := outbox.Add(sampleMessage()); addErr != nil {
				return addErr
			}
			return sentinel
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTransaction() error = %v, want %v", err, sentinel)
	}

	if !transaction.rolledBack {
		t.Fatal("transaction should have been rolled back")
	}
	if transaction.committed {
		t.Fatal("transaction should not have been committed")
	}
	if got := len(transport.published()); got != 0 {
		t.Fatalf("published %d messages after a rollback, want 0", got)
	}
}

// An unpublishable message must abort the transaction rather than commit work
// whose follow-up can never be enqueued.
func TestWithinTransactionRejectsInvalidMessageBeforeCommitting(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{}

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, outbox *Outbox) error {
			// Bypasses Outbox.Add validation the way a future refactor might.
			outbox.messages = append(outbox.messages, Message{Type: "rank.update_element"})
			return nil
		})
	if !errors.Is(err, ErrEmptyQueue) {
		t.Fatalf("WithinTransaction() error = %v, want %v", err, ErrEmptyQueue)
	}
	if transaction.committed {
		t.Fatal("transaction should not commit when a message is unpublishable")
	}
	if !transaction.rolledBack {
		t.Fatal("transaction should have been rolled back")
	}
	if got := len(transport.published()); got != 0 {
		t.Fatalf("published %d messages, want 0", got)
	}
}

// The rollback error must not hide the error that caused the abandonment.
func TestWithinTransactionKeepsOriginalErrorWhenRollbackFails(t *testing.T) {
	publisher := newTestPublisher(t, &recordingTransport{})
	transaction := &fakeTx{rollbackErr: errors.New("connection reset")}
	sentinel := errors.New("original failure")

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, _ *Outbox) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTransaction() error = %v, want it to wrap %v", err, sentinel)
	}
}

// A commit with nothing queued must not call the transport at all.
func TestWithinTransactionSkipsTransportWhenOutboxEmpty(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{}

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, _ *Outbox) error { return nil })
	if err != nil {
		t.Fatalf("WithinTransaction() error = %v", err)
	}
	if !transaction.committed {
		t.Fatal("transaction should have been committed")
	}
	if len(transport.batches) != 0 {
		t.Fatalf("transport was called %d times, want 0", len(transport.batches))
	}
}

// A transport failure after a successful commit is distinct: the data is
// durable but the follow-up work is missing, so it must be reported clearly
// rather than looking like a failed write.
func TestWithinTransactionReportsPublishFailureAfterCommit(t *testing.T) {
	transport := &recordingTransport{err: errors.New("redis unreachable")}
	publisher := newTestPublisher(t, transport)
	transaction := &fakeTx{}

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{transaction: transaction}, nil,
		func(_ context.Context, _ Tx, outbox *Outbox) error {
			return outbox.Add(sampleMessage())
		})
	if err == nil {
		t.Fatal("WithinTransaction() should report the publish failure")
	}
	if !transaction.committed {
		t.Fatal("the commit must stand even though publishing failed")
	}
}

func TestWithinTransactionReturnsBeginError(t *testing.T) {
	publisher := newTestPublisher(t, &recordingTransport{})
	sentinel := errors.New("too many connections")

	err := publisher.WithinTransaction(context.Background(), &fakeBeginner{err: sentinel}, nil,
		func(_ context.Context, _ Tx, _ *Outbox) error {
			t.Fatal("fn must not run when the transaction cannot start")
			return nil
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTransaction() error = %v, want %v", err, sentinel)
	}
}

func TestOutboxRejectsMessageWithoutQueueOrType(t *testing.T) {
	outbox := &Outbox{}

	if err := outbox.Add(Message{Type: "rank.update_element"}); !errors.Is(err, ErrEmptyQueue) {
		t.Fatalf("Add() error = %v, want %v", err, ErrEmptyQueue)
	}
	if err := outbox.Add(Message{Queue: "default"}); !errors.Is(err, ErrEmptyType) {
		t.Fatalf("Add() error = %v, want %v", err, ErrEmptyType)
	}
	if outbox.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", outbox.Len())
	}
}

func TestOutboxRejectsInvalidJSONPayload(t *testing.T) {
	outbox := &Outbox{}
	message := sampleMessage()
	message.Payload = json.RawMessage(`{"element_id":`)

	if err := outbox.Add(message); !errors.Is(err, ErrEmptyPayload) {
		t.Fatalf("Add() error = %v, want %v", err, ErrEmptyPayload)
	}
}

func TestPublishStampsEnqueuedAtAndFirstAttempt(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)

	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	published := transport.published()
	if len(published) != 1 {
		t.Fatalf("published = %#v", published)
	}
	if published[0].Attempt != 1 {
		t.Fatalf("Attempt = %d, want 1", published[0].Attempt)
	}
	if !published[0].EnqueuedAt.Equal(fixedClock()()) {
		t.Fatalf("EnqueuedAt = %s", published[0].EnqueuedAt)
	}
	if published[0].IdempotencyKey != "element-42-v1" {
		t.Fatalf("IdempotencyKey = %q", published[0].IdempotencyKey)
	}
}

func TestPublishRejectsInvalidMessage(t *testing.T) {
	transport := &recordingTransport{}
	publisher := newTestPublisher(t, transport)

	if err := publisher.Publish(context.Background(), Message{Queue: "default"}); !errors.Is(err, ErrEmptyType) {
		t.Fatalf("Publish() error should be %v", ErrEmptyType)
	}
	if len(transport.batches) != 0 {
		t.Fatal("an invalid message must not reach the transport")
	}
}

func TestNewPublisherRequiresTransport(t *testing.T) {
	if _, err := NewPublisher(nil); !errors.Is(err, ErrNoTransport) {
		t.Fatalf("NewPublisher(nil) error = %v, want %v", err, ErrNoTransport)
	}
}
