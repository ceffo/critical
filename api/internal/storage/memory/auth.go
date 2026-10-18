package memory

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// storedInvite is an invites row (ADR 0029 step 2).
type storedInvite struct {
	invite     authsvc.Invite
	cancelled  bool
	acceptedBy uuid.UUID
}

func (i storedInvite) pending() bool { return !i.cancelled && i.acceptedBy == uuid.Nil }

// Auth is the users, invites and sessions (authsvc.DAL). It refuses what the
// schema refuses: a second user of an email or a subject, a second root, a
// root that is not an enabled admin.
type Auth struct{ s *Store }

var _ authsvc.DAL = Auth{}

// Auth returns the login's storage port.
func (s *Store) Auth() Auth { return Auth{s} }

var errConstraint = errors.New("constraint violation")

// copyUser is a deep copy: the stored value never shares LastLogin.
func copyUser(u authsvc.User) *authsvc.User {
	if u.LastLogin != nil {
		t := *u.LastLogin
		u.LastLogin = &t
	}
	return &u
}

func (p Auth) findUser(ctx context.Context, what string, match func(authsvc.User) bool) (*authsvc.User, error) {
	var out *authsvc.User
	err := p.s.read(ctx, func(t *tables) error {
		for _, u := range t.users {
			if match(u) {
				out = copyUser(u)
				out.CurrentCharacterID = t.currentOf(u.ID)
				return nil
			}
		}
		return fmt.Errorf("%w: %s", domain.ErrNotFound, what)
	})
	return out, err
}

// UserByID implements authsvc.DAL.
func (p Auth) UserByID(ctx context.Context, id uuid.UUID) (*authsvc.User, error) {
	return p.findUser(ctx, "user", func(u authsvc.User) bool { return u.ID == id })
}

// UserBySubject implements authsvc.DAL.
func (p Auth) UserBySubject(ctx context.Context, subject string) (*authsvc.User, error) {
	return p.findUser(ctx, "user by subject", func(u authsvc.User) bool { return subject != "" && u.GoogleSubject == subject })
}

// UserByEmail implements authsvc.DAL.
func (p Auth) UserByEmail(ctx context.Context, email string) (*authsvc.User, error) {
	return p.findUser(ctx, "user by email", func(u authsvc.User) bool { return u.Email == email })
}

// RootUser implements authsvc.DAL.
func (p Auth) RootUser(ctx context.Context) (*authsvc.User, error) {
	return p.findUser(ctx, "root user", func(u authsvc.User) bool { return u.Root })
}

// checkUser refuses u where the schema would (migration 003's constraints).
func (t *tables) checkUser(u authsvc.User) error {
	if u.Email == "" || u.Root && (u.Role != authsvc.RoleAdmin || u.Disabled) {
		return fmt.Errorf("%w: user %s", errConstraint, u.ID)
	}
	for _, o := range t.users {
		if o.ID == u.ID {
			continue
		}
		if o.Email == u.Email || (u.GoogleSubject != "" && o.GoogleSubject == u.GoogleSubject) || (u.Root && o.Root) {
			return fmt.Errorf("%w: user %s duplicates user %s", errConstraint, u.ID, o.ID)
		}
	}
	return nil
}

// InsertUser implements authsvc.DAL.
func (p Auth) InsertUser(ctx context.Context, u authsvc.User) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, ok := t.users[u.ID]; ok {
			return fmt.Errorf("%w: user %s exists", errConstraint, u.ID)
		}
		if err := t.checkUser(u); err != nil {
			return err
		}
		u.CurrentCharacterID = 0
		t.users[u.ID] = *copyUser(u)
		return nil
	})
}

// UpdateUser implements authsvc.DAL: the mutable fields, not the email or
// the creation time.
func (p Auth) UpdateUser(ctx context.Context, u authsvc.User) error {
	return p.s.write(ctx, func(t *tables) error {
		old, ok := t.users[u.ID]
		if !ok {
			return fmt.Errorf("%w: user %s", domain.ErrNotFound, u.ID)
		}
		u.Email, u.CreatedAt, u.CurrentCharacterID = old.Email, old.CreatedAt, 0
		if err := t.checkUser(u); err != nil {
			return err
		}
		t.users[u.ID] = *copyUser(u)
		return nil
	})
}

// PendingInvite implements authsvc.DAL.
func (p Auth) PendingInvite(ctx context.Context, email string) (*authsvc.Invite, error) {
	var out *authsvc.Invite
	err := p.s.read(ctx, func(t *tables) error {
		for _, i := range t.invites {
			if i.pending() && i.invite.Email == email {
				inv := i.invite
				out = &inv
				return nil
			}
		}
		return fmt.Errorf("%w: pending invite", domain.ErrNotFound)
	})
	return out, err
}

// AcceptInvite implements authsvc.DAL.
func (p Auth) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID, _ time.Time) error {
	return p.s.write(ctx, func(t *tables) error {
		i, ok := t.invites[inviteID]
		if !ok || !i.pending() {
			return fmt.Errorf("%w: pending invite %s", domain.ErrNotFound, inviteID)
		}
		i.acceptedBy = userID
		t.invites[inviteID] = i
		return nil
	})
}

// InsertSession implements authsvc.DAL.
func (p Auth) InsertSession(ctx context.Context, s authsvc.Session) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, ok := t.sessions[s.ID]; ok {
			return fmt.Errorf("%w: session exists", errConstraint)
		}
		if _, ok := t.users[s.UserID]; !ok {
			return fmt.Errorf("%w: session of unknown user %s", errConstraint, s.UserID)
		}
		t.sessions[s.ID] = s
		return nil
	})
}

// SessionByID implements authsvc.DAL.
func (p Auth) SessionByID(ctx context.Context, id string) (*authsvc.Session, error) {
	var out *authsvc.Session
	err := p.s.read(ctx, func(t *tables) error {
		s, ok := t.sessions[id]
		if !ok {
			return fmt.Errorf("%w: session", domain.ErrNotFound)
		}
		out = &s
		return nil
	})
	return out, err
}

// ExtendSession implements authsvc.DAL.
func (p Auth) ExtendSession(ctx context.Context, id string, lastUsed, expires time.Time) error {
	return p.s.write(ctx, func(t *tables) error {
		if s, ok := t.sessions[id]; ok {
			s.LastUsed, s.Expires = lastUsed, expires
			t.sessions[id] = s
		}
		return nil
	})
}

// DeleteSession implements authsvc.DAL.
func (p Auth) DeleteSession(ctx context.Context, id string) error {
	return p.s.write(ctx, func(t *tables) error {
		delete(t.sessions, id)
		return nil
	})
}

// DeleteUserSessions implements authsvc.DAL.
func (p Auth) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	return p.s.write(ctx, func(t *tables) error {
		for id, s := range t.sessions {
			if s.UserID == userID {
				delete(t.sessions, id)
			}
		}
		return nil
	})
}

// currentOf is the user's current character as the database reads it: one
// the user owns and has not retired, else 0.
func (t *tables) currentOf(userID uuid.UUID) int64 {
	id, ok := t.current[userID]
	if !ok || t.owners[id] != userID || t.retired[id] {
		return 0
	}
	return id
}

// AdoptUnownedCharacters implements authsvc.DAL.
func (p Auth) AdoptUnownedCharacters(ctx context.Context, userID uuid.UUID) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, ok := t.users[userID]; !ok {
			return fmt.Errorf("%w: characters of unknown user %s", errConstraint, userID)
		}
		var first int64
		for _, id := range slices.Sorted(maps.Keys(t.characters)) {
			if _, owned := t.owners[id]; !owned {
				t.owners[id] = userID
			}
			if first == 0 && t.owners[id] == userID && !t.retired[id] {
				first = id
			}
		}
		if _, set := t.current[userID]; !set && first != 0 {
			t.current[userID] = first
		}
		return nil
	})
}
