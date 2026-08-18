package mailer

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Encryption values, matching the MAIL_ENCRYPTION variable Laravel already reads.
const (
	// EncryptionSTARTTLS upgrades a plain connection. This is port 587, and what
	// smtp.gmail.com expects.
	EncryptionSTARTTLS = "tls"
	// EncryptionTLS opens the connection inside TLS from the first byte. This is port
	// 465.
	EncryptionTLS = "ssl"
)

// smtpTimeout bounds the whole conversation.
//
// A reset request holds an HTTP request open while this runs, so an unreachable relay
// must fail rather than hang: the caller answers "sent" either way and the failure goes
// to the log.
const smtpTimeout = 15 * time.Second

// SMTPSender delivers through a submission server.
type SMTPSender struct {
	host       string
	port       int
	encryption string
	username   string
	password   string
	from       Address
}

// SMTPConfig is what a submission server needs.
type SMTPConfig struct {
	Host string
	Port int
	// Encryption is "tls" for STARTTLS, "ssl" for implicit TLS, or empty for a plain
	// connection. Empty is only sensible for a relay on the same host.
	Encryption string
	// Username empty means the relay accepts unauthenticated submission. Gmail does
	// not; it needs an app password, which is not the account password.
	Username string
	Password string
	From     Address
}

func NewSMTPSender(config SMTPConfig) (*SMTPSender, error) {
	if strings.TrimSpace(config.Host) == "" {
		return nil, fmt.Errorf("mailer: smtp host is required")
	}
	if config.Port <= 0 {
		return nil, fmt.Errorf("mailer: smtp port is required")
	}
	if strings.TrimSpace(config.From.Address) == "" {
		return nil, fmt.Errorf("mailer: from address is required")
	}
	switch config.Encryption {
	case "", EncryptionSTARTTLS, EncryptionTLS:
	default:
		return nil, fmt.Errorf(
			"mailer: unknown encryption %q, expected %q, %q or empty",
			config.Encryption, EncryptionSTARTTLS, EncryptionTLS)
	}
	return &SMTPSender{
		host:       config.Host,
		port:       config.Port,
		encryption: config.Encryption,
		username:   config.Username,
		password:   config.Password,
		from:       config.From,
	}, nil
}

// Send delivers one message.
//
// Written against smtp.Client rather than smtp.SendMail because SendMail cannot be told
// to use implicit TLS and cannot be given a deadline.
func (sender *SMTPSender) Send(ctx context.Context, message Message) error {
	if err := message.validate(); err != nil {
		return err
	}

	address := net.JoinHostPort(sender.host, fmt.Sprint(sender.port))
	dialer := &net.Dialer{Timeout: smtpTimeout}

	var connection net.Conn
	var err error
	if sender.encryption == EncryptionTLS {
		connection, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: sender.host})
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("mailer: connect to %s: %w", address, err)
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(smtpTimeout))
	}

	client, err := smtp.NewClient(connection, sender.host)
	if err != nil {
		return fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	defer client.Close()

	if sender.encryption == EncryptionSTARTTLS {
		// A failed upgrade is a failed send. Carrying on in the clear would put the
		// reset link and the relay password on the wire, which is worse than not
		// sending the mail at all.
		if err := client.StartTLS(&tls.Config{ServerName: sender.host}); err != nil {
			return fmt.Errorf("mailer: starttls: %w", err)
		}
	}

	if sender.username != "" {
		// PlainAuth refuses to send credentials over an unencrypted connection unless
		// the host is localhost, which is the check we want rather than one of our own.
		auth := smtp.PlainAuth("", sender.username, sender.password, sender.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: authenticate: %w", err)
		}
	}

	if err := client.Mail(sender.from.Address); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := writer.Write([]byte(sender.render(message))); err != nil {
		return fmt.Errorf("mailer: write body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: close body: %w", err)
	}
	return client.Quit()
}

// render builds the RFC 5322 message.
//
// The body is base64 rather than 8bit or quoted-printable: the text is Chinese or
// Japanese, so quoted-printable would encode nearly every byte anyway, and base64 in
// fixed-length lines cannot run into the 998-octet line limit or need dot-stuffing.
func (sender *SMTPSender) render(message Message) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(message.TextBody))
	var body strings.Builder
	for index := 0; index < len(encoded); index += 76 {
		end := index + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		body.WriteString(encoded[index:end])
		body.WriteString("\r\n")
	}

	headers := []string{
		"From: " + sender.from.String(),
		"To: " + message.To,
		"Subject: " + mime.QEncoding.Encode("utf-8", message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
		// No Date or Message-ID: the submission server adds both, and a wrong clock
		// here would be worse than not stating one.
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body.String()
}
