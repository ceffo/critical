package ruletablesadpt_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0040 step 1: the fixture's five classes parse into the registry with
// the fields read so far. Expected values are the PHB's, not the artifact's:
// the groups and prime requisites (chapter 3), the minimums (Table 13, p.25),
// the alignments (chapter 3: the thief any but lawful good, the bard partly
// neutral; the others any) and the XP of levels 5 and 6 (Tables 14, 20, 23
// and 25). A class without a field, or with a per-language name, is refused
// by the format-11 schema.
func TestClassCatalogue_Step1_EveryClassCarriesEveryField(t *testing.T) {
	defs, err := phb(t).Definitions()
	require.NoError(t, err)
	type want struct {
		group    string
		primes   []string
		mins     []classes.AbilityMinimum
		aligns   []string
		xp5, xp6 int
	}
	am := func(a string, m int) classes.AbilityMinimum { return classes.AbilityMinimum{Ability: a, Min: m} }
	page := map[string]want{
		"fighter": {"warrior", []string{"str"}, []classes.AbilityMinimum{am("str", 9)}, classes.Alignments, 16_000, 32_000},
		"mage":    {"wizard", []string{"int"}, []classes.AbilityMinimum{am("int", 9)}, classes.Alignments, 20_000, 40_000},
		"cleric":  {"priest", []string{"wis"}, []classes.AbilityMinimum{am("wis", 9)}, classes.Alignments, 13_000, 27_500},
		"thief":   {"rogue", []string{"dex"}, []classes.AbilityMinimum{am("dex", 9)}, classes.Alignments[1:], 10_000, 20_000},
		"bard": {
			"rogue",
			[]string{"dex", "cha"},
			[]classes.AbilityMinimum{am("dex", 12), am("int", 13), am("cha", 15)},
			[]string{"neutral_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "neutral_evil"},
			10_000, 20_000,
		},
	}
	assert.Equal(t, []string{"fighter", "mage", "cleric", "thief", "bard"}, defs.ClassIDs())
	for id, w := range page {
		c, err := defs.Get(id)
		require.NoError(t, err)
		assert.Equal(t, w.group, c.Group, id)
		assert.Equal(t, w.primes, c.PrimeRequisites, id)
		assert.Equal(t, w.mins, c.AbilityMinimums, id)
		assert.Equal(t, w.aligns, c.Alignments, id)
		assert.Equal(t, w.xp5, c.Experience[4], id)
		assert.Equal(t, w.xp6, c.Experience[5], id)
		assert.NotEmpty(t, c.Name["en"], id)
		assert.NotEmpty(t, c.Name["fr"], id)
	}

	raw, err := os.ReadFile(cataloguetest.Path())
	require.NoError(t, err)
	for name, mutate := range map[string]func(c map[string]any){
		"no prime requisites":              func(c map[string]any) { delete(c, "prime_requisites") },
		"no experience":                    func(c map[string]any) { delete(c, "experience") },
		"a name in a language":             func(c map[string]any) { c["name"] = "Guerrier" },
		"a field the schema does not know": func(c map[string]any) { c["title"] = "Guerrier" },
	} {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal(raw, &doc))
			mutate(doc["classes"].([]any)[0].(map[string]any))
			out, err := json.Marshal(doc)
			require.NoError(t, err)
			_, err = catalogueadpt.ParseArtifact(out)
			require.ErrorContains(t, err, "schema: at '/classes/0")
		})
	}
}

// ADR 0040 step 8: the import refuses to start on a row keyed by a class,
// group or race the definitions do not hold, and on a class whose group has no
// rows, naming the table.
func TestImport_Step8_RefusesARowForAnUnknownClass(t *testing.T) {
	tables := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a sphere row of an unknown class", func(d map[string]any) { classRow(d, "sphere_access", 0)["class"] = "paladin" }, "paladin"},
		{"a level limit of an unknown class", func(d map[string]any) { classRow(d, levelLimits, 0)["class"] = "paladin" }, `unknown "paladin"`},
		{"a level limit of an unknown race", func(d map[string]any) { classRow(d, levelLimits, 0)["race"] = "orc" }, `unknown race "orc"`},
		{"a hit dice row of an unknown group", func(d map[string]any) { classRow(d, "hit_dice", 0)["group"] = "psionicist" }, "psionicist"},
		{"a save row of an unknown group", func(d map[string]any) { classRow(d, "saving_throws", 0)["group"] = "psionicist" }, "psionicist"},
	}
	for _, tc := range tables {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classTablesOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
	defs := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a class of an unknown group", func(d map[string]any) {
			d["classes"].([]any)[0].(map[string]any)["group"] = "psionicist"
		}, `class "fighter": unknown group "psionicist"`},
		{"a class twice", func(d map[string]any) {
			d["classes"] = append(d["classes"].([]any), d["classes"].([]any)[0])
		}, `"fighter" twice`},
	}
	for _, tc := range defs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := definitionsOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// ADR 0040 step 1.6: the equipment lists map as the PHB gives them (mage p.30, cleric p.33, thief
// p.39, bard p.41; the fighter p.26 wears and wields anything) and the pick types derive from them
// and the weapon catalogue (ADR 0037 step 7).
func TestDefinitions_Step1_6_TheEquipmentListsMap(t *testing.T) {
	defs, err := phb(t).Definitions()
	require.NoError(t, err)
	eq := func(id string) classes.Equipment {
		c, err := defs.Get(id)
		require.NoError(t, err)
		return c.Equipment
	}
	assert.Equal(t, classes.Equipment{Shields: true}, eq("fighter"))
	assert.Equal(t, classes.Equipment{
		Armour: []string{"none"}, WeaponTypes: []string{"dagger", "dart", "sling"},
		Weapons:   []classes.WeaponRef{{Code: "knife", WeaponType: "other_melee"}, {Code: "quarterstaff", WeaponType: "other_melee"}},
		PickTypes: []string{"dagger", "dart", "sling", "other_melee"},
	}, eq("mage"))
	assert.Equal(t, classes.Equipment{Shields: true, BluntOnly: true, PickTypes: []string{"sling", "other_melee", "other_missile"}}, eq("cleric"))
	thief := eq("thief")
	assert.Equal(t, []string{"none", "leather", "padded_studded", "elven_chain"}, thief.Armour)
	assert.False(t, thief.Shields)
	assert.Equal(t, []string{"dagger", "dart", "hand_crossbow", "short_bow", "sling", "short_sword", "long_sword"}, thief.WeaponTypes)
	assert.Equal(t, []string{"none", "leather", "padded_studded", "elven_chain", "metal"}, eq("bard").Armour)
	assert.Nil(t, eq("bard").PickTypes, "the bard picks any weapon type")
}

// ADR 0040 steps 2 and 4: the six races carry their traits and PHB Tables 10 and 11 rows. Expected
// values are the book's: the dwarf's, gnome's and halfling's Constitution saves (Table 9, PHB
// chapter 2) and the halfling's lack of an exceptional Strength (Table 7 note); the human has no
// trait.
func TestRaceCatalogue_Step2_SixRacesWithTheirTraitsAndIdentityRows(t *testing.T) {
	defs, err := phb(t).Definitions()
	require.NoError(t, err)
	assert.Equal(t, []string{"dwarf", "elf", "gnome", "half_elf", "halfling", "human"}, defs.RaceIDs())
	kinds := map[string][]string{}
	for _, r := range defs.Races() {
		for _, tr := range r.Traits {
			kinds[r.ID] = append(kinds[r.ID], tr.Kind)
		}
	}
	assert.Equal(t, map[string][]string{
		"dwarf": {classes.TraitConSaveBonus}, "gnome": {classes.TraitConSaveBonus},
		"halfling": {classes.TraitConSaveBonus, classes.TraitNoExceptionalStrength},
	}, kinds)
	gnome, _ := defs.Race("gnome")
	tr, ok := gnome.Trait(classes.TraitConSaveBonus)
	require.True(t, ok)
	assert.Equal(t, []string{"rod_staff_wand", "spell"}, tr.Categories)
	assert.False(t, tr.Poison, "the gnome's poison save gets no Table 9 bonus")
	dwarf, _ := defs.Race("dwarf")
	assert.Equal(t, classes.Identity{
		HeightMale: 43, HeightFemale: 41, HeightDice: classes.Dice{Count: 1, Sides: 10},
		WeightMale: 130, WeightFemale: 105, WeightDice: classes.Dice{Count: 4, Sides: 10},
		StartAge: 40, StartAgeDice: classes.Dice{Count: 5, Sides: 6}, MaxAge: 250, MaxAgeDice: classes.Dice{Count: 2, Sides: 100},
	}, dwarf.Identity)
}
