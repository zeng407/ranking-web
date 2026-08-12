// Package jobs maps queue message types to handlers and states each handler's
// execution contract.
//
// Laravel's job classes declare none of this: no $tries, no $timeout, no
// backoff, and no dead-letter policy. Anything registered here must say what its
// timeout and attempt budget are, and whether two deliveries touching the same
// row may run concurrently.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"2pick.app/backend/internal/queue"
)

// ErrUnknownType means no handler is registered for a message type. It is not
// retryable: redelivering will not conjure a handler.
var ErrUnknownType = errors.New("jobs: no handler registered for message type")

// Handler performs one job. Returning an error triggers the retry policy unless
// the error is wrapped with Permanent.
type Handler interface {
	Handle(ctx context.Context, message queue.Message) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, message queue.Message) error

func (fn HandlerFunc) Handle(ctx context.Context, message queue.Message) error {
	return fn(ctx, message)
}

// permanentError marks a failure that redelivery cannot fix, such as a malformed
// payload. It is dead-lettered immediately instead of burning the attempt budget
// and delaying the queue behind exponential backoff.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as not worth retrying.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

// IsPermanent reports whether err should skip the retry policy.
func IsPermanent(err error) bool {
	var permanent permanentError
	return errors.As(err, &permanent)
}

// SerialKeyFunc derives the serialization key for a message, or "" when the job
// is safe to run concurrently with itself.
//
// Ranking jobs must set this, and it is a correctness requirement rather than an
// optimisation. The `ranks` table has no unique index on
// (post_id, element_id, rank_type, record_date), so Laravel's updateOrCreate is a
// SELECT followed by an INSERT or UPDATE with nothing to make it atomic: two
// concurrent runs for the same element both find no row and both insert. That
// race has already produced duplicate rows in production data. Until a unique
// index exists, this lock is the only thing preventing it.
type SerialKeyFunc func(message queue.Message) (string, error)

// Registration is one handler and its execution contract.
type Registration struct {
	// Type is the queue message type this handles.
	Type string
	// Handler does the work.
	Handler Handler
	// Timeout bounds one attempt. It must stay under the worker's job timeout,
	// which in turn stays within Laravel's redis retry_after of 90s.
	Timeout time.Duration
	// MaxAttempts bounds redelivery before the dead-letter queue.
	MaxAttempts int
	// SerialKey optionally serializes deliveries that touch the same rows.
	SerialKey SerialKeyFunc
	// LaravelJob names the class this replaces, for traceability during cutover.
	LaravelJob string
}

// Registry resolves message types to registrations.
type Registry struct {
	byType map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{byType: make(map[string]Registration)}
}

// Register adds a handler, rejecting an incomplete or duplicate registration.
func (registry *Registry) Register(registration Registration) error {
	if strings.TrimSpace(registration.Type) == "" {
		return errors.New("jobs: registration needs a message type")
	}
	if registration.Handler == nil {
		return fmt.Errorf("jobs: %q has no handler", registration.Type)
	}
	if registration.Timeout <= 0 {
		return fmt.Errorf("jobs: %q needs a positive timeout", registration.Type)
	}
	if registration.MaxAttempts < 1 {
		return fmt.Errorf("jobs: %q needs at least one attempt", registration.Type)
	}
	if _, exists := registry.byType[registration.Type]; exists {
		return fmt.Errorf("jobs: %q is already registered", registration.Type)
	}
	registry.byType[registration.Type] = registration
	return nil
}

// MustRegister panics on a bad registration. Intended for wiring at startup,
// where a mistake should stop the process rather than surface as a runtime
// dead-letter.
func (registry *Registry) MustRegister(registrations ...Registration) {
	for _, registration := range registrations {
		if err := registry.Register(registration); err != nil {
			panic(err)
		}
	}
}

// Lookup returns the registration for a message type.
func (registry *Registry) Lookup(messageType string) (Registration, error) {
	registration, ok := registry.byType[messageType]
	if !ok {
		return Registration{}, fmt.Errorf("%w: %q", ErrUnknownType, messageType)
	}
	return registration, nil
}

// Types returns the registered message types, sorted, for the startup log.
func (registry *Registry) Types() []string {
	types := make([]string, 0, len(registry.byType))
	for messageType := range registry.byType {
		types = append(types, messageType)
	}
	sort.Strings(types)
	return types
}

// Len reports how many handlers are registered.
func (registry *Registry) Len() int {
	return len(registry.byType)
}
