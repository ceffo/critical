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

// ADR 0037 step 2: seeds/proficiencies.json holds PHB Table 37 once (65 rows) and the psionicist's Rejuvenation (ADR 0047 item 6), each with a known
// ability (or none), its slots, its groups, and an en and a fr name in the web's locale files.
func TestNonweaponCatalogue_Step2_Table37Rows(t *testing.T) {
	p, err := ruletablesadpt.ReadProficiencies(seedsDir, cataloguetest.Definitions(t))
	require.NoError(t, err)
	rows := p.All()
	require.Len(t, rows, 66)
	byID := map[string]ruletablesvc.Proficiency{}
	multi := map[int]int{}
	for _, r := range rows {
		byID[r.ID] = r
		multi[len(r.Groups)]++
	}
	// Table 37 prints 82 lines: 65 proficiencies, Ancient History and Navigation under three
	// groups, thirteen others under two.
	assert.Equal(t, map[int]int{1: 51, 2: 13, 3: 2}, multi)
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
		// CPsiH Table 10 (ADR 0047 item 6).
		{ID: "rejuvenation", Ability: "wis", Modifier: -1, Slots: 1, Groups: []string{"psionicist"}},
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
// his eight proficiencies at their cost (1 each, all in a bard's groups; since step 22.7 the four a
// bard gets free at 0, cost 0), accepted by the editor's checks; each check target is the one his
// sheet gave (no extra slot). His pools are TestKelsen_Step22_7_FourFreePicksAndThreeToSpend's.
func TestKelsen_Step16_PicksAndPools(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": seedsDir, "test fixture": fixtureSeedsDir} {
		t.Run(name, func(t *testing.T) {
			book, c, f, _ := kelsenFigures(t, dir)
			_, err := skills.ValidPicks(book, c.Class, c.Level, c.Picks(), skills.Picks{})
			require.NoError(t, err, "the editor accepts the seed's picks")
			for i, s := range kelsenSheet {
				got := f.Picks[i]
				assert.Equal(t, s.id, got.ID)
				assert.Equal(t, freeOfTheBard[s.id], got.Free, s.id)
				assert.Equal(t, got.Slots, got.Cost, "%s: at its cost", s.id)
				assert.Zero(t, got.Extra, s.id)
				assert.Equal(t, s.target, got.Target, "%s: the target his sheet gave", s.id)
			}
		})
	}
}

// freeOfTheBard are the ids a bard gets free (ADR 0037 step 22.1), as the seeds' Kelsen holds them.
var freeOfTheBard = map[string]bool{"reading_writing": true, "local_history": true, "singing": true, "musical_instrument": true}

// kelsenFigures reads the seed bard of dir and his proficiency figures at his level, the check
// targets set, with the language slots he spends.
func kelsenFigures(t *testing.T, dir string) (skills.Book, seedfile.Character, skills.ProficiencySlots, int) {
	t.Helper()
	book := weaponBook(t, dir)
	c, ok, err := seedfile.ReadCharacter(filepath.Join(dir, seedfile.CharacterFile))
	require.NoError(t, err)
	require.True(t, ok)
	picks := c.Picks()
	require.Len(t, picks.Nonweapon, len(kelsenSheet))
	catalogue, ok := parseTables(t, fixtureCatalogue).Languages()
	require.True(t, ok)
	langs := skills.LanguageSlots(c.Languages, catalogue.Native(c.Race))
	sheet := &charactersvc.Character{Str: c.Str, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha}
	f, ok := skills.ProficiencyFigures(book, c.Class, c.Level, c.Int, skills.Spending{Picks: picks, Languages: langs})
	require.True(t, ok)
	f.SetTargets(func(a string) (int, bool) { return sheet.Score(charactersvc.Ability(a)) })
	return book, c, f, langs
}

// ADR 0037 step 22.7 (Consequences, as amended) on the seeds: the seed bard holds the four a bard
// gets free at 0 slots, so his non-weapon pool at level 15 with Intelligence 14 (3 + 3 + 4 = 10)
// is spent by his four other picks and Elvish, Orcish and Dwarvish (step 6: a half-elf's native
// language is Common): 7 of 10, 3 to spend and no slots_over_budget (it was 11 of 10). Nothing is
// offered: he holds the four. The test fixture's Kelsen knows no language but Common: 4 of 10. A
// save that keeps the picks or drops a language is stored.
func TestKelsen_Step22_7_FourFreePicksAndThreeToSpend(t *testing.T) {
	for name, tc := range map[string]struct {
		dir         string
		langs, left int
	}{
		"campaign seeds": {seedsDir, 3, 3},
		"test fixture":   {fixtureSeedsDir, 0, 6},
	} {
		t.Run(name, func(t *testing.T) {
			book, c, f, langs := kelsenFigures(t, tc.dir)
			assert.Equal(t, tc.langs, langs, "the campaign's Elvish, Orcish and Dwarvish; Common is a half-elf's own")
			assert.Equal(t, slots.NonweaponPool{
				Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 4 + tc.langs, Left: tc.left, Languages: tc.langs,
			}, f.Nonweapon)
			left := tc.left
			assert.Contains(t, skills.SlotWarnings(f), skills.Warning{Code: skills.CodeSlotsToSpend, Field: "nonweapon", Pool: "nonweapon", Left: &left})
			for _, w := range skills.SlotWarnings(f) {
				assert.NotEqual(t, skills.CodeSlotsOverBudget, w.Code, "%+v", w)
				assert.NotEqual(t, skills.CodeFreeProficiencyToTake, w.Code, "he holds the four: %+v", w)
			}
			spend := skills.Spending{Picks: c.Picks(), Languages: langs}
			require.NoError(t, skills.PicksWithinBudget(book, c.Class, c.Level, c.Int, spend, spend), "kept as is")
			fewer := skills.Spending{Picks: c.Picks(), Languages: max(langs-1, 0)}
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
		{Class: "abjurer", Groups: []string{"wizard", "general"}},
		{Class: "conjurer", Groups: []string{"wizard", "general"}},
		{Class: "diviner", Groups: []string{"wizard", "general"}},
		{Class: "enchanter", Groups: []string{"wizard", "general"}},
		{Class: "illusionist", Groups: []string{"wizard", "general"}},
		{Class: "invoker", Groups: []string{"wizard", "general"}},
		{Class: "necromancer", Groups: []string{"wizard", "general"}},
		{Class: "transmuter", Groups: []string{"wizard", "general"}},
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
