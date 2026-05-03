package engine

import "github.com/moncef-naji/critical/api/internal/domain"

// BuffModifiers holds the aggregated values from all active buffs
type BuffModifiers struct {
	SpellsBuff  float64
	WeaponBuff  int
	Multipliers float64
}

// ApplyBuffModifiers aggregates all active buff effects
func ApplyBuffModifiers(char domain.Character, activeBuffs []domain.BuffEffect) BuffModifiers {
	mods := BuffModifiers{
		SpellsBuff:  1.0,
		WeaponBuff:  0,
		Multipliers: 1.0,
	}

	for _, buff := range activeBuffs {
		// Check requirements
		if minStr, ok := buff.Effects["min_str"].(float64); ok {
			if float64(char.Str) < minStr {
				continue
			}
		} else if minStr, ok := buff.Effects["min_str"].(int); ok {
			if char.Str < minStr {
				continue
			}
		}

		if sb, ok := buff.Effects["spells_buff"].(float64); ok {
			mods.SpellsBuff *= sb
		}
		if wb, ok := buff.Effects["weapon_buff"].(float64); ok {
			mods.WeaponBuff += int(wb)
		} else if wb, ok := buff.Effects["weapon_buff"].(int); ok {
			mods.WeaponBuff += wb
		}
		if m, ok := buff.Effects["multiplier"].(float64); ok {
			mods.Multipliers *= m
		} else if m, ok := buff.Effects["multiplier"].(int); ok {
			mods.Multipliers *= float64(m)
		}
	}

	return mods
}
