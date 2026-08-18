package mailer

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogSenderWritesTheWholeBodySoTheLinkCanBeRead(t *testing.T) {
	var output bytes.Buffer
	sender := NewLogSender(slog.New(slog.NewTextHandler(&output, nil)))

	err := sender.Send(context.Background(), Message{
		To:       "reader@example.com",
		Subject:  "重設你的密碼",
		TextBody: "open https://example.com/zh-tw/password/reset/a-token",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	logged := output.String()
	for _, expected := range []string{"reader@example.com", "password/reset/a-token"} {
		if !strings.Contains(logged, expected) {
			t.Errorf("the log is missing %q:\n%s", expected, logged)
		}
	}
}

func TestLogSenderRefusesAnEmptyBody(t *testing.T) {
	sender := NewLogSender(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	if err := sender.Send(context.Background(), Message{To: "reader@example.com", Subject: "s"}); err == nil {
		t.Fatal("an empty body was accepted")
	}
}
