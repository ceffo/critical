package seedcheck_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/slots"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// ADR 0037 step 2: seeds/proficiencies.json holds PHB Table 37 once, 65 rows, each with a known
// ability (or none), its slots, its groups, and an en and a fr name in the web's locale files.
func TestNonweaponCatalogue_Step2_Table37Rows(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
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

// kelsenSheet are the seed bard's eight proficiencies as seeds/skills.json stated them before they
// became picks (ADR 0014: id, ability, modifier from his sheet), and the check target each gave
// with his scores (STR 12, DEX 17, CON 16, INT 14, WIS 8, CHA 16).
var kelsenSheet = []struct {
	id, ability      string
	modifier, target int
}{
	{"musical_instrument", "dex", -1, 16},
	{"tumbling", "dex", 0, 17},
	{"ventriloquism", "int", -2, 12},
	{"riding_land", "wis", 3, 11},
	{"appraising", "int", 0, 14},
	{"singing", "cha", 0, 16},
	{"reading_writing", "int", 1, 15},
	{"local_history", "cha", 0, 16},
}

// The seed bard's eight proficiencies keep their ids, abilities and modifiers in the catalogue.
func TestNonweaponCatalogue_Step2_SeedBardIdsKeepTheirFigures(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
	require.NoError(t, err)
	for _, s := range kelsenSheet {
		got, ok := p.Get(s.id)
		require.True(t, ok, s.id)
		assert.Equal(t, s.ability, got.Ability, s.id)
		assert.Equal(t, s.modifier, got.Modifier, s.id)
	}
}

// ADR 0037 step 16 on the seeds (the campaign's and the test fixture's): the seed bard's picks are
// his eight proficiencies at their cost (1 each, all in a bard's groups), accepted by the editor's
// checks; each check target is the one his sheet gave (no extra slot); his non-weapon pool at
// level 15 with Intelligence 14 is 3 + 3 + 4 = 10, and his eight picks with Elvish, Orcish and
// Dwarvish (step 6: a half-elf's native language is Common) spend 11: over budget, a warning he
// loads with, and a save that keeps or shrinks it is stored (ADR 0037 Consequences). The test
// fixture's Kelsen knows no language but Common: 8 of 10, 2 to spend.
func TestKelsen_Step16_PicksAndPools(t *testing.T) {
	for name, tc := range map[string]struct {
		dir   string
		langs int
		code  string
	}{
		"campaign seeds": {seedsDir, 3, skills.CodeSlotsOverBudget},
		"test fixture":   {fixtureSeedsDir, 0, skills.CodeSlotsToSpend},
	} {
		dir := tc.dir
		t.Run(name, func(t *testing.T) {
			book, err := ruletablesadpt.ReadProficiencies(dir, cataloguetest.Definitions(t))
			require.NoError(t, err)
			c, ok, err := seedfile.ReadCharacter(filepath.Join(dir, seedfile.CharacterFile))
			require.NoError(t, err)
			require.True(t, ok)
			picks := c.Picks()
			require.Len(t, picks.Nonweapon, len(kelsenSheet))
			_, err = skills.ValidPicks(book, c.Class, picks, skills.Picks{})
			require.NoError(t, err, "the editor accepts the seed's picks")

			catalogue, ok := parseTables(t, fixtureCatalogue).Languages()
			require.True(t, ok)
			langs := skills.LanguageSlots(c.Languages, catalogue.Native(c.Race))
			assert.Equal(t, tc.langs, langs, "the campaign's Elvish, Orcish and Dwarvish; Common is a half-elf's own")
			spend := skills.Spending{Picks: picks, Languages: langs}
			sheet := &charactersvc.Character{Str: c.Str, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha}
			f, ok := skills.ProficiencyFigures(book, c.Class, c.Level, c.Int, spend)
			require.True(t, ok)
			f.SetTargets(func(a string) (int, bool) { return sheet.Score(charactersvc.Ability(a)) })
			for i, s := range kelsenSheet {
				got := f.Picks[i]
				assert.Equal(t, s.id, got.ID)
				assert.Equal(t, 1, got.Cost, s.id)
				assert.Zero(t, got.Extra, s.id)
				assert.Equal(t, s.target, got.Target, "%s: the target his sheet gave", s.id)
			}
			assert.Equal(t, slots.NonweaponPool{
				Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 8 + tc.langs, Left: 2 - tc.langs, Languages: tc.langs,
			}, f.Nonweapon)
			w := skills.SlotWarnings(f)
			require.NotEmpty(t, w)
			assert.Equal(t, tc.code, w[0].Code, "a warning, never an error")
			require.NoError(t, skills.PicksWithinBudget(book, c.Class, c.Level, c.Int, spend, spend), "kept as is")
			fewer := skills.Spending{Picks: picks, Languages: max(langs-1, 0)}
			require.NoError(t, skills.PicksWithinBudget(book, c.Class, c.Level, c.Int, spend, fewer), "a language dropped")
		})
	}
}

// ADR 0037 step 1: Table 38's crossovers for the five classes and the Complete Psionics Handbook's psionicist.
func TestProficiencyTables_Step1_Table38Crossovers(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.Crossover{
		{Class: "fighter", Groups: []string{"warrior", "general"}},
		{Class: "mage", Groups: []string{"wizard", "general"}},
		{Class: "cleric", Groups: []string{"priest", "general"}},
		{Class: "thief", Groups: []string{"rogue", "general"}},
		{Class: "bard", Groups: []string{"rogue", "warrior", "wizard", "general"}},
		{Class: "psionicist", Groups: []string{"psionicist", "general"}},
	}, p.Crossovers())
}

// A name is the web's i18n key skills.names.<id>, in en and fr.
func TestNonweaponCatalogue_Step2_EveryIdHasAnEnAndFrName(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
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

// ADR 0037 step 1: PHB Table 34's four rows (p.50), read from the seed file.
func TestProficiencyTables_Step1_Table34AndTable38(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
	require.NoError(t, err)
	assert.Equal(t, []slots.Row{
		{Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
		{Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
		{Group: "psionicist", WeaponInitial: 2, WeaponLevels: 5, Penalty: -4, NonweaponInitial: 3, NonweaponLevels: 3},
	}, p.SlotRows())
	for class, group := range map[string]string{"fighter": "warrior", "mage": "wizard", "cleric": "priest", "thief": "rogue", "bard": "rogue", "psionicist": "psionicist"} {
		row, ok := p.SlotRowOf(class)
		require.True(t, ok, class)
		assert.Equal(t, group, row.Group, class)
	}
	assert.Equal(t, []string{"rogue", "warrior", "wizard", "general"}, p.GroupsOf("bard"))
}
