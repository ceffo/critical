// Package authsvc owns users, sessions and login (ADR 0029): the Google
// OpenID Connect login with its invites (step 3), the server-side sessions
// (step 4), the root admin (step 5) and the dev login of a server without
// OAuth settings (step 8). It never reads a cookie or a header: the transport
// passes tokens, states and codes in, and gets tokens and users back.
package authsvc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// SessionLifetime is how long a session lasts from its last extension (ADR
// 0029 step 4.3).
const SessionLifetime = 30 * 24 * time.Hour

// ExtendAfter is how old the last extension of a session must be before a
// request extends it again (step 4.3): at most one write a day per session.
const ExtendAfter = 24 * time.Hour

// LoginStateLifetime is how long a started Google login may take (step 3.1):
// the state and the PKCE verifier live this long.
const LoginStateLifetime = 10 * time.Minute

// DevEmail is the root admin the dev login creates when it is asked for no
// email and the database has no root admin yet (step 8.1).
const DevEmail = "dev@localhost"

// SVC is the driver port.
type SVC interface {
	domain.Service
	// DevLoginEnabled reports whether the server runs without OAuth settings,
	// so that the dev login exists (step 8.1).
	DevLoginEnabled() bool
	// StartLogin begins a Google login (step 3.1): a fresh state and PKCE
	// verifier, and the provider URL to send the browser to. Without OAuth
	// settings it wraps domain.ErrNotFound.
	StartLogin(ctx context.Context) (*LoginStart, error)
	// FinishLogin completes a Google login (steps 3.1 to 3.6): it checks the
	// state, exchanges the code, then logs in the user bound to the token's
	// subject, or binds a first login to its pending invite (or to a user
	// with that email and no subject yet). Refusals are ErrStateMismatch,
	// ErrLoginFailed, ErrEmailUnverified, ErrNotInvited and ErrUserDisabled;
	// a refusal writes nothing.
	FinishLogin(ctx context.Context, in FinishLogin) (*NewSession, error)
	// DevLogin starts a session without Google (step 8.1), only when
	// DevLoginEnabled (else it wraps domain.ErrNotFound). An email names the
	// user, created as an admin when unknown; no email is the root admin,
	// created as DevEmail when there is none.
	DevLogin(ctx context.Context, email string) (*NewSession, error)
	// Authenticate resolves a session token to its user (step 4): an
	// unknown, expired or disabled user's session is ErrUnauthenticated (the
	// expired one is deleted, and every session of a disabled user). A
	// session last extended ExtendAfter ago or more is extended.
	Authenticate(ctx context.Context, token string) (*Principal, error)
	// Logout deletes the token's session (step 4.4); an unknown token is no
	// error.
	Logout(ctx context.Context, token string) error
	// SetDisabled disables (deleting every session of the user, in the same
	// transaction) or re-enables a user (steps 1.2, 4.4). The root admin
	// cannot be disabled: ErrRootProtected.
	SetDisabled(ctx context.Context, userID uuid.UUID, disabled bool) error
	// SetRole makes a user an admin or a player (step 1.3). The root admin
	// cannot be demoted: ErrRootProtected.
	SetRole(ctx context.Context, userID uuid.UUID, role Role) error
	// ListUsers is every user with the names of their characters (step 1.3),
	// in creation order.
	ListUsers(ctx context.Context) ([]UserSummary, error)
	// ListInvites is the pending invites (neither cancelled nor used), oldest
	// first.
	ListInvites(ctx context.Context) ([]Invite, error)
	// CreateInvite authorises email to log in through Google (step 2.1),
	// invited by the user by, then mails the invitee in lang (empty:
	// DefaultLanguage; steps 2.6, 2.8). An email that is not one, or a
	// language that is not en or fr, is a field error; an email that has a
	// user is ErrEmailHasUser, one with a pending invite ErrAlreadyInvited.
	// The invite exists whatever the MailOutcome.
	CreateInvite(ctx context.Context, by uuid.UUID, email string, lang Language) (*Invite, MailOutcome, error)
	// ResendInvite mails the invitee of a pending invite again (step 2.7).
	// An unknown invite is ErrInviteNotFound, a cancelled or used one
	// ErrInviteNotPending.
	ResendInvite(ctx context.Context, id uuid.UUID) (*Invite, MailOutcome, error)
	// CancelInvite cancels a pending invite (step 2.3); one that is not
	// pending (cancelled, used, unknown) is ErrInviteNotFound.
	CancelInvite(ctx context.Context, id uuid.UUID) error
	// ChangeUser applies an admin's change to a user in one transaction
	// (steps 1.2, 1.3, 4.4, 5.2): disabling deletes every session of the
	// user; the root admin can be neither disabled nor demoted
	// (ErrRootProtected). An unknown user is ErrUserNotFound. It returns the
	// user as ListUsers shows them.
	ChangeUser(ctx context.Context, id uuid.UUID, change UserChange) (*UserSummary, error)
	// SetRootAdmin makes the user of email the root admin (step 5.1),
	// creating it as an admin without a Google subject when unknown. It is
	// idempotent; with another email than the current root's it refuses
	// (ErrRootTaken) unless that root never logged in (no subject), which is
	// then disabled and loses the flag.
	SetRootAdmin(ctx context.Context, email string) (*User, error)
	// AdoptUnownedCharacters gives every character without an owner to the
	// root admin, and makes the first of them current when the root admin has
	// none, or one they no longer own or have retired (ADR 0029 steps 5.3
	// and 7.2): the start-up runs it, so the
	// characters stored before characters had owners are the root admin's.
	// Without a root admin it does nothing. SetRootAdmin and the dev login do
	// the same for the root admin they make.
	AdoptUnownedCharacters(ctx context.Context) error
	// ForgetUser drops what this server remembers of the user's sessions
	// (step 4.7), so its next request reads the user again: the user's
	// current character changed.
	ForgetUser(userID uuid.UUID)
	// FreshCurrentCharacter is p's user's current character as the database
	// holds it now (step 7.2; 0: none), not as this server remembers it. When
	// it is not p's, this server forgets the user's sessions, so their next
	// request reads the user again.
	FreshCurrentCharacter(ctx context.Context, p *Principal) (int64, error)
}

// DAL is the storage port over users, invites and sessions. A missing row is
// domain.ErrNotFound. Every method joins the ctx transaction.
type DAL interface {
	UserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UserBySubject(ctx context.Context, subject string) (*User, error)
	// UserByEmail takes a lower-cased email.
	UserByEmail(ctx context.Context, email string) (*User, error)
	RootUser(ctx context.Context) (*User, error)
	InsertUser(ctx context.Context, u User) error
	// UpdateUser writes every mutable field of u (subject, role, root,
	// disabled, last login) to the user of u.ID.
	UpdateUser(ctx context.Context, u User) error
	// PendingInvite is the invite of a lower-cased email that is neither
	// cancelled nor accepted.
	PendingInvite(ctx context.Context, email string) (*Invite, error)
	// InviteByID is any invite, with Cancelled and Accepted set.
	InviteByID(ctx context.Context, id uuid.UUID) (*Invite, error)
	AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID, at time.Time) error
	// ListUsers is every user (CurrentCharacterID not read) with the names
	// of their characters not retired, both in creation order.
	ListUsers(ctx context.Context) ([]UserSummary, error)
	// ListPendingInvites is the invites neither cancelled nor accepted, in
	// creation order.
	ListPendingInvites(ctx context.Context) ([]Invite, error)
	// InsertInvite stores a pending invite (a lower-cased email).
	InsertInvite(ctx context.Context, inv Invite) error
	// CancelInvite cancels the pending invite id at at; an invite that is not
	// pending is domain.ErrNotFound.
	CancelInvite(ctx context.Context, id uuid.UUID, at time.Time) error
	InsertSession(ctx context.Context, s Session) error
	SessionByID(ctx context.Context, id string) (*Session, error)
	ExtendSession(ctx context.Context, id string, lastUsed, expires time.Time) error
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
	// AdoptUnownedCharacters gives every character without an owner to the
	// user, then makes the user's first character (lowest id, not retired)
	// current when the user has no current character: none set, or one the
	// user no longer owns or has retired.
	AdoptUnownedCharacters(ctx context.Context, userID uuid.UUID) error
	// MoveCharacters gives every character of from to to, and leaves from
	// without a current character.
	MoveCharacters(ctx context.Context, from, to uuid.UUID) error
}

// IdentityProvider is Google's OpenID Connect (step 3.1): the authorisation
// URL of a login, and the exchange of its code for a verified ID token.
type IdentityProvider interface {
	// AuthURL is the provider URL a login starts at, carrying state and the
	// S256 challenge of verifier.
	AuthURL(state, verifier string) string
	// Exchange trades the code (with the PKCE verifier) for an ID token and
	// verifies it: signature, issuer, audience, expiry. Any failure is an
	// error.
	Exchange(ctx context.Context, code, verifier string) (*Identity, error)
}

// Deps are the service's collaborators. Provider nil runs the server in dev
// login mode (step 8.1). Notifier nil sends no mail (MailNotConfigured). Now, NewID and NewToken default to the clock, random
// UUIDs and 256-bit tokens from crypto/rand.
type Deps struct {
	DAL      DAL
	Tx       domain.TxRunner
	Provider IdentityProvider
	Notifier Notifier
	// MailTimeout defaults to the MailTimeout constant.
	MailTimeout time.Duration
	Now         func() time.Time
	NewID       func() uuid.UUID
	NewToken    func() (string, error)
}

type service struct {
	dal         DAL
	tx          domain.TxRunner
	idp         IdentityProvider
	notifier    Notifier
	mailTimeout time.Duration
	// mails counts the background mails (step 2.9) Close waits for.
	mails    sync.WaitGroup
	now      func() time.Time
	newID    func() uuid.UUID
	newToken func() (string, error)
	// sessions are the sessions this server found valid recently.
	sessions *sessionCache
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Tx == nil {
		return nil, fmt.Errorf("authsvc: nil dependency")
	}
	s := &service{
		dal: d.DAL, tx: d.Tx, idp: d.Provider, notifier: d.Notifier, mailTimeout: d.MailTimeout, now: d.Now, newID: d.NewID, newToken: d.NewToken,
		sessions: &sessionCache{},
	}
	if s.mailTimeout == 0 {
		s.mailTimeout = MailTimeout
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.newID == nil {
		s.newID = uuid.New
	}
	if s.newToken == nil {
		s.newToken = RandomToken
	}
	return s, nil
}

func (s *service) Health(context.Context) error { return nil }

// Close waits for the mails still being sent (step 2.9).
func (s *service) Close(context.Context) error {
	s.mails.Wait()
	return nil
}

// Module declares no codec: the login is no game move.
func (s *service) Module() domain.Module { return domain.Module{Name: "authsvc"} }

func (s *service) DevLoginEnabled() bool { return s.idp == nil }

// RandomToken is 256 bits from the system's secure generator, base64url
// without padding (43 characters): a session token, a login state or a PKCE
// verifier (ADR 0029 steps 3.1 and 4.1).
func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
