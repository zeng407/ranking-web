package realtime

import (
	"context"
	"os"
	"testing"
	"time"

	"2pick.app/backend/internal/realtime/pushertest"
)

// These tests answer one question: does a broadcast reach a client that is
// connected to a different Soketi instance than the one it was published to?
//
// They need the two-instance harness:
//
//	docker compose -f compose.soketi-scaling.yml up -d
//
// and the addresses supplied through the environment:
//
//	SOKETI_A_WS   ws://localhost:6011
//	SOKETI_B_HTTP localhost:6012
//
// Without those they skip, so the release build stays hermetic.
const (
	testAppID     = "scaling-test"
	testAppKey    = "scaling-test-key"
	testAppSecret = "scaling-test-secret"
	// Generous: it covers the publish round trip plus the Redis hop between nodes.
	deliveryTimeout = 10 * time.Second
)

func requireHarness(t *testing.T) (wsA, httpB string) {
	t.Helper()
	wsA = os.Getenv("SOKETI_A_WS")
	httpB = os.Getenv("SOKETI_B_HTTP")
	if wsA == "" || httpB == "" {
		t.Skip("SOKETI_A_WS and SOKETI_B_HTTP are not set; skipping the Soketi scaling test")
	}
	return wsA, httpB
}

func publisherFor(t *testing.T, hostPort string) *PusherPublisher {
	t.Helper()

	host, port := splitHostPort(t, hostPort)
	publisher, err := NewPusherPublisher(Config{
		AppID:  testAppID,
		Key:    testAppKey,
		Secret: testAppSecret,
		Host:   host,
		Port:   port,
		Secure: false,
	})
	if err != nil {
		t.Fatalf("NewPusherPublisher() error = %v", err)
	}
	return publisher
}

func splitHostPort(t *testing.T, value string) (string, string) {
	t.Helper()
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] == ':' {
			return value[:index], value[index+1:]
		}
	}
	t.Fatalf("SOKETI_B_HTTP must be host:port, got %q", value)
	return "", ""
}

// The baseline: publishing to the same instance the client is connected to always
// works, adapter or not. Without this the cross-instance result cannot be
// attributed to the adapter.
func TestBroadcastReachesTheSameInstance(t *testing.T) {
	wsA, _ := requireHarness(t)
	hostPortA := os.Getenv("SOKETI_A_HTTP")
	if hostPortA == "" {
		t.Skip("SOKETI_A_HTTP is not set; skipping the same-instance baseline")
	}

	channel := pushertest.UniqueChannel("game-room.scaling-", "same")
	client := pushertest.Dial(t, wsA, testAppKey)
	client.Subscribe(t, channel)

	publisher := publisherFor(t, hostPortA)
	if err := publisher.Publish(context.Background(), channel, "voted", map[string]string{"who": "same"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	frame, ok := client.AwaitEvent(t, channel, "voted", deliveryTimeout)
	if !ok {
		t.Fatal("the event did not arrive on the instance it was published to")
	}
	if frame.Data == "" {
		t.Fatalf("event arrived with no payload: %#v", frame)
	}
}

// THE POINT OF THE HARNESS. Subscribe on instance A, publish to instance B.
//
// With ADAPTER_DRIVER=redis this passes. With the default `local` it fails, and
// that failure is silent in production: the publish returns 200, nothing is logged,
// and the clients on the other node simply never update.
func TestBroadcastCrossesInstances(t *testing.T) {
	wsA, httpB := requireHarness(t)

	channel := pushertest.UniqueChannel("game-room.scaling-", "cross")
	client := pushertest.Dial(t, wsA, testAppKey)
	client.Subscribe(t, channel)

	publisher := publisherFor(t, httpB)
	// The publish itself succeeds either way. That is exactly why the failure mode
	// is invisible without a test like this one.
	if err := publisher.Publish(context.Background(), channel, "voted", map[string]string{"who": "cross"}); err != nil {
		t.Fatalf("Publish() to instance B error = %v", err)
	}

	_, ok := client.AwaitEvent(t, channel, "voted", deliveryTimeout)

	expectDelivery := os.Getenv("SOKETI_EXPECT_CROSS_DELIVERY") != "false"
	if expectDelivery && !ok {
		t.Fatal("a client on instance A did not receive an event published to instance B; " +
			"ADAPTER_DRIVER is almost certainly not redis")
	}
	if !expectDelivery && ok {
		t.Fatal("the event crossed instances with the adapter disabled, so the control case is not testing what it claims")
	}
	t.Logf("cross-instance delivery: %v (expected %v)", ok, expectDelivery)
}

// A room the size of the largest one in production. 1,088 users were in one room,
// and every vote fans out to all of them, so this checks the fan-out crosses
// instances too rather than only single subscribers.
func TestBroadcastFansOutAcrossInstances(t *testing.T) {
	wsA, httpB := requireHarness(t)
	if os.Getenv("SOKETI_EXPECT_CROSS_DELIVERY") == "false" {
		t.Skip("fan-out is only meaningful when cross-instance delivery is expected")
	}

	const subscribers = 25
	channel := pushertest.UniqueChannel("game-room.scaling-", "fanout")

	clients := make([]*pushertest.Subscriber, 0, subscribers)
	for index := 0; index < subscribers; index++ {
		client := pushertest.Dial(t, wsA, testAppKey)
		client.Subscribe(t, channel)
		clients = append(clients, client)
	}

	publisher := publisherFor(t, httpB)
	started := time.Now()
	if err := publisher.Publish(context.Background(), channel, "voted", map[string]string{"who": "fanout"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	delivered := 0
	for _, client := range clients {
		if _, ok := client.AwaitEvent(t, channel, "voted", deliveryTimeout); ok {
			delivered++
		}
	}

	if delivered != subscribers {
		t.Fatalf("delivered to %d of %d subscribers", delivered, subscribers)
	}
	t.Logf("one publish to instance B reached all %d subscribers on instance A in %s",
		subscribers, time.Since(started).Round(time.Millisecond))
}

func TestNewPusherPublisherValidatesConfig(t *testing.T) {
	base := Config{AppID: "a", Key: "k", Secret: "s", Host: "h"}
	for name, mutate := range map[string]func(*Config){
		"no app id": func(c *Config) { c.AppID = "" },
		"no key":    func(c *Config) { c.Key = "" },
		"no secret": func(c *Config) { c.Secret = "" },
		"no host":   func(c *Config) { c.Host = "" },
	} {
		configuration := base
		mutate(&configuration)
		if _, err := NewPusherPublisher(configuration); err == nil {
			t.Errorf("NewPusherPublisher() should reject the %s case", name)
		}
	}
	if _, err := NewPusherPublisher(base); err != nil {
		t.Fatalf("NewPusherPublisher() error = %v", err)
	}
}

// The channel names must match what the frontend already subscribes to, or the
// browser contract breaks even when the adapter is right.
func TestChannelNamesMatchTheLaravelEvents(t *testing.T) {
	if got := GameRoomChannel("abc123"); got != "game-room.abc123" {
		t.Fatalf("GameRoomChannel() = %q", got)
	}
	if got := GameRoomGameChannel("abc123", "xyz789"); got != "game-room.abc123.game-serial.xyz789" {
		t.Fatalf("GameRoomGameChannel() = %q", got)
	}
}

func TestPublishRejectsEmptyChannelAndEvent(t *testing.T) {
	publisher, err := NewPusherPublisher(Config{AppID: "a", Key: "k", Secret: "s", Host: "127.0.0.1", Port: "1"})
	if err != nil {
		t.Fatalf("NewPusherPublisher() error = %v", err)
	}
	if err := publisher.Publish(context.Background(), "", "voted", nil); err == nil {
		t.Error("an empty channel must be rejected")
	}
	if err := publisher.Publish(context.Background(), "game-room.a", "", nil); err == nil {
		t.Error("an empty event name must be rejected")
	}
}
