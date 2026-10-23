package ruletablesvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// NonweaponPool is ADR 0037 step 3's non-weapon pool, owned by slots (ADR 0046 step 4.1).
type NonweaponPool = slots.NonweaponPool

// NonweaponSlots is slots.Nonweapon with Table 4's "# of Lang." for the sheet's intelligence
// (ADR 0037 step 3, ADR 0046 step 4.1).
func NonweaponSlots(row SlotRow, level, intelligence, spent int) NonweaponPool {
	return slots.Nonweapon(row, level, LanguageLimit(intelligence), spent)
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

// ExtraSlots is slots.Extra (ADR 0037 step 5, ADR 0046 step 4.1).
func ExtraSlots(n, cost int) int { return slots.Extra(n, cost) }

// WeaponPool is ADR 0037 step 3's weapon pool, owned by slots (ADR 0046 step 4.1).
type WeaponPool = slots.WeaponPool

// WeaponSlots is slots.Weapon (ADR 0037 step 3, ADR 0046 step 4.1).
func WeaponSlots(row SlotRow, level, spent int) WeaponPool { return slots.Weapon(row, level, spent) }

// SlotsGainedAt is ADR 0037 step 15 on step 3's pools: the weapon and the
// non-weapon slots level grants, available at level minus available at the
// level below (the Intelligence term is the same at both). None at level 1
// (its slots are the initial ones) or under.
func SlotsGainedAt(row SlotRow, level int) (weapon, nonweapon int) {
	if level < 2 {
		return 0, 0
	}
	weapon = slots.Weapon(row, level, 0).Available - slots.Weapon(row, level-1, 0).Available
	nonweapon = slots.Nonweapon(row, level, 0, 0).Available - slots.Nonweapon(row, level-1, 0, 0).Available
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
