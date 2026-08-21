package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// DefaultKeyPrefix namespaces the Go queues away from Laravel's own Redis keys.
// Laravel writes to "queues:<name>" under its own REDIS_PREFIX, and during the
// migration both applications share one Redis, so the two key spaces must not
// overlap. Each schedule is cut over by turning the Laravel entry off and the Go
// entry on, so no cross-consumption is intended or supported.
const DefaultKeyPrefix = "2pick:go:queue:"

type RedisTransport struct {
	// UniversalClient rather than Cmdable: the blocking reserve issues
	// BRPOPLPUSH through Do, because the typed helper cannot express a
	// sub-second timeout. See blockingReserve.
	client    redis.UniversalClient
	keyPrefix string
}

func NewRedisTransport(client redis.UniversalClient, keyPrefix string) (*RedisTransport, error) {
	if client == nil {
		return nil, errors.New("queue: redis client is required")
	}
	if strings.TrimSpace(keyPrefix) == "" {
		keyPrefix = DefaultKeyPrefix
	}
	return &RedisTransport{client: client, keyPrefix: keyPrefix}, nil
}

// Key returns the Redis list backing a logical queue name.
func (transport *RedisTransport) Key(queue string) string {
	return transport.keyPrefix + queue
}

// Publish appends messages to their queues. LPUSH pairs with the worker's
// BRPOP so delivery is FIFO.
func (transport *RedisTransport) Publish(ctx context.Context, messages []Message) error {
	if len(messages) == 0 {
		return nil
	}

	// Grouped so one queue receiving several messages becomes a single LPUSH,
	// and encoded up front so a marshal failure cannot leave a partial batch in
	// Redis.
	order := make([]string, 0, len(messages))
	encoded := make(map[string][]any, len(messages))
	for _, message := range messages {
		body, err := encodeMessage(message)
		if err != nil {
			return err
		}
		key := transport.Key(message.Queue)
		if _, seen := encoded[key]; !seen {
			order = append(order, key)
		}
		encoded[key] = append(encoded[key], body)
	}

	pipeline := transport.client.Pipeline()
	for _, key := range order {
		pipeline.LPush(ctx, key, encoded[key]...)
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("queue: publish to redis: %w", err)
	}
	return nil
}

func encodeMessage(message Message) ([]byte, error) {
	if err := message.validate(); err != nil {
		return nil, fmt.Errorf("queue: %w", err)
	}
	body, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("queue: encode message %q: %w", message.Type, err)
	}
	return body, nil
}

// Decode parses a raw queue entry.
func Decode(body []byte) (Message, error) {
	var message Message
	if err := json.Unmarshal(body, &message); err != nil {
		return Message{}, fmt.Errorf("queue: decode message: %w", err)
	}
	if err := message.validate(); err != nil {
		return Message{}, fmt.Errorf("queue: %w", err)
	}
	return message, nil
}
