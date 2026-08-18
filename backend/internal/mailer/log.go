package mailer

import (
	"context"
	"log/slog"
)

// LogSender writes the message to the log instead of delivering it.
//
// This is how the reset flow is exercised without an SMTP account: there is no mail
// container in either compose file, and the local .env has MAIL_USERNAME and
// MAIL_PASSWORD commented out. The whole body is logged, link included, because reading
// that link out of the log is the point.
//
// IT MUST NOT BE USED IN PRODUCTION. A reset link in the log is a reset link available
// to anyone who can read the log, and no mail reaches the account holder. The
// deployment note says so, and the api logs a warning at startup when this is the
// transport.
type LogSender struct {
	logger *slog.Logger
}

func NewLogSender(logger *slog.Logger) *LogSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogSender{logger: logger}
}

func (sender *LogSender) Send(ctx context.Context, message Message) error {
	if err := message.validate(); err != nil {
		return err
	}
	sender.logger.InfoContext(ctx, "mail_not_sent_logged_instead",
		"to", message.To, "subject", message.Subject, "body", message.TextBody)
	return nil
}
