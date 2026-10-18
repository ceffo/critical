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
	// Search pages the catalogue; Filter.Mine keeps the spells of the
	// character's spellbook.
	Search(ctx context.Context, characterID int64, f Filter, p PageRequest) (Page, error)
	// Get returns the catalogue spell with unique key id, every text included,
	// or ErrSpellNotFound.
	Get(ctx context.Context, id string) (Spell, error)
	// Names lists every catalogue spell's id and names, in the Reader's order.
	Names(ctx context.Context) ([]SpellName, error)
	// Levels lists the spell levels the catalogue holds, ascending, each once.
	Levels(ctx context.Context) ([]int, error)
}

// DAL is the write-side driven port (slot ledger).
type DAL interface {
	GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []SpellSlot) error
}

// Reader is the read-side driven port (catalog search). Search applies the
// stored columns of the filter (Class, Levels) and returns the rows ordered
// by level, fallback-language name, class, with only the texts a search
// filters and ranks on (Name, School, Sphere) besides the typed stats;
// Details returns the full spells of ids (every text), in the order of ids,
// skipping an id it does not hold. A page's texts are read by one Details
// call, never per spell or per field.
type Reader interface {
	Search(ctx context.Context, f Filter) ([]Spell, error)
	Details(ctx context.Context, ids []string) ([]Spell, error)
}

// Spellbook is each character's spellbook (the "Mine" filter).
type Spellbook interface {
	// SpellKeys is the catalogue keys of the spells in the character's
	// spellbook.
	SpellKeys(ctx context.Context, characterID int64) ([]string, error)
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
