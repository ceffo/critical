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

// ListPowers returns the carried items' powers that are on at the
// character's effective STR (ADR 0016 step 13).
func (s *service) ListPowers(ctx context.Context, characterID int64) ([]ItemPower, error) {
	base, err := s.baseStr(ctx, characterID)
	if err != nil {
		return nil, err
	}
	buffs, powers, err := s.effectSources(ctx, characterID)
	if err != nil {
		return nil, err
	}
	return powersOn(buffs, powers, base), nil
}

// Aggregate evaluates all active buffs and the carried items' powers against
// the character's base STR for a roll with a weapon of kind, and returns the
// stacked modifiers. Used by combatsvc as a port (clean_architecture.md
// §4.5).
func (s *service) Aggregate(ctx context.Context, characterID int64, str int, kind WeaponKind) (BuffModifiers, error) {
	buffs, powers, err := s.effectSources(ctx, characterID)
	if err != nil {
		return BuffModifiers{}, err
	}
	return aggregate(buffs, powers, str, kind), nil
}

// effectSources reads the active buffs and the carried items' powers.
func (s *service) effectSources(ctx context.Context, characterID int64) ([]BuffEffect, []ItemPower, error) {
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, nil, fmt.Errorf("get active buffs: %w", err)
	}
	powers, err := s.items.ItemPowers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get item powers: %w", err)
	}
	return buffs, powers, nil
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
