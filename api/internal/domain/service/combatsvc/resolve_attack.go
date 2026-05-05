package combatsvc

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ResolveAttack runs one full attack: hit calc, crit detection, damage roll
// with buff stacking. Outputs the same RollResult shape the previous engine
// produced (handlers map this to JSON for the frontend).
func (s *service) ResolveAttack(ctx context.Context, characterID int64, weaponID string, targetAC int) (*RollResult, error) {
	weapon, ok := s.weapons.Get(weaponID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrWeaponNotFound, weaponID)
	}
	char, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}

	trace := newRollTrace(s.dice, char.Thac0Base)
	res := trace.Result()

	res.D20Roll = trace.RollD20()
	res.D100CritRoll = trace.RollD100()

	if char.Str >= 18 {
		trace.AddModifier("Strength", 1)
	}

	res.EffectiveThac0 = res.BaseThac0 - trace.SumModifiers()
	res.TargetACNeeded = res.EffectiveThac0 - res.D20Roll
	res.Hit = res.D20Roll >= (res.EffectiveThac0 - targetAC)
	res.HitMargin = res.D20Roll - (res.EffectiveThac0 - targetAC)

	requiredToHit := res.EffectiveThac0 - targetAC
	if (res.D20Roll - res.D100CritRoll) >= requiredToHit {
		res.IsCrit = true
		_ = trace.RollD100() // second D100 — preserves prior dice-stream behavior
		res.CritMultiplier = 2
	} else {
		res.CritMultiplier = 1
	}

	if res.Hit {
		res.BaseDamage = rollDamage(s.dice, weapon.BaseDamage)

		mods, err := s.buffs.Aggregate(ctx, characterID, char.Str)
		if err != nil {
			return nil, fmt.Errorf("aggregate buffs: %w", err)
		}
		res.SpellsBuff = mods.SpellsBuff
		res.WeaponBuff = mods.WeaponBuff
		res.Multipliers = float64(res.CritMultiplier) * mods.Multipliers

		if char.Str >= 18 {
			res.StrengthBonus = 2
		}

		total := (float64(res.BaseDamage)*res.SpellsBuff+float64(res.WeaponBuff))*res.Multipliers + float64(res.StrengthBonus)
		res.TotalDamage = int(math.Floor(total))
		res.DamageBreakdown = trace.ExplainDamage()
	}

	return res, nil
}

func rollDamage(d Dice, diceStr string) int {
	parts := strings.Split(strings.ToLower(diceStr), "d")
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.Atoi(parts[0])
	sides, _ := strconv.Atoi(parts[1])
	total := 0
	for i := 0; i < num; i++ {
		total += d.RollDie(sides)
	}
	return total
}
