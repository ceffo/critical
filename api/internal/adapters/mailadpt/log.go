package mailadpt

import (
	"context"
	"log/slog"
)

// LogMailer prints every mail to the server log instead of sending it: the
// mailer of a local run (`just dev`, e2e), which never sends a real email.
type LogMailer struct{ Log *slog.Logger }

var _ Mailer = LogMailer{}

// Send implements Mailer.
func (m LogMailer) Send(ctx context.Context, msg Message) error {
	log := m.Log
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "mail (not sent: local run)", "to", msg.To, "sender_name", msg.SenderName, "subject", msg.Subject, "body", msg.TextBody)
	return nil
}
