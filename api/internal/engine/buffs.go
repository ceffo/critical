package engine

import "github.com/moncef-naji/critical/api/internal/domain"

// BuffModifiers holds the aggregated values from all active buffs
type BuffModifiers struct {
	SpellsBuff  float64
	WeaponBuff  int
	Multipliers float64
}

// EffectEngine encapsulates the logic for evaluating and stacking buff effects
// against a character's specific attributes and state.
type EffectEngine struct {
	char domain.Character
}

func NewEffectEngine(char domain.Character) *EffectEngine {
	return &EffectEngine{char: char}
}

// Aggregate evaluates all active buffs against the character's stats 
// and returns the final stacked modifiers.
func (e *EffectEngine) Aggregate(buffs []domain.BuffEffect) BuffModifiers {
	mods := BuffModifiers{
		SpellsBuff:  1.0,
		WeaponBuff:  0,
		Multipliers: 1.0,
	}

	for _, buff := range buffs {
		if !e.meetsRequirements(buff.Effects) {
			continue
		}
		e.applyEffects(&mods, buff.Effects)
	}

	return mods
}

func (e *EffectEngine) meetsRequirements(effects domain.BuffEffects) bool {
	if effects.MinStr != nil && e.char.Str < *effects.MinStr {
		return false
	}
	return true
}

func (e *EffectEngine) applyEffects(mods *BuffModifiers, effects domain.BuffEffects) {
	if effects.SpellsBuff != nil {
		mods.SpellsBuff *= *effects.SpellsBuff
	}
	if effects.WeaponBuff != nil {
		mods.WeaponBuff += *effects.WeaponBuff
	}
	if effects.Multiplier != nil {
		mods.Multipliers *= *effects.Multiplier
	}
}

// ApplyBuffModifiers is a convenience wrapper for backward compatibility.
// It uses the EffectEngine internally to ensure consistency.
func ApplyBuffModifiers(char domain.Character, activeBuffs []domain.BuffEffect) BuffModifiers {
	return NewEffectEngine(char).Aggregate(activeBuffs)
}
