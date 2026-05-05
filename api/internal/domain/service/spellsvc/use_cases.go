package spellsvc

import (
	"context"
	"fmt"
)

// GetSlots returns the persisted slot list.
func (s *service) GetSlots(ctx context.Context, characterID int64) ([]SpellSlot, error) {
	return s.dal.GetSlots(ctx, characterID)
}

// Cast marks the slot at slotIndex used. Returns ErrInvalidSlotIndex or
// ErrSlotAlreadyUsed for the corresponding business-rule violations.
func (s *service) Cast(ctx context.Context, characterID int64, slotIndex int) ([]SpellSlot, error) {
	slots, err := s.dal.GetSlots(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get slots: %w", err)
	}
	if slotIndex < 0 || slotIndex >= len(slots) {
		return nil, fmt.Errorf("%w: %d", ErrInvalidSlotIndex, slotIndex)
	}
	if slots[slotIndex].Used {
		return nil, fmt.Errorf("%w: %d", ErrSlotAlreadyUsed, slotIndex)
	}
	updated := make([]SpellSlot, len(slots))
	copy(updated, slots)
	updated[slotIndex].Used = true
	if err := s.dal.SaveSlots(ctx, characterID, updated); err != nil {
		return nil, fmt.Errorf("save slots: %w", err)
	}
	return updated, nil
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

// Search delegates to the read port.
func (s *service) Search(ctx context.Context, query, class string) ([]Spell, error) {
	return s.reader.Search(ctx, query, class)
}
