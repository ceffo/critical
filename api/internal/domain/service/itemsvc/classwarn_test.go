package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func warned(class string, pieces ...itemsvc.Item) []itemsvc.Warning {
	return itemsvc.ClassWarnings(class, pieces)
}

func w(id, code string) itemsvc.Warning { return itemsvc.Warning{ItemID: id, Code: code} }

// ADR 0033 step 20, wizard: any worn armour or shield is armour_not_allowed; a wielded weapon
// whose type is not dagger, dart or sling, or whose catalogue code is not knife or quarterstaff,
// is weapon_not_allowed. Carried pieces and unlisted homebrew types are not warned.
func TestClassWarnings_Step20_Wizard(t *testing.T) {
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed"), w("s", "armour_not_allowed")},
		warned("mage", armour("p", "none_allowed"), shield("s")))
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned("mage", armour("p", "leather")), "even leather")
	carried := armour("c", "metal")
	carried.State = itemsvc.StateCarried
	assert.Empty(t, warned("mage", carried), "carried is not worn")

	assert.Empty(t, warned("mage", weapon("d", "dagger", "dagger", "p")))
	assert.Empty(t, warned("mage", weapon("sl", "sling", "sling")))
	assert.Empty(t, warned("mage", weapon("k", "knife", "other_melee", "p", "s")))
	assert.Empty(t, warned("mage", weapon("q", "quarterstaff", "other_melee", "b")))
	assert.Equal(t, []itemsvc.Warning{w("l", "weapon_not_allowed")}, warned("mage", weapon("l", "long_sword", "long_sword", "s")))
	assert.Equal(t, []itemsvc.Warning{w("a", "weapon_not_allowed")}, warned("mage", weapon("a", "battle_axe", "other_melee", "s")),
		"a catalogue weapon of an unlisted code")
	assert.Empty(t, warned("mage", weapon("h", "", "other_melee", "b")), "homebrew other_melee is not settled by its type")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed")}, warned("mage", weapon("h", "", "long_bow")))
	notWielded := weapon("l", "long_sword", "long_sword", "s")
	notWielded.State = itemsvc.StateCarried
	assert.Empty(t, warned("mage", notWielded))
}

// ADR 0033 step 20, cleric: a wielded weapon with no b damage type is weapon_not_allowed (a P/B
// weapon such as the morning star is allowed); armour and shields are free.
func TestClassWarnings_Step20_Cleric(t *testing.T) {
	assert.Empty(t, warned("cleric", armour("p", "none_allowed"), shield("s")))
	assert.Empty(t, warned("cleric", weapon("m", "footmans_mace", "other_melee", "b")))
	assert.Empty(t, warned("cleric", weapon("ms", "morning_star", "other_melee", "p", "b")), "P/B is allowed")
	assert.Equal(t, []itemsvc.Warning{w("s", "weapon_not_allowed")}, warned("cleric", weapon("s", "long_sword", "long_sword", "s")))
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")}, warned("cleric", weapon("b", "long_bow", "long_bow")), "a catalogue bow does no b")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed"), w("h", "weapon_type_missing")}, warned("cleric", weapon("h", "", "", "p", "s")))
	assert.Empty(t, warned("cleric", weapon("h", "", "other_melee")), "homebrew with no damage type is not settled")
}

// ADR 0033 step 20, thief: worn armour whose Table 29 column is not none, leather,
// padded_studded or elven_chain; any worn shield; a wielded weapon outside PHB p.39's list.
func TestClassWarnings_Step20_Thief(t *testing.T) {
	for _, col := range []string{"none", "leather", "padded_studded", "elven_chain"} {
		assert.Empty(t, warned("thief", armour("p", col)), col)
	}
	for _, col := range []string{"metal", "none_allowed"} {
		assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned("thief", armour("p", col)), col)
	}
	assert.Equal(t, []itemsvc.Warning{w("s", "armour_not_allowed")}, warned("thief", shield("s")))

	for _, ok := range []itemsvc.Item{
		weapon("1", "club", "other_melee", "b"), weapon("2", "dagger", "dagger", "p"), weapon("3", "dart", "dart", "p"),
		weapon("4", "hand_crossbow", "hand_crossbow"), weapon("5", "knife", "other_melee", "p", "s"),
		weapon("6", "short_bow", "short_bow"), weapon("7", "composite_short_bow", "short_bow"), weapon("8", "sling", "sling"),
		weapon("9", "broad_sword", "other_melee", "s"), weapon("10", "long_sword", "long_sword", "s"),
		weapon("11", "short_sword", "short_sword", "p"), weapon("12", "quarterstaff", "other_melee", "b"),
	} {
		assert.Empty(t, warned("thief", ok), ok.CatalogueCode)
	}
	assert.Equal(t, []itemsvc.Warning{w("a", "weapon_not_allowed")}, warned("thief", weapon("a", "battle_axe", "other_melee", "s")))
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")}, warned("thief", weapon("b", "long_bow", "long_bow")))
	assert.Empty(t, warned("thief", weapon("h", "", "other_melee", "s")), "homebrew other_melee is not settled")
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_not_allowed")}, warned("thief", weapon("h", "", "heavy_crossbow")))
}

// ADR 0033 step 20, bard: worn armour better than chain mail (the none_allowed column); any
// worn shield; every weapon is free.
func TestClassWarnings_Step20_Bard(t *testing.T) {
	for _, col := range []string{"none", "leather", "padded_studded", "elven_chain", "metal"} {
		assert.Empty(t, warned("bard", armour("p", col)), col)
	}
	assert.Equal(t, []itemsvc.Warning{w("p", "armour_not_allowed")}, warned("bard", armour("p", "none_allowed")))
	assert.Equal(t, []itemsvc.Warning{w("s", "armour_not_allowed")}, warned("bard", shield("s")))
	assert.Empty(t, warned("bard", weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0033 step 20, fighter: none.
func TestClassWarnings_Step20_FighterNone(t *testing.T) {
	assert.Empty(t, warned("fighter", armour("p", "none_allowed"), shield("s"), weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0033 step 20, everyone: a two-handed weapon wielded with a worn shield is `hands`, on
// both pieces, for every class; with no shield, or a one-handed weapon, nothing.
func TestClassWarnings_Step20_HandsEveryone(t *testing.T) {
	two := weapon("g", "two_handed_sword", "other_melee", "s")
	two.TwoHanded = true
	for _, class := range []string{"fighter", "bard", "thief", "mage", "cleric"} {
		got := warned(class, two, shield("s"))
		assert.Contains(t, got, w("g", "hands"), class)
		assert.Contains(t, got, w("s", "hands"), class)
	}
	assert.Empty(t, warned("fighter", two), "no shield")
	assert.Empty(t, warned("fighter", weapon("o", "long_sword", "long_sword", "s"), shield("s")), "one-handed")
	carriedShield := shield("s")
	carriedShield.State = itemsvc.StateCarried
	assert.Empty(t, warned("fighter", two, carriedShield), "a shield not worn")
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
	assert.Empty(t, warned("cleric", launcher("s", "sling", "sling", "bullet")), "bullets are b")
	assert.Equal(t, []itemsvc.Warning{w("b", "weapon_not_allowed")},
		warned("cleric", launcher("b", "short_bow", "short_bow", "arrow")), "arrows pierce")
	arrows := itemsvc.Item{ID: "ar", Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", DamageTypes: []string{"p", "b"}}
	assert.Empty(t, warned("cleric", launcher("b", "short_bow", "short_bow", "arrow"), arrows), "the ammunition piece says b")
	assert.Empty(t, warned("cleric", weapon("ms", "morning_star", "other_melee", "p", "b")))
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_type_missing")}, warned("cleric", weapon("h", "", "")),
		"homebrew with no damage type and no code is not settled: only its missing type is warned")
	assert.Equal(t, []itemsvc.Warning{w("d", "weapon_not_allowed")}, warned("cleric", weapon("d", "dagger", "dagger", "p")))
}

// ADR 0033 step 20, thief with a shield and a quarterstaff (two-handed): armour_not_allowed on the
// shield and hands on both pieces, the staff itself being on the list.
func TestClassWarnings_Step20_ThiefShieldAndQuarterstaff(t *testing.T) {
	staff := weapon("q", "quarterstaff", "other_melee", "b")
	staff.TwoHanded = true
	got := warned("thief", shield("s"), staff)
	assert.ElementsMatch(t, []itemsvc.Warning{w("s", "armour_not_allowed"), w("s", "hands"), w("q", "hands")}, got)
}

// ADR 0033 step 20, bard and thief armour by Table 29 column: plate mail (none_allowed) warns a
// bard, chain mail (metal) does not; elven chain mail does not warn a thief, chain mail does.
func TestClassWarnings_Step20_ArmourByColumn(t *testing.T) {
	assert.Equal(t, []itemsvc.Warning{w("plate_mail", "armour_not_allowed")}, warned("bard", armour("plate_mail", "none_allowed")))
	assert.Empty(t, warned("bard", armour("chain_mail", "metal")))
	assert.Empty(t, warned("thief", armour("elven_chain_mail", "elven_chain")))
	assert.Equal(t, []itemsvc.Warning{w("chain_mail", "armour_not_allowed")}, warned("thief", armour("chain_mail", "metal")))
	assert.Empty(t, warned("fighter", armour("plate_mail", "none_allowed"), shield("s"), weapon("a", "battle_axe", "other_melee", "s")))
}

// ADR 0037 step 8 (proposed there): a wielded weapon with no weapon type carries
// weapon_type_missing, for every class (the fighter too, whom step 20 never warns), since its
// attacks take no non-proficiency penalty until the player sets one. A typed weapon, or a
// typeless one not wielded, is not warned.
func TestClassWarnings_Step8_WeaponTypeMissing(t *testing.T) {
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		got := warned(class, weapon("h", "", "", "b"))
		assert.Contains(t, got, w("h", "weapon_type_missing"), class)
		assert.NotContains(t, warned(class, weapon("d", "dagger", "dagger", "p")), w("d", "weapon_type_missing"), class)
		carried := weapon("c", "", "", "b")
		carried.State = itemsvc.StateCarried
		assert.Empty(t, warned(class, carried), class)
	}
	assert.Equal(t, []itemsvc.Warning{w("h", "weapon_type_missing")}, warned("fighter", weapon("h", "", "")))
}
