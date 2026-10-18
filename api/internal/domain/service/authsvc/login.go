package authsvc

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// StartLogin implements SVC (ADR 0029 step 3.1).
func (s *service) StartLogin(context.Context) (*LoginStart, error) {
	if s.idp == nil {
		return nil, ErrGoogleLoginDisabled
	}
	state, err := s.newToken()
	if err != nil {
		return nil, err
	}
	verifier, err := s.newToken()
	if err != nil {
		return nil, err
	}
	return &LoginStart{URL: s.idp.AuthURL(state, verifier), State: state, Verifier: verifier}, nil
}

// FinishLogin implements SVC (ADR 0029 steps 3.1 to 3.6).
func (s *service) FinishLogin(ctx context.Context, in FinishLogin) (*NewSession, error) {
	if s.idp == nil {
		return nil, ErrGoogleLoginDisabled
	}
	// Step 3.1: the state must be the one this browser started with.
	if in.ExpectedState == "" || in.Verifier == "" ||
		subtle.ConstantTimeCompare([]byte(in.State), []byte(in.ExpectedState)) != 1 {
		return nil, ErrStateMismatch
	}
	if in.ProviderError != "" {
		return nil, fmt.Errorf("%w: provider error %q", ErrLoginFailed, in.ProviderError)
	}
	if in.Code == "" {
		return nil, fmt.Errorf("%w: no code", ErrLoginFailed)
	}
	id, err := s.idp.Exchange(ctx, in.Code, in.Verifier)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}
	// Step 3.2: only a verified email.
	if !id.EmailVerified {
		return nil, ErrEmailUnverified
	}
	email := NormalizeEmail(id.Email)
	if id.Subject == "" || email == "" {
		return nil, fmt.Errorf("%w: the ID token has no subject or email", ErrLoginFailed)
	}

	var out *NewSession
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.dal.UserBySubject(ctx, id.Subject)
		switch {
		case err == nil:
			// Step 3.3: the subject is the identity.
			if u.Disabled {
				return ErrUserDisabled
			}
			out, err = s.login(ctx, u, false)
			return err
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		u, inv, err := s.firstLogin(ctx, id.Subject, email)
		if err != nil {
			return err
		}
		if out, err = s.login(ctx, u, inv != nil); err != nil || inv == nil {
			return err
		}
		// Step 2.4: the first login consumes the invite.
		return s.dal.AcceptInvite(ctx, inv.ID, u.ID, *u.LastLogin)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// firstLogin matches an unknown subject (step 3.4): an existing user with the
// email and no subject yet (the root admin of `just root-admin`) gets the
// subject bound; else a pending invite of the email makes a new player (inv
// set: the caller inserts it and accepts the invite). Anything else is
// ErrNotInvited. It writes nothing.
func (s *service) firstLogin(ctx context.Context, subject, email string) (*User, *Invite, error) {
	u, err := s.dal.UserByEmail(ctx, email)
	switch {
	case err == nil:
		if u.GoogleSubject != "" {
			// The email belongs to a user bound to another Google account
			// (step 3.3: a changed email logs nobody else in).
			return nil, nil, ErrNotInvited
		}
		if u.Disabled {
			return nil, nil, ErrUserDisabled
		}
		u.GoogleSubject = subject
		return u, nil, nil
	case !errors.Is(err, domain.ErrNotFound):
		return nil, nil, err
	}
	inv, err := s.dal.PendingInvite(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil, ErrNotInvited
	}
	if err != nil {
		return nil, nil, err
	}
	return &User{ID: s.newID(), Email: email, GoogleSubject: subject, Role: RolePlayer, CreatedAt: s.now()}, inv, nil
}

// DevLogin implements SVC (ADR 0029 step 8.1).
func (s *service) DevLogin(ctx context.Context, email string) (*NewSession, error) {
	if s.idp != nil {
		return nil, ErrDevLoginDisabled
	}
	email = NormalizeEmail(email)
	var out *NewSession
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		u, isNew, err := s.devUser(ctx, email)
		if err != nil {
			return err
		}
		if u.Disabled {
			return ErrUserDisabled
		}
		out, err = s.login(ctx, u, isNew)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// devUser is the dev login's user: the named one (created as an admin when
// unknown), or with no email the root admin (DevEmail made root when there is
// none).
func (s *service) devUser(ctx context.Context, email string) (*User, bool, error) {
	if email == "" {
		root, err := s.dal.RootUser(ctx)
		if err == nil {
			return root, false, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, false, err
		}
		email = DevEmail
	}
	u, err := s.dal.UserByEmail(ctx, email)
	switch {
	case err == nil:
		if email == DevEmail && !u.Root {
			u.Root, u.Role, u.Disabled = true, RoleAdmin, false
		}
		return u, false, nil
	case !errors.Is(err, domain.ErrNotFound):
		return nil, false, err
	}
	u = &User{ID: s.newID(), Email: email, Role: RoleAdmin, Root: email == DevEmail, CreatedAt: s.now()}
	return u, true, nil
}

// login starts a session for u and sets its last login (steps 3.6, 4.1),
// inserting u when isNew, else writing it back (a bound subject, the root
// flag of the dev login, the last login).
func (s *service) login(ctx context.Context, u *User, isNew bool) (*NewSession, error) {
	now := s.now()
	u.LastLogin = &now
	write := s.dal.UpdateUser
	if isNew {
		write = s.dal.InsertUser
	}
	if err := write(ctx, *u); err != nil {
		return nil, err
	}
	token, err := s.newToken()
	if err != nil {
		return nil, err
	}
	expires := now.Add(SessionLifetime)
	if err := s.dal.InsertSession(ctx, Session{
		ID: SessionID(token), UserID: u.ID, CreatedAt: now, LastUsed: now, Expires: expires,
	}); err != nil {
		return nil, err
	}
	return &NewSession{Token: token, Expires: expires, User: *u}, nil
}
