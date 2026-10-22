// Package spheresvc is the spell source of preparation and of the lookup's
// "Mine" filter (ADR 0030 step 15): a class with a sphere access (a cleric)
// prepares from the catalogue's priest spells its spheres open, at the spell
// levels it has slots of; any other class from its stored spellbook.
package spheresvc

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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

// Deps are the collaborators. Book and Characters are required; Access is
// nil when the catalogue carries no sphere access (format 5).
type Deps struct {
	Book       Book
	Characters Characters
	Access     *ruletablesvc.SphereAccess
	Catalogue  catalogue.Catalogue
}

// Spheres serves the spells a character prepares from (prepsvc.Spellbook,
// spellsvc.Spellbook): for a class with a sphere access (a cleric, ADR 0030
// step 15) the catalogue's priest spells its spheres open at the spell
// levels it has slots of (a ruling: a level without a slot offers nothing),
// in level then id order; for any other class its stored spellbook. Without
// a sphere access (a format-5 catalogue) a cleric has no stored spellbook,
// so it prepares nothing.
type Spheres struct {
	book   Book
	chars  Characters
	access *ruletablesvc.SphereAccess
	priest []catalogue.Spell
}

var (
	_ prepsvc.Spellbook  = (*Spheres)(nil)
	_ spellsvc.Spellbook = (*Spheres)(nil)
)

// New returns the spell source.
func New(d Deps) (*Spheres, error) {
	if d.Book == nil || d.Characters == nil {
		return nil, errors.New("spheresvc: a book and the characters are required")
	}
	book, chars, access, cat := d.Book, d.Characters, d.Access, d.Catalogue
	var priest []catalogue.Spell
	var codes []string
	for _, s := range cat.Spells {
		codes = append(codes, s.Spheres...)
		if s.Class == "Priest" {
			priest = append(priest, s)
		}
	}
	if err := access.CheckSpheres(codes); err != nil {
		return nil, err
	}
	slices.SortStableFunc(priest, func(a, b catalogue.Spell) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.ID, b.ID))
	})
	return &Spheres{book: book, chars: chars, access: access, priest: priest}, nil
}

// open returns the spells the character's spheres open within its slots,
// ok false for a class without a sphere access.
func (s *Spheres) open(ctx context.Context, characterID int64, entry string) ([]catalogue.Spell, bool, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, false, err
	}
	// ADR 0042 steps 12 and 14: the entry granting the sphere access, with
	// its own slots; a named entry is that one (a cleric/mage's mage entry
	// has no sphere access: its stored spellbook).
	class := c.Derived.FeatureClass(classes.KindSphereAccess, c.Class)
	if entry != "" {
		class = entry
	}
	if !s.access.HasSpheres(class) {
		return nil, false, nil
	}
	slots := c.Derived.SpellSlots
	if e, ok := c.Derived.Entry(class); ok {
		slots = e.SpellSlots
	}
	out := []catalogue.Spell{}
	for _, sp := range s.priest {
		if slots[sp.Level] > 0 && s.access.Opens(class, sp.Level, sp.Spheres) {
			out = append(out, sp)
		}
	}
	return out, true, nil
}

// All implements prepsvc.Spellbook.
func (s *Spheres) All(ctx context.Context, characterID int64) ([]prepsvc.SpellbookEntry, error) {
	return s.Of(ctx, characterID, "")
}

// Of implements prepsvc.Spellbook: the spells the entry of class prepares
// from (ADR 0042 step 14), All's for an empty class.
func (s *Spheres) Of(ctx context.Context, characterID int64, class string) ([]prepsvc.SpellbookEntry, error) {
	open, ok, err := s.open(ctx, characterID, class)
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
	open, ok, err := s.open(ctx, characterID, "")
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
