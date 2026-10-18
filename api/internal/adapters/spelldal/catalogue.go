package spelldal

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Catalogue serves the spell catalogue from memory. The catalogue only changes
// when a server imports a new artifact at boot (ADR 0021 step 5), so each
// server reads it once, right after its own import, and never again: a page
// load (the lookup, its filters, the spell names) costs the database nothing.
// The shared PostgreSQL node is small (ADR 0024 step 2), and re-reading the
// whole catalogue and its texts on every request weighed on it for nothing.
//
// During a rolling deploy, a server of the previous version keeps serving the
// catalogue of its own artifact until it is replaced.
//
// It satisfies spellsvc.Reader and castsvc.Texts. The spells it returns share
// their texts with the cache: callers read them, never modify them.
type Catalogue struct {
	// spells are every served spell, full texts included, in the order of
	// the inner reader's Search (level, fallback-language name, class).
	spells []spellsvc.Spell
	byID   map[string]int
}

var _ spellsvc.Reader = (*Catalogue)(nil)

// LoadCatalogue reads every served spell of r once, with all its texts: one
// Search for the rows, one Details call for their texts.
func LoadCatalogue(ctx context.Context, r spellsvc.Reader) (*Catalogue, error) {
	rows, err := r.Search(ctx, spellsvc.Filter{})
	if err != nil {
		return nil, fmt.Errorf("load the spell catalogue: %w", err)
	}
	ids := make([]string, len(rows))
	for i, sp := range rows {
		ids[i] = sp.ID
	}
	full, err := r.Details(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load the spell catalogue texts: %w", err)
	}
	c := &Catalogue{spells: full, byID: make(map[string]int, len(full))}
	for i, sp := range full {
		c.byID[sp.ID] = i
	}
	return c, nil
}

// Search returns the spells of f's class (case-insensitive; empty is any)
// and levels (none is any), in the catalogue's order. The service applies the
// rest of the filter.
func (c *Catalogue) Search(_ context.Context, f spellsvc.Filter) ([]spellsvc.Spell, error) {
	class := strings.ToLower(f.Class)
	out := make([]spellsvc.Spell, 0, len(c.spells))
	for _, sp := range c.spells {
		if class != "" && strings.ToLower(sp.Class) != class {
			continue
		}
		if len(f.Levels) > 0 && !slices.Contains(f.Levels, sp.Level) {
			continue
		}
		out = append(out, sp)
	}
	return out, nil
}

// Details returns the spells of ids, in that order, skipping an id the
// catalogue does not hold.
func (c *Catalogue) Details(_ context.Context, ids []string) ([]spellsvc.Spell, error) {
	out := make([]spellsvc.Spell, 0, len(ids))
	for _, id := range ids {
		if i, ok := c.byID[id]; ok {
			out = append(out, c.spells[i])
		}
	}
	return out, nil
}

// Describe finds the spell whose catalogue key is spellID.
func (c *Catalogue) Describe(_ context.Context, spellID string) (spellsvc.Spell, bool, error) {
	i, ok := c.byID[spellID]
	if !ok {
		return spellsvc.Spell{}, false, nil
	}
	return c.spells[i], true, nil
}
