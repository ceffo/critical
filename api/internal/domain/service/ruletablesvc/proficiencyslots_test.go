package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0037 step 4: Table 37's slots in one of the class's Table 38 groups, one more otherwise; a
// proficiency of several groups costs the table's slots when any of them is the class's.
func TestNonweaponCost_Step4_CrossoverAddsOne(t *testing.T) {
	groups := map[string][]string{
		"fighter": {"warrior", "general"}, "mage": {"wizard", "general"}, "cleric": {"priest", "general"},
		"thief": {"rogue", "general"}, "bard": {"rogue", "warrior", "wizard", "general"},
	}
	tumbling := ruletablesvc.Proficiency{ID: "tumbling", Ability: "dex", Slots: 1, Groups: []string{"rogue"}}
	tracking := ruletablesvc.Proficiency{ID: "tracking", Ability: "wis", Slots: 2, Groups: []string{"warrior"}}
	cooking := ruletablesvc.Proficiency{ID: "cooking", Ability: "int", Slots: 1, Groups: []string{"general"}}
	ancient := ruletablesvc.Proficiency{ID: "ancient_history", Ability: "int", Modifier: -1, Slots: 1, Groups: []string{"priest", "rogue", "wizard"}}
	healing := ruletablesvc.Proficiency{ID: "healing", Ability: "wis", Modifier: -2, Slots: 2, Groups: []string{"priest"}}
	cases := []struct {
		class string
		p     ruletablesvc.Proficiency
		want  int
	}{
		{"bard", tumbling, 1},
		{"fighter", tumbling, 2},
		{"thief", tumbling, 1},
		{"mage", tumbling, 2},
		{"cleric", tumbling, 2},
		{"bard", tracking, 2},
		{"fighter", tracking, 2},
		{"thief", tracking, 3},
		{"mage", tracking, 3},
		{"cleric", tracking, 3},
		{"bard", cooking, 1},
		{"fighter", cooking, 1},
		{"thief", cooking, 1},
		{"mage", cooking, 1},
		{"cleric", cooking, 1},
		{"bard", ancient, 1},
		{"fighter", ancient, 2},
		{"thief", ancient, 1},
		{"mage", ancient, 1},
		{"cleric", ancient, 1},
		{"bard", healing, 3},
		{"fighter", healing, 3},
		{"cleric", healing, 2},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ruletablesvc.NonweaponCost(c.p, groups[c.class]), "%s %s", c.class, c.p.ID)
	}
}

// ADR 0037 step 21.2 (ADR 0040 step 1.6 as amended): the catalogue weapons each class may pick are
// derived from its equipment list and the weapon catalogue by the wielding warnings' rule, with no
// Go list of them: the mage every row of dagger, dart and sling and the knife and quarterstaff
// (p.30); the cleric each row dealing bludgeoning damage and the sling and staff sling by their
// bullets (p.33); the thief every row of its seven types and the club, knife, broad sword and
// quarterstaff (p.39). The fighter and the bard pick any weapon. A blunt-only class is judged by
// the weapon's damage types: the halberd, an other_melee row like the lucern hammer, is refused.
func TestClassWeaponCodes_Step21_2_DerivedFromTheEquipmentList(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	pick := func(class string) classes.Equipment {
		c, ok := defs.Class(class)
		require.True(t, ok, class)
		return c.Equipment
	}
	assert.ElementsMatch(t, []string{"dagger", "dart", "sling", "knife", "quarterstaff"}, pick("mage").PickCodes)
	assert.ElementsMatch(t, []string{
		"club", "footmans_flail", "horsemans_flail", "footmans_mace", "horsemans_mace", "morning_star",
		"quarterstaff", "warhammer", "bec_de_corbin", "lucern_hammer", "sling", "staff_sling",
	}, pick("cleric").PickCodes)
	assert.ElementsMatch(t, []string{
		"dagger", "dart", "hand_crossbow", "short_bow", "composite_short_bow", "sling", "short_sword", "long_sword",
		"club", "knife", "broad_sword", "quarterstaff",
	}, pick("thief").PickCodes)
	assert.Nil(t, pick("fighter").PickCodes)
	assert.Nil(t, pick("bard").PickCodes)
	assert.False(t, pick("mage").WeaponPickAllowed("long_sword"))
	assert.False(t, pick("mage").WeaponPickAllowed("heavy_horse_lance"), "a type pick of other_melee covered it")
	assert.False(t, pick("cleric").WeaponPickAllowed("halberd"))
	assert.True(t, pick("cleric").WeaponPickAllowed("lucern_hammer"))
	assert.False(t, pick("thief").WeaponPickAllowed("long_bow"))
	assert.True(t, pick("fighter").WeaponPickAllowed("heavy_horse_lance"))
	assert.True(t, pick("bard").WeaponPickAllowed("heavy_crossbow"))
}

// ADR 0037 step 21.2: the pick codes of a class the code has never seen come out of its list and
// the catalogue with no code change: a test class listing the dagger type and the spear by code
// gets the dagger row and the spear; a blunt-only one gets the blunt catalogue weapons, a launcher
// by its ammunition; a class with no weapon limit is absent from the map. Without a catalogue, a
// class gets the codes its list names.
func TestWeaponPickCodes_Step21_2_AnyClassFromItsEquipment(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	mk := func(id string, e classes.Equipment) classes.Class {
		c, _ := defs.Class("fighter")
		c.ID, c.Equipment = id, e
		return c
	}
	reg, err := classes.New([]classes.Class{
		mk("test_lister", classes.Equipment{WeaponTypes: []string{"dagger"}, Weapons: []classes.WeaponRef{{Code: "spear", WeaponType: "other_melee"}}}),
		mk("test_blunt", classes.Equipment{BluntOnly: true}),
		mk("test_free", classes.Equipment{Shields: true}),
	}, defs.GroupDefs(), defs.Races())
	require.NoError(t, err)
	eq, ok := cataloguetest.Tables(t).Equipment()
	require.True(t, ok)

	got := ruletablesvc.WeaponPickCodes(reg, eq)
	assert.Equal(t, []string{"dagger", "spear"}, got["test_lister"])
	assert.Contains(t, got["test_blunt"], "sling")
	assert.Contains(t, got["test_blunt"], "morning_star")
	assert.NotContains(t, got["test_blunt"], "halberd")
	assert.NotContains(t, got["test_blunt"], "long_bow")
	assert.NotContains(t, got, "test_free")
	assert.Equal(t, map[string][]string{"test_lister": {"spear"}, "test_blunt": {}}, ruletablesvc.WeaponPickCodes(reg, nil))
}

// ADR 0037 steps 21.1 and 21.6: the weapons a pick may name are the catalogue's rows, in the
// weapon types' display order, then the catalogue's order within a type: the long bows first (the
// composite before the plain one, as the catalogue lists them), the other_melee rows after the
// swords types, the quarterstaff and the heavy horse lance among them.
func TestPickWeapons_Step21_6_TheCatalogueRowsInTypeOrder(t *testing.T) {
	eq, ok := cataloguetest.Tables(t).Equipment()
	require.True(t, ok)
	ws := eq.PickWeapons()
	require.Len(t, ws, len(eq.Weapons))
	assert.Equal(t, classes.WeaponRef{Code: "composite_long_bow", WeaponType: "long_bow"}, ws[0])
	assert.Equal(t, classes.WeaponRef{Code: "long_bow", WeaponType: "long_bow"}, ws[1])
	order := map[string]int{}
	for i, t := range ruletablesvc.WeaponTypes {
		order[t] = i
	}
	for i := 1; i < len(ws); i++ {
		assert.LessOrEqual(t, order[ws[i-1].WeaponType], order[ws[i].WeaponType], "%v before %v", ws[i-1], ws[i])
	}
	assert.Contains(t, ws, classes.WeaponRef{Code: "heavy_horse_lance", WeaponType: "other_melee"})
	assert.Contains(t, ws, classes.WeaponRef{Code: "quarterstaff", WeaponType: "other_melee"})
	assert.Nil(t, (*ruletablesvc.Equipment)(nil).PickWeapons())
}
