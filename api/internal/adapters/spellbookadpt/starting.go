package spellbookadpt

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// StartingFileName is the seed file, under the seeds dir, of the spells a new
// character's starting spellbook is offered pre-ticked, by class (ADR 0030
// step 20.7): {"<class>": ["<catalogue id>", ...]}.
const StartingFileName = "starting_spellbook.json"

// StartingSeed is the starting spellbook seed: by class, the catalogue ids
// offered pre-ticked, in the file's order.
type StartingSeed map[string][]string

// ReadStartingSeed reads <seedsDir>/starting_spellbook.json and checks it
// against the catalogue (ParseStartingSeed).
func ReadStartingSeed(seedsDir string, cat catalogue.Catalogue) (StartingSeed, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Clean(seedsDir), StartingFileName)) //nolint:gosec // operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("starting spellbook: read: %w", err)
	}
	return ParseStartingSeed(raw, cat)
}

// ParseStartingSeed maps a starting spellbook seed and checks it: each class
// keeps a spellbook (ruletablesvc.SpellbookMaxLevel), each id is a wizard
// spell of the catalogue, once per class. A spell the catalogue lacks is a
// start-up error rather than a pre-tick silently dropped.
func ParseStartingSeed(raw []byte, cat catalogue.Catalogue) (StartingSeed, error) {
	var seed StartingSeed
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&seed); err != nil {
		return nil, fmt.Errorf("starting spellbook: parse: %w", err)
	}
	spells := cat.ByID()
	for class, ids := range seed {
		if _, ok := ruletablesvc.SpellbookMaxLevel(class); !ok {
			return nil, fmt.Errorf("starting spellbook: class %q keeps no spellbook", class)
		}
		seen := map[string]bool{}
		for _, id := range ids {
			s, ok := spells[id]
			if !ok || !spellbook.FromCatalogue(s).Wizard {
				return nil, fmt.Errorf("starting spellbook: %s: %q is not a Wizard spell of the catalogue", class, id)
			}
			if seen[id] {
				return nil, fmt.Errorf("starting spellbook: %s: duplicate spell %q", class, id)
			}
			seen[id] = true
		}
	}
	return seed, nil
}

// Creation is the spell catalogue as the creation's starting spellbook reads
// it (charactersvc.SpellCatalogue, ADR 0030 step 20.7): each spell as the
// spellbook's rules see it, the wizard spells offered, and the seed's
// pre-ticked spells.
type Creation struct {
	spells   map[string]catalogue.Spell
	wizard   []charactersvc.SpellOption
	starting StartingSeed
}

var _ charactersvc.SpellCatalogue = (*Creation)(nil)

// NewCreation returns the creation's reading of cat, with the pre-ticked
// spells of starting (ReadStartingSeed).
func NewCreation(cat catalogue.Catalogue, starting StartingSeed) *Creation {
	out := &Creation{spells: cat.ByID(), wizard: []charactersvc.SpellOption{}, starting: starting}
	for id, s := range out.spells {
		if sp := spellbook.FromCatalogue(s); sp.Wizard {
			out.wizard = append(out.wizard, charactersvc.SpellOption{ID: id, Level: sp.Level})
		}
	}
	slices.SortFunc(out.wizard, func(a, b charactersvc.SpellOption) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// Spell is the catalogue spell id as the spellbook's rules read it; ok is
// false for an id the catalogue does not hold.
func (c *Creation) Spell(id string) (spellbook.Spell, bool) {
	s, ok := c.spells[id]
	if !ok {
		return spellbook.Spell{}, false
	}
	return spellbook.FromCatalogue(s), true
}

// WizardSpells are the catalogue's wizard spells, by spell level then id.
func (c *Creation) WizardSpells() []charactersvc.SpellOption {
	return slices.Clone(c.wizard)
}

// StartingSpells are the seed's pre-ticked spells of class, a copy; empty for
// a class the seed does not list.
func (c *Creation) StartingSpells(class string) []string {
	return append([]string{}, c.starting[class]...)
}
