package ruletablesvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

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

// WeaponTypes is every weapon type code of ADR 0030 step 23, in display order, and the one owner
// of the list: itemsvc's weapon pieces and the weapon proficiency picks (ADR 0037 step 7) read
// it. itemsvc names each code as a constant.
var WeaponTypes = []string{
	"long_bow", "short_bow", "light_crossbow", "heavy_crossbow", "hand_crossbow", "dagger",
	"dart", "sling", "staff_sling", "short_sword", "long_sword", "other_melee", "other_thrown",
	"other_missile",
}

// WeaponPickCodes is ADR 0037 step 21.2, derived from the class equipment
// lists (ADR 0040 step 1.6) and the weapon catalogue: for each class whose
// weapons are limited, the catalogue weapons it may pick a proficiency in, in
// catalogue order: each row the class may wield by the rule of the wielding
// warnings (classes.Equipment.AllowsWeapon on the row: its code, its type, its
// damage types, a launcher by its ammunition's). Without a weapon catalogue,
// the codes the class's list names. A class with no weapon limit is absent: it
// picks any weapon.
func WeaponPickCodes(defs *classes.Registry, e *Equipment) map[string][]string {
	out := map[string][]string{}
	for _, c := range defs.Classes() {
		eq := c.Equipment
		if !eq.Limited() {
			continue
		}
		codes := []string{}
		if e == nil {
			for _, w := range eq.Weapons {
				codes = append(codes, w.Code)
			}
		} else {
			for _, w := range e.Weapons {
				if eq.AllowsWeapon(e.CatalogueWeapon(w)) {
					codes = append(codes, w.Code)
				}
			}
		}
		out[c.ID] = codes
	}
	return out
}

// CatalogueWeapon is a catalogue weapon row as the class limits judge it
// (ADR 0033 step 20): its code, its type and its damage types (a launcher's,
// its ammunition's).
func (e *Equipment) CatalogueWeapon(w Weapon) classes.Weapon {
	return classes.Weapon{CatalogueCode: w.Code, WeaponType: w.WeaponType, DamageTypes: e.damageTypesOf(w)}
}

// PickWeapons are the catalogue weapons a weapon pick may name (ADR 0037 step
// 21.1), with their type, in the order the picks are offered (step 21.6): the
// weapon types' display order (WeaponTypes), then the catalogue's order
// within a type. A row of a type that is no weapon type is left out. None
// without a catalogue (e nil).
func (e *Equipment) PickWeapons() []classes.WeaponRef {
	if e == nil {
		return nil
	}
	out := make([]classes.WeaponRef, 0, len(e.Weapons))
	for _, t := range WeaponTypes {
		for _, w := range e.Weapons {
			if w.WeaponType == t {
				out = append(out, classes.WeaponRef{Code: w.Code, WeaponType: w.WeaponType})
			}
		}
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
