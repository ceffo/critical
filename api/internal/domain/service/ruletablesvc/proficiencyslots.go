package ruletablesvc

import "slices"

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

// classWeaponTypes are the weapon types (ADR 0030 step 23) each class may pick
// a weapon proficiency in (ADR 0037 step 7): a type is allowed when the
// class's PHB weapon list, as ADR 0033 step 20 reads it, names the type or a
// weapon of it; for the cleric, when a catalogue weapon of the type deals
// bludgeoning damage (a launcher by its ammunition). A class absent here (the
// fighter, PHB p.26; the bard, p.41) picks any type. seedcheck derives the
// lists again from itemsvc's step 20 table and the catalogue.
var classWeaponTypes = map[string][]string{
	// PHB p.30: dagger, staff, darts, knives, slings (the knife and the
	// quarterstaff are other_melee).
	"mage": {"dagger", "dart", "sling", "other_melee"},
	// PHB p.33: blunt, bludgeoning weapons only (club, maces, flails, morning
	// star, quarterstaff, warhammer; the sling and the staff sling by their
	// bullets).
	"cleric": {"sling", "other_melee", "other_missile"},
	// PHB p.39: club, dagger, dart, hand crossbow, knife, lasso, short bow,
	// sling, broad sword, long sword, short sword, staff.
	"thief": {
		"short_bow", "hand_crossbow", "dagger", "dart", "sling", "short_sword", "long_sword",
		"other_melee", "other_thrown",
	},
}

// ClassWeaponTypes are the weapon types class may pick (ADR 0037 step 7), in
// the weapon types' display order; nil when the class may pick any.
func ClassWeaponTypes(class string) []string {
	return slices.Clone(classWeaponTypes[class])
}

// WeaponTypeAllowed is ADR 0037 step 7: whether class may pick a weapon
// proficiency in weaponType. Wielding is never refused (ADR 0033 step 20).
func WeaponTypeAllowed(class, weaponType string) bool {
	list, limited := classWeaponTypes[class]
	return !limited || slices.Contains(list, weaponType)
}
