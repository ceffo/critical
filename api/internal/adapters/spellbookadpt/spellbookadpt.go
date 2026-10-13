// Package spellbookadpt loads the Bard's spellbook (seeds/spellbook.json) and
// serves it to prepsvc. Each entry is cross-checked against the Wizard spells
// of seeds/spells.json (name and level), so a typo or a wrong level is a
// startup error. Entities live in prepsvc; this package only maps files.
package spellbookadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// FileName and CatalogFileName are the seed files under the seeds dir.
const (
	FileName        = "spellbook.json"
	CatalogFileName = "spells.json"
)

type entryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Sheet string `json:"sheet"` // the sheet's own (French) name, kept for review
}

type catalogDTO struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// Book implements prepsvc.Spellbook.
type Book struct {
	entries []prepsvc.SpellbookEntry
	byID    map[string]prepsvc.SpellbookEntry
}

var _ prepsvc.Spellbook = (*Book)(nil)

// New loads <seedsDir>/spellbook.json and validates it against spells.json.
func New(seedsDir string) (*Book, error) {
	dir := filepath.Clean(seedsDir)
	book, err := os.ReadFile(filepath.Join(dir, FileName)) //nolint:gosec // operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("spellbook: read: %w", err)
	}
	catalog, err := os.ReadFile(filepath.Join(dir, CatalogFileName)) //nolint:gosec // operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("spellbook: read catalog: %w", err)
	}
	return Parse(book, catalog)
}

// Parse maps and validates the two seed files.
func Parse(book, catalog []byte) (*Book, error) {
	var entries []entryDTO
	dec := json.NewDecoder(bytes.NewReader(book))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("spellbook: parse: %w", err)
	}
	var spells []catalogDTO
	if err := json.Unmarshal(catalog, &spells); err != nil {
		return nil, fmt.Errorf("spellbook: parse catalog: %w", err)
	}
	levels := map[string]int{}
	for _, s := range spells {
		if s.Class == "Wizard" {
			levels[s.Name] = s.Level
		}
	}

	b := &Book{byID: map[string]prepsvc.SpellbookEntry{}}
	for _, e := range entries {
		if e.ID != spelleffectsvc.Slug(e.Name) {
			return nil, fmt.Errorf("spellbook: id %q is not the slug of %q", e.ID, e.Name)
		}
		level, ok := levels[e.Name]
		if !ok {
			return nil, fmt.Errorf("spellbook: %q is not a Wizard spell in %s", e.Name, CatalogFileName)
		}
		if level != e.Level {
			return nil, fmt.Errorf("spellbook: %q is level %d in %s, not %d", e.Name, level, CatalogFileName, e.Level)
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

// Contains reports whether the spell with unique key id is in the book
// (spellsvc.Spellbook). The book's ids are slugs of Wizard names, which are
// the Wizard spells' unique keys.
func (b *Book) Contains(id string) bool {
	_, ok := b.byID[id]
	return ok
}

// Get returns the entry with id.
func (b *Book) Get(id string) (prepsvc.SpellbookEntry, bool) {
	e, ok := b.byID[id]
	return e, ok
}
