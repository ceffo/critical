package mailadpt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

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

// Step 2.5: the bodies name a person "Name (email)" when the name is known,
// the email alone otherwise; the subject uses the name alone. The invite's
// sender name is the inviter's; the accepted notice's is empty (the app's).
func TestNotifier_Step2_5_NamesThePeople(t *testing.T) {
	for _, tc := range []struct {
		name         string
		notice       authsvc.Notice
		subject      string
		inBody       string
		senderName   string
		notInBodyOld string
	}{
		{
			"invite with name",
			authsvc.Notice{Kind: authsvc.NoticeInvite, Language: "en", InviterName: "Moncef", InviterEmail: "m@example.com", InviteeEmail: "c@example.com"},
			"Moncef invited you to Critical Hit", "Moncef (m@example.com) invited you", "Moncef", "",
		},
		{
			"invite without name",
			authsvc.Notice{Kind: authsvc.NoticeInvite, Language: "fr", InviterEmail: "m@example.com", InviteeEmail: "c@example.com"},
			"m@example.com vous invite sur Critical Hit", "m@example.com vous invite", "", "()",
		},
		{
			"accepted with name",
			authsvc.Notice{Kind: authsvc.NoticeAccepted, Language: "fr", InviterName: "Moncef", InviterEmail: "m@example.com", InviteeName: "Cid", InviteeEmail: "c@example.com"},
			"Cid a accepté votre invitation sur Critical Hit", "Cid (c@example.com) a accepté", "", "",
		},
		{
			"accepted without name",
			authsvc.Notice{Kind: authsvc.NoticeAccepted, Language: "fr", InviterEmail: "m@example.com", InviteeEmail: "c@example.com"},
			"c@example.com a accepté votre invitation sur Critical Hit", "c@example.com a accepté", "", "()",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &mailadpt.Recorder{}
			n, err := mailadpt.New(rec, "https://critical-hit.fly.dev")
			require.NoError(t, err)
			tc.notice.To = "to@example.com"
			require.NoError(t, n.Notify(context.Background(), tc.notice))
			sent := rec.Sent()[0]
			assert.Equal(t, tc.subject, sent.Subject)
			assert.Equal(t, tc.senderName, sent.SenderName)
			assert.Contains(t, sent.TextBody, tc.inBody)
			assert.Contains(t, sent.HTMLBody, tc.inBody)
			if tc.notInBodyOld != "" {
				assert.NotContains(t, sent.TextBody, tc.notInBodyOld)
			}
		})
	}
}

// Step 2.5: a hostile name cannot break a header (no control character or
// newline survives, length is capped) nor the HTML body (escaped).
func TestNotifier_Step2_5_HostileNameIsMadeSafe(t *testing.T) {
	hostile := "Eve\r\nBcc: x@evil.test\x00<script>alert(1)</script>\u202e" + strings.Repeat("é", 200)
	rec := &mailadpt.Recorder{}
	n, err := mailadpt.New(rec, "https://critical-hit.fly.dev")
	require.NoError(t, err)
	require.NoError(t, n.Notify(context.Background(), authsvc.Notice{
		Kind: authsvc.NoticeInvite, To: "x@example.com", Language: "en",
		InviterName: hostile, InviterEmail: "e@example.com", InviteeEmail: "x@example.com",
	}))
	sent := rec.Sent()[0]
	for _, field := range []string{sent.SenderName, sent.Subject} {
		assert.NotContains(t, field, "\n")
		assert.NotContains(t, field, "\r")
		assert.NotContains(t, field, "\x00")
		assert.NotContains(t, field, "\u202e")
	}
	assert.LessOrEqual(t, utf8.RuneCountInString(sent.SenderName), mailadpt.MaxNameRunes)
	assert.NotContains(t, sent.HTMLBody, "<script>")
	assert.Contains(t, sent.HTMLBody, "&lt;script&gt;")
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"  Ann   Lee ":          "Ann Lee",
		"a\nb\tc":               "a b c",
		"":                      "",
		"\x07\x1b":              "",
		strings.Repeat("x", 80): strings.Repeat("x", mailadpt.MaxNameRunes),
	} {
		assert.Equal(t, want, mailadpt.CleanName(in))
	}
}

// Step 2.5: the sender name is "<person> via <app>"; the person alone without
// an app name; the app alone without a person.
func TestBrevo_Step2_5_SenderName(t *testing.T) {
	for _, tc := range []struct{ person, app, want string }{
		{"Moncef", "Critical Hit", "Moncef via Critical Hit"},
		{"", "Critical Hit", "Critical Hit"},
		{"Moncef", "", "Moncef"},
		{"", "", ""},
		{"Mon\ncef", "Critical Hit", "Mon cef via Critical Hit"},
	} {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &got)
			w.WriteHeader(http.StatusCreated)
		}))
		b, err := mailadpt.NewBrevo(mailadpt.BrevoConfig{APIKey: "k", From: "hit@example.com", FromName: tc.app, Endpoint: srv.URL})
		require.NoError(t, err)
		require.NoError(t, b.Send(context.Background(), mailadpt.Message{To: "c@example.com", Subject: "S", TextBody: "T", SenderName: tc.person}))
		srv.Close()
		sender := got["sender"].(map[string]any)
		assert.Equal(t, "hit@example.com", sender["email"], "the address never changes")
		if tc.want == "" {
			assert.NotContains(t, sender, "name")
		} else {
			assert.Equal(t, tc.want, sender["name"])
		}
	}
}

// The dev log mailer prints the sender name.
func TestLogMailer_PrintsTheSenderName(t *testing.T) {
	var buf bytes.Buffer
	m := mailadpt.LogMailer{Log: slog.New(slog.NewTextHandler(&buf, nil))}
	require.NoError(t, m.Send(context.Background(), mailadpt.Message{To: "c@example.com", Subject: "S", TextBody: "T", SenderName: "Moncef"}))
	assert.Contains(t, buf.String(), "sender_name=Moncef")
}
