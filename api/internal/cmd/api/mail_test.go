package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/mailadpt"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// ADR 0029 step 2.10: no mail setting is a valid configuration (the server
// starts and sends nothing); a half-set one is refused; the key and the
// sender go together.
func TestConfigValidate_Step2_10_MailSettingsAreOptional(t *testing.T) {
	base := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c", Env: EnvDev}
	cases := []struct {
		name string
		f    func(*Config)
		ok   bool
	}{
		{"none", func(*Config) {}, true},
		{"key and sender", func(c *Config) { c.BrevoAPIKey, c.MailFrom = "k", "a@example.com" }, true},
		{"key, sender and name", func(c *Config) { c.BrevoAPIKey, c.MailFrom, c.MailFromName = "k", "a@example.com", "Hit" }, true},
		{"key alone", func(c *Config) { c.BrevoAPIKey = "k" }, false},
		{"sender alone", func(c *Config) { c.MailFrom = "a@example.com" }, false},
		{"name alone", func(c *Config) { c.MailFromName = "Hit" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.f(&c)
			if err := c.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

// Step 2.10: which notifier the settings select. A local run (CRITICAL_ENV=dev)
// never sends a real mail, even with Brevo settings; elsewhere Brevo when
// configured, else nothing is sent (not_configured).
func TestConfigNotifier_Step2_10(t *testing.T) {
	ctx := context.Background()
	notice := authsvc.Notice{Kind: authsvc.NoticeInvite, To: "x@example.com", Language: "en", InviterEmail: "a@example.com", InviteeEmail: "x@example.com"}

	t.Run("not configured", func(t *testing.T) {
		n, err := Config{Env: EnvProduction}.notifier(quiet)
		if err != nil {
			t.Fatal(err)
		}
		if err := n.Notify(ctx, notice); !errors.Is(err, authsvc.ErrMailNotConfigured) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("dev logs the mail, not configured or not", func(t *testing.T) {
		for _, c := range []Config{{Env: EnvDev}, {Env: EnvDev, BrevoAPIKey: "k", MailFrom: "a@example.com"}} {
			n, err := c.notifier(quiet)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := n.(*mailadpt.Notifier); !ok {
				t.Fatalf("got %T", n)
			}
			if err := n.Notify(ctx, notice); err != nil {
				t.Fatalf("the log mailer sends nothing and never fails: %v", err)
			}
		}
	})
	t.Run("brevo", func(t *testing.T) {
		n, err := Config{Env: EnvProduction, BrevoAPIKey: "k", MailFrom: "a@example.com", PublicURL: "https://critical-hit.fly.dev"}.notifier(quiet)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := n.(*mailadpt.Notifier); !ok {
			t.Fatalf("got %T", n)
		}
	})
}

// Step 2.10: the settings are read from the environment names the deployment
// doc gives.
func TestLoadConfig_Step2_10_MailSettingsFromEnv(t *testing.T) {
	devEnv(t)
	t.Setenv("CRITICAL_BREVO_API_KEY", "k")
	t.Setenv("CRITICAL_MAIL_FROM", "hit@example.com")
	t.Setenv("CRITICAL_MAIL_FROM_NAME", "Critical Hit")
	cfg, err := loadConfig()
	if err != nil || cfg.BrevoAPIKey != "k" || cfg.MailFrom != "hit@example.com" || cfg.MailFromName != "Critical Hit" {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}
