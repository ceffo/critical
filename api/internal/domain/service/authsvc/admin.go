package authsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// SetDisabled implements SVC (ADR 0029 steps 1.2, 4.4 and 5.2).
func (s *service) SetDisabled(ctx context.Context, userID uuid.UUID, disabled bool) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.dal.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		if u.Root && disabled {
			return ErrRootProtected
		}
		if u.Disabled == disabled {
			return nil
		}
		u.Disabled = disabled
		if err := s.dal.UpdateUser(ctx, *u); err != nil {
			return err
		}
		if !disabled {
			return nil
		}
		// Step 4.4: in the same transaction, so the next request is refused.
		return s.dal.DeleteUserSessions(ctx, userID)
	})
}

// SetRole implements SVC (ADR 0029 steps 1.3 and 5.2).
func (s *service) SetRole(ctx context.Context, userID uuid.UUID, role Role) error {
	if role != RoleAdmin && role != RolePlayer {
		return domain.FieldErrorf("role", "role must be %q or %q", RoleAdmin, RolePlayer)
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.dal.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		if u.Root && role != RoleAdmin {
			return ErrRootProtected
		}
		if u.Role == role {
			return nil
		}
		u.Role = role
		return s.dal.UpdateUser(ctx, *u)
	})
}

// SetRootAdmin implements SVC (ADR 0029 step 5.1).
func (s *service) SetRootAdmin(ctx context.Context, email string) (*User, error) {
	email = NormalizeEmail(email)
	if !strings.Contains(email, "@") {
		return nil, domain.FieldErrorf("email", "%q is not an email address", email)
	}
	var out *User
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		root, err := s.dal.RootUser(ctx)
		switch {
		case err == nil && root.Email == email:
			out = root
			return nil
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
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		u, err := s.dal.UserByEmail(ctx, email)
		switch {
		case err == nil:
			u.Root, u.Role, u.Disabled = true, RoleAdmin, false
			out = u
			return s.dal.UpdateUser(ctx, *u)
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		out = &User{ID: s.newID(), Email: email, Role: RoleAdmin, Root: true, CreatedAt: s.now()}
		return s.dal.InsertUser(ctx, *out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
