package authsvc

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// SetDisabled implements SVC (ADR 0029 steps 1.2, 4.4 and 5.2).
func (s *service) SetDisabled(ctx context.Context, userID uuid.UUID, disabled bool) error {
	defer s.sessions.forgetUser(userID)
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		return s.changeUser(ctx, userID, UserChange{Disabled: &disabled})
	})
}

// SetRole implements SVC (ADR 0029 steps 1.3 and 5.2).
func (s *service) SetRole(ctx context.Context, userID uuid.UUID, role Role) error {
	if err := checkRole(role); err != nil {
		return err
	}
	defer s.sessions.forgetUser(userID)
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		return s.changeUser(ctx, userID, UserChange{Role: &role})
	})
}

func checkRole(role Role) error {
	if role != RoleAdmin && role != RolePlayer {
		return domain.FieldErrorf("role", "role must be %q or %q", RoleAdmin, RolePlayer)
	}
	return nil
}

// ChangeUser implements SVC (ADR 0029 steps 1.2, 1.3, 4.4 and 5.2). This
// server forgets the user's sessions once the change committed (step 4.7);
// the other server refuses them, or sees the new role, within
// SessionCacheTTL.
func (s *service) ChangeUser(ctx context.Context, id uuid.UUID, change UserChange) (*UserSummary, error) {
	if change.Role != nil {
		if err := checkRole(*change.Role); err != nil {
			return nil, err
		}
	}
	defer s.sessions.forgetUser(id)
	var out *UserSummary
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.changeUser(ctx, id, change); err != nil {
			return err
		}
		users, err := s.dal.ListUsers(ctx)
		if err != nil {
			return err
		}
		for i := range users {
			if users[i].ID == id {
				out = &users[i]
				return nil
			}
		}
		return ErrUserNotFound
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// changeUser applies change to the user id in the ctx transaction: both
// fields are checked before anything is written, so a refused change writes
// nothing.
func (s *service) changeUser(ctx context.Context, id uuid.UUID, change UserChange) error {
	u, err := s.dal.UserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	disable := change.Disabled != nil && *change.Disabled && !u.Disabled
	// Step 5.2: the root admin stays an enabled admin.
	if u.Root && ((change.Disabled != nil && *change.Disabled) || (change.Role != nil && *change.Role != RoleAdmin)) {
		return ErrRootProtected
	}
	next := *u
	if change.Disabled != nil {
		next.Disabled = *change.Disabled
	}
	if change.Role != nil {
		next.Role = *change.Role
	}
	if next.Disabled == u.Disabled && next.Role == u.Role {
		return nil
	}
	if err := s.dal.UpdateUser(ctx, next); err != nil {
		return err
	}
	if !disable {
		return nil
	}
	// Step 4.4: in the same transaction, so the next request is refused.
	return s.dal.DeleteUserSessions(ctx, id)
}

// ListUsers implements SVC (ADR 0029 step 1.3).
func (s *service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	return s.dal.ListUsers(ctx)
}

// ListInvites implements SVC (ADR 0029 step 2).
func (s *service) ListInvites(ctx context.Context) ([]Invite, error) {
	return s.dal.ListPendingInvites(ctx)
}

// CreateInvite implements SVC (ADR 0029 step 2.1). The checks and the
// insert are one transaction (the writer lock serialises them), and the
// schema allows one pending invite per email besides.
func (s *service) CreateInvite(ctx context.Context, by uuid.UUID, email string) (*Invite, error) {
	email, err := checkEmail(email)
	if err != nil {
		return nil, err
	}
	inv := Invite{ID: s.newID(), Email: email, InvitedBy: by, CreatedAt: s.now()}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		_, err := s.dal.UserByEmail(ctx, email)
		switch {
		case err == nil:
			return ErrEmailHasUser
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		_, err = s.dal.PendingInvite(ctx, email)
		switch {
		case err == nil:
			return ErrAlreadyInvited
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		return s.dal.InsertInvite(ctx, inv)
	})
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// checkEmail is email normalised (step 1.1) when it is a bare address (no
// display name), else a field error.
func checkEmail(email string) (string, error) {
	email = NormalizeEmail(email)
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email {
		return "", domain.FieldErrorf("email", "%q is not an email address", email)
	}
	return email, nil
}

// CancelInvite implements SVC (ADR 0029 step 2.3).
func (s *service) CancelInvite(ctx context.Context, id uuid.UUID) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		err := s.dal.CancelInvite(ctx, id, s.now())
		if errors.Is(err, domain.ErrNotFound) {
			return ErrInviteNotFound
		}
		return err
	})
}

// SetRootAdmin implements SVC (ADR 0029 step 5.1), and gives the root admin
// the characters without an owner (step 5.3).
func (s *service) SetRootAdmin(ctx context.Context, email string) (*User, error) {
	email = NormalizeEmail(email)
	if !strings.Contains(email, "@") {
		return nil, domain.FieldErrorf("email", "%q is not an email address", email)
	}
	defer s.sessions.forgetAll()
	var out *User
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		// moved is the root that never logged in, when it loses the flag.
		var moved *User
		root, err := s.dal.RootUser(ctx)
		switch {
		case err == nil && root.Email == email:
			out, err = s.adoptAsRoot(ctx, root)
			return err
		case err == nil && root.GoogleSubject != "":
			return ErrRootTaken
		case err == nil:
			// A root that never logged in (a typo, a lost account) moves:
			// it loses the flag and is disabled, never deleted (step 1.2).
			root.Root, root.Disabled = false, true
			if err := s.dal.UpdateUser(ctx, *root); err != nil {
				return err
			}
			if err := s.dal.DeleteUserSessions(ctx, root.ID); err != nil {
				return err
			}
			moved = root
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		u, err := s.dal.UserByEmail(ctx, email)
		switch {
		case err == nil:
			u.Root, u.Role, u.Disabled = true, RoleAdmin, false
			if err := s.dal.UpdateUser(ctx, *u); err != nil {
				return err
			}
		case !errors.Is(err, domain.ErrNotFound):
			return err
		default:
			u = &User{ID: s.newID(), Email: email, Role: RoleAdmin, Root: true, CreatedAt: s.now()}
			if err := s.dal.InsertUser(ctx, *u); err != nil {
				return err
			}
		}
		// The moved root's characters (adopted when it was made) go with the flag: a root
		// set with a typo, then with the right email, leaves the new root
		// playing them, not the disabled one.
		if moved != nil {
			if err := s.dal.MoveCharacters(ctx, moved.ID, u.ID); err != nil {
				return err
			}
		}
		// Step 5.3: the characters without an owner are the root admin's.
		out, err = s.adoptAsRoot(ctx, u)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AdoptUnownedCharacters implements SVC (ADR 0029 steps 5.3 and 7.2).
func (s *service) AdoptUnownedCharacters(ctx context.Context) error {
	var rootID uuid.UUID
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		root, err := s.dal.RootUser(ctx)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		rootID = root.ID
		return s.dal.AdoptUnownedCharacters(ctx, root.ID)
	})
	if rootID != uuid.Nil {
		s.sessions.forgetUser(rootID)
	}
	return err
}

// ForgetUser implements SVC (ADR 0029 step 4.7).
func (s *service) ForgetUser(userID uuid.UUID) { s.sessions.forgetUser(userID) }

// adoptAsRoot gives the characters without an owner to u, the root admin
// just made, and returns u read again (with its current character).
func (s *service) adoptAsRoot(ctx context.Context, u *User) (*User, error) {
	if err := s.dal.AdoptUnownedCharacters(ctx, u.ID); err != nil {
		return nil, err
	}
	return s.dal.UserByID(ctx, u.ID)
}

// FreshCurrentCharacter implements SVC (ADR 0029 step 7.2). It reads the user
// only: the web calls it when the character it shows is not the one this
// server remembers (a switch made on the other server), not on every request.
func (s *service) FreshCurrentCharacter(ctx context.Context, p *Principal) (int64, error) {
	u, err := s.dal.UserByID(ctx, p.User.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return 0, ErrUnauthenticated
	}
	if err != nil {
		return 0, err
	}
	if u.CurrentCharacterID != p.User.CurrentCharacterID {
		s.sessions.forgetUser(u.ID)
	}
	return u.CurrentCharacterID, nil
}
