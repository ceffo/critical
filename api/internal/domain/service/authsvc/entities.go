package authsvc

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role is a user's role (ADR 0029 step 1.1).
type Role string

// The roles.
const (
	RoleAdmin  Role = "admin"
	RolePlayer Role = "player"
)

// User is a person who can log in (step 1.1). GoogleSubject is empty until
// the first Google login binds it; LastLogin is nil before any login.
// CurrentCharacterID is the character the user plays (step 7.2), 0 when they
// have none: it is read through the user's ownership (a character they do not
// own, or retired, is none) and never written by UpdateUser.
type User struct {
	ID            uuid.UUID
	Email         string
	GoogleSubject string
	Role          Role
	Root          bool
	Disabled      bool
	LastLogin     *time.Time
	CreatedAt     time.Time
	// CurrentCharacterID is read only: it changes through the characters'
	// own port (charactersvc), which checks the ownership.
	CurrentCharacterID int64
}

// Invite authorises an email to log in through Google (step 2.1).
type Invite struct {
	ID        uuid.UUID
	Email     string
	InvitedBy uuid.UUID
	CreatedAt time.Time
}

// Session is a server-side session row (step 4.1). ID is the hash of the
// cookie's token (SessionID), never the token.
type Session struct {
	ID        string
	UserID    uuid.UUID
	CreatedAt time.Time
	LastUsed  time.Time
	Expires   time.Time
}

// Identity is what a verified Google ID token says (step 3.2).
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// LoginStart is a started Google login: the browser goes to URL; State and
// Verifier are kept (in a short-lived cookie) for FinishLogin.
type LoginStart struct {
	URL      string
	State    string
	Verifier string
}

// FinishLogin is what the provider's redirect brings back (Code, State) and
// what StartLogin kept (ExpectedState, Verifier). ProviderError is the
// provider's error parameter (the user cancelled, for one).
type FinishLogin struct {
	Code          string
	State         string
	ExpectedState string
	Verifier      string
	ProviderError string
}

// NewSession is a started session: Token goes into the cookie, which expires
// at Expires.
type NewSession struct {
	Token   string
	Expires time.Time
	User    User
}

// Principal is the user of an authenticated request. Extended reports that
// this request extended the session to Expires (the cookie is set again).
type Principal struct {
	User     User
	Expires  time.Time
	Extended bool
}

// SessionID is the stored id of a session token: its SHA-256, in hex. The
// database never holds a token a stolen copy could use.
func SessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NormalizeEmail is the form emails are stored and compared in (step 1.1):
// trimmed and lower-cased.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
