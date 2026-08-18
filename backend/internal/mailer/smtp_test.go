package mailer

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"testing"
)

// fakeSMTP is the smallest server net/smtp will talk to: a greeting, an EHLO answer that
// offers AUTH PLAIN, and one message.
type fakeSMTP struct {
	host    string
	port    int
	lines   []string
	message string
	done    chan struct{}
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split the listener address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse the listener port: %v", err)
	}

	server := &fakeSMTP{host: host, port: port, done: make(chan struct{})}
	go func() {
		defer close(server.done)
		defer listener.Close()
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		server.converse(connection)
	}()
	t.Cleanup(func() { listener.Close() })
	return server
}

func (server *fakeSMTP) converse(connection net.Conn) {
	reader := bufio.NewReader(connection)
	write := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }

	write("220 fake ESMTP")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		server.lines = append(server.lines, line)
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			write("250-fake greets you")
			write("250 AUTH PLAIN")
		case strings.HasPrefix(line, "AUTH PLAIN"):
			write("235 authenticated")
		case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
			write("250 ok")
		case line == "DATA":
			write("354 go ahead")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			server.message = body.String()
			write("250 queued")
		case line == "QUIT":
			write("221 bye")
			return
		default:
			write("250 ok")
		}
	}
}

func TestSMTPSenderDeliversAnEncodedMessage(t *testing.T) {
	server := startFakeSMTP(t)

	sender, err := NewSMTPSender(SMTPConfig{
		Host:     server.host,
		Port:     server.port,
		Username: "2pick.app@gmail.com",
		Password: "app-password",
		From:     Address{Address: "2pick.app@gmail.com", Name: "殘酷二選一"},
	})
	if err != nil {
		t.Fatalf("build the sender: %v", err)
	}

	err = sender.Send(context.Background(), Message{
		To:       "reader@example.com",
		Subject:  "重設你的密碼",
		TextBody: "https://example.com/zh-tw/password/reset/a-token",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	<-server.done

	conversation := strings.Join(server.lines, "\n")
	for _, expected := range []string{
		"AUTH PLAIN",
		"MAIL FROM:<2pick.app@gmail.com>",
		"RCPT TO:<reader@example.com>",
		"QUIT",
	} {
		if !strings.Contains(conversation, expected) {
			t.Errorf("the conversation is missing %q:\n%s", expected, conversation)
		}
	}

	// The display name and the subject are not ASCII, so both must be RFC 2047 encoded
	// rather than sent raw.
	if !strings.Contains(server.message, "=?utf-8?q?") && !strings.Contains(server.message, "=?utf-8?b?") {
		t.Errorf("the headers are not encoded:\n%s", server.message)
	}
	if strings.Contains(server.message, "殘酷二選一") {
		t.Errorf("the display name was sent raw:\n%s", server.message)
	}
	if !strings.Contains(server.message, "Content-Transfer-Encoding: base64") {
		t.Errorf("the body is not declared base64:\n%s", server.message)
	}

	_, encoded, found := strings.Cut(server.message, "\r\n\r\n")
	if !found {
		t.Fatalf("the message has no body:\n%s", server.message)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(encoded), "\r\n", ""))
	if err != nil {
		t.Fatalf("decode the body: %v", err)
	}
	if string(decoded) != "https://example.com/zh-tw/password/reset/a-token" {
		t.Errorf("the body arrived as %q", decoded)
	}
}

func TestSMTPSenderRefusesARecipientWithAHeaderInIt(t *testing.T) {
	sender, err := NewSMTPSender(SMTPConfig{
		Host: "127.0.0.1", Port: 25,
		From: Address{Address: "2pick.app@gmail.com"},
	})
	if err != nil {
		t.Fatalf("build the sender: %v", err)
	}

	// No connection is attempted: the address is rejected before the dial, which is what
	// keeps a form field from turning into extra headers.
	err = sender.Send(context.Background(), Message{
		To:       "reader@example.com>\r\nBcc: everyone@example.com",
		Subject:  "subject",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("a recipient carrying a header was accepted")
	}
}

func TestNewSMTPSenderRejectsAnUnknownEncryption(t *testing.T) {
	_, err := NewSMTPSender(SMTPConfig{
		Host: "smtp.example.com", Port: 587, Encryption: "starttls",
		From: Address{Address: "from@example.com"},
	})
	if err == nil {
		t.Fatal("an unknown encryption was accepted")
	}
}

func TestAddressLeavesAnASCIINameAlone(t *testing.T) {
	address := Address{Address: "from@example.com", Name: "2Pick"}
	if got := address.String(); got != "2Pick <from@example.com>" {
		t.Errorf("the header value is %q", got)
	}
}
