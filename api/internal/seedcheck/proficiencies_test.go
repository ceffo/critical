package seedcheck_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0037 step 2: seeds/proficiencies.json holds PHB Table 37 once, 65 rows, each with a known
// ability (or none), its slots, its groups, and an en and a fr name in the web's locale files.
func TestNonweaponCatalogue_Step2_Table37Rows(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir)
	require.NoError(t, err)
	rows := p.All()
	require.Len(t, rows, 65)
	byID := map[string]ruletablesvc.Proficiency{}
	multi := map[int]int{}
	for _, r := range rows {
		byID[r.ID] = r
		multi[len(r.Groups)]++
	}
	// Table 37 prints 82 lines: 65 proficiencies, Ancient History and Navigation under three
	// groups, thirteen others under two.
	assert.Equal(t, map[int]int{1: 50, 2: 13, 3: 2}, multi)
	assert.Equal(t, []string{"priest", "rogue", "wizard"}, byID["ancient_history"].Groups)
	assert.Equal(t, []string{"priest", "warrior", "wizard"}, byID["navigation"].Groups)

	sample := []ruletablesvc.Proficiency{
		{ID: "mining", Ability: "wis", Modifier: -3, Slots: 2, Groups: []string{"general"}},
		{ID: "charioteering", Ability: "dex", Modifier: 2, Slots: 1, Groups: []string{"warrior"}},
		{ID: "running", Ability: "con", Modifier: -6, Slots: 1, Groups: []string{"warrior"}},
		{ID: "weaponsmithing", Ability: "int", Modifier: -3, Slots: 3, Groups: []string{"warrior"}},
		{ID: "gem_cutting", Ability: "dex", Modifier: -2, Slots: 2, Groups: []string{"rogue", "wizard"}},
		{ID: "languages_modern", Ability: "int", Modifier: 0, Slots: 1, Groups: []string{"general"}},
		{ID: "blind_fighting", Slots: 2, Groups: []string{"rogue", "warrior"}},
		{ID: "mountaineering", Slots: 1, Groups: []string{"warrior"}},
	}
	for _, want := range sample {
		assert.Equal(t, want, byID[want.ID], want.ID)
	}
}

// The seed bard's eight proficiencies keep their ids, abilities and modifiers in the catalogue.
func TestNonweaponCatalogue_Step2_SeedBardIdsKeepTheirFigures(t *testing.T) {
	var skills struct {
		Proficiencies []struct {
			ID       string `json:"id"`
			Ability  string `json:"ability"`
			Modifier int    `json:"modifier"`
		} `json:"proficiencies"`
	}
	readJSON(t, "skills.json", &skills)
	require.Len(t, skills.Proficiencies, 8)
	p, err := ruletablesadpt.ReadProficiencies(seedsDir)
	require.NoError(t, err)
	byID := map[string]ruletablesvc.Proficiency{}
	for _, r := range p.All() {
		byID[r.ID] = r
	}
	for _, s := range skills.Proficiencies {
		got, ok := byID[s.ID]
		require.True(t, ok, s.ID)
		assert.Equal(t, s.Ability, got.Ability, s.ID)
		assert.Equal(t, s.Modifier, got.Modifier, s.ID)
	}
}

// ADR 0037 step 1: Table 38's crossovers for the five classes.
func TestProficiencyTables_Step1_Table38Crossovers(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.Crossover{
		{Class: "fighter", Groups: []string{"warrior", "general"}},
		{Class: "mage", Groups: []string{"wizard", "general"}},
		{Class: "cleric", Groups: []string{"priest", "general"}},
		{Class: "thief", Groups: []string{"rogue", "general"}},
		{Class: "bard", Groups: []string{"rogue", "warrior", "wizard", "general"}},
	}, p.Crossovers())
}

// A name is the web's i18n key skills.names.<id>, in en and fr.
func TestNonweaponCatalogue_Step2_EveryIdHasAnEnAndFrName(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir)
	require.NoError(t, err)
	for _, lang := range []string{"en", "fr"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "i18n", "locales", lang, "common.json")) //nolint:gosec // fixed test path
		require.NoError(t, err)
		var f struct {
			Skills struct {
				Names map[string]string `json:"names"`
			} `json:"skills"`
		}
		require.NoError(t, json.Unmarshal(raw, &f))
		for _, r := range p.All() {
			assert.NotEmpty(t, f.Skills.Names[r.ID], "%s name of %s", lang, r.ID)
		}
	}
}
