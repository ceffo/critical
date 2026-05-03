package engine

import (
	"math"
	"strconv"
	"strings"

	"github.com/moncef-naji/critical/api/internal/domain"
)

func ResolveAttack(dr DiceRoller, weapon domain.Weapon, targetAC int, actor *domain.Actor) domain.RollResult {
	trace := NewRollTrace(dr, actor.Stats.Thac0Base)
	res := trace.Result()

	// 1. Roll D20 and D100 simultaneously
	res.D20Roll = trace.RollD20()
	res.D100CritRoll = trace.RollD100()

	// 2. Calculate Effective THAC0
	if actor.Stats.Str >= 18 {
		trace.AddModifier("Strength", 1)
	}

	res.EffectiveThac0 = res.BaseThac0 - trace.SumModifiers()
	res.TargetACNeeded = res.EffectiveThac0 - res.D20Roll
	res.Hit = res.D20Roll >= (res.EffectiveThac0 - targetAC)
	res.HitMargin = res.D20Roll - (res.EffectiveThac0 - targetAC)

	// 3. Check Crit: (D20 - D100) >= required_to_hit
	requiredToHit := res.EffectiveThac0 - targetAC
	if (res.D20Roll - res.D100CritRoll) >= requiredToHit {
		res.IsCrit = true
		_ = trace.RollD100() // second D100
		res.CritMultiplier = 2
	} else {
		res.CritMultiplier = 1
	}

	if res.Hit {
		// 4. Resolve Damage: ((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus
		res.BaseDamage = rollDamage(dr, weapon.BaseDamage)

		buffMods := ApplyBuffModifiers(actor)
		res.SpellsBuff = buffMods.SpellsBuff
		res.WeaponBuff = buffMods.WeaponBuff
		res.Multipliers = float64(res.CritMultiplier) * buffMods.Multipliers

		if actor.Stats.Str >= 18 {
			res.StrengthBonus = 2
		}

		// Damage = ((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus
		total := (float64(res.BaseDamage)*res.SpellsBuff + float64(res.WeaponBuff)) * res.Multipliers + float64(res.StrengthBonus)
		res.TotalDamage = int(math.Floor(total))

		res.DamageBreakdown = trace.ExplainDamage()
	}

	return *res
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
