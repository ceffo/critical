package authsvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// Language is the language an invite's mails are written in (ADR 0029 step
// 2.5): a language code stored on the invite, never a column per language
// (ADR 0023).
type Language string

// The languages of the mails.
const (
	LanguageEnglish Language = "en"
	LanguageFrench  Language = "fr"
)

// DefaultLanguage is the language of an invite created without one (ADR 0029
// step 2.6): the admin screen of releases before the language choice sends
// none.
const DefaultLanguage = LanguageEnglish

// checkLanguage is lang, or DefaultLanguage when empty; any other code is a
// field error (step 2.6).
func checkLanguage(lang Language) (Language, error) {
	switch lang {
	case "":
		return DefaultLanguage, nil
	case LanguageEnglish, LanguageFrench:
		return lang, nil
	}
	return "", domain.FieldErrorf("language", "language must be %q or %q", LanguageEnglish, LanguageFrench)
}

// MailOutcome is what became of the mail of an invite (ADR 0029 step 2.8).
// The web words it; the invite exists whichever it is.
type MailOutcome string

// The outcomes.
const (
	// MailSent: the mail provider accepted the mail.
	MailSent MailOutcome = "sent"
	// MailFailed: the send failed or timed out. Nothing is retried.
	MailFailed MailOutcome = "failed"
	// MailNotConfigured: the server has no mail settings, so nothing was
	// sent.
	MailNotConfigured MailOutcome = "not_configured"
)

// MailTimeout is how long the server waits for a mail to be sent (step 2.8):
// short, as nothing is queued or retried.
const MailTimeout = 8 * time.Second

// NoticeKind names a mail.
type NoticeKind string

// The mails (steps 2.6, 2.7 and 2.9). A resend is the same mail as the
// first one.
const (
	// NoticeInvite goes to the invitee: the app's address, and who invited.
	NoticeInvite NoticeKind = "invite"
	// NoticeAccepted goes to the inviter: the invitee logged in.
	NoticeAccepted NoticeKind = "accepted"
)

// Notice is a mail as the domain knows it: structured data, no wording. The
// notifier adapter words it in Language and adds the application's address
// (ADR 0029 step 2.5).
type Notice struct {
	Kind NoticeKind
	// To is the recipient: the invitee, or for NoticeAccepted the inviter.
	To       string
	Language Language
	// InviterEmail is who invited, InviterName their display name (empty
	// when unknown). The mails name the inviter by both.
	InviterEmail, InviterName string
	// InviteeEmail is the invited address, InviteeName the display name of
	// the invitee (empty when unknown: only the accepted notice has one, as
	// the invitee has just logged in).
	InviteeEmail, InviteeName string
}

// ErrMailNotConfigured is what a notifier without mail settings returns: the
// outcome is MailNotConfigured, not a failure (step 2.8).
var ErrMailNotConfigured = errors.New("mail is not configured")

// Notifier sends the mails of the invites (ADR 0029 step 2.5). The adapter
// words and sends them; the service only says which, to whom, in which
// language.
type Notifier interface {
	Notify(ctx context.Context, n Notice) error
}

// send gives the outcome of notifying n: the caller's invite exists already,
// so nothing here returns an error (step 2.8).
func (s *service) send(ctx context.Context, n Notice) MailOutcome {
	if s.notifier == nil {
		return MailNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, s.mailTimeout)
	defer cancel()
	err := s.notifier.Notify(ctx, n)
	switch {
	case err == nil:
		return MailSent
	case errors.Is(err, ErrMailNotConfigured):
		return MailNotConfigured
	}
	slog.Warn("invite mail not sent", "kind", n.Kind, "error", err)
	return MailFailed
}

// notifyAccepted tells the inviter that the invite was accepted (step 2.9),
// in the background: a slow or failing mail never holds up a login. Close
// waits for it.
func (s *service) notifyAccepted(n Notice) {
	if s.notifier == nil {
		return
	}
	s.mailMu.Lock()
	if s.closed {
		s.mailMu.Unlock()
		return
	}
	s.mails.Add(1)
	s.mailMu.Unlock()
	go func() {
		defer s.mails.Done()
		s.send(context.Background(), n)
	}()
}

// ResendInvite implements SVC (ADR 0029 step 2.7).
func (s *service) ResendInvite(ctx context.Context, id uuid.UUID) (*Invite, MailOutcome, error) {
	inv, err := s.dal.InviteByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, "", ErrInviteNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if inv.Cancelled || inv.Accepted {
		return nil, "", ErrInviteNotPending
	}
	return inv, s.sendInvite(ctx, *inv), nil
}

// sendInvite mails the invitee of inv, after the transaction that created it
// committed (or for a resend, with none): a failure to find the inviter is a
// failed mail, not an error.
func (s *service) sendInvite(ctx context.Context, inv Invite) MailOutcome {
	if s.notifier == nil {
		return MailNotConfigured
	}
	inviter, err := s.dal.UserByID(ctx, inv.InvitedBy)
	if err != nil {
		slog.Warn("invite mail not sent: inviter unreadable", "error", err)
		return MailFailed
	}
	return s.send(ctx, Notice{
		Kind: NoticeInvite, To: inv.Email, Language: inv.Language,
		InviterEmail: inviter.Email, InviterName: inviter.DisplayName, InviteeEmail: inv.Email,
	})
}
