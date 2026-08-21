package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Retry and dead-letter policy.
//
// None of Laravel's 24 job classes declares $tries, $timeout or backoff; they
// all inherit whatever the worker was started with. These constants make the
// contract explicit and are overridable per handler.
const (
	// DefaultMaxAttempts bounds redelivery. Past it the message is dead-lettered
	// rather than retried forever, because a message that has failed this often
	// is almost always malformed rather than unlucky.
	DefaultMaxAttempts = 5
	// BaseRetryDelay is the first backoff step; it doubles per attempt.
	BaseRetryDelay = 5 * time.Second
	// MaxRetryDelay caps the exponential growth.
	MaxRetryDelay = 10 * time.Minute
	// ReserveBlockTimeout is how long a consumer waits for work before looping.
	// It must stay well under the shutdown timeout so a drain is not held up by
	// one idle blocking read.
	//
	// It is also the worst-case delay for every queue that is NOT the one the
	// blocking read sits on: those are drained by a non-blocking pop once per
	// loop, so a message on `game_room` published while the worker is parked on
	// `low` waits for this window to expire. That queue carries the pairing a
	// room is looking at, so the window is a quarter of a second rather than
	// seconds — it cost a measured 800ms of lag at 2s. The price is one
	// non-blocking pop per queue per window on an idle worker, which is a few
	// dozen Redis commands a second and nothing next to a single job.
	ReserveBlockTimeout = 250 * time.Millisecond
	// ReserveFailurePause is how long a consumer waits after a FAILED reserve, so
	// a Redis outage does not become a hot loop. Unlike the block above this is
	// not on any message's path — nothing is being delivered while Redis is down.
	ReserveFailurePause = 2 * time.Second
)

// processingSuffix names the in-flight list for a queue. Reserving moves the
// message here so a crash cannot lose it: a plain BRPOP would delete the message
// before the handler ran.
const processingSuffix = ":processing"

// deadLetterSuffix names the queue holding messages that exhausted their
// attempts.
const deadLetterSuffix = ":dead"

// Reservation is a message that has been taken off a queue but not yet
// acknowledged. Exactly one of Ack, Retry or DeadLetter must be called.
type Reservation struct {
	Message Message
	// raw is the exact bytes on the processing list, needed to remove that entry
	// by value.
	raw       []byte
	queueKey  string
	transport *RedisTransport
}

// RetryDelay returns the backoff for the next attempt of this message.
func RetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := BaseRetryDelay << (attempt - 1)
	if delay > MaxRetryDelay || delay <= 0 {
		return MaxRetryDelay
	}
	return delay
}

// Reserve takes the next message from the first queue that has one, checking
// queues in the order given so priority is respected.
//
// It returns a nil Reservation and no error when every queue is empty, which the
// caller should treat as "idle", not as a failure.
func (transport *RedisTransport) Reserve(ctx context.Context, queues []string, block time.Duration) (*Reservation, error) {
	if len(queues) == 0 {
		return nil, errors.New("queue: at least one queue is required to reserve")
	}

	// Higher-priority queues are drained without blocking so a message on `high`
	// is never left waiting while a blocking read sits on `low`.
	for index, name := range queues {
		key := transport.Key(name)
		processing := key + processingSuffix

		var (
			body []byte
			err  error
		)
		if index == len(queues)-1 {
			// Only the lowest-priority queue blocks, so an idle worker parks
			// instead of spinning.
			body, err = transport.blockingReserve(ctx, key, processing, block)
		} else {
			body, err = transport.client.RPopLPush(ctx, key, processing).Bytes()
		}
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("queue: reserve from %q: %w", key, err)
		}

		message, decodeErr := Decode(body)
		if decodeErr != nil {
			// An undecodable entry can never succeed, so it goes straight to the
			// dead-letter queue instead of blocking the processing list forever.
			if dlqErr := transport.deadLetterRaw(ctx, key, body); dlqErr != nil {
				return nil, errors.Join(decodeErr, dlqErr)
			}
			return nil, decodeErr
		}

		return &Reservation{
			Message:   message,
			raw:       body,
			queueKey:  key,
			transport: transport,
		}, nil
	}
	return nil, nil
}

// blockingReserve is BRPOPLPUSH with a sub-second timeout.
//
// The command has taken a fractional timeout since Redis 6.0, but the client's
// typed helper formats it as whole seconds and floors anything smaller to one,
// which would silently make ReserveBlockTimeout a second whatever it says. That
// second is the delay paid by every queue the blocking read is NOT sitting on,
// so it is worth issuing the command directly.
func (transport *RedisTransport) blockingReserve(
	ctx context.Context, key, processing string, block time.Duration,
) ([]byte, error) {
	seconds := strconv.FormatFloat(block.Seconds(), 'f', -1, 64)
	value, err := transport.client.Do(ctx, "brpoplpush", key, processing, seconds).Text()
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// Ack removes the message from the processing list, completing it.
func (reservation *Reservation) Ack(ctx context.Context) error {
	return reservation.removeFromProcessing(ctx)
}

// Retry returns the message to its queue with an incremented attempt count,
// after the backoff for that attempt.
//
// The delay is applied by the caller before republishing rather than with a
// Redis delayed-set, keeping the backend a plain list. The message stays on the
// processing list until it is republished, so a crash mid-retry leaves it
// recoverable rather than lost.
func (reservation *Reservation) Retry(ctx context.Context) error {
	next := reservation.Message
	next.Attempt++

	body, err := encodeMessage(next)
	if err != nil {
		return err
	}
	if err := reservation.transport.client.LPush(ctx, reservation.queueKey, body).Err(); err != nil {
		return fmt.Errorf("queue: requeue for retry: %w", err)
	}
	return reservation.removeFromProcessing(ctx)
}

// Requeue returns the message to its queue unchanged, without consuming an
// attempt.
//
// This is for a benign conflict, such as another delivery already holding the
// serialization lock for the same key: the message is not failing, it just
// cannot run right now. Using Retry there would burn the attempt budget and
// eventually dead-letter healthy work. LPUSH puts it at the tail of the FIFO
// order, so it goes behind whatever else is waiting rather than being retried
// immediately.
func (reservation *Reservation) Requeue(ctx context.Context) error {
	if err := reservation.transport.client.LPush(ctx, reservation.queueKey, reservation.raw).Err(); err != nil {
		return fmt.Errorf("queue: requeue: %w", err)
	}
	return reservation.removeFromProcessing(ctx)
}

// DeadLetter moves the message to the dead-letter queue.
func (reservation *Reservation) DeadLetter(ctx context.Context) error {
	if err := reservation.transport.client.LPush(ctx, reservation.queueKey+deadLetterSuffix, reservation.raw).Err(); err != nil {
		return fmt.Errorf("queue: dead-letter: %w", err)
	}
	return reservation.removeFromProcessing(ctx)
}

// Attempt reports which delivery this is, starting at 1.
func (reservation *Reservation) Attempt() int {
	if reservation.Message.Attempt < 1 {
		return 1
	}
	return reservation.Message.Attempt
}

func (reservation *Reservation) removeFromProcessing(ctx context.Context) error {
	// Count 1 removes a single matching entry, so two identical messages in
	// flight do not both disappear when one finishes.
	if err := reservation.transport.client.LRem(ctx, reservation.queueKey+processingSuffix, 1, reservation.raw).Err(); err != nil {
		return fmt.Errorf("queue: remove from processing: %w", err)
	}
	return nil
}

func (transport *RedisTransport) deadLetterRaw(ctx context.Context, queueKey string, body []byte) error {
	pipeline := transport.client.Pipeline()
	pipeline.LPush(ctx, queueKey+deadLetterSuffix, body)
	pipeline.LRem(ctx, queueKey+processingSuffix, 1, body)
	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("queue: dead-letter undecodable entry: %w", err)
	}
	return nil
}

// RecoverProcessing returns everything stranded on a queue's processing list to
// the queue itself.
//
// This must run at startup. A worker killed mid-handler leaves its message on
// the processing list, where nothing would ever pick it up again; the list is
// what makes the reservation crash-safe, and this is what closes the loop.
// Recovered messages keep their attempt count, so a message that keeps killing
// the worker still reaches the dead-letter queue instead of looping forever.
func (transport *RedisTransport) RecoverProcessing(ctx context.Context, queues []string) (int, error) {
	recovered := 0
	for _, name := range queues {
		key := transport.Key(name)
		processing := key + processingSuffix
		for {
			err := transport.client.RPopLPush(ctx, processing, key).Err()
			if errors.Is(err, redis.Nil) {
				break
			}
			if err != nil {
				return recovered, fmt.Errorf("queue: recover %q: %w", processing, err)
			}
			recovered++
		}
	}
	return recovered, nil
}

// DeadLetterLength reports how many messages are parked for a queue, for
// alerting.
func (transport *RedisTransport) DeadLetterLength(ctx context.Context, name string) (int64, error) {
	length, err := transport.client.LLen(ctx, transport.Key(name)+deadLetterSuffix).Result()
	if err != nil {
		return 0, fmt.Errorf("queue: dead-letter length for %q: %w", name, err)
	}
	return length, nil
}

// DeadLetterKey exposes the dead-letter list key, for operational tooling.
func (transport *RedisTransport) DeadLetterKey(name string) string {
	return transport.Key(name) + deadLetterSuffix
}

// ProcessingKey exposes the in-flight list key, for operational tooling.
func (transport *RedisTransport) ProcessingKey(name string) string {
	return transport.Key(name) + processingSuffix
}
