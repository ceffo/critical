package buffsvc

import (
	"context"
	"fmt"
)

// ListActive returns the persisted set of active buffs.
func (s *service) ListActive(ctx context.Context, characterID int64) ([]BuffEffect, error) {
	return s.dal.GetActiveBuffs(ctx, characterID)
}

// ListStatus returns the active buffs with whether each one's effects apply
// at the character's effective STR.
func (s *service) ListStatus(ctx context.Context, characterID int64) ([]BuffStatus, error) {
	base, err := s.baseStr(ctx, characterID)
	if err != nil {
		return nil, err
	}
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get active buffs: %w", err)
	}
	return statuses(buffs, base), nil
}

// GetRound returns the persisted round number.
func (s *service) GetRound(ctx context.Context, characterID int64) (int, error) {
	return s.dal.GetRoundNumber(ctx, characterID)
}

// Aggregate evaluates all active buffs against the character's base STR and
// returns the stacked modifiers. Used by combatsvc as a port
// (clean_architecture.md §4.5).
func (s *service) Aggregate(ctx context.Context, characterID int64, str int) (BuffModifiers, error) {
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return BuffModifiers{}, fmt.Errorf("get active buffs: %w", err)
	}
	return aggregate(buffs, str), nil
}

// GetEffectiveStr returns the sheet's STR and the STR with active buffs.
func (s *service) GetEffectiveStr(ctx context.Context, characterID int64) (Strength, error) {
	base, err := s.baseStr(ctx, characterID)
	if err != nil {
		return Strength{}, err
	}
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return Strength{}, fmt.Errorf("get active buffs: %w", err)
	}
	return Strength{Base: base, Effective: effectiveStr(buffs, base)}, nil
}

func (s *service) baseStr(ctx context.Context, characterID int64) (int, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return 0, fmt.Errorf("get character: %w", err)
	}
	return c.Str, nil
}
