// Package spellsvc owns the spell slot ledger (read, reset on a long rest;
// castsvc spends slots) and the read-side spell catalog search. The catalog is exposed via a separate Reader port —
// reads don't need the write-side DAL detour (clean_architecture.md §5.4).
package spellsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error)
	ResetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error)
	Search(ctx context.Context, query, class string) ([]Spell, error)
}

// DAL is the write-side driven port (slot ledger).
type DAL interface {
	GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []SpellSlot) error
}

// Reader is the read-side driven port (catalog search).
type Reader interface {
	Search(ctx context.Context, query, class string) ([]Spell, error)
}

type service struct {
	dal    DAL
	reader Reader
}

// New constructs the service.
func New(dal DAL, reader Reader) (SVC, error) {
	if dal == nil {
		return nil, fmt.Errorf("spellsvc: nil dal")
	}
	if reader == nil {
		return nil, fmt.Errorf("spellsvc: nil reader")
	}
	return &service{dal: dal, reader: reader}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL, reader Reader) (SVC, error) { return New(dal, reader) }

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
