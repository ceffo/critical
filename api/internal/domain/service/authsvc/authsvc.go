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
	AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID, at time.Time) error
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
// login mode (step 8.1). Now, NewID and NewToken default to the clock, random
// UUIDs and 256-bit tokens from crypto/rand.
type Deps struct {
	DAL      DAL
	Tx       domain.TxRunner
	Provider IdentityProvider
	Now      func() time.Time
	NewID    func() uuid.UUID
	NewToken func() (string, error)
}

type service struct {
	dal      DAL
	tx       domain.TxRunner
	idp      IdentityProvider
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
		dal: d.DAL, tx: d.Tx, idp: d.Provider, now: d.Now, newID: d.NewID, newToken: d.NewToken,
		sessions: &sessionCache{},
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
func (s *service) Close(context.Context) error  { return nil }

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
