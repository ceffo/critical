package authsvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// Refusals of ADR 0029. Each has a stable code in internal/render, worded by
// the web (step 3.7).
var (
	// ErrUnauthenticated: no session, or an unknown, expired or disabled
	// user's one (steps 4.4, 6.2).
	ErrUnauthenticated = errors.New("authentication required")
	// ErrSessionExpired: the request carried a session cookie the server no
	// longer knows (expired, logged out elsewhere, user disabled): the login
	// page says the session has ended (step 4.4). It is an ErrUnauthenticated.
	ErrSessionExpired = fmt.Errorf("session expired: %w", ErrUnauthenticated)
	// ErrStateMismatch: the login's state does not match the one it started
	// with, or the login cookie is gone (step 3.1).
	ErrStateMismatch = errors.New("login state mismatch")
	// ErrLoginFailed: the provider refused, or the code exchange or the ID
	// token check failed (step 3.1).
	ErrLoginFailed = errors.New("login failed")
	// ErrEmailUnverified: the ID token's email is not verified (step 3.2).
	ErrEmailUnverified = errors.New("email not verified")
	// ErrNotInvited: a first login with no pending invite (step 3.4).
	ErrNotInvited = errors.New("not invited")
	// ErrUserDisabled: the user is disabled (step 3.5).
	ErrUserDisabled = errors.New("user disabled")
	// ErrCrossOrigin: a state-changing request whose Origin (or Referer)
	// names another host than the application's (step 4.5).
	ErrCrossOrigin = errors.New("cross-origin request refused")
	// ErrRootProtected: the root admin cannot be disabled or demoted (step
	// 5.2).
	ErrRootProtected = fmt.Errorf("the root admin cannot be disabled or demoted: %w", domain.ErrConflict)
	// ErrAdminOnly: a player asked for an admin route (step 1.3).
	ErrAdminOnly = errors.New("only an admin may do this")
	// ErrEmailHasUser: an invite for an email that already has a user (step
	// 2.1).
	ErrEmailHasUser = fmt.Errorf("this email already has a user: %w", domain.ErrConflict)
	// ErrAlreadyInvited: an invite for an email with a pending invite (step
	// 2.1).
	ErrAlreadyInvited = fmt.Errorf("this email already has a pending invite: %w", domain.ErrConflict)
	// ErrInviteNotFound: no pending invite has this id (cancelled, used or
	// unknown).
	ErrInviteNotFound = fmt.Errorf("pending invite: %w", domain.ErrNotFound)
	// ErrInviteNotPending: a resend of an invite that was cancelled or
	// used (step 2.7).
	ErrInviteNotPending = fmt.Errorf("this invite is cancelled or used: %w", domain.ErrConflict)
	// ErrUserNotFound: no user has this id.
	ErrUserNotFound = fmt.Errorf("user: %w", domain.ErrNotFound)
	// ErrRootTaken: another user already is the bound root admin (step 5.1).
	ErrRootTaken = fmt.Errorf("another user is the root admin and has logged in: %w", domain.ErrConflict)
	// ErrDevLoginDisabled: the server has OAuth settings, so the dev login
	// does not exist (step 8.2).
	ErrDevLoginDisabled = fmt.Errorf("dev login is not available: %w", domain.ErrNotFound)
	// ErrGoogleLoginDisabled: the server has no OAuth settings (dev login
	// mode).
	ErrGoogleLoginDisabled = fmt.Errorf("google login is not configured: %w", domain.ErrNotFound)
)
