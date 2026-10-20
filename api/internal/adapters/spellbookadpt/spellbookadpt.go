// Package spellbookadpt serves each character's spellbook to prepsvc and
// spellsvc, and checks the spellbook seed file (seeds/spellbook.json, the
// starting spellbook of the seed character) before the seed writes it.
//
// The spellbook is stored per character by catalogue key (Store); a Book
// names each entry from the spell catalogue. The catalogue cross-check of an
// entry (a Wizard spell of the catalogue, its English name and level) happens
// when the entry is written: the seed file is checked by ReadSeed, and a
// typo, a wrong level or an id the catalogue does not hold is a startup
// error before anything is stored. Entities live in prepsvc; this package
// only maps.
package spellbookadpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// FileName is the seed file under the seeds dir.
const FileName = "spellbook.json"

type entryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// Store reads the stored spellbooks (spellbookdal, the in-memory store).
type Store interface {
	// Keys returns the catalogue keys of the character's spellbook, in the
	// sheet's order (none for a character without a spellbook).
	Keys(ctx context.Context, characterID int64) ([]string, error)
	// Entries returns the character's spellbook in the sheet's order, each
	// entry with its origin (ADR 0030 step 20.5).
	Entries(ctx context.Context, characterID int64) ([]spellbook.Entry, error)
}

// Book implements prepsvc.Spellbook and spellsvc.Spellbook over the stored
// spellbooks, named from the catalogue.
type Book struct {
	store  Store
	spells map[string]catalogue.Spell
}

var (
	_ prepsvc.Spellbook  = (*Book)(nil)
	_ spellsvc.Spellbook = (*Book)(nil)
)

// New returns the Book over store, naming its entries from cat.
func New(store Store, cat catalogue.Catalogue) *Book {
	return &Book{store: store, spells: cat.ByID()}
}

// All returns the character's spellbook in the sheet's order, each entry
// with the catalogue's English name and level and its origin (ADR 0030 step
// 20.5). An entry the catalogue does
// not hold (the catalogue import keeps every spell a spellbook refers to, so
// only a catalogue file that dropped it) is an error naming it.
func (b *Book) All(ctx context.Context, characterID int64) ([]prepsvc.SpellbookEntry, error) {
	entries, err := b.store.Entries(ctx, characterID)
	if err != nil {
		return nil, err
	}
	out := make([]prepsvc.SpellbookEntry, 0, len(entries))
	for _, e := range entries {
		s, ok := b.spells[e.Key]
		if !ok {
			return nil, fmt.Errorf("spellbook of character %d: %q is not in the spell catalogue", characterID, e.Key)
		}
		out = append(out, prepsvc.SpellbookEntry{
			ID: e.Key, Name: s.Name.Default(), Level: s.Level, Origin: e.Origin, GrantLevel: e.GrantLevel,
		})
	}
	return out, nil
}

// SpellKeys returns the catalogue keys of the character's spellbook
// (spellsvc.Spellbook, the "Mine" filter).
func (b *Book) SpellKeys(ctx context.Context, characterID int64) ([]string, error) {
	return b.store.Keys(ctx, characterID)
}

// ReadSeed reads <seedsDir>/spellbook.json and checks it against the
// catalogue (ParseSeed). It returns the catalogue keys in the file's order.
func ReadSeed(seedsDir string, cat catalogue.Catalogue) ([]string, error) {
	book, err := os.ReadFile(filepath.Join(filepath.Clean(seedsDir), FileName)) //nolint:gosec // operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("spellbook: read: %w", err)
	}
	return ParseSeed(book, cat)
}

// ParseSeed maps a spellbook seed file and checks every entry against the
// catalogue: a Wizard spell of the catalogue, with the catalogue's English
// name and level, each once. It returns the keys in the file's order.
func ParseSeed(book []byte, cat catalogue.Catalogue) ([]string, error) {
	var entries []entryDTO
	dec := json.NewDecoder(bytes.NewReader(book))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("spellbook: parse: %w", err)
	}
	spells := cat.ByID()

	keys := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		s, ok := spells[e.ID]
		if !ok || s.Class != "Wizard" {
			return nil, fmt.Errorf("spellbook: %q is not a Wizard spell of the catalogue", e.ID)
		}
		if s.Name.Default() != e.Name { // the seed holds the English name
			return nil, fmt.Errorf("spellbook: %q is named %q in the catalogue, not %q", e.ID, s.Name.Default(), e.Name)
		}
		if s.Level != e.Level {
			return nil, fmt.Errorf("spellbook: %q is level %d in the catalogue, not %d", e.ID, s.Level, e.Level)
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("spellbook: duplicate spell %q", e.ID)
		}
		seen[e.ID] = true
		keys = append(keys, e.ID)
	}
	return keys, nil
}

// Spell is the catalogue spell id as learnsvc reads it (ADR 0030 step 20):
// its level and whether it is a wizard spell; ok is false for an id the
// catalogue does not hold.
func (b *Book) Spell(id string) (learnsvc.Spell, bool) {
	s, ok := b.spells[id]
	if !ok {
		return learnsvc.Spell{}, false
	}
	return learnsvc.Spell{Level: s.Level, Wizard: s.Class == "Wizard"}, true
}
