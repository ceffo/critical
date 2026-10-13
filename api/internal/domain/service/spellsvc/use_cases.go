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

// Search reads the catalog rows matching the filter's stored columns, then
// keeps those whose typed stats match the rest of it. It never truncates: the
// whole catalog is a few hundred rows.
func (s *service) Search(ctx context.Context, f Filter) ([]Spell, error) {
	rows, err := s.reader.Search(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]Spell, 0, len(rows))
	for _, sp := range rows {
		if !f.matchStats(sp.Stats) {
			continue
		}
		out = append(out, sp)
	}
	return out, nil
}
