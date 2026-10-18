// Package spellbookdal is the storage adapter of the spellbooks
// (spellbook_entries): the catalogue keys of the spells each character knows.
// It delegates to sqlc-generated queries in the queries/ subpackage.
package spellbookdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries"
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
	keys, err := d.q.ListSpellbook(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list spellbook of character %d: %w", characterID, err)
	}
	return keys, nil
}
