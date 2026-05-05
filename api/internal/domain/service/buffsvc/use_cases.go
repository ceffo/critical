package buffsvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListActive returns the persisted set of active buffs.
func (s *service) ListActive(ctx context.Context, characterID int64) ([]BuffEffect, error) {
	return s.dal.GetActiveBuffs(ctx, characterID)
}

// Activate appends buff to the active list. Assigns a UUID when ID is empty.
func (s *service) Activate(ctx context.Context, characterID int64, buff BuffEffect) ([]BuffEffect, error) {
	if buff.ID == "" {
		buff.ID = uuid.New().String()
	}
	current, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get active buffs: %w", err)
	}
	current = append(current, buff)
	if err := s.dal.SaveActiveBuffs(ctx, characterID, current); err != nil {
		return nil, fmt.Errorf("save active buffs: %w", err)
	}
	return current, nil
}

// Dismiss removes the buff with id. No-op when id is unknown.
func (s *service) Dismiss(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error) {
	current, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get active buffs: %w", err)
	}
	filtered := current[:0]
	for _, b := range current {
		if b.ID != buffID {
			filtered = append(filtered, b)
		}
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, filtered); err != nil {
		return nil, fmt.Errorf("save active buffs: %w", err)
	}
	return filtered, nil
}

// Trigger decrements charges on a charge-typed buff; removes it at zero.
func (s *service) Trigger(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error) {
	current, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get active buffs: %w", err)
	}
	updated := triggerCharge(current, buffID)
	if err := s.dal.SaveActiveBuffs(ctx, characterID, updated); err != nil {
		return nil, fmt.Errorf("save active buffs: %w", err)
	}
	return updated, nil
}

// GetRound returns the persisted round number.
func (s *service) GetRound(ctx context.Context, characterID int64) (int, error) {
	return s.dal.GetRoundNumber(ctx, characterID)
}

// AdvanceRound atomically increments the round counter and ages duration buffs.
// The whole operation runs inside a single tx so a crash mid-flight cannot
// leave the counter and the buffs out of sync.
func (s *service) AdvanceRound(ctx context.Context, characterID int64) (active []BuffEffect, expired []BuffEffect, round int, err error) {
	err = s.dal.WrapTx(ctx, func(tx DAL) error {
		r, txErr := tx.GetRoundNumber(ctx, characterID)
		if txErr != nil {
			return fmt.Errorf("get round: %w", txErr)
		}
		round = r + 1
		if txErr := tx.SaveRoundNumber(ctx, characterID, round); txErr != nil {
			return fmt.Errorf("save round: %w", txErr)
		}
		current, txErr := tx.GetActiveBuffs(ctx, characterID)
		if txErr != nil {
			return fmt.Errorf("get active buffs: %w", txErr)
		}
		active, expired = advanceRoundBuffs(current)
		if txErr := tx.SaveActiveBuffs(ctx, characterID, active); txErr != nil {
			return fmt.Errorf("save active buffs: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return nil, nil, 0, err
	}
	return active, expired, round, nil
}

// Aggregate evaluates all active buffs against the character's STR and returns
// the stacked modifiers. Used by combatsvc as a port (clean_architecture.md §4.5).
func (s *service) Aggregate(ctx context.Context, characterID int64, str int) (BuffModifiers, error) {
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return BuffModifiers{}, fmt.Errorf("get active buffs: %w", err)
	}
	return aggregate(buffs, str), nil
}
