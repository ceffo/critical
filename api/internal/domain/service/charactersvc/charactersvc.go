// Package charactersvc owns read access to character core attributes (stats,
// HP max, THAC0, saving throws), and which characters a user owns and plays
// (ADR 0029 step 7). The sheets are written via seeding in the storage layer.
package charactersvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// ErrNoCurrentCharacter is a per-character request of a user who has no
// current character (ADR 0029 step 7.2): a new player, before they create
// their first one. Its stable code tells the web to offer that.
var ErrNoCurrentCharacter = fmt.Errorf("no current character: %w", domain.ErrNotFound)

// SVC is the driver port.
type SVC interface {
	domain.Service
	GetCharacter(ctx context.Context, id int64) (*Character, error)
	// List is the owner's characters they have not retired, in creation
	// order (ADR 0029 steps 7.1 and 7.3).
	List(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrent makes the owner's character id their current character
	// (step 7.2). Another user's character, a retired one and an unknown id
	// are all domain.ErrCharacterNotFound (step 7.1: a foreign character is
	// not revealed).
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
}

// Reader is the read-side driven port. Per clean_architecture.md §5.4, reads
// don't need the full DAL detour; this port returns domain entities directly.
type Reader interface {
	GetCharacter(ctx context.Context, id int64) (*Character, error)
}

// Owners is the driven port over who owns and plays which character.
type Owners interface {
	// CharactersOf is the owner's characters they have not retired, by id.
	CharactersOf(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrentCharacter sets the user's current character when the user
	// owns it and has not retired it, else returns
	// domain.ErrCharacterNotFound and changes nothing (one statement: no
	// check-then-write).
	SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error
}

// Sessions is the login's session cache (authsvc.SVC satisfies it): a user
// whose current character changed is read again by their next request.
type Sessions interface {
	ForgetUser(userID uuid.UUID)
}

// Deps are the service's collaborators.
type Deps struct {
	Reader   Reader
	Owners   Owners
	Tx       domain.TxRunner
	Sessions Sessions
}

type service struct {
	reader   Reader
	owners   Owners
	tx       domain.TxRunner
	sessions Sessions
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Reader == nil || d.Owners == nil || d.Tx == nil || d.Sessions == nil {
		return nil, errors.New("charactersvc: nil dependency")
	}
	return &service{reader: d.Reader, owners: d.Owners, tx: d.Tx, sessions: d.Sessions}, nil
}

// NewWithConfig is here for symmetry with the other services. There is no
// configurable surface today.
func NewWithConfig(_ *Config, d Deps) (SVC, error) { return New(d) }

func (s *service) GetCharacter(ctx context.Context, id int64) (*Character, error) {
	return s.reader.GetCharacter(ctx, id)
}

func (s *service) List(ctx context.Context, owner uuid.UUID) ([]Summary, error) {
	return s.owners.CharactersOf(ctx, owner)
}

// SetCurrent writes under the writer lock like every mutation (ADR 0025 step
// 4), outside the action log: choosing a character is no move of the game and
// is not undone. This server forgets the owner's sessions once it committed, so
// their next request plays the new character (ADR 0029 step 4.7).
func (s *service) SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error {
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		return s.owners.SetCurrentCharacter(ctx, owner, id)
	}); err != nil {
		return err
	}
	s.sessions.ForgetUser(owner)
	return nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
