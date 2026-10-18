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
	// ErrRootTaken: another user already is the bound root admin (step 5.1).
	ErrRootTaken = fmt.Errorf("another user is the root admin and has logged in: %w", domain.ErrConflict)
	// ErrDevLoginDisabled: the server has OAuth settings, so the dev login
	// does not exist (step 8.2).
	ErrDevLoginDisabled = fmt.Errorf("dev login is not available: %w", domain.ErrNotFound)
	// ErrGoogleLoginDisabled: the server has no OAuth settings (dev login
	// mode).
	ErrGoogleLoginDisabled = fmt.Errorf("google login is not configured: %w", domain.ErrNotFound)
)
