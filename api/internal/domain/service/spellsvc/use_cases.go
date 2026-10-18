package spellsvc

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

// GetSlots returns the persisted slot list.
func (s *service) GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error) {
	return s.dal.GetSlots(ctx, characterID)
}

// ResetSlots marks every slot unused and keeps the preparation (ADR 0008
// step 11); invoked on a long rest.
func (s *service) ResetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error) {
	slots, err := s.dal.GetSlots(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get slots: %w", err)
	}
	// A long rest ends the extra slots (ADR 0008 step 9).
	updated := make([]SpellSlot, 0, len(slots))
	for _, s := range slots {
		if s.Extra {
			continue
		}
		s.Used = false
		updated = append(updated, s)
	}
	if err := s.dal.SaveSlots(ctx, characterID, updated); err != nil {
		return nil, fmt.Errorf("save slots: %w", err)
	}
	return updated, nil
}

// Default and maximum page sizes of a catalog search.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// PageRequest selects one page of a search: Page is 1-based. Zero values mean
// the first page of DefaultPageSize; a size above MaxPageSize is clamped.
type PageRequest struct {
	Page     int
	PageSize int
}

// Page is one page of matches. Total counts every match after all filters,
// so the caller can always reach the rest.
type Page struct {
	Items    []Spell
	Total    int
	Page     int
	PageSize int
}

// Search reads the catalog rows matching the filter's stored columns, keeps
// those matching the rest of it, ranks them, cuts the requested page, then
// reads that page's full texts in one Details call.
// Rows come ordered by level, name, class; a search text ranks exact name
// matches first, then prefix, then substring matches (over the names in
// every language), keeping that order within a tier, so pages are
// stable. Filtering runs before paging: Total and the pages are exact.
func (s *service) Search(ctx context.Context, characterID int64, f Filter, p PageRequest) (Page, error) {
	rows, err := s.reader.Search(ctx, f)
	if err != nil {
		return Page{}, err
	}
	mine := map[string]bool{}
	if f.Mine {
		keys, err := s.book.SpellKeys(ctx, characterID)
		if err != nil {
			return Page{}, fmt.Errorf("read spellbook: %w", err)
		}
		for _, k := range keys {
			mine[k] = true
		}
	}
	type ranked struct {
		tier  Tier
		spell Spell
	}
	found := make([]ranked, 0, len(rows))
	for _, sp := range rows {
		t := f.tier(sp)
		if t == TierNone || (f.Mine && !mine[sp.ID]) || !f.matchGroups(sp) || !f.matchStats(sp.Stats) {
			continue
		}
		found = append(found, ranked{t, sp})
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Compare(a.tier, b.tier) })
	matches := make([]Spell, len(found))
	for i, r := range found {
		matches[i] = r.spell
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	p.PageSize = min(p.PageSize, MaxPageSize)
	start := min((p.Page-1)*p.PageSize, len(matches))
	end := min(start+p.PageSize, len(matches))
	ids := make([]string, 0, end-start)
	for _, sp := range matches[start:end] {
		ids = append(ids, sp.ID)
	}
	items, err := s.reader.Details(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: len(matches), Page: p.Page, PageSize: p.PageSize}, nil
}

// Get reads one catalogue spell by its id with all its texts (one Details call);
// an id the catalogue does not hold is ErrSpellNotFound.
func (s *service) Get(ctx context.Context, id string) (Spell, error) {
	items, err := s.reader.Details(ctx, []string{id})
	if err != nil {
		return Spell{}, err
	}
	if len(items) == 0 {
		return Spell{}, fmt.Errorf("%w: %s", ErrSpellNotFound, id)
	}
	return items[0], nil
}

// Names lists every catalogue spell's id and names in every language: the client
// names a spell it knows only by id (spellbook, slots, journal) from it.
func (s *service) Names(ctx context.Context) ([]SpellName, error) {
	rows, err := s.reader.Search(ctx, Filter{})
	if err != nil {
		return nil, err
	}
	out := make([]SpellName, 0, len(rows))
	for _, sp := range rows {
		out = append(out, SpellName{ID: sp.ID, Name: sp.Name})
	}
	return out, nil
}

// Levels lists the spell levels present in the catalogue, ascending, each once:
// the lookup's level filter offers only these (ADR 0027 step 13.2).
func (s *service) Levels(ctx context.Context) ([]int, error) {
	rows, err := s.reader.Search(ctx, Filter{})
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(rows))
	for _, sp := range rows {
		out = append(out, sp.Level)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}
