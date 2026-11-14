package seedcheck_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0050 step 2: the real artifact's eight specialist wizards grant the school_specialization
// kind with PHB Table 22's row, row for row (PHB p.31; typed here by hand from the book's table, the
// diviner's lesser divination being the ADR's Q3 reading), and the fixture's copies of them are the
// real artifact's, so the tests on the fixture check the data the app uses.
func TestFeatures_Step2_TheEightSpecialistsGrantTheKind(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)

	type row struct {
		schools, races, opposed []string
		second                  string
		secondMin               int
	}
	table22 := map[string]row{
		"abjurer":     {[]string{"abjuration"}, []string{"human"}, []string{"alteration", "illusion"}, "wis", 15},
		"conjurer":    {[]string{"conjuration_summoning"}, []string{"human", "half_elf"}, []string{"greater_divination", "invocation_evocation"}, "con", 15},
		"diviner":     {[]string{"greater_divination", "lesser_divination"}, []string{"human", "half_elf", "elf"}, []string{"conjuration_summoning"}, "wis", 16},
		"enchanter":   {[]string{"enchantment_charm"}, []string{"human", "half_elf", "elf"}, []string{"invocation_evocation", "necromancy"}, "cha", 16},
		"illusionist": {[]string{"illusion"}, []string{"human", "gnome"}, []string{"necromancy", "invocation_evocation", "abjuration"}, "dex", 16},
		"invoker":     {[]string{"invocation_evocation"}, []string{"human"}, []string{"enchantment_charm", "conjuration_summoning"}, "con", 16},
		"necromancer": {[]string{"necromancy"}, []string{"human"}, []string{"illusion", "enchantment_charm"}, "wis", 16},
		"transmuter":  {[]string{"alteration"}, []string{"human", "half_elf"}, []string{"abjuration", "necromancy"}, "dex", 15},
	}
	var holders []string
	for _, c := range defs.Classes() {
		if _, ok := c.Lists(classes.KindSchoolSpecialization); ok {
			holders = append(holders, c.ID)
		}
	}
	slices.Sort(holders)
	assert.Equal(t, []string{"abjurer", "conjurer", "diviner", "enchanter", "illusionist", "invoker", "necromancer", "transmuter"}, holders,
		"exactly the eight specialists list the kind")

	for id, want := range table22 {
		c, err := defs.Get(id)
		require.NoError(t, err)
		f, ok := c.Lists(classes.KindSchoolSpecialization)
		require.True(t, ok, id)
		assert.Equal(t, want.schools, f.SchoolSpecialization.Schools, "%s: school", id)
		assert.Equal(t, want.races, f.SchoolSpecialization.Races, "%s: races", id)
		assert.Equal(t, want.opposed, f.SchoolSpecialization.OppositionSchools, "%s: opposition", id)
		assert.Contains(t, c.AbilityMinimums, classes.AbilityMinimum{Ability: want.second, Min: want.secondMin}, id)
		assert.Equal(t, []string{"int", want.second}, c.PrimeRequisites, id)
		assert.True(t, c.Implemented(), "%s is creatable (ADR 0050 step 17.2)", id)
	}

	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
	require.NoError(t, err)
	fdefs, err := fromFixture.Definitions()
	require.NoError(t, err)
	for id := range table22 {
		r, err := defs.Get(id)
		require.NoError(t, err)
		f, err := fdefs.Get(id)
		require.NoError(t, err)
		assert.Equal(t, r, f, "%s: the fixture's class is the real artifact's", id)
	}
}

// ADR 0050 step 1: every spell of the real artifact carries school codes from the nine PHB schools,
// and the spells carry all nine (the builder refuses a text it cannot read, so none is left
// untyped by accident).
func TestRealCatalogue_Step1_EverySpellsSchoolsAreTheNine(t *testing.T) {
	cat := realCatalogue(t)
	carried := map[string]bool{}
	for _, s := range cat.Spells {
		for _, c := range s.Schools {
			assert.Contains(t, classes.Schools, c, "%s", s.ID)
			carried[c] = true
		}
	}
	for _, c := range classes.Schools {
		assert.True(t, carried[c], "no spell carries the school %s", c)
	}
}

// ADR 0050 step 9: the real starting spellbook seed gives each of the eight specialists the mage's
// pre-ticked spells, Read Magic and Detect Magic (DMG p.40), which the real artifact's specialists
// accept (both lesser divination, open to every wizard: no class opposes them, or the seed would
// stop the start).
func TestStartingSeed_Step9_TheSpecialistsArePreTickedLikeTheMage(t *testing.T) {
	cat := realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	seed, err := spellbookadpt.ReadStartingSeed(seedsDir, cat, defs)
	require.NoError(t, err)
	assert.Equal(t, []string{"read_magic", "detect_magic"}, seed["mage"])
	for _, id := range []string{"abjurer", "conjurer", "diviner", "enchanter", "illusionist", "invoker", "necromancer", "transmuter"} {
		assert.Equal(t, seed["mage"], seed[id], id)
	}
}
