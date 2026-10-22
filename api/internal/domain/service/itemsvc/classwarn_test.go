package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

func armour(id, column string) itemsvc.Item {
	return itemsvc.Item{ID: id, Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: column}
}

func shield(id string) itemsvc.Item {
	return itemsvc.Item{ID: id, Category: itemsvc.CategoryShield, State: itemsvc.StateWorn}
}

func weapon(id, code, weaponType string, types ...string) itemsvc.Item {
	return itemsvc.Item{
		ID: id, Category: itemsvc.CategoryWeapon, State: itemsvc.StateWielded,
		CatalogueCode: code, WeaponType: weaponType, DamageTypes: types,
	}
}

// warned is the class warnings of the pieces under the fixture catalogue's definition of class.
func warned(t testing.TB, class string, pieces ...itemsvc.Item) []itemsvc.Warning {
	return multi(t, []string{class}, pieces...)
}

// multi is the class warnings of the pieces for a character of the fixture classes cls (its
// active class entries, ADR 0042 step 16.1).
func multi(t testing.TB, cls []string, pieces ...itemsvc.Item) []itemsvc.Warning {
	limits := make([]classes.Equipment, 0, len(cls))
	for _, class := range cls {
		def, ok := cataloguetest.Definitions(t).Class(class)
		require.True(t, ok, class)
		limits = append(limits, def.Equipment)
	}
	return itemsvc.ClassWarnings(limits, ruleset.WeaponLift{}, itemsvc.OldClasses{}, pieces)
}

func w(id, code string) itemsvc.Warning { return itemsvc.Warning{ItemID: id, Code: code} }

// ADR 0033 step 20, wizard: any worn armour or shield is armour_not_allowed; a wielded weapon
// whose type is not dagger, dart or sling, or whose catalogue code is not knife or quarterstaff,
// is weapon_not_allowed. Carried pieces and unlisted homebrew types are not warned.
func TestClassWarnings_Step20_Wizard(t *testing.T) {
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed"), w("s", "armour_not_allowed")},
		warned(t, "mage", armour("p", "none_allowed"), shield("s")))
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned(t, "mage", armour("p", "leather")), "even leather")
	carried := armour("c", "metal")
	carried.State = itemsvc.StateCarried
	assert.Empty(t, warned(t, "mage", carried), "carried is not worn")

	assert.Empty(t, warned(t, "mage", weapon("d", "dagger", "dagger", "p")))
	assert.Empty(t, warned(t, "mage", weapon("sl", "sling", "sling")))
	assert.Empty(t, warned(t, "mage", weapon("k", "knife", "other_melee", "p", "s")))
	assert.Empty(t, warned(t, "mage", weapon("q", "quarterstaff", "other_melee", "b")))
	assert.Equal(t, []itemsvc.Warning{w("l", "weapon_not_allowed")}, warned(t, "mage", weapon("l", "long_sword", "long_sword", "s")))
	assert.Equal(t, []itemsvc.Warning{w("a", "weapon_not_allowed")}, warned(t, "mage", weapon("a", "battle_axe", "other_melee", "s")),
		"a catalogue weapon of an unlisted code")
	assert.Empty(t, warned(t, "mage", weapon("h", "", "other_melee", "b")), "homebrew other_melee is not settled by its type")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed")}, warned(t, "mage", weapon("h", "", "long_bow")))
	notWielded := weapon("l", "long_sword", "long_sword", "s")
	notWielded.State = itemsvc.StateCarried
	assert.Empty(t, warned(t, "mage", notWielded))
}

// ADR 0033 step 20, cleric: a wielded weapon with no b damage type is weapon_not_allowed (a P/B
// weapon such as the morning star is allowed); armour and shields are free.
func TestClassWarnings_Step20_Cleric(t *testing.T) {
	assert.Empty(t, warned(t, "cleric", armour("p", "none_allowed"), shield("s")))
	assert.Empty(t, warned(t, "cleric", weapon("m", "footmans_mace", "other_melee", "b")))
	assert.Empty(t, warned(t, "cleric", weapon("ms", "morning_star", "other_melee", "p", "b")), "P/B is allowed")
	assert.Equal(t, []itemsvc.Warning{w("s", "weapon_not_allowed")}, warned(t, "cleric", weapon("s", "long_sword", "long_sword", "s")))
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")}, warned(t, "cleric", weapon("b", "long_bow", "long_bow")), "a catalogue bow does no b")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed"), w("h", "weapon_type_missing")}, warned(t, "cleric", weapon("h", "", "", "p", "s")))
	assert.Empty(t, warned(t, "cleric", weapon("h", "", "other_melee")), "homebrew with no damage type is not settled")
}

// ADR 0033 step 20, thief: worn armour whose Table 29 column is not none, leather,
// padded_studded or elven_chain; any worn shield; a wielded weapon outside PHB p.39's list.
func TestClassWarnings_Step20_Thief(t *testing.T) {
	for _, col := range []string{"none", "leather", "padded_studded", "elven_chain"} {
		assert.Empty(t, warned(t, "thief", armour("p", col)), col)
	}
	for _, col := range []string{"metal", "none_allowed"} {
		assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned(t, "thief", armour("p", col)), col)
	}
	assert.Equal(t, []itemsvc.Warning{w("s", "armour_not_allowed")}, warned(t, "thief", shield("s")))

	for _, ok := range []itemsvc.Item{
		weapon("1", "club", "other_melee", "b"), weapon("2", "dagger", "dagger", "p"), weapon("3", "dart", "dart", "p"),
		weapon("4", "hand_crossbow", "hand_crossbow"), weapon("5", "knife", "other_melee", "p", "s"),
		weapon("6", "short_bow", "short_bow"), weapon("7", "composite_short_bow", "short_bow"), weapon("8", "sling", "sling"),
		weapon("9", "broad_sword", "other_melee", "s"), weapon("10", "long_sword", "long_sword", "s"),
		weapon("11", "short_sword", "short_sword", "p"), weapon("12", "quarterstaff", "other_melee", "b"),
	} {
		assert.Empty(t, warned(t, "thief", ok), ok.CatalogueCode)
	}
	assert.Equal(t, []itemsvc.Warning{w("a", "weapon_not_allowed")}, warned(t, "thief", weapon("a", "battle_axe", "other_melee", "s")))
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")}, warned(t, "thief", weapon("b", "long_bow", "long_bow")))
	assert.Empty(t, warned(t, "thief", weapon("h", "", "other_melee", "s")), "homebrew other_melee is not settled")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed")}, warned(t, "thief", weapon("h", "", "heavy_crossbow")))
}

// ADR 0033 step 20, bard: worn armour better than chain mail (the none_allowed column); any
// worn shield; every weapon is free.
func TestClassWarnings_Step20_Bard(t *testing.T) {
	for _, col := range []string{"none", "leather", "padded_studded", "elven_chain", "metal"} {
		assert.Empty(t, warned(t, "bard", armour("p", col)), col)
	}
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned(t, "bard", armour("p", "none_allowed")))
	assert.Equal(t, []itemsvc.Warning{w("s", "armour_not_allowed")}, warned(t, "bard", shield("s")))
	assert.Empty(t, warned(t, "bard", weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0033 step 20, fighter: none.
func TestClassWarnings_Step20_FighterNone(t *testing.T) {
	assert.Empty(t, warned(t, "fighter", armour("p", "none_allowed"), shield("s"), weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0033 step 20, everyone: a two-handed weapon wielded with a worn shield is `hands`, on
// both pieces, for every class; with no shield, or a one-handed weapon, nothing.
func TestClassWarnings_Step20_HandsEveryone(t *testing.T) {
	two := weapon("g", "two_handed_sword", "other_melee", "s")
	two.TwoHanded = true
	for _, class := range []string{"fighter", "bard", "thief", "mage", "cleric"} {
		got := warned(t, class, two, shield("s"))
		assert.Contains(t, got, w("g", "hands"), class)
		assert.Contains(t, got, w("s", "hands"), class)
	}
	assert.Empty(t, warned(t, "fighter", two), "no shield")
	assert.Empty(t, warned(t, "fighter", weapon("o", "long_sword", "long_sword", "s"), shield("s")), "one-handed")
	carriedShield := shield("s")
	carriedShield.State = itemsvc.StateCarried
	assert.Empty(t, warned(t, "fighter", two, carriedShield), "a shield not worn")
}

func launcher(id, code, weaponType, fires string) itemsvc.Item {
	it := weapon(id, code, weaponType)
	it.WeaponKind, it.Fires = "missile", fires
	return it
}

// ADR 0033 step 20, cleric and a launcher: PHB p.33 (only blunt, bludgeoning weapons) and step 4
// (a launcher deals its ammunition's damage type) make a sling, whose bullets are b, allowed and a
// bow, whose arrows pierce, not; an ammunition piece's own types win over the kind's.
func TestClassWarnings_Step20_ClericLauncherJudgedByItsAmmunition(t *testing.T) {
	assert.Empty(t, warned(t, "cleric", launcher("s", "sling", "sling", "bullet")), "bullets are b")
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")},
		warned(t, "cleric", launcher("b", "short_bow", "short_bow", "arrow")), "arrows pierce")
	arrows := itemsvc.Item{ID: "ar", Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", DamageTypes: []string{"p", "b"}}
	assert.Empty(t, warned(t, "cleric", launcher("b", "short_bow", "short_bow", "arrow"), arrows), "the ammunition piece says b")
	assert.Empty(t, warned(t, "cleric", weapon("ms", "morning_star", "other_melee", "p", "b")))
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_type_missing")}, warned(t, "cleric", weapon("h", "", "")),
		"homebrew with no damage type and no code is not settled: only its missing type is warned")
	assert.Equal(t, []itemsvc.Warning{w("d", "weapon_not_allowed")}, warned(t, "cleric", weapon("d", "dagger", "dagger", "p")))
}

// ADR 0033 step 20, thief with a shield and a quarterstaff (two-handed): armour_not_allowed on the
// shield and hands on both pieces, the staff itself being on the list.
func TestClassWarnings_Step20_ThiefShieldAndQuarterstaff(t *testing.T) {
	staff := weapon("q", "quarterstaff", "other_melee", "b")
	staff.TwoHanded = true
	got := warned(t, "thief", shield("s"), staff)
	assert.ElementsMatch(t, []itemsvc.Warning{w("s", "armour_not_allowed"), w("s", "hands"), w("q", "hands")}, got)
}

// ADR 0033 step 20, bard and thief armour by Table 29 column: plate mail (none_allowed) warns a
// bard, chain mail (metal) does not; elven chain mail does not warn a thief, chain mail does.
func TestClassWarnings_Step20_ArmourByColumn(t *testing.T) {
	assert.Equal(t, []itemsvc.Warning{w("plate_mail", "armour_not_allowed")}, warned(t, "bard", armour("plate_mail", "none_allowed")))
	assert.Empty(t, warned(t, "bard", armour("chain_mail", "metal")))
	assert.Empty(t, warned(t, "thief", armour("elven_chain_mail", "elven_chain")))
	assert.Equal(t, []itemsvc.Warning{w("chain_mail", "armour_not_allowed")}, warned(t, "thief", armour("chain_mail", "metal")))
	assert.Empty(t, warned(t, "fighter", armour("plate_mail", "none_allowed"), shield("s"), weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0037 step 8 (proposed there): a wielded weapon with no weapon type carries
// weapon_type_missing, for every class (the fighter too, whom step 20 never warns), since its
// attacks take no non-proficiency penalty until the player sets one. A typed weapon, or a
// typeless one not wielded, is not warned.
func TestClassWarnings_Step8_WeaponTypeMissing(t *testing.T) {
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		got := warned(t, class, weapon("h", "", "", "b"))
		assert.Contains(t, got, w("h", "weapon_type_missing"), class)
		assert.NotContains(t, warned(t, class, weapon("d", "dagger", "dagger", "p")), w("d", "weapon_type_missing"), class)
		carried := weapon("c", "", "", "b")
		carried.State = itemsvc.StateCarried
		assert.Empty(t, warned(t, class, carried), class)
	}
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_type_missing")}, warned(t, "fighter", weapon("h", "", "")))
}

// ADR 0040 step 1.6: a class the code has never seen is limited by its equipment alone: a test
// class that wears no armour but leather, no shield, and wields only a spear (by code) or a
// dagger (by type) is warned for the rest, and a class listing nothing is never warned.
func TestClassWarnings_Step20_ALimitComesFromTheDataOfAnyClass(t *testing.T) {
	skald := classes.Equipment{
		Armour:      []string{"leather"},
		WeaponTypes: []string{"dagger"},
		Weapons:     []classes.WeaponRef{{Code: "spear", WeaponType: "other_melee"}},
	}
	pieces := []itemsvc.Item{
		armour("a", "leather"), armour("b", "metal"), shield("s"),
		weapon("w1", "spear", "other_melee"), weapon("w2", "dagger", "dagger"), weapon("w3", "club", "other_melee"),
	}
	assert.Equal(t, []itemsvc.Warning{
		w("b", "armour_not_allowed"), w("s", "armour_not_allowed"), w("w3", "weapon_not_allowed"),
	}, itemsvc.ClassWarnings([]classes.Equipment{skald}, ruleset.WeaponLift{}, itemsvc.OldClasses{}, pieces))
	assert.Empty(t, itemsvc.ClassWarnings([]classes.Equipment{{Shields: true}}, ruleset.WeaponLift{}, itemsvc.OldClasses{}, pieces))
	assert.Empty(t, itemsvc.ClassWarnings(nil, ruleset.WeaponLift{}, itemsvc.OldClasses{}, pieces), "a class the catalogue does not define is not limited")
}

// ADR 0042 step 16.1 and the wearing half of 16.2: a multi-class character's pieces are judged
// on all its active classes together, whichever comes first. A piece is allowed when any class
// allows it (a mage/fighter wears chain and a shield, a thief/fighter wields a long sword and a
// thief/mage wears leather), except that a blunt_only class restricts every weapon to its own
// list (a fighter/cleric is warned for a long sword, not for a mace).
func TestWeapons_Step16_1_ABluntOnlyEntryRestrictsTheUnion(t *testing.T) {
	sword := weapon("l", "long_sword", "long_sword", "s")
	mace := weapon("m", "footmans_mace", "other_melee", "b")
	assert.Equal(t, []itemsvc.Warning{w("l", "weapon_not_allowed")}, multi(t, []string{"fighter", "cleric"}, sword))
	assert.Equal(t, []itemsvc.Warning{w("l", "weapon_not_allowed")}, multi(t, []string{"cleric", "fighter"}, sword))
	assert.Empty(t, multi(t, []string{"fighter", "cleric"}, mace))
	assert.Empty(t, multi(t, []string{"thief", "fighter"}, sword, weapon("b", "battle_axe", "other_melee", "s")), "the fighter allows any weapon")
	assert.Equal(t, []itemsvc.Warning{w("l", "weapon_not_allowed")}, multi(t, []string{"cleric", "thief"}, sword),
		"the thief's long sword is still not blunt")

	assert.Empty(t, multi(t, []string{"mage", "fighter"}, armour("p", "metal"), shield("s")))
	assert.Empty(t, multi(t, []string{"mage", "thief"}, armour("p", "leather")), "the thief allows leather")
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed"), w("s", "armour_not_allowed")},
		multi(t, []string{"mage", "thief"}, armour("p", "metal"), shield("s")), "neither allows metal nor a shield")
	assert.Empty(t, multi(t, []string{"mage", "thief"}, weapon("lb", "short_bow", "short_bow", "p")), "the thief's short bow")
}
