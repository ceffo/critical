// Package spellbookadpt loads the Bard's spellbook (seeds/spellbook.json) and
// serves it to prepsvc. Each entry is cross-checked against the Wizard spells
// of the spell catalogue (id, English name and level), so a typo, a wrong
// level or an id the catalogue does not hold is a startup error. Entities
// live in prepsvc; this package only maps files.
package spellbookadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
)

// FileName is the seed file under the seeds dir.
const FileName = "spellbook.json"

type entryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Sheet string `json:"sheet"` // the sheet's own (French) name, kept for review
}

// Book implements prepsvc.Spellbook.
type Book struct {
	entries []prepsvc.SpellbookEntry
	byID    map[string]prepsvc.SpellbookEntry
}

var _ prepsvc.Spellbook = (*Book)(nil)

// New loads <seedsDir>/spellbook.json and validates it against the catalogue.
func New(seedsDir string, cat catalogue.Catalogue) (*Book, error) {
	book, err := os.ReadFile(filepath.Join(filepath.Clean(seedsDir), FileName)) //nolint:gosec // operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("spellbook: read: %w", err)
	}
	return Parse(book, cat)
}

// Parse maps the spellbook file and validates it against the catalogue.
func Parse(book []byte, cat catalogue.Catalogue) (*Book, error) {
	var entries []entryDTO
	dec := json.NewDecoder(bytes.NewReader(book))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("spellbook: parse: %w", err)
	}
	spells := cat.ByID()

	b := &Book{byID: map[string]prepsvc.SpellbookEntry{}}
	for _, e := range entries {
		s, ok := spells[e.ID]
		if !ok || s.Class != "Wizard" {
			return nil, fmt.Errorf("spellbook: %q is not a Wizard spell of the catalogue", e.ID)
		}
		if s.Name != e.Name {
			return nil, fmt.Errorf("spellbook: %q is named %q in the catalogue, not %q", e.ID, s.Name, e.Name)
		}
		if s.Level != e.Level {
			return nil, fmt.Errorf("spellbook: %q is level %d in the catalogue, not %d", e.ID, s.Level, e.Level)
		}
		if _, dup := b.byID[e.ID]; dup {
			return nil, fmt.Errorf("spellbook: duplicate spell %q", e.ID)
		}
		entry := prepsvc.SpellbookEntry{ID: e.ID, Name: e.Name, Level: e.Level}
		b.entries = append(b.entries, entry)
		b.byID[e.ID] = entry
	}
	return b, nil
}

// All returns the spellbook in file order (level, then name).
func (b *Book) All() []prepsvc.SpellbookEntry {
	return append([]prepsvc.SpellbookEntry(nil), b.entries...)
}

// Contains reports whether the spell with catalogue key id is in the book
// (spellsvc.Spellbook).
func (b *Book) Contains(id string) bool {
	_, ok := b.byID[id]
	return ok
}

// Get returns the entry with id.
func (b *Book) Get(id string) (prepsvc.SpellbookEntry, bool) {
	e, ok := b.byID[id]
	return e, ok
}
