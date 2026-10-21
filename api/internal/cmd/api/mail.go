package api

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ceffo/critical/api/internal/adapters/mailadpt"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// devAppURL is the address the mails of a local run link to when
// CRITICAL_PUBLIC_URL is not set: Vite, which serves the web app in
// `just dev`.
const devAppURL = "http://localhost:5173"

// validateMail refuses a half-set mail configuration (ADR 0029 step 2.10): the
// API key and the sender go together, and the sender's name needs them. No
// mail setting at all is fine: nothing is sent and the server starts.
func (c Config) validateMail() error {
	switch {
	case c.BrevoAPIKey == "" && c.MailFrom == "" && c.MailFromName == "":
		return nil
	case c.BrevoAPIKey == "" || c.MailFrom == "":
		return errors.New("CRITICAL_BREVO_API_KEY and CRITICAL_MAIL_FROM must be set together (CRITICAL_MAIL_FROM_NAME is optional); set none to send no mail")
	}
	return nil
}

// notifier is the notifier the settings select (ADR 0029 step 2.10): in a local
// run (CRITICAL_ENV=dev) the mail is printed in the server log and never
// sent, even when Brevo is configured; elsewhere Brevo when configured, else
// a notifier that sends nothing (the outcome is not_configured).
func (c Config) notifier(log *slog.Logger) (authsvc.Notifier, error) {
	appURL := strings.TrimSuffix(c.PublicURL, "/")
	if c.Env == EnvDev {
		if appURL == "" {
			appURL = devAppURL
		}
		if c.BrevoAPIKey != "" {
			log.Warn("mail: CRITICAL_ENV=dev prints mails in the log; the Brevo settings are ignored")
		}
		log.Info("mail: printed in the server log (CRITICAL_ENV=dev)")
		return mailadpt.New(mailadpt.LogMailer{Log: log}, appURL)
	}
	if c.BrevoAPIKey == "" {
		log.Info("mail: not configured, invite mails are not sent")
		return mailadpt.Unconfigured{}, nil
	}
	b, err := mailadpt.NewBrevo(mailadpt.BrevoConfig{APIKey: c.BrevoAPIKey, From: c.MailFrom, FromName: c.MailFromName})
	if err != nil {
		return nil, fmt.Errorf("mail: %w", err)
	}
	log.Info("mail: Brevo", "from", c.MailFrom)
	return mailadpt.New(b, appURL)
}
