package ruletablesvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// NonweaponPool is ADR 0037 step 3's non-weapon pool with the terms of its
// formula: Available = Initial (Table 34) + LevelGained (one slot at every
// realized level divisible by PerLevels) + Intelligence (Table 4's
// "# of Lang." for the sheet's score). Spent is what the picks spend, the
// languages included, and Left = Available - Spent (negative when over
// budget).
type NonweaponPool struct {
	Initial      int
	PerLevels    int
	LevelGained  int
	Intelligence int
	Available    int
	Spent        int
	Left         int
	// Languages is the part of Spent the character's languages spend (ADR
	// 0037 step 6: one slot for each beyond the native one and Common).
	Languages int
}

// NonweaponSlots is ADR 0037 step 3 for the non-weapon pool at the realized
// level (never the target level) with intelligence the sheet's score, the
// picks spending spent slots. A level under 1 gains no slot.
func NonweaponSlots(row SlotRow, level, intelligence, spent int) NonweaponPool {
	gained := 0
	if level > 0 && row.NonweaponLevels > 0 {
		gained = level / row.NonweaponLevels
	}
	out := NonweaponPool{
		Initial: row.NonweaponInitial, PerLevels: row.NonweaponLevels, LevelGained: gained,
		Intelligence: LanguageLimit(intelligence), Spent: spent,
	}
	out.Available = out.Initial + out.LevelGained + out.Intelligence
	out.Left = out.Available - out.Spent
	return out
}

// NonweaponCost is ADR 0037 step 4: Table 37's slots when one of the
// proficiency's groups is among the class's Table 38 groups, one more
// otherwise.
func NonweaponCost(p Proficiency, classGroups []string) int {
	for _, g := range p.Groups {
		if slices.Contains(classGroups, g) {
			return p.Slots
		}
	}
	return p.Slots + 1
}

// ExtraSlots is ADR 0037 step 5: the slots a pick spends beyond its cost, each
// +1 to its check target (0 when it spends the cost or less).
func ExtraSlots(slots, cost int) int {
	return max(slots-cost, 0)
}

// WeaponPool is ADR 0037 step 3's weapon pool with the terms of its formula:
// Available = Initial (Table 34) + LevelGained (one slot at every realized
// level divisible by PerLevels). Spent is what the weapon picks spend and Left
// = Available - Spent (negative when over budget).
type WeaponPool struct {
	Initial     int
	PerLevels   int
	LevelGained int
	Available   int
	Spent       int
	Left        int
}

// WeaponSlots is ADR 0037 step 3 for the weapon pool at the realized level
// (never the target level), the picks spending spent slots. A level under 1
// gains no slot.
func WeaponSlots(row SlotRow, level, spent int) WeaponPool {
	gained := 0
	if level > 0 && row.WeaponLevels > 0 {
		gained = level / row.WeaponLevels
	}
	out := WeaponPool{Initial: row.WeaponInitial, PerLevels: row.WeaponLevels, LevelGained: gained, Spent: spent}
	out.Available = out.Initial + out.LevelGained
	out.Left = out.Available - out.Spent
	return out
}

// SlotsGainedAt is ADR 0037 step 15 on step 3's pools: the weapon and the
// non-weapon slots level grants, available at level minus available at the
// level below (the Intelligence term is the same at both). None at level 1
// (its slots are the initial ones) or under.
func SlotsGainedAt(row SlotRow, level int) (weapon, nonweapon int) {
	if level < 2 {
		return 0, 0
	}
	weapon = WeaponSlots(row, level, 0).Available - WeaponSlots(row, level-1, 0).Available
	nonweapon = NonweaponSlots(row, level, 0, 0).Available - NonweaponSlots(row, level-1, 0, 0).Available
	return weapon, nonweapon
}

// WeaponTypes is every weapon type code of ADR 0030 step 23, in display order, and the one owner
// of the list: itemsvc's weapon pieces and the weapon proficiency picks (ADR 0037 step 7) read
// it. itemsvc names each code as a constant.
var WeaponTypes = []string{
	"long_bow", "short_bow", "light_crossbow", "heavy_crossbow", "hand_crossbow", "dagger",
	"dart", "sling", "short_sword", "long_sword", "other_melee", "other_thrown", "other_missile",
}

// WeaponPickTypes is ADR 0037 step 7, derived from the class equipment lists
// (ADR 0040 step 1.6) and the weapon catalogue: for each class whose weapons
// are limited, the weapon types it may pick a proficiency in, in display
// order. A type is allowed when the class's list names it or a weapon of it,
// or when a catalogue weapon of the type is one the class may wield (for a
// blunt-only class, one dealing bludgeoning damage, a launcher by its
// ammunition). A class with no weapon limit is absent: it picks any type.
func WeaponPickTypes(defs *classes.Registry, e *Equipment) map[string][]string {
	out := map[string][]string{}
	for _, c := range defs.Classes() {
		eq := c.Equipment
		if !eq.Limited() {
			continue
		}
		allowed := map[string]bool{}
		for _, t := range eq.WeaponTypes {
			allowed[t] = true
		}
		for _, w := range eq.Weapons {
			allowed[w.WeaponType] = true
		}
		if e != nil {
			for _, w := range e.Weapons {
				if eq.AllowsWeapon(classes.Weapon{
					CatalogueCode: w.Code, WeaponType: w.WeaponType, DamageTypes: e.damageTypesOf(w),
				}) {
					allowed[w.WeaponType] = true
				}
			}
		}
		types := []string{}
		for _, t := range WeaponTypes {
			if allowed[t] {
				types = append(types, t)
			}
		}
		out[c.ID] = types
	}
	return out
}

// damageTypesOf are the damage types of a catalogue weapon: its own, or for a
// launcher those of the ammunition it fires (ADR 0033 step 4).
func (e *Equipment) damageTypesOf(w Weapon) []string {
	if len(w.Fires) == 0 {
		return w.Types
	}
	var types []string
	for _, a := range e.Ammunition {
		if a.AmmoKind == w.Fires[0] {
			types = append(types, a.Types...)
		}
	}
	if len(types) == 0 {
		return []string{"p"}
	}
	return types
}
