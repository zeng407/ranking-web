// Package pushertest is a minimal Pusher-protocol WebSocket client for tests.
//
// It exists so more than one package can assert that a broadcast actually reaches a
// subscriber. pusher-js is not available here, and the handshake is small enough to
// speak directly: connect, wait for pusher:connection_established, send
// pusher:subscribe, wait for the confirmation, then read events.
//
// Test-only. It is a normal package rather than a _test.go file so it can be
// imported, but nothing in the production build refers to it.
package pushertest

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Frame is an inbound frame from the server.
//
// The protocol is asymmetric about `data`, which is easy to get wrong: server to
// client it is a JSON *string* holding encoded JSON, client to server it is a plain
// object. UnmarshalJSON accepts either form so a frame that breaks the rule still
// decodes instead of failing a test for the wrong reason.
type Frame struct {
	Event   string
	Channel string
	Data    string
}

func (frame *Frame) UnmarshalJSON(raw []byte) error {
	var wire struct {
		Event   string          `json:"event"`
		Channel string          `json:"channel"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}

	frame.Event = wire.Event
	frame.Channel = wire.Channel
	frame.Data = ""
	if len(wire.Data) == 0 {
		return nil
	}
	var text string
	if err := json.Unmarshal(wire.Data, &text); err == nil {
		frame.Data = text
		return nil
	}
	frame.Data = string(wire.Data)
	return nil
}

// outboundFrame is a client to server frame. Data is an object, not a string:
// sending a string makes Soketi read `data.channel` off a primitive, and the
// resulting TypeError is uncaught and takes the whole node process down.
type outboundFrame struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

// Subscriber is one connected client.
type Subscriber struct {
	connection *websocket.Conn
	socketID   string
}

// Dial connects to a Pusher-protocol server and completes the handshake. wsURL is
// the origin, such as "ws://127.0.0.1:6001"; the app path is appended.
//
// Use 127.0.0.1 rather than localhost from inside a container: localhost resolves to
// ::1 first and published Docker ports are IPv4 only.
func Dial(t *testing.T, wsURL, appKey string) *Subscriber {
	t.Helper()

	endpoint, err := url.Parse(wsURL + "/app/" + appKey)
	if err != nil {
		t.Fatalf("parse ws url: %v", err)
	}
	query := endpoint.Query()
	query.Set("protocol", "7")
	query.Set("client", "go-test")
	query.Set("version", "1.0")
	endpoint.RawQuery = query.Encode()

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	connection, _, err := dialer.Dial(endpoint.String(), nil)
	if err != nil {
		t.Fatalf("dial %s: %v", endpoint.String(), err)
	}
	t.Cleanup(func() { connection.Close() })

	client := &Subscriber{connection: connection}

	// The first frame carries the socket id and confirms the app key was accepted.
	frame := client.Read(t, 10*time.Second)
	if frame.Event != "pusher:connection_established" {
		t.Fatalf("first frame = %q, want pusher:connection_established (data: %s)", frame.Event, frame.Data)
	}
	var established struct {
		SocketID string `json:"socket_id"`
	}
	if err := json.Unmarshal([]byte(frame.Data), &established); err != nil {
		t.Fatalf("decode connection_established: %v", err)
	}
	client.socketID = established.SocketID
	return client
}

// SocketID is the id the server assigned this connection.
func (client *Subscriber) SocketID() string { return client.socketID }

func (client *Subscriber) send(t *testing.T, frame outboundFrame) {
	t.Helper()

	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("encode %s frame: %v", frame.Event, err)
	}
	if err := client.connection.WriteMessage(websocket.TextMessage, encoded); err != nil {
		t.Fatalf("send %s: %v", frame.Event, err)
	}
}

// Subscribe joins a channel and waits for the server to confirm.
//
// Confirming before publishing is what makes a delivery test deterministic:
// publishing into a channel with no subscriber yet looks exactly like a delivery
// failure.
func (client *Subscriber) Subscribe(t *testing.T, channel string) {
	t.Helper()

	client.send(t, outboundFrame{
		Event: "pusher:subscribe",
		Data:  map[string]string{"channel": channel},
	})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		received := client.Read(t, time.Until(deadline))
		if received.Event == "pusher_internal:subscription_succeeded" && received.Channel == channel {
			return
		}
	}
	t.Fatalf("no subscription confirmation for %q", channel)
}

// Read takes the next frame, failing the test if none arrives in time.
func (client *Subscriber) Read(t *testing.T, timeout time.Duration) Frame {
	t.Helper()

	if err := client.connection.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, message, err := client.connection.ReadMessage()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}

	var frame Frame
	if err := json.Unmarshal(message, &frame); err != nil {
		t.Fatalf("decode frame %q: %v", message, err)
	}
	return frame
}

// AwaitEvent waits for a named event on a channel, ignoring the protocol's own
// keepalive traffic. It reports whether the event arrived, so a caller can assert
// either delivery or silence.
func (client *Subscriber) AwaitEvent(t *testing.T, channel, event string, timeout time.Duration) (Frame, bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return Frame{}, false
		}

		if err := client.connection.SetReadDeadline(time.Now().Add(remaining)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, message, err := client.connection.ReadMessage()
		if err != nil {
			// A timeout here means the event never arrived, which is a result rather
			// than a test failure: a control case expects exactly this.
			return Frame{}, false
		}

		var frame Frame
		if err := json.Unmarshal(message, &frame); err != nil {
			continue
		}
		if frame.Event == "pusher:ping" {
			client.send(t, outboundFrame{Event: "pusher:pong", Data: map[string]string{}})
			continue
		}
		if frame.Event == event && frame.Channel == channel {
			return frame, true
		}
	}
}

// UniqueChannel builds a channel name no earlier run can have subscribed to, so a
// leftover subscription cannot make a broken configuration look like it works.
func UniqueChannel(prefix, suffix string) string {
	return fmt.Sprintf("%s%d-%s", prefix, time.Now().UnixNano(), suffix)
}
