// Package authdal is the storage adapter for users, invites and sessions
// (authsvc.DAL, ADR 0029). It delegates to sqlc-generated queries.
package authdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/adapters/authdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// DAL is the implementation of authsvc.DAL over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

var _ authsvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// notFound maps sql.ErrNoRows to domain.ErrNotFound naming what was missing.
func notFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, what)
	}
	return fmt.Errorf("%s: %w", what, err)
}

// user maps a user read; every read has the columns of UserByIDRow (the
// other rows convert to it).
func user(r queries.UserByIDRow) *authsvc.User {
	u := &authsvc.User{
		ID: r.ID, Email: r.Email, GoogleSubject: r.GoogleSubject.String, Role: authsvc.Role(r.Role),
		Root: r.IsRoot, Disabled: r.Disabled, CreatedAt: r.CreatedAt,
		CurrentCharacterID: r.CurrentCharacterID.Int64,
	}
	if r.LastLogin.Valid {
		t := r.LastLogin.Time
		u.LastLogin = &t
	}
	return u
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// UserByID implements authsvc.DAL.
func (d *DAL) UserByID(ctx context.Context, id uuid.UUID) (*authsvc.User, error) {
	r, err := d.q.UserByID(ctx, id)
	if err != nil {
		return nil, notFound(err, "user")
	}
	return user(r), nil
}

// UserBySubject implements authsvc.DAL.
func (d *DAL) UserBySubject(ctx context.Context, subject string) (*authsvc.User, error) {
	r, err := d.q.UserBySubject(ctx, nullString(subject))
	if err != nil {
		return nil, notFound(err, "user by subject")
	}
	return user(queries.UserByIDRow(r)), nil
}

// UserByEmail implements authsvc.DAL.
func (d *DAL) UserByEmail(ctx context.Context, email string) (*authsvc.User, error) {
	r, err := d.q.UserByEmail(ctx, email)
	if err != nil {
		return nil, notFound(err, "user by email")
	}
	return user(queries.UserByIDRow(r)), nil
}

// RootUser implements authsvc.DAL.
func (d *DAL) RootUser(ctx context.Context) (*authsvc.User, error) {
	r, err := d.q.RootUser(ctx)
	if err != nil {
		return nil, notFound(err, "root user")
	}
	return user(queries.UserByIDRow(r)), nil
}

// InsertUser implements authsvc.DAL.
func (d *DAL) InsertUser(ctx context.Context, u authsvc.User) error {
	if err := d.q.InsertUser(ctx, queries.InsertUserParams{
		ID: u.ID, Email: u.Email, GoogleSubject: nullString(u.GoogleSubject), Role: string(u.Role),
		IsRoot: u.Root, Disabled: u.Disabled, LastLogin: nullTime(u.LastLogin), CreatedAt: u.CreatedAt,
	}); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// UpdateUser implements authsvc.DAL; an unknown id is domain.ErrNotFound.
func (d *DAL) UpdateUser(ctx context.Context, u authsvc.User) error {
	n, err := d.q.UpdateUser(ctx, queries.UpdateUserParams{
		ID: u.ID, GoogleSubject: nullString(u.GoogleSubject), Role: string(u.Role),
		IsRoot: u.Root, Disabled: u.Disabled, LastLogin: nullTime(u.LastLogin),
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: user %s", domain.ErrNotFound, u.ID)
	}
	return nil
}

// PendingInvite implements authsvc.DAL.
func (d *DAL) PendingInvite(ctx context.Context, email string) (*authsvc.Invite, error) {
	r, err := d.q.PendingInvite(ctx, email)
	if err != nil {
		return nil, notFound(err, "pending invite")
	}
	return &authsvc.Invite{ID: r.ID, Email: r.Email, InvitedBy: r.InvitedBy, CreatedAt: r.CreatedAt}, nil
}

// AcceptInvite implements authsvc.DAL; an invite that is no longer pending is
// domain.ErrNotFound.
func (d *DAL) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID, at time.Time) error {
	n, err := d.q.AcceptInvite(ctx, queries.AcceptInviteParams{
		ID: inviteID, AcceptedBy: uuid.NullUUID{UUID: userID, Valid: true}, AcceptedAt: sql.NullTime{Time: at, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: pending invite %s", domain.ErrNotFound, inviteID)
	}
	return nil
}

// InsertSession implements authsvc.DAL.
func (d *DAL) InsertSession(ctx context.Context, s authsvc.Session) error {
	if err := d.q.InsertSession(ctx, queries.InsertSessionParams{
		ID: s.ID, UserID: s.UserID, CreatedAt: s.CreatedAt, LastUsed: s.LastUsed, ExpiresAt: s.Expires,
	}); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// SessionByID implements authsvc.DAL.
func (d *DAL) SessionByID(ctx context.Context, id string) (*authsvc.Session, error) {
	r, err := d.q.SessionByID(ctx, id)
	if err != nil {
		return nil, notFound(err, "session")
	}
	return &authsvc.Session{ID: r.ID, UserID: r.UserID, CreatedAt: r.CreatedAt, LastUsed: r.LastUsed, Expires: r.ExpiresAt}, nil
}

// ExtendSession implements authsvc.DAL.
func (d *DAL) ExtendSession(ctx context.Context, id string, lastUsed, expires time.Time) error {
	if err := d.q.ExtendSession(ctx, queries.ExtendSessionParams{ID: id, LastUsed: lastUsed, ExpiresAt: expires}); err != nil {
		return fmt.Errorf("extend session: %w", err)
	}
	return nil
}

// DeleteSession implements authsvc.DAL.
func (d *DAL) DeleteSession(ctx context.Context, id string) error {
	if err := d.q.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteUserSessions implements authsvc.DAL.
func (d *DAL) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	if err := d.q.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}

// AdoptUnownedCharacters implements authsvc.DAL.
func (d *DAL) AdoptUnownedCharacters(ctx context.Context, userID uuid.UUID) error {
	if err := d.q.AdoptUnownedCharacters(ctx, uuid.NullUUID{UUID: userID, Valid: true}); err != nil {
		return fmt.Errorf("adopt unowned characters: %w", err)
	}
	if err := d.q.SetFirstCurrentCharacter(ctx, userID); err != nil {
		return fmt.Errorf("set first current character: %w", err)
	}
	return nil
}

// MoveCharacters implements authsvc.DAL: every character of from becomes
// to's, and from keeps no current character.
func (d *DAL) MoveCharacters(ctx context.Context, from, to uuid.UUID) error {
	if err := d.q.MoveCharacters(ctx, queries.MoveCharactersParams{
		FromUser: uuid.NullUUID{UUID: from, Valid: true},
		ToUser:   uuid.NullUUID{UUID: to, Valid: true},
	}); err != nil {
		return fmt.Errorf("move characters: %w", err)
	}
	if err := d.q.ClearCurrentCharacter(ctx, from); err != nil {
		return fmt.Errorf("clear current character: %w", err)
	}
	return nil
}
