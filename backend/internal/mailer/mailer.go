// Package mailer sends the API's outbound e-mail.
//
// There is exactly one message in the whole system — the password reset link — which is
// why this package is a Sender interface and two implementations rather than a template
// engine with a queue behind it. Everything Laravel sent by mail was that one
// notification; nothing else in the product mails anybody.
//
// Standard library only. net/smtp covers a submission server that speaks STARTTLS or
// implicit TLS with a username and password, which is what Gmail's smtp.gmail.com:587
// and every other hosted relay in reach of this deployment need.
package mailer

import (
	"context"
	"fmt"
	"mime"
	"net/mail"
	"strings"
)

// Message is one outbound e-mail.
//
// Plain text only. The reset mail is a sentence and a link, and an HTML alternative
// would add a second body to keep in step with the first for no gain the reader can see.
type Message struct {
	To       string
	Subject  string
	TextBody string
}

// Sender delivers a message.
//
// The interface exists because the transport is a deployment choice, not a code one:
// LogSender is how a developer without an SMTP account reads the reset link, and
// SMTPSender is what production uses. Callers hold the interface so neither of them is
// special.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// Address is the envelope sender.
type Address struct {
	// Address is the mailbox, e.g. 2pick.app@gmail.com.
	Address string
	// Name is the display name, which is usually not ASCII here — the application is
	// called 殘酷二選一 — so it is RFC 2047 encoded on the way out.
	Name string
}

// String renders the From header value with the display name encoded.
func (address Address) String() string {
	if address.Name == "" {
		return address.Address
	}
	// mime.QEncoding leaves ASCII alone and encodes anything else, so a name that
	// happens to be ASCII is not needlessly wrapped in =?utf-8?q?…?=.
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", address.Name), address.Address)
}

// validate rejects a message that cannot be delivered before any connection is opened.
//
// The recipient in particular: it arrives from a form, and a header injection through a
// newline in an address is the classic way a mail form becomes an open relay.
func (message Message) validate() error {
	recipient, err := mail.ParseAddress(strings.TrimSpace(message.To))
	if err != nil {
		return fmt.Errorf("mailer: recipient address: %w", err)
	}
	if recipient.Address != strings.TrimSpace(message.To) {
		// ParseAddress accepts "Name <addr>" too. Callers here pass a bare address, and
		// anything else is a sign the value came from somewhere it should not have.
		return fmt.Errorf("mailer: recipient must be a bare address")
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return fmt.Errorf("mailer: subject must not contain a newline")
	}
	if strings.TrimSpace(message.TextBody) == "" {
		return fmt.Errorf("mailer: body is empty")
	}
	return nil
}
