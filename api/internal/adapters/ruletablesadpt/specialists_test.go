package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// specialist is a row of PHB Table 22 (p.31) as the book prints it, typed here by hand: the school,
// the races, the second ability and the opposition schools, plus what ADR 0050 step 3 reads off the
// book's text around it.
type specialist struct {
	school, second string
	secondMin      int
	races, opposed []string
	multiclass     []string // multiclass_races (PHB p.31: a gnome illusionist)
	level          map[string]int
}

// mageLevels are DMG Table 7's Mage row (elf 15, half-elf 12; dwarf, gnome, halfling a dash).
var mageLevels = map[string]int{"dwarf": 0, "elf": 15, "gnome": 0, "half_elf": 12, "halfling": 0}

var specialists = map[string]specialist{
	"abjurer": {
		school: "abjuration", second: "wis", secondMin: 15, races: []string{"human"},
		opposed: []string{"alteration", "illusion"}, level: mageLevels,
	},
	"conjurer": {
		school: "conjuration_summoning", second: "con", secondMin: 15, races: []string{"human", "half_elf"},
		opposed: []string{"greater_divination", "invocation_evocation"}, level: mageLevels,
	},
	"diviner": {
		school: "greater_divination", second: "wis", secondMin: 16, races: []string{"human", "half_elf", "elf"},
		opposed: []string{"conjuration_summoning"}, level: mageLevels,
	},
	"enchanter": {
		school: "enchantment_charm", second: "cha", secondMin: 16, races: []string{"human", "half_elf", "elf"},
		opposed: []string{"invocation_evocation", "necromancy"}, level: mageLevels,
	},
	// DMG Table 7's Illusionist row: gnome 15, a dash elsewhere.
	"illusionist": {
		school: "illusion", second: "dex", secondMin: 16, races: []string{"human", "gnome"},
		opposed: []string{"necromancy", "invocation_evocation", "abjuration"}, multiclass: []string{"gnome"},
		level: map[string]int{"dwarf": 0, "elf": 0, "gnome": 15, "half_elf": 0, "halfling": 0},
	},
	"invoker": {
		school: "invocation_evocation", second: "con", secondMin: 16, races: []string{"human"},
		opposed: []string{"enchantment_charm", "conjuration_summoning"}, level: mageLevels,
	},
	"necromancer": {
		school: "necromancy", second: "wis", secondMin: 16, races: []string{"human"},
		opposed: []string{"illusion", "enchantment_charm"}, level: mageLevels,
	},
	"transmuter": {
		school: "alteration", second: "dex", secondMin: 15, races: []string{"human", "half_elf"},
		opposed: []string{"abjuration", "necromancy"}, level: mageLevels,
	},
}

// ADR 0050 step 3: each of the eight specialist wizards is the mage's definition plus Table 22's
// second ability as a minimum and a prime requisite, and the school_specialization feature with
// the table's school, opposition and races (the diviner's lesser divination too, Q3); the seven
// that are never part of a multi-class character say so, the illusionist's feature names the gnome;
// the level limits of DMG Table 7 are a row per race for each. The eight are in the catalogue and
// not creatable until the engine item (step 17.2).
func TestDefinitions_Step3_TheEightSpecialists(t *testing.T) {
	tables := cataloguetest.Tables(t)
	defs := cataloguetest.Definitions(t)
	mage, err := defs.Get("mage")
	require.NoError(t, err)
	for id, want := range specialists {
		t.Run(id, func(t *testing.T) {
			c, err := defs.Get(id)
			require.NoError(t, err)
			assert.Equal(t, mage.Group, c.Group)
			assert.Equal(t, []string{"int", want.second}, c.PrimeRequisites, "Intelligence and Table 22's ability")
			minima := c.AbilityMinimums
			assert.Contains(t, minima, classes.AbilityMinimum{Ability: "int", Min: 9}, "the mage's Intelligence 9 stays")
			assert.Contains(t, minima, classes.AbilityMinimum{Ability: want.second, Min: want.secondMin})
			assert.Len(t, minima, 2)
			assert.Equal(t, mage.SpellProgression, c.SpellProgression)
			assert.Equal(t, mage.Experience, c.Experience, "Table 20's Mage/Specialist column")
			assert.Equal(t, id != "illusionist", c.NoMultiClass)

			f, ok := c.Feature(classes.KindSchoolSpecialization, 1)
			require.True(t, ok, "lists the kind")
			schools := []string{want.school}
			if id == "diviner" {
				schools = append(schools, "lesser_divination")
			}
			assert.Equal(t, &classes.SchoolSpecialization{
				Schools: schools, OppositionSchools: want.opposed, Races: want.races, MulticlassRaces: want.multiclass,
			}, f.SchoolSpecialization)
			_, ok = c.Feature(classes.KindSpellbook, 1)
			assert.True(t, ok, "a specialist keeps a spellbook")
			assert.True(t, c.Implemented(), "creatable: the engine computes the kind (ADR 0050 step 17.2)")

			for race, level := range want.level {
				row, ok := tables.LevelLimit(race, id)
				require.True(t, ok, "%s %s has a row", race, id)
				assert.False(t, row.Unlimited, "%s %s", race, id)
				assert.Equal(t, level, row.MaxLevel, "%s %s", race, id)
			}
		})
	}
}

// ADR 0050 step 2.1 (the start-up checks, each naming the class): a school_specialization the
// schema accepts but the registry cannot (a school in both lists, a race that is defined nowhere, a
// multi-class race outside the races, lesser divination opposed, no spellbook) stops the import
// naming the class.
func TestDefinitions_Step2_ARefusedSpecializationStopsTheImport(t *testing.T) {
	transmuter := func(d map[string]any) map[string]any {
		for _, c := range d["classes"].([]any) {
			if m := c.(map[string]any); m["id"] == "transmuter" {
				return m
			}
		}
		require.FailNow(t, "the fixture has no transmuter")
		return nil
	}
	params := func(d map[string]any) map[string]any {
		for _, f := range transmuter(d)["features"].([]any) {
			if m := f.(map[string]any); m["kind"] == "school_specialization" {
				return m["params"].(map[string]any)
			}
		}
		require.FailNow(t, "the transmuter lists no school_specialization")
		return nil
	}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a school in both lists", func(d map[string]any) { params(d)["opposition_schools"] = []any{"alteration"} }, `school "alteration" is also in the opposition`},
		{"lesser divination opposed", func(d map[string]any) { params(d)["opposition_schools"] = []any{"lesser_divination"} }, "open to every wizard"},
		{"a race defined nowhere", func(d map[string]any) { params(d)["races"] = []any{"human", "orc"} }, `unknown race "orc"`},
		{"a multi-class race outside the races", func(d map[string]any) { params(d)["multiclass_races"] = []any{"gnome"} }, `multi-class race "gnome" is not among the races`},
		{"no spellbook", func(d map[string]any) {
			c := transmuter(d)
			c["features"] = []any{c["features"].([]any)[1]}
		}, "lists no spellbook"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := definitionsOf(t, tc.mutate)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `class "transmuter"`)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
