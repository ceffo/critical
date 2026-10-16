package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Spells is the spell port: the spell slots (spellsvc.DAL), the catalogue
// reads (spellsvc.Reader) and the spell texts (castsvc.Texts).
type Spells struct{ s *Store }

var (
	_ spellsvc.DAL    = Spells{}
	_ spellsvc.Reader = Spells{}
	_ castsvc.Texts   = Spells{}
)

// Spells returns the spell port.
func (s *Store) Spells() Spells { return Spells{s} }

// GetSlots returns the character's spell slots (empty when none).
func (p Spells) GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error) {
	out := []spellsvc.SpellSlot{}
	err := p.s.read(ctx, func(t *tables) error {
		if slots := t.slots[characterID]; len(slots) > 0 {
			out = clone(slots)
		}
		return nil
	})
	return out, err
}

// SaveSlots replaces the character's spell slots.
func (p Spells) SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error {
	return p.s.write(ctx, func(t *tables) error {
		t.slots[characterID] = clone(slots)
		return nil
	})
}

// Search returns the catalogue spells of f's class (any case; empty: any) and
// levels (none: any), ordered by level, fallback-language name, class, with
// the fields the database adapter reads for a search: the name, school and
// sphere texts and the typed stats, never the descriptions. The service
// applies the rest of the filter.
func (p Spells) Search(ctx context.Context, f spellsvc.Filter) ([]spellsvc.Spell, error) {
	out := []spellsvc.Spell{}
	class := strings.ToLower(f.Class)
	err := p.s.read(ctx, func(t *tables) error {
		for _, sp := range t.spells {
			if class != "" && strings.ToLower(sp.Class) != class {
				continue
			}
			if len(f.Levels) > 0 && !slices.Contains(f.Levels, sp.Level) {
				continue
			}
			out = append(out, clone(spellsvc.Spell{
				ID: sp.ID, Class: sp.Class, Level: sp.Level, Source: sp.Source,
				Name: sp.Name, School: sp.School, Sphere: sp.Sphere, Stats: sp.Stats,
			}))
		}
		return nil
	})
	slices.SortFunc(out, func(a, b spellsvc.Spell) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.Name.Default(), b.Name.Default()),
			cmp.Compare(a.Class, b.Class), cmp.Compare(a.ID, b.ID))
	})
	return out, err
}

// Details returns the full spells of ids, in that order, skipping an id the
// catalogue does not hold.
func (p Spells) Details(ctx context.Context, ids []string) ([]spellsvc.Spell, error) {
	out := []spellsvc.Spell{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, id := range ids {
			if sp, ok := t.spells[id]; ok {
				out = append(out, clone(sp))
			}
		}
		return nil
	})
	return out, err
}

// Describe finds the spell whose catalogue key is spellID.
func (p Spells) Describe(ctx context.Context, spellID string) (spellsvc.Spell, bool, error) {
	found, err := p.Details(ctx, []string{spellID})
	if err != nil || len(found) == 0 {
		return spellsvc.Spell{}, false, err
	}
	return found[0], true, nil
}

// stored is a text as the database keeps it: one row per language with a
// body (ADR 0023), nil when there is none.
func stored(t localized.Text) localized.Text { return t.Clean() }

// named is a name as the database adapters read it back: a text, empty
// rather than nil when it has no row.
func named(t localized.Text) localized.Text {
	out := localized.Text{}
	for _, lang := range t.Languages() {
		out[lang] = t[lang]
	}
	return out
}
