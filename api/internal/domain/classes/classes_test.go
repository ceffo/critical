package classes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// xp is an experience table of MaxLevel rows from 0, step apart.
func xp(step int) []int {
	out := make([]int, classes.MaxLevel)
	for i := range out {
		out[i] = i * step
	}
	return out
}

// skald is a class the code has never seen: a rogue with two prime
// requisites, a minimum in each and a narrowed alignment list.
func skald() classes.Class {
	return classes.Class{
		ID: "test_skald", Name: localized.Text{"en": "Skald"}, Group: "rogue",
		PrimeRequisites: []string{"cha", "str"},
		AbilityMinimums: []classes.AbilityMinimum{{Ability: "cha", Min: 14}, {Ability: "str", Min: 11}},
		Alignments:      []string{"chaotic_neutral", "chaotic_good"},
		Experience:      xp(1000),
	}
}

// identity is a race's Tables 10 and 11 rows, the figures this package's tests do not read.
func identity() classes.Identity {
	d := classes.Dice{Count: 1, Sides: 6}
	return classes.Identity{
		HeightMale: 60, HeightFemale: 59, HeightDice: d, WeightMale: 140, WeightFemale: 100, WeightDice: d,
		StartAge: 15, StartAgeDice: d, MaxAge: 90, MaxAgeDice: d,
	}
}

// groups are class groups with a flat THAC0 table, the figures this
// package's tests do not read.
func groups(ids ...string) []classes.Group {
	out := make([]classes.Group, len(ids))
	for i, id := range ids {
		out[i] = classes.Group{ID: id, THAC0: xp(0)}
	}
	return out
}

func registry(t *testing.T, cls ...classes.Class) *classes.Registry {
	t.Helper()
	r, err := classes.New(cls, groups("warrior", "rogue"), []classes.Race{{ID: "human", Identity: identity(), Name: localized.Text{"en": "Human"}}})
	require.NoError(t, err)
	return r
}

// ADR 0040 step 5: every answer about a class comes from its definition
// through the registry, so a class the code has never seen (a rogue skald)
// gets its group, prime requisites, minimums, alignments and levels with no
// code change, and a class the catalogue does not define is an
// ErrUnknownClass naming it.
func TestLookup_Step5_ANewClassOfKnownFieldsNeedsNoCode(t *testing.T) {
	plain := classes.Class{ID: "test_brute", Group: "warrior", PrimeRequisites: []string{"str"}, Experience: xp(2000)}
	r := registry(t, plain, skald())

	assert.Equal(t, []string{"test_brute", "test_skald"}, r.ClassIDs(), "catalogue order")
	g, err := r.Group("test_skald")
	require.NoError(t, err)
	assert.Equal(t, "rogue", g)

	c, err := r.Get("test_skald")
	require.NoError(t, err)
	assert.Equal(t, []string{"cha", "str"}, c.PrimeRequisites)
	assert.Equal(t, []classes.AbilityMinimum{{Ability: "str", Min: 11}, {Ability: "cha", Min: 14}}, c.AbilityMinimums,
		"the minimums in the sheet's ability order")
	assert.Equal(t, []string{"chaotic_good", "chaotic_neutral"}, c.Alignments, "the book's alignment order")
	assert.True(t, c.AlignmentAllowed("chaotic_good"))
	assert.False(t, c.AlignmentAllowed("lawful_good"))
	assert.Equal(t, []string{"str"}, c.UnmetMinimums(map[string]int{"str": 10, "cha": 14}), "cha 14 meets its minimum")
	assert.Empty(t, c.UnmetMinimums(map[string]int{"dex": 3}), "an ability with no score yet is not unmet")
	assert.Equal(t, 1, c.LevelFor(999))
	assert.Equal(t, 2, c.LevelFor(1000))
	assert.Equal(t, classes.MaxLevel, c.LevelFor(1_000_000))
	th, err := c.XPThreshold(5)
	require.NoError(t, err)
	assert.Equal(t, 4000, th)
	_, err = c.XPThreshold(classes.MaxLevel + 1)
	require.Error(t, err)

	b, ok := r.Class("test_brute")
	require.True(t, ok)
	assert.Equal(t, classes.Alignments, b.Alignments, "no list: every alignment")

	_, err = r.Get("paladin")
	require.ErrorIs(t, err, classes.ErrUnknownClass)
	require.ErrorContains(t, err, `"paladin"`)
	_, err = r.Group("paladin")
	require.ErrorIs(t, err, classes.ErrUnknownClass)
}

// ADR 0040 step 5: the registry hands out copies; a reader editing a list
// changes nothing for the next reader.
func TestLookup_Step5_TheDefinitionsAreCopies(t *testing.T) {
	r := registry(t, skald())
	c, _ := r.Class("test_skald")
	c.PrimeRequisites[0], c.Experience[1] = "dex", 7
	again, _ := r.Class("test_skald")
	assert.Equal(t, []string{"cha", "str"}, again.PrimeRequisites)
	assert.Equal(t, 1000, again.Experience[1])
}

// ADR 0040 step 8, the rules New holds (the importer runs it at start-up):
// each refusal names the class and what is wrong.
func TestImport_Step8_TheRegistryRefusesBrokenDefinitions(t *testing.T) {
	with := func(edit func(*classes.Class)) classes.Class {
		c := skald()
		edit(&c)
		return c
	}
	tests := []struct {
		name string
		cls  []classes.Class
		want string
	}{
		{"an unknown group", []classes.Class{with(func(c *classes.Class) { c.Group = "psionicist" })}, `class "test_skald": unknown group "psionicist"`},
		{"no prime requisite", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = nil })}, "no prime requisite"},
		{"an unknown ability", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = []string{"luck"} })}, `unknown ability "luck"`},
		{"a prime requisite twice", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = []string{"str", "str"} })}, `"str" twice`},
		{"a minimum of an unknown ability", []classes.Class{with(func(c *classes.Class) {
			c.AbilityMinimums = []classes.AbilityMinimum{{Ability: "luck", Min: 9}}
		})}, "ability minimums: an unknown ability"},
		{"an unknown alignment", []classes.Class{with(func(c *classes.Class) { c.Alignments = []string{"neutral"} })}, `unknown alignment "neutral"`},
		{"19 experience rows", []classes.Class{with(func(c *classes.Class) { c.Experience = c.Experience[:19] })}, "got 19 rows"},
		{"a class twice", []classes.Class{skald(), skald()}, `classes: "test_skald" twice`},
		{"a class without an id", []classes.Class{with(func(c *classes.Class) { c.ID = "" })}, "an empty id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classes.New(tc.cls, groups("warrior", "rogue"), []classes.Race{{ID: "human", Identity: identity()}})
			require.ErrorContains(t, err, tc.want)
		})
	}
	_, err := classes.New(nil, groups("rogue"), []classes.Race{{ID: "human", Identity: identity()}})
	require.ErrorContains(t, err, "needs classes, groups and races")
	_, err = classes.New([]classes.Class{skald()}, groups("rogue"), []classes.Race{{ID: "elf", Identity: identity()}, {ID: "elf", Identity: identity()}})
	require.ErrorContains(t, err, `races: "elf" twice`)

	// A group with no THAC0 rows (a new catalogue group without its Table 53
	// column) is refused here, at start-up, not at the first Derive.
	_, err = classes.New([]classes.Class{skald()}, []classes.Group{{ID: "rogue"}}, []classes.Race{{ID: "human", Identity: identity()}})
	require.ErrorContains(t, err, `group "rogue": THAC0 needs levels 1..20, got 0 rows`)

	for name, req := range map[string]classes.AbilityRange{
		`requirements: unknown ability "luck"`:             {Ability: "luck", Min: 3, Max: 18},
		"requirements: str minimum 18 above its maximum 3": {Ability: "str", Min: 18, Max: 3},
	} {
		_, err = classes.New([]classes.Class{skald()}, groups("rogue"), []classes.Race{{ID: "elf", Identity: identity(), Requirements: []classes.AbilityRange{req}}})
		require.ErrorContains(t, err, `race "elf": `+name)
	}
}

// ADR 0040 step 7: a group's THAC0 is its definition's row, so a new group
// is a definition, never a column; a race is restricted (it has racial
// tables) when its definition carries ability requirements.
func TestLookup_Step7_THAC0AndRacesComeFromTheDefinitions(t *testing.T) {
	thac0 := make([]int, classes.MaxLevel)
	for i := range thac0 {
		thac0[i] = 21 - i
	}
	r, err := classes.New([]classes.Class{skald()}, []classes.Group{{ID: "rogue", THAC0: thac0}},
		[]classes.Race{{ID: "test_dwarf", Identity: identity(), Requirements: []classes.AbilityRange{{Ability: "con", Min: 11, Max: 18}}}, {ID: "test_human", Identity: identity()}})
	require.NoError(t, err)

	got, err := r.THAC0("rogue", 5)
	require.NoError(t, err)
	assert.Equal(t, 17, got)
	_, err = r.THAC0("rogue", 21)
	require.ErrorContains(t, err, "level 21 out of range 1..20")
	_, err = r.THAC0("psionicist", 1)
	require.ErrorContains(t, err, `unknown class group "psionicist"`)

	dwarf, ok := r.Race("test_dwarf")
	require.True(t, ok)
	assert.True(t, dwarf.Restricted())
	human, ok := r.Race("test_human")
	require.True(t, ok)
	assert.False(t, human.Restricted())
	_, ok = r.Race("orc")
	assert.False(t, ok)
}

// ADR 0040 step 1.6: the equipment of a class the code has never seen answers the limits.
func TestEquipment_Step1_6_TheListsAnswerTheLimits(t *testing.T) {
	none := classes.Equipment{Armour: []string{classes.ArmourNone}}
	assert.True(t, none.NoArmour())
	assert.False(t, none.AllowsArmour("leather"))
	assert.False(t, none.AllowsArmour("none"), "no armour at all, even a worn piece of the none column")
	some := classes.Equipment{Armour: []string{"none", "leather"}}
	assert.False(t, some.NoArmour())
	assert.True(t, some.AllowsArmour("leather"))
	assert.False(t, some.AllowsArmour("metal"))
	assert.True(t, classes.Equipment{}.AllowsArmour("metal"), "absent: any armour")

	blunt := classes.Equipment{BluntOnly: true}
	assert.True(t, blunt.AllowsWeapon(classes.Weapon{CatalogueCode: "mace", WeaponType: "other_melee", DamageTypes: []string{"b", "p"}}))
	assert.False(t, blunt.AllowsWeapon(classes.Weapon{CatalogueCode: "bow", WeaponType: "long_bow", DamageTypes: []string{"p"}}))
	assert.True(t, blunt.AllowsWeapon(classes.Weapon{WeaponType: "other_melee"}), "a bare homebrew melee weapon is not settled")
	assert.False(t, blunt.AllowsWeapon(classes.Weapon{WeaponType: "long_bow"}))
	assert.False(t, blunt.AllowsWeapon(classes.Weapon{CatalogueCode: "club", WeaponType: "other_melee"}), "a catalogue row without a damage type")
	assert.True(t, blunt.Limited())
	assert.False(t, classes.Equipment{Shields: true}.Limited())

	assert.True(t, classes.Equipment{PickTypes: []string{"dagger"}}.WeaponTypeAllowed("dagger"))
	assert.False(t, classes.Equipment{PickTypes: []string{"dagger"}}.WeaponTypeAllowed("sling"))
	assert.True(t, classes.Equipment{}.WeaponTypeAllowed("sling"), "no derived list: any type")
}

// ADR 0040 step 8: a broken equipment list or race trait is refused at start.
func TestImport_Step8_TheRegistryRefusesBrokenEquipmentAndTraits(t *testing.T) {
	withEquipment := func(e classes.Equipment) error {
		c := skald()
		c.Equipment = e
		_, err := classes.New([]classes.Class{c}, groups("rogue"), []classes.Race{{ID: "human", Identity: identity()}})
		return err
	}
	require.ErrorContains(t, withEquipment(classes.Equipment{Armour: []string{"leather", "leather"}}), "armour")
	require.ErrorContains(t, withEquipment(classes.Equipment{Weapons: []classes.WeaponRef{{Code: "spear"}}}), "a weapon without a type")
	require.ErrorContains(t, withEquipment(classes.Equipment{BluntOnly: true, WeaponTypes: []string{"dagger"}}), "blunt_only")
	require.NoError(t, withEquipment(classes.Equipment{Shields: true}))

	withTrait := func(tr classes.Trait) error {
		_, err := classes.New([]classes.Class{skald()}, groups("rogue"), []classes.Race{{ID: "human", Identity: identity(), Traits: []classes.Trait{tr}}})
		return err
	}
	require.ErrorContains(t, withTrait(classes.Trait{Kind: "flies"}), `unknown trait kind "flies"`)
	require.ErrorContains(t, withTrait(classes.Trait{Kind: classes.TraitConSaveBonus}), "missing")
	require.ErrorContains(t, withTrait(classes.Trait{Kind: classes.TraitConSaveBonus, Categories: []string{"luck"}}), "unknown save category")
	require.ErrorContains(t, withTrait(classes.Trait{Kind: classes.TraitNoExceptionalStrength, Poison: true}), "unexpected")
	require.NoError(t, withTrait(classes.Trait{Kind: classes.TraitConSaveBonus, Categories: []string{"spell"}}))
	_, err := classes.New([]classes.Class{skald()}, groups("rogue"), []classes.Race{{ID: "human"}})
	require.ErrorContains(t, err, "identity")
}
