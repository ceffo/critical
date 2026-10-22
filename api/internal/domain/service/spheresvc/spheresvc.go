// Package spheresvc is the spell source of preparation and of the lookup's
// "Mine" filter (ADR 0030 step 15): a class with a sphere access (a cleric)
// prepares from the catalogue's priest spells its spheres open, at the spell
// levels it has slots of; any other class from its stored spellbook.
package spheresvc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Characters reads a character with its derived values (charactersvc.SVC).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Book is the stored spellbook (spellbookadpt.Book).
type Book interface {
	prepsvc.Spellbook
	spellsvc.Spellbook
}

// Deps are the collaborators. All but Catalogue are required: Classes holds
// the class definitions whose sphere_access feature says what a class
// prepares from.
type Deps struct {
	Book       Book
	Characters Characters
	Classes    *classes.Registry
	Catalogue  catalogue.Catalogue
}

// Spheres serves the spells a character prepares from (prepsvc.Spellbook,
// spellsvc.Spellbook): for a class with a sphere access (a cleric, ADR 0030
// step 15) the catalogue's priest spells its spheres open at the spell
// levels it has slots of (a ruling: a level without a slot offers nothing),
// in level then id order; for any other class its stored spellbook. A class
// has a sphere access when it is granted the sphere_access feature at its
// realized level (ADR 0040 step 3), read from its definition: no class id.
type Spheres struct {
	book   Book
	chars  Characters
	defs   *classes.Registry
	priest []catalogue.Spell
}

var (
	_ prepsvc.Spellbook  = (*Spheres)(nil)
	_ spellsvc.Spellbook = (*Spheres)(nil)
)

// New returns the spell source.
func New(d Deps) (*Spheres, error) {
	if d.Book == nil || d.Characters == nil || d.Classes == nil {
		return nil, errors.New("spheresvc: a book, the characters and the class definitions are required")
	}
	book, chars, defs, cat := d.Book, d.Characters, d.Classes, d.Catalogue
	var priest []catalogue.Spell
	var codes []string
	for _, s := range cat.Spells {
		codes = append(codes, s.Spheres...)
		if s.Class == "Priest" {
			priest = append(priest, s)
		}
	}
	// A sphere code on no spell (a typo) would silently close the sphere.
	for _, id := range defs.ClassIDs() {
		def, _ := defs.Class(id)
		f, ok := def.Lists(classes.KindSphereAccess)
		if !ok {
			continue
		}
		for _, sphere := range f.Spheres.Codes() {
			if !slices.Contains(codes, sphere) {
				return nil, fmt.Errorf("sphere_access: %s sphere %q is on no spell of the catalogue", id, sphere)
			}
		}
	}
	slices.SortStableFunc(priest, func(a, b catalogue.Spell) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.ID, b.ID))
	})
	return &Spheres{book: book, chars: chars, defs: defs, priest: priest}, nil
}

// open returns the spells the character's spheres open within its slots,
// ok false for a class without a sphere access.
func (s *Spheres) open(ctx context.Context, characterID int64) ([]catalogue.Spell, bool, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, false, err
	}
	def, found := s.defs.Class(c.Class)
	if !found {
		return nil, false, nil
	}
	f, ok := def.Feature(classes.KindSphereAccess, c.Derived.RealizedLevel)
	if !ok {
		return nil, false, nil
	}
	out := []catalogue.Spell{}
	for _, sp := range s.priest {
		if c.Derived.SpellSlots[sp.Level] > 0 && f.Spheres.Opens(sp.Level, sp.Spheres) {
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
