package engine

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

type SaveCategory string

const (
	SaveParaPoison   SaveCategory = "para_poison"
	SaveRodStaffWand SaveCategory = "rod_staff_wand"
	SavePetriPoly    SaveCategory = "petri_poly"
	SaveBreath       SaveCategory = "breath"
	SaveSpell        SaveCategory = "spell"
)

func ResolveSavingThrow(dr DiceRoller, category SaveCategory, actor *domain.Actor) domain.RollResult {
	target := 20
	label := ""

	switch category {
	case SaveParaPoison:
		target = actor.Stats.SaveParaPoison
		label = "Paralyze/Poison"
	case SaveRodStaffWand:
		target = actor.Stats.SaveRodStaffWand
		label = "Rod/Staff/Wand"
	case SavePetriPoly:
		target = actor.Stats.SavePetriPoly
		label = "Petrify/Polymorph"
	case SaveBreath:
		target = actor.Stats.SaveBreath
		label = "Breath Weapon"
	case SaveSpell:
		target = actor.Stats.SaveSpell
		label = "Spells"
	}

	trace := NewRollTrace(dr, target)
	res := trace.Result()

	res.D20Roll = trace.RollD20()

	// Apply buff modifiers to saving throws
	buffMods := ApplyBuffModifiers(actor)
	if buffMods.SaveBonus != 0 {
		trace.AddModifier("Buffs", buffMods.SaveBonus)
	}

	// In AD&D, modifiers can be added to the roll or subtracted from the target.
	// We'll add them to the roll (higher is better).
	res.EffectiveThac0 = target

	totalRoll := res.D20Roll + buffMods.SaveBonus
	res.Hit = totalRoll >= target
	res.HitMargin = totalRoll - target

	// Repurpose fields for Saving Throw transparency
	res.DamageBreakdown = fmt.Sprintf("Roll: %d + Mod: %d = %d vs Target: %d (%s)",
		res.D20Roll, buffMods.SaveBonus, totalRoll, target, label)

	return *res
}
