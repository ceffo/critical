// Package spellbookdal is the storage adapter of the spellbooks
// (spellbook_entries): the catalogue keys of the spells each character knows,
// each with its origin (ADR 0030 step 20.5). It delegates to sqlc-generated
// queries in the queries/ subpackage.
package spellbookdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// DAL is the store of the spellbooks spellbookadpt's Book reads (its Store
// port, wired by the composition root), over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// Keys returns the catalogue keys of the character's spellbook, in the
// sheet's order.
func (d *DAL) Keys(ctx context.Context, characterID int64) ([]string, error) {
	entries, err := d.Entries(ctx, characterID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return keys, nil
}

// Entries returns the character's spellbook in the sheet's order, each entry
// with its origin. An origin the domain does not know is an error naming the
// entry (the column's check refuses one).
func (d *DAL) Entries(ctx context.Context, characterID int64) ([]spellbook.Entry, error) {
	rows, err := d.q.ListSpellbook(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list spellbook of character %d: %w", characterID, err)
	}
	out := make([]spellbook.Entry, len(rows))
	for i, r := range rows {
		o := spellbook.Origin(r.Origin)
		if !o.Valid() {
			return nil, fmt.Errorf("spellbook of character %d: %q has the unknown origin %q", characterID, r.SpellKey, r.Origin)
		}
		out[i] = spellbook.Entry{Key: r.SpellKey, Origin: o, GrantLevel: int(r.GrantLevel)}
	}
	return out, nil
}

// AddEntry writes the entry at the end of the character's spellbook. A spell
// already there is left as it is, origin and all: an add never relabels an
// entry.
func (d *DAL) AddEntry(ctx context.Context, characterID int64, e spellbook.Entry) error {
	if err := d.q.AddSpellbookEntry(ctx, queries.AddSpellbookEntryParams{
		CharacterID: characterID, SpellKey: e.Key, Origin: string(e.Origin), GrantLevel: int64(e.GrantLevel),
	}); err != nil {
		return fmt.Errorf("add %q to the spellbook of character %d: %w", e.Key, characterID, err)
	}
	return nil
}

// RestoreEntry writes the entry as an undo or a redo's snapshot holds it: at
// the end of the spellbook, or, for a spell already there, in its place with
// the entry's origin and grant level.
func (d *DAL) RestoreEntry(ctx context.Context, characterID int64, e spellbook.Entry) error {
	if err := d.q.RestoreSpellbookEntry(ctx, queries.RestoreSpellbookEntryParams{
		CharacterID: characterID, SpellKey: e.Key, Origin: string(e.Origin), GrantLevel: int64(e.GrantLevel),
	}); err != nil {
		return fmt.Errorf("restore %q in the spellbook of character %d: %w", e.Key, characterID, err)
	}
	return nil
}

// RemoveEntry takes the spell out of the character's spellbook (none: no
// change).
func (d *DAL) RemoveEntry(ctx context.Context, characterID int64, spellKey string) error {
	if err := d.q.RemoveSpellbookEntry(ctx, queries.RemoveSpellbookEntryParams{CharacterID: characterID, SpellKey: spellKey}); err != nil {
		return fmt.Errorf("remove %q from the spellbook of character %d: %w", spellKey, characterID, err)
	}
	return nil
}

// LearnFailure returns the realized level the character failed to learn the
// spell at (ADR 0030 step 20.2); ok is false when there is no failure.
func (d *DAL) LearnFailure(ctx context.Context, characterID int64, spellKey string) (int, bool, error) {
	levels, err := d.q.GetLearnFailure(ctx, queries.GetLearnFailureParams{CharacterID: characterID, SpellKey: spellKey})
	if err != nil {
		return 0, false, fmt.Errorf("read learn failure of %q for character %d: %w", spellKey, characterID, err)
	}
	if len(levels) == 0 {
		return 0, false, nil
	}
	return int(levels[0]), true, nil
}

// SetLearnFailure records (or replaces) the character's failure to learn the
// spell at level.
func (d *DAL) SetLearnFailure(ctx context.Context, characterID int64, spellKey string, level int) error {
	if err := d.q.SetLearnFailure(ctx, queries.SetLearnFailureParams{CharacterID: characterID, SpellKey: spellKey, Level: int64(level)}); err != nil {
		return fmt.Errorf("set learn failure of %q for character %d: %w", spellKey, characterID, err)
	}
	return nil
}

// DeleteLearnFailure lifts the character's failure to learn the spell (none:
// no change).
func (d *DAL) DeleteLearnFailure(ctx context.Context, characterID int64, spellKey string) error {
	if err := d.q.DeleteLearnFailure(ctx, queries.DeleteLearnFailureParams{CharacterID: characterID, SpellKey: spellKey}); err != nil {
		return fmt.Errorf("delete learn failure of %q for character %d: %w", spellKey, characterID, err)
	}
	return nil
}
