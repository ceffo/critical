package mailadpt_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/mailadpt"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// Step 2.5: each mail is worded from the embedded template of its language,
// carries the inviter's name, the invitee and the app link, and nothing else
// (no token).
func TestNotifier_Step2_5_WordsTheMailsPerLanguage(t *testing.T) {
	for _, tc := range []struct {
		kind    authsvc.NoticeKind
		lang    authsvc.Language
		subject string
		body    string
		button  string
	}{
		{authsvc.NoticeInvite, "en", "ann@example.com invited you to Critical Hit", "Sign in with the Google account", "Open Critical Hit"},
		{authsvc.NoticeInvite, "fr", "ann@example.com vous invite sur Critical Hit", "Connectez-vous avec le compte Google", "Ouvrir Critical Hit"},
		{authsvc.NoticeAccepted, "en", "cid@example.com accepted your invitation to Critical Hit", "signed in to Critical Hit for the first time", "Open Critical Hit"},
		{authsvc.NoticeAccepted, "fr", "cid@example.com a accepté votre invitation sur Critical Hit", "pour la première fois", "Ouvrir Critical Hit"},
	} {
		t.Run(string(tc.kind)+" "+string(tc.lang), func(t *testing.T) {
			rec := &mailadpt.Recorder{}
			n, err := mailadpt.New(rec, "https://critical-hit.fly.dev")
			require.NoError(t, err)
			require.NoError(t, n.Notify(context.Background(), authsvc.Notice{
				Kind: tc.kind, To: "to@example.com", Language: tc.lang,
				InviterEmail: "ann@example.com", InviteeEmail: "cid@example.com",
			}))
			sent := rec.Sent()
			require.Len(t, sent, 1)
			assert.Equal(t, "to@example.com", sent[0].To)
			assert.Equal(t, tc.subject, sent[0].Subject)
			for _, body := range []string{sent[0].TextBody, sent[0].HTMLBody} {
				assert.Contains(t, body, "https://critical-hit.fly.dev")
				assert.Contains(t, body, "cid@example.com")
			}
			assert.Contains(t, sent[0].TextBody, tc.body)
			assert.Contains(t, sent[0].HTMLBody, tc.body)
			// The branded layout: the title header, the button linking to the
			// application's address, inline styles only, nothing remote.
			html := sent[0].HTMLBody
			assert.Contains(t, html, "Critical Hit</td>")
			assert.Contains(t, html, `<a href="https://critical-hit.fly.dev" style=`)
			assert.Contains(t, html, ">"+tc.button+"</a>")
			assert.Contains(t, sent[0].TextBody, tc.button)
			assert.NotContains(t, html, "<img")
			assert.NotContains(t, html, "<style")
			assert.NotContains(t, html, "<link")
			assert.NotContains(t, html, "src=")
			assert.NotContains(t, html, "url(")
		})
	}
}

// Step 2.5: the HTML body escapes what it is given.
func TestNotifier_Step2_5_HTMLIsEscaped(t *testing.T) {
	rec := &mailadpt.Recorder{}
	n, err := mailadpt.New(rec, "https://critical-hit.fly.dev")
	require.NoError(t, err)
	require.NoError(t, n.Notify(context.Background(), authsvc.Notice{
		Kind: authsvc.NoticeInvite, To: "x@example.com", Language: "en", InviterEmail: `a"<b>@example.com`, InviteeEmail: "x@example.com",
	}))
	assert.NotContains(t, rec.Sent()[0].HTMLBody, "<b>")
}

// Step 2.10: a notifier without mail settings answers ErrMailNotConfigured.
func TestUnconfigured_Step2_10(t *testing.T) {
	err := mailadpt.Unconfigured{}.Notify(context.Background(), authsvc.Notice{})
	require.ErrorIs(t, err, authsvc.ErrMailNotConfigured)
}

// Step 2.10: the local mailer prints the mail to the log and sends nothing.
func TestLogMailer_Step2_10_PrintsTheMail(t *testing.T) {
	var out strings.Builder
	m := mailadpt.LogMailer{Log: slog.New(slog.NewTextHandler(&out, nil))}
	require.NoError(t, m.Send(context.Background(), mailadpt.Message{To: "x@example.com", Subject: "S", TextBody: "B"}))
	assert.Contains(t, out.String(), "x@example.com")
}

// Step 2.10: the Brevo request: POST, the api-key header, JSON with the sender,
// the recipient, the subject and both bodies. No test calls the real service.
func TestBrevo_Step2_10_RequestShape(t *testing.T) {
	var gotKey, gotMethod, gotType string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotMethod, gotType = r.Header.Get("api-key"), r.Method, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"<1@brevo>"}`))
	}))
	defer srv.Close()
	b, err := mailadpt.NewBrevo(mailadpt.BrevoConfig{APIKey: "k-123", From: "hit@example.com", FromName: "Critical Hit", Endpoint: srv.URL})
	require.NoError(t, err)

	require.NoError(t, b.Send(context.Background(), mailadpt.Message{To: "cid@example.com", Subject: "S", TextBody: "T", HTMLBody: "<p>H</p>"}))
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "k-123", gotKey)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, map[string]any{
		"sender":      map[string]any{"name": "Critical Hit", "email": "hit@example.com"},
		"to":          []any{map[string]any{"email": "cid@example.com"}},
		"subject":     "S",
		"textContent": "T",
		"htmlContent": "<p>H</p>",
	}, got)
}

// Step 2.10: a non-2xx answer is an error carrying Brevo's message but never
// the key; an unreachable server is an error; a cancelled context stops the
// call.
func TestBrevo_Step2_10_ErrorMapping(t *testing.T) {
	status := http.StatusUnauthorized
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"code":"unauthorized","message":"Key not found"}`))
	}))
	b, err := mailadpt.NewBrevo(mailadpt.BrevoConfig{APIKey: "k-secret", From: "hit@example.com", Endpoint: srv.URL})
	require.NoError(t, err)
	msg := mailadpt.Message{To: "cid@example.com", Subject: "S", TextBody: "T"}

	err = b.Send(context.Background(), msg)
	require.ErrorIs(t, err, mailadpt.ErrBrevo)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "Key not found")
	assert.NotContains(t, err.Error(), "k-secret")

	status = http.StatusInternalServerError
	require.ErrorIs(t, b.Send(context.Background(), msg), mailadpt.ErrBrevo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, b.Send(ctx, msg))

	srv.Close()
	require.ErrorIs(t, b.Send(context.Background(), msg), mailadpt.ErrBrevo, "an unreachable server")
}

func TestBrevo_RequiresKeyAndSender(t *testing.T) {
	_, err := mailadpt.NewBrevo(mailadpt.BrevoConfig{From: "a@example.com"})
	require.Error(t, err)
	_, err = mailadpt.NewBrevo(mailadpt.BrevoConfig{APIKey: "k"})
	require.Error(t, err)
}

// The recorder returns its Err and still records.
func TestRecorder(t *testing.T) {
	r := &mailadpt.Recorder{Err: errors.New("boom")}
	require.Error(t, r.Send(context.Background(), mailadpt.Message{To: "a"}))
	assert.Len(t, r.Sent(), 1)
}
