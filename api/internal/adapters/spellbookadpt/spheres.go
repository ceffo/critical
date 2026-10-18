package spellbookadpt

import (
	"cmp"
	"context"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Characters reads a character with its derived values (charactersvc.SVC).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Spheres serves the spells a character prepares from (prepsvc.Spellbook,
// spellsvc.Spellbook): for a class with a sphere access (a cleric, ADR 0030
// step 15) the catalogue's priest spells its spheres open at the spell
// levels it has slots of, in level then id order; for any other class its
// stored spellbook (Book). Without a sphere access (a format-5 catalogue) a
// cleric has no stored spellbook, so it prepares nothing.
type Spheres struct {
	book   *Book
	chars  Characters
	access *ruletablesvc.SphereAccess
	priest []catalogue.Spell
}

var (
	_ prepsvc.Spellbook  = (*Spheres)(nil)
	_ spellsvc.Spellbook = (*Spheres)(nil)
)

// NewSpheres returns the spell source over book; access is nil when the
// catalogue carries none.
func NewSpheres(book *Book, chars Characters, access *ruletablesvc.SphereAccess, cat catalogue.Catalogue) *Spheres {
	var priest []catalogue.Spell
	for _, s := range cat.Spells {
		if s.Class == "Priest" {
			priest = append(priest, s)
		}
	}
	slices.SortStableFunc(priest, func(a, b catalogue.Spell) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.ID, b.ID))
	})
	return &Spheres{book: book, chars: chars, access: access, priest: priest}
}

// open returns the spells the character's spheres open within its slots,
// ok false for a class without a sphere access.
func (s *Spheres) open(ctx context.Context, characterID int64) ([]catalogue.Spell, bool, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, false, err
	}
	if !s.access.HasSpheres(c.Class) {
		return nil, false, nil
	}
	out := []catalogue.Spell{}
	for _, sp := range s.priest {
		if c.Derived.SpellSlots[sp.Level] > 0 && s.access.Opens(c.Class, sp.Level, sp.Spheres) {
			out = append(out, sp)
		}
	}
	return out, true, nil
}

// All implements prepsvc.Spellbook.
func (s *Spheres) All(ctx context.Context, characterID int64) ([]prepsvc.SpellbookEntry, error) {
	open, ok, err := s.open(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.book.All(ctx, characterID)
	}
	out := make([]prepsvc.SpellbookEntry, len(open))
	for i, sp := range open {
		out[i] = prepsvc.SpellbookEntry{ID: sp.ID, Name: sp.Name.Default(), Level: sp.Level}
	}
	return out, nil
}

// SpellKeys implements spellsvc.Spellbook (the lookup's "Mine" filter).
func (s *Spheres) SpellKeys(ctx context.Context, characterID int64) ([]string, error) {
	open, ok, err := s.open(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.book.SpellKeys(ctx, characterID)
	}
	out := make([]string, len(open))
	for i, sp := range open {
		out[i] = sp.ID
	}
	return out, nil
}
