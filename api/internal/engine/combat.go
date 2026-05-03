package engine

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/moncef-naji/critical/api/internal/domain"
)

func ResolveAttack(dr DiceRoller, weapon domain.Weapon, targetAC int, char domain.Character, activeBuffs []domain.BuffEffect) domain.RollResult {
	res := domain.RollResult{
		BaseThac0: char.Thac0Base,
		Modifiers: []domain.Modifier{},
	}

	// 1. Roll D20 and D100 simultaneously
	res.D20Roll = dr.RollD20()
	res.D100CritRoll = dr.RollD100()

	// 2. Calculate Effective THAC0
	// For now, let's assume no THAC0 modifiers from buffs unless specified
	// But let's check STR bonus to hit (STR 18 -> +1 to hit)
	if char.Str >= 18 {
		res.Modifiers = append(res.Modifiers, domain.Modifier{Source: "Strength", Value: 1})
	}

	sumModifiers := 0
	for _, m := range res.Modifiers {
		sumModifiers += m.Value
	}

	res.EffectiveThac0 = res.BaseThac0 - sumModifiers
	res.TargetACNeeded = res.EffectiveThac0 - res.D20Roll
	res.Hit = res.D20Roll >= (res.EffectiveThac0 - targetAC)
	res.HitMargin = res.D20Roll - (res.EffectiveThac0 - targetAC)

	// 3. Check Crit: (D20 - D100) >= required_to_hit
	// required_to_hit = EffectiveThac0 - targetAC
	requiredToHit := res.EffectiveThac0 - targetAC
	if (res.D20Roll - res.D100CritRoll) >= requiredToHit {
		res.IsCrit = true
		// Roll second D100 and lookup crit_table.json (stubbed for now)
		_ = dr.RollD100() // second D100
		// TODO: Implement crit_table.json lookup
		res.CritMultiplier = 2
	} else {
		res.CritMultiplier = 1
	}

	if res.Hit {
		// 4. Resolve Damage: ((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus
		res.BaseDamage = rollDamage(dr, weapon.BaseDamage)

		// Buff modifiers (stubbed as identity values for now as per instructions)
		res.SpellsBuff = 1.0
		res.WeaponBuff = 0
		
		// Check for specific buffs in activeBuffs (stubbed for now)
		for _, buff := range activeBuffs {
			if sb, ok := buff.Effects["spells_buff"].(float64); ok {
				res.SpellsBuff *= sb
			}
			if wb, ok := buff.Effects["weapon_buff"].(float64); ok {
				res.WeaponBuff += int(wb)
			}
			if wb, ok := buff.Effects["weapon_buff"].(int); ok {
				res.WeaponBuff += wb
			}
		}

		res.Multipliers = float64(res.CritMultiplier)
		// Check for other multipliers from buffs
		for _, buff := range activeBuffs {
			if m, ok := buff.Effects["multiplier"].(float64); ok {
				res.Multipliers *= m
			}
		}

		if char.Str >= 18 {
			res.StrengthBonus = 2
		}

		// Damage = ((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus
		total := (float64(res.BaseDamage)*res.SpellsBuff + float64(res.WeaponBuff)) * res.Multipliers + float64(res.StrengthBonus)
		res.TotalDamage = int(math.Floor(total))

		res.DamageBreakdown = fmt.Sprintf("((%d × %.1f) + %d) × %.1f + %d = %d",
			res.BaseDamage, res.SpellsBuff, res.WeaponBuff, res.Multipliers, res.StrengthBonus, res.TotalDamage)
	}

	return res
}

func rollDamage(dr DiceRoller, diceStr string) int {
	// Simple parser for "1d8", "2d6", etc.
	parts := strings.Split(strings.ToLower(diceStr), "d")
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.Atoi(parts[0])
	sides, _ := strconv.Atoi(parts[1])
	
	total := 0
	for i := 0; i < num; i++ {
		total += dr.RollDie(sides)
	}
	return total
}
