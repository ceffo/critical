package combatsvc

import (
	"context"
	"fmt"
)

// ResolveSavingThrow rolls a D20 against the character's per-category target,
// applying buff SaveBonus modifiers.
func (s *service) ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory) (*RollResult, error) {
	char, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}

	target := 20
	label := ""
	switch category {
	case SaveParaPoison:
		target, label = char.SaveParaPoison, "Paralyze/Poison"
	case SaveRodStaffWand:
		target, label = char.SaveRodStaffWand, "Rod/Staff/Wand"
	case SavePetriPoly:
		target, label = char.SavePetriPoly, "Petrify/Polymorph"
	case SaveBreath:
		target, label = char.SaveBreath, "Breath Weapon"
	case SaveSpell:
		target, label = char.SaveSpell, "Spells"
	}

	trace := newRollTrace(s.dice, target)
	res := trace.Result()

	res.D20Roll = trace.RollD20()

	mods, err := s.buffs.Aggregate(ctx, characterID, char.Str)
	if err != nil {
		return nil, fmt.Errorf("aggregate buffs: %w", err)
	}
	if mods.SaveBonus != 0 {
		trace.AddModifier(ModBuff, "Buffs", mods.SaveBonus)
	}

	res.EffectiveThac0 = target

	totalRoll := res.D20Roll + mods.SaveBonus
	res.Hit = totalRoll >= target
	res.HitMargin = totalRoll - target

	res.DamageBreakdown = fmt.Sprintf("Roll: %d + Mod: %d = %d vs Target: %d (%s)",
		res.D20Roll, mods.SaveBonus, totalRoll, target, label)

	return res, nil
}
