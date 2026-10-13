package spellsvc

import (
	"context"
	"fmt"
)

// GetSlots returns the persisted slot list.
func (s *service) GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error) {
	return s.dal.GetSlots(ctx, characterID)
}

// ResetSlots marks every slot unused — invoked on a long rest.
func (s *service) ResetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error) {
	slots, err := s.dal.GetSlots(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get slots: %w", err)
	}
	updated := make([]SpellSlot, len(slots))
	for i, s := range slots {
		s.Used = false
		updated[i] = s
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
// those whose typed stats match the rest of it, then cuts the requested page.
// Rows come ordered by level, name, class, so pages are stable. The typed
// filter runs before paging: Total and the pages are exact.
func (s *service) Search(ctx context.Context, f Filter, p PageRequest) (Page, error) {
	rows, err := s.reader.Search(ctx, f)
	if err != nil {
		return Page{}, err
	}
	matches := make([]Spell, 0, len(rows))
	for _, sp := range rows {
		if f.matchStats(sp.Stats) {
			matches = append(matches, sp)
		}
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
	return Page{Items: matches[start:end:end], Total: len(matches), Page: p.Page, PageSize: p.PageSize}, nil
}
