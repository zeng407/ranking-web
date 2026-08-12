// Package queue publishes and reserves background work.
//
// The publish side deliberately makes "enqueue only after the database commit"
// the easy path. Laravel's redis queue connection runs with after_commit: false,
// so a job could observe — or act on — a row that a later rollback removed. The
// Go implementation must not reproduce that, so transactional callers collect
// messages in an Outbox and the Publisher flushes them only once the commit has
// actually succeeded.
package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrEmptyQueue   = errors.New("queue name is required")
	ErrEmptyType    = errors.New("message type is required")
	ErrNoTransport  = errors.New("queue transport is required")
	ErrEmptyPayload = errors.New("message payload must be valid JSON")
)

// Message is one unit of background work.
type Message struct {
	// Queue is the logical queue name, matching the names the Laravel jobs
	// already declare (high, default, low, rank_report_history, game_room).
	Queue string `json:"queue"`
	// Type selects the handler.
	Type string `json:"type"`
	// Payload is handler-specific JSON.
	Payload json.RawMessage `json:"payload"`
	// IdempotencyKey lets a handler detect a redelivery of work it already
	// completed. Every message that writes ranks, votes or game progress must
	// set it.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// EnqueuedAt is stamped by the Publisher.
	EnqueuedAt time.Time `json:"enqueued_at"`
	// Attempt counts deliveries, starting at 1.
	Attempt int `json:"attempt"`
}

func (message Message) validate() error {
	if strings.TrimSpace(message.Queue) == "" {
		return ErrEmptyQueue
	}
	if strings.TrimSpace(message.Type) == "" {
		return ErrEmptyType
	}
	if len(message.Payload) > 0 && !json.Valid(message.Payload) {
		return ErrEmptyPayload
	}
	return nil
}

// Transport moves messages to and from the durable queue backend.
type Transport interface {
	Publish(ctx context.Context, messages []Message) error
}

// Tx is the transaction surface a caller needs: the query methods it runs work
// through, plus commit and rollback. Kept narrow in the style of the existing
// repository queryer interfaces so the commit path is testable without a
// database. *sql.Tx satisfies it.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}

// TxBeginner starts a transaction.
type TxBeginner interface {
	BeginTx(ctx context.Context, options *sql.TxOptions) (Tx, error)
}

// SQLBeginner adapts *sql.DB, whose BeginTx returns the concrete *sql.Tx and so
// does not satisfy TxBeginner directly.
type SQLBeginner struct {
	DB *sql.DB
}

func (beginner SQLBeginner) BeginTx(ctx context.Context, options *sql.TxOptions) (Tx, error) {
	transaction, err := beginner.DB.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

// Outbox collects the messages a transaction intends to publish. Nothing here
// reaches the transport until the surrounding commit succeeds.
type Outbox struct {
	messages []Message
}

func (outbox *Outbox) Add(messages ...Message) error {
	for _, message := range messages {
		if err := message.validate(); err != nil {
			return fmt.Errorf("outbox: %w", err)
		}
	}
	outbox.messages = append(outbox.messages, messages...)
	return nil
}

func (outbox *Outbox) Len() int {
	return len(outbox.messages)
}

type Publisher struct {
	transport Transport
	now       func() time.Time
}

type PublisherOption func(*Publisher)

// WithClock overrides the timestamp source, mirroring auth.Verifier.
func WithClock(now func() time.Time) PublisherOption {
	return func(publisher *Publisher) {
		if now != nil {
			publisher.now = now
		}
	}
}

func NewPublisher(transport Transport, options ...PublisherOption) (*Publisher, error) {
	if transport == nil {
		return nil, ErrNoTransport
	}
	publisher := &Publisher{transport: transport, now: time.Now}
	for _, option := range options {
		option(publisher)
	}
	return publisher, nil
}

// Publish sends immediately. Use it only when no database transaction is open;
// inside one, use WithinTransaction so a rollback cannot leave a published
// message behind.
func (publisher *Publisher) Publish(ctx context.Context, messages ...Message) error {
	if len(messages) == 0 {
		return nil
	}
	prepared, err := publisher.prepare(messages)
	if err != nil {
		return err
	}
	return publisher.transport.Publish(ctx, prepared)
}

// WithinTransaction runs fn inside a transaction and publishes whatever fn put
// in the Outbox, but only after the commit succeeds.
//
// If fn returns an error the transaction is rolled back and nothing is
// published. If the commit itself fails nothing is published either, because
// the work the messages describe does not exist.
func (publisher *Publisher) WithinTransaction(
	ctx context.Context,
	beginner TxBeginner,
	options *sql.TxOptions,
	fn func(context.Context, Tx, *Outbox) error,
) error {
	if beginner == nil {
		return errors.New("queue: transaction beginner is required")
	}
	if fn == nil {
		return errors.New("queue: transaction function is required")
	}

	transaction, err := beginner.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("queue: begin transaction: %w", err)
	}

	outbox := &Outbox{}
	if err := fn(ctx, transaction, outbox); err != nil {
		// Rollback failures are joined rather than replacing the original error,
		// which is the one that explains why the work was abandoned.
		if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("queue: rollback: %w", rollbackErr))
		}
		return err
	}

	// Prepared before the commit so an invalid message aborts the transaction
	// instead of committing work whose follow-up can never be published.
	prepared, err := publisher.prepare(outbox.messages)
	if err != nil {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("queue: rollback: %w", rollbackErr))
		}
		return err
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("queue: commit: %w", err)
	}

	if len(prepared) == 0 {
		return nil
	}
	if err := publisher.transport.Publish(ctx, prepared); err != nil {
		// The commit already happened, so this is not recoverable by retrying the
		// transaction. Surfacing it distinctly lets the caller decide between a
		// reconciliation sweep and an alert.
		return fmt.Errorf("queue: publish after commit: %w", err)
	}
	return nil
}

func (publisher *Publisher) prepare(messages []Message) ([]Message, error) {
	if len(messages) == 0 {
		return nil, nil
	}
	now := publisher.now().UTC()
	prepared := make([]Message, 0, len(messages))
	for _, message := range messages {
		if err := message.validate(); err != nil {
			return nil, fmt.Errorf("queue: %w", err)
		}
		if message.EnqueuedAt.IsZero() {
			message.EnqueuedAt = now
		}
		if message.Attempt <= 0 {
			message.Attempt = 1
		}
		prepared = append(prepared, message)
	}
	return prepared, nil
}
