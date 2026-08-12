// Package realtime publishes game room events to the Pusher-protocol server.
//
// The server is Soketi, which the frontend already talks to through pusher-js, so
// keeping the protocol means the browser contract does not change during the
// migration.
package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pusher "github.com/pusher/pusher-http-go/v5"
)

// PublishTimeout bounds one publish. A broadcast is a single HTTP call to Soketi;
// anything slower means Soketi is unhealthy, and the caller should not wait on it.
const PublishTimeout = 5 * time.Second

// Config points the publisher at Soketi.
//
// The field names mirror Laravel's PUSHER_* variables so one .env drives both
// during the cutover.
type Config struct {
	AppID   string
	Key     string
	Secret  string
	Host    string
	Port    string
	Secure  bool
	Cluster string
}

// Publisher sends events to channels.
type Publisher interface {
	Publish(ctx context.Context, channel, event string, payload any) error
}

// PusherPublisher publishes over the Pusher HTTP API.
type PusherPublisher struct {
	client *pusher.Client
}

func NewPusherPublisher(configuration Config) (*PusherPublisher, error) {
	for name, value := range map[string]string{
		"app id": configuration.AppID,
		"key":    configuration.Key,
		"secret": configuration.Secret,
		"host":   configuration.Host,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("realtime: pusher %s is required", name)
		}
	}

	host := configuration.Host
	if configuration.Port != "" {
		host = host + ":" + configuration.Port
	}

	return &PusherPublisher{client: &pusher.Client{
		AppID:   configuration.AppID,
		Key:     configuration.Key,
		Secret:  configuration.Secret,
		Host:    host,
		Secure:  configuration.Secure,
		Cluster: configuration.Cluster,
	}}, nil
}

// Publish sends one event to one channel.
func (publisher *PusherPublisher) Publish(ctx context.Context, channel, event string, payload any) error {
	if strings.TrimSpace(channel) == "" {
		return errors.New("realtime: channel is required")
	}
	if strings.TrimSpace(event) == "" {
		return errors.New("realtime: event name is required")
	}

	publishContext, cancel := context.WithTimeout(ctx, PublishTimeout)
	defer cancel()

	// The library takes no context, so the timeout is enforced by racing it. A
	// broadcast that outlives the deadline is abandoned rather than holding the
	// caller: the event is best-effort, and the room refreshes on the next one.
	done := make(chan error, 1)
	go func() {
		done <- publisher.client.Trigger(channel, event, payload)
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("realtime: publish %q to %q: %w", event, channel, err)
		}
		return nil
	case <-publishContext.Done():
		return fmt.Errorf("realtime: publish %q to %q: %w", event, channel, publishContext.Err())
	}
}

// GameRoomChannel is the channel name the frontend subscribes to, matching
// BroadcastGameVoted, BroadcastGameBetRank and BroadcastGameRoomRefresh:
// "game-room.{serial}".
func GameRoomChannel(roomSerial string) string {
	return "game-room." + roomSerial
}

// GameRoomGameChannel matches BroadcastGameBet, which uses the longer form
// "game-room.{roomSerial}.game-serial.{gameSerial}".
func GameRoomGameChannel(roomSerial, gameSerial string) string {
	return fmt.Sprintf("game-room.%s.game-serial.%s", roomSerial, gameSerial)
}
