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
	Search(ctx context.Context, f Filter, p PageRequest) (Page, error)
}

// DAL is the write-side driven port (slot ledger).
type DAL interface {
	GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []SpellSlot) error
}

// Reader is the read-side driven port (catalog search). It applies the
// stored columns of the filter (Class, Levels) and returns the rows ordered
// by level, name, class.
type Reader interface {
	Search(ctx context.Context, f Filter) ([]Spell, error)
}

// Spellbook is the Bard's spellbook (the "Mine" filter).
type Spellbook interface {
	// Contains reports whether the spell with unique key id is in the book.
	Contains(id string) bool
}

type service struct {
	dal    DAL
	reader Reader
	book   Spellbook
}

// New constructs the service.
func New(dal DAL, reader Reader, book Spellbook) (SVC, error) {
	if dal == nil {
		return nil, fmt.Errorf("spellsvc: nil dal")
	}
	if reader == nil {
		return nil, fmt.Errorf("spellsvc: nil reader")
	}
	if book == nil {
		return nil, fmt.Errorf("spellsvc: nil spellbook")
	}
	return &service{dal: dal, reader: reader, book: book}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL, reader Reader, book Spellbook) (SVC, error) {
	return New(dal, reader, book)
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
