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

// ADR 0037 step 7 (ADR 0040 step 1.6): the weapon types each class may pick are derived from its
// equipment list and the weapon catalogue, with no Go list of them: the mage's p.30, the
// cleric's p.33 (blunt weapons, a launcher by its bullets) and the thief's p.39 lists. The
// fighter and the bard pick any type. The thief's lasso is in no catalogue weapon, so its
// other_thrown type is not derived.
func TestClassWeaponTypes_Step7_DerivedFromTheEquipmentList(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	pick := func(class string) classes.Equipment {
		c, ok := defs.Class(class)
		require.True(t, ok, class)
		return c.Equipment
	}
	assert.Equal(t, []string{"dagger", "dart", "sling", "other_melee"}, pick("mage").PickTypes)
	assert.Equal(t, []string{"sling", "other_melee", "other_missile"}, pick("cleric").PickTypes)
	assert.Equal(t, []string{
		"short_bow", "hand_crossbow", "dagger", "dart", "sling", "short_sword", "long_sword", "other_melee",
	}, pick("thief").PickTypes)
	assert.Nil(t, pick("fighter").PickTypes)
	assert.Nil(t, pick("bard").PickTypes)
	assert.False(t, pick("mage").WeaponTypeAllowed("long_sword"))
	assert.False(t, pick("cleric").WeaponTypeAllowed("short_sword"))
	assert.False(t, pick("thief").WeaponTypeAllowed("long_bow"))
	assert.True(t, pick("fighter").WeaponTypeAllowed("long_bow"))
	assert.True(t, pick("bard").WeaponTypeAllowed("heavy_crossbow"))
}

// ADR 0037 step 7: the pick types of a class the code has never seen come out of its list and the
// catalogue with no code change: a test class listing the dagger type and the spear by code gets
// both types; a blunt-only one gets the types of the blunt catalogue weapons, a launcher by its
// ammunition; a class with no weapon limit is absent from the map.
func TestWeaponPickTypes_Step7_AnyClassFromItsEquipment(t *testing.T) {
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

	got := ruletablesvc.WeaponPickTypes(reg, eq)
	assert.Equal(t, map[string][]string{
		"test_lister": {"dagger", "other_melee"},
		"test_blunt":  {"sling", "other_melee", "other_missile"},
	}, got)
}
