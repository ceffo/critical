package ruletablesadpt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/pkg/must"
)

// fixture is the catalogue artifact CI runs on, carrying the rule tables
// (ADR 0021 step 9.7).
var fixture = filepath.Join("..", "..", "..", "testdata", "catalogue.json")

// fixtureRuleTables is the fixture's rule_tables object, as written.
func fixtureRuleTables(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixture) //nolint:gosec // test path
	require.NoError(t, err)
	var a struct {
		RuleTables json.RawMessage `json:"rule_tables"`
	}
	require.NoError(t, json.Unmarshal(raw, &a))
	require.NotEmpty(t, a.RuleTables)
	return a.RuleTables
}

// fixtureLanguages is the fixture's top-level languages list (format 8).
func fixtureLanguages(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixture) //nolint:gosec // test path
	require.NoError(t, err)
	var a struct {
		Languages json.RawMessage `json:"languages"`
	}
	require.NoError(t, json.Unmarshal(raw, &a))
	require.NotEmpty(t, a.Languages)
	return a.Languages
}

// fixtureEquipment is the fixture's top-level equipment object (format 7).
func fixtureEquipment(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixture) //nolint:gosec // test path
	require.NoError(t, err)
	var a struct {
		Equipment json.RawMessage `json:"equipment"`
	}
	require.NoError(t, json.Unmarshal(raw, &a))
	require.NotEmpty(t, a.Equipment)
	return a.Equipment
}

// phb parses the fixture's rule tables, equipment and languages, as the composition root
// does for the artifact's.
func phb(t *testing.T) *ruletablesvc.Tables {
	t.Helper()
	return must.Do(ruletablesadpt.Parse(fixtureRuleTables(t), fixtureEquipment(t), fixtureLanguages(t)))
}

// Expected values below are read from the PHB 2e pages (Table 1 and 2 on
// p.14, Table 32 on p.42), not derived from the artifact.

func TestStrengthMods(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		name      string
		str, exc  int
		hit, dmg  int
		wantError bool
	}{
		{"3", 3, 0, -3, -1, false},
		{"6", 6, 0, -1, 0, false},
		{"12 (the Bard's base)", 12, 0, 0, 0, false},
		{"15", 15, 0, 0, 0, false},
		{"16", 16, 0, 0, 1, false},
		{"17", 17, 0, 1, 1, false},
		{"18", 18, 0, 1, 2, false},
		{"18/01", 18, 1, 1, 3, false},
		{"18/50", 18, 50, 1, 3, false},
		{"18/51", 18, 51, 2, 3, false},
		{"18/76", 18, 76, 2, 4, false},
		{"18/99", 18, 99, 2, 5, false},
		{"18/00", 18, 100, 3, 6, false},
		{"exceptional ignored below 18", 17, 50, 1, 1, false},
		{"19", 19, 0, 3, 7, false},
		{"25", 25, 0, 7, 14, false},
		{"0 is out of range", 0, 0, 0, 0, true},
		{"26 is out of range", 26, 0, 0, 0, true},
		{"exceptional above 100", 18, 101, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, dmg, err := tbl.StrengthMods(tt.str, tt.exc)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.hit, hit, "hit")
			assert.Equal(t, tt.dmg, dmg, "dmg")
		})
	}
}

func TestDexMods(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		name        string
		dex         int
		missile, ac int
		wantError   bool
	}{
		{"3", 3, -3, 4, false},
		{"5", 5, -1, 2, false},
		{"6", 6, 0, 1, false},
		{"10", 10, 0, 0, false},
		{"15", 15, 0, -1, false},
		{"16", 16, 1, -2, false},
		{"17 (the Bard)", 17, 2, -3, false},
		{"18", 18, 2, -4, false},
		{"21", 21, 4, -5, false},
		{"25", 25, 5, -6, false},
		{"0 is out of range", 0, 0, 0, true},
		{"26 is out of range", 26, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missile, ac, err := tbl.DexMods(tt.dex)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.missile, missile, "missile")
			assert.Equal(t, tt.ac, ac, "ac")
		})
	}
}

func TestBardSpellsPerDay(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		level int
		want  map[int]int
	}{
		{1, map[int]int{}},
		{2, map[int]int{1: 1}},
		{4, map[int]int{1: 2, 2: 1}},
		{9, map[int]int{1: 3, 2: 3, 3: 2}},
		{12, map[int]int{1: 3, 2: 3, 3: 3, 4: 2}},
		{15, map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2}},
		{16, map[int]int{1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1}},
		{20, map[int]int{1: 4, 2: 4, 3: 4, 4: 4, 5: 4, 6: 3}},
	}
	for _, tt := range tests {
		got, err := tbl.BardSpellsPerDay(tt.level)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "level %d", tt.level)
	}
	for _, bad := range []int{0, 21} {
		_, err := tbl.BardSpellsPerDay(bad)
		require.Error(t, err, "level %d", bad)
	}
}

// The Thief/Bard column of PHB Table 25 (p.38), as printed.
func TestBardXPThreshold(t *testing.T) {
	tbl := phb(t)
	tests := []struct{ level, want int }{
		{1, 0},
		{2, 1250},
		{5, 10000},
		{10, 160000},
		{11, 220000},
		{12, 440000},
		{13, 660000},
		{14, 880000},
		{15, 1100000},
		{16, 1320000},
		{19, 1980000},
		{20, 2200000},
	}
	for _, tt := range tests {
		got, err := tbl.BardXPThreshold(tt.level)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "level %d", tt.level)
	}
	for _, bad := range []int{0, 21} {
		_, err := tbl.BardXPThreshold(bad)
		require.Error(t, err, "level %d", bad)
	}
}

// ADR 0021 step 9.3: a gap, an overlap or a malformed row refuses the
// tables, naming the table.
func TestParse_Step9_3_RejectsBrokenTables(t *testing.T) {
	good := fixtureRuleTables(t)
	tests := map[string]struct{ from, to, table string }{
		"gap in strength":    {`{"str_min": 8, "str_max": 15,`, `{"str_min": 9, "str_max": 15,`, "strength table"},
		"gap in 18/xx":       {`"exc_min": 51, "exc_max": 75`, `"exc_min": 52, "exc_max": 75`, "strength table"},
		"dex overlap":        {`"dex_min": 16, "dex_max": 16`, `"dex_min": 15, "dex_max": 16`, "dexterity table"},
		"bard row short":     {`"per_spell_level": [0, 0, 0, 0, 0, 0]`, `"per_spell_level": [0, 0]`, "bard level 1"},
		"unknown field":      {`"strength": [`, `"extra": 1, "strength": [`, "extra"},
		"duplicate bard lvl": {`"level": 2,`, `"level": 1,`, "bard table"},
		"xp level skipped":   {`{"level": 3, "xp": 2500}`, `{"level": 4, "xp": 2500}`, "experience table"},
		"xp not increasing":  {`{"level": 3, "xp": 2500}`, `{"level": 3, "xp": 1250}`, "experience table"},
		"xp level 1 above 0": {`{"level": 1, "xp": 0}`, `{"level": 1, "xp": 5}`, "experience table"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, string(good), tt.from)
			_, err := ruletablesadpt.Parse([]byte(strings.Replace(string(good), tt.from, tt.to, 1)), fixtureEquipment(t), fixtureLanguages(t))
			require.ErrorContains(t, err, tt.table)
		})
	}
	_, err := ruletablesadpt.Parse(good, fixtureEquipment(t), fixtureLanguages(t))
	require.NoError(t, err)
}

// classTablesOf parses the fixture's rule tables with mutate applied to the
// decoded object first.
func classTablesOf(t *testing.T, mutate func(map[string]any)) (*ruletablesvc.Tables, error) {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &doc))
	mutate(doc)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return ruletablesadpt.Parse(raw, fixtureEquipment(t), fixtureLanguages(t))
}

func classRows(d map[string]any, table string) []any { return d[table].([]any) }

func classRow(d map[string]any, table string, i int) map[string]any {
	return classRows(d, table)[i].(map[string]any)
}

// ADR 0021 step 10.1 (ADR 0030 step 23): the class and
// race tables are read. Expected values are read from the PHB pages (p.15-16
// Tables 3-4, p.26 Table 14, p.33 Table 24, p.91 Table 53, p.101 Table 60,
// p.103 Table 61), not derived from the artifact.
func TestParse_Step10_1_ReadsTheClassTables(t *testing.T) {
	c, ok := phb(t).Classes()
	require.True(t, ok, "the artifact carries the class tables")
	assert.Equal(t, 3000000, c.Experience[19].Fighter, "fighter, level 20")
	assert.Equal(t, 1100000, c.Experience[14].Bard, "bard, level 15")
	assert.Equal(t, 13, c.THAC0[14].Rogue, "rogue THAC0, level 15")
	assert.Equal(t, 1, c.THAC0[19].Warrior, "warrior THAC0, level 20")
	var rogue ruletablesvc.SavingThrowRow
	for _, r := range c.SavingThrows {
		if r.Group == "rogue" && r.LevelMin <= 15 && 15 <= r.LevelMax {
			rogue = r
		}
	}
	assert.Equal(t, [5]int{10, 8, 9, 13, 9},
		[5]int{rogue.ParaPoison, rogue.RodStaffWand, rogue.PetriPoly, rogue.Breath, rogue.Spell}, "rogue saves, levels 13-16")
	assert.Equal(t, ruletablesvc.HitDiceRow{Group: "warrior", Die: 10, NameLevel: 9, GainAfter: 3}, c.HitDice[0])
	assert.Equal(t, ruletablesvc.ConstitutionRow{ConMin: 20, ConMax: 20, HP: 2, HPWarrior: 5, LowDieMin: 2}, c.Constitution[19])
	assert.True(t, c.Intelligence[18].NoMaximum, "INT 19: no maximum of spells per level")
	assert.Equal(t, 9, c.Intelligence[13].MaxSpells, "INT 14")
	assert.Equal(t, []int{6, 6, 6, 5, 3, 2, 1}, c.PriestSpells[13].PerSpellLevel, "priest, level 14")
	assert.Equal(t, []ruletablesvc.PriestSpellWisdomRow{{SpellLevel: 6, WisMin: 17}, {SpellLevel: 7, WisMin: 18}}, c.PriestSpellWisdom)
	skeleton := c.TurningUndead[0]
	assert.Equal(t, "skeleton", skeleton.Undead)
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 10}, skeleton.ByLevel[0], "level 1")
	assert.Equal(t, ruletablesvc.TurningCell{Outcome: "destroy_more"}, skeleton.ByLevel[19], "level 20")
}

// ADR 0021 step 10.3: a missing class or race table, or one that breaks the
// rules the schema cannot express, refuses the tables, naming the table.
func TestParse_Step10_3_RejectsAMissingOrBrokenClassTable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a missing table", func(d map[string]any) { delete(d, "saving_throws") }, "missing: saving_throws"},
		{"XP not increasing", func(d map[string]any) { classRow(d, "experience", 3)["cleric"] = 1 }, "experience table: cleric level 4"},
		{"a level twice", func(d map[string]any) { classRow(d, "thac0", 4)["level"] = 4 }, "thac0 table: row 4: level 5 expected, got 4"},
		{"a gap in a level band", func(d map[string]any) { classRow(d, "saving_throws", 1)["level_min"] = 5 }, "saving_throws table: priest: row 1: level 4 expected, got 5"},
		{"a gap in the scores", func(d map[string]any) {
			r := classRows(d, "wisdom")
			d["wisdom"] = append(r[:3:3], r[4:]...)
		}, "wisdom table: row 3: score 4 expected, got 5"},
		{"a race skill twice", func(d map[string]any) { classRow(d, "thief_skill_race", 5)["skill"] = "pick_pockets" }, "thief_skill_race table: \"dwarf pick_pockets\" twice"},
		{"an unknown group", func(d map[string]any) { classRow(d, "hit_dice", 0)["group"] = "paladin" }, "hit_dice table: unknown \"paladin\""},
		{"a turning cell with a roll and an outcome", func(d map[string]any) {
			classRow(d, "turning_undead", 0)["by_level"].([]any)[0].(map[string]any)["outcome"] = "turn"
		}, "turning_undead table: skeleton level 1"},
		{"19 turning levels", func(d map[string]any) {
			r := classRow(d, "turning_undead", 0)
			r["by_level"] = r["by_level"].([]any)[1:]
		}, "turning_undead table: skeleton: 19 levels"},
		{"an unknown field", func(d map[string]any) { classRow(d, "backstab", 0)["note"] = 1 }, "note"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classTablesOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// levelLimits is the race_level_limits key of rule_tables (ADR 0021 step 11).
const levelLimits = "race_level_limits"

// ADR 0021 step 11.1 (ADR 0030 step 21): the racial level
// limits are read, one cell per race and class. Expected values are read
// from DMG Table 7 on the page (DMG p.15), not derived from the artifact:
// a number is the highest level, "U" no limit, "-" the race cannot belong
// to the class. Humans have no row.
func TestParse_Step11_1_ReadsTheLevelLimits(t *testing.T) {
	l, ok := phb(t).LevelLimits()
	require.True(t, ok, "the artifact carries the level limits")
	_, ok = phb(t).Classes()
	require.True(t, ok, "the artifact carries the class tables too")
	page := map[string][5]string{ // dwarf, elf, gnome, half-elf, halfling
		"bard":    {"-", "-", "-", "U", "-"},
		"cleric":  {"10", "12", "9", "14", "8"},
		"fighter": {"15", "12", "11", "14", "9"},
		"mage":    {"-", "15", "-", "12", "-"},
		"thief":   {"12", "12", "13", "12", "15"},
	}
	for class, cells := range page {
		for i, race := range []string{"dwarf", "elf", "gnome", "half_elf", "halfling"} {
			got, ok := l.Limit(race, class)
			require.True(t, ok, "%s %s", race, class)
			cell := strconv.Itoa(got.MaxLevel)
			switch {
			case got.Unlimited:
				cell = "U"
			case got.MaxLevel == 0:
				cell = "-"
			}
			assert.Equal(t, cells[i], cell, "%s %s", race, class)
		}
	}
	_, ok = l.Limit("human", "fighter")
	assert.False(t, ok, "humans have no row")
}

// ADR 0021 step 11.3: an artifact without the level limits, or with
// limits that break the rules the schema cannot express, refuses the
// tables, naming the table.
func TestParse_Step11_3_RejectsMissingOrBrokenLevelLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a missing table", func(d map[string]any) { delete(d, levelLimits) }, "the artifact needs the race_level_limits table"},
		{"a missing class table", func(d map[string]any) { delete(d, "thac0") }, "missing: thac0"},
		{
			"a race and class twice", func(d map[string]any) { classRow(d, levelLimits, 1)["class"] = "fighter" },
			"race_level_limits table: \"dwarf fighter\" twice",
		},
		{
			"a missing race and class", func(d map[string]any) { d[levelLimits] = classRows(d, levelLimits)[1:] },
			"race_level_limits table: missing \"dwarf fighter\"",
		},
		{
			"a class the app does not have", func(d map[string]any) { classRow(d, levelLimits, 0)["class"] = "paladin" },
			"race_level_limits table: unknown \"dwarf paladin\"",
		},
		{"a human row", func(d map[string]any) {
			classRow(d, levelLimits, 0)["race"] = "human"
		}, "race_level_limits table: unknown \"human fighter\""},
		{
			"a level above 20", func(d map[string]any) { classRow(d, levelLimits, 0)["max_level"] = 21 },
			"race_level_limits table: dwarf fighter: level 21 outside 0..20",
		},
		{
			"a negative level", func(d map[string]any) { classRow(d, levelLimits, 0)["max_level"] = -1 },
			"race_level_limits table: dwarf fighter: level -1 outside 0..20",
		},
		{"an unknown field", func(d map[string]any) { classRow(d, levelLimits, 0)["note"] = 1 }, "note"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classTablesOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// ADR 0030 step 21 (as built for character creation): a race may belong to a
// class unless DMG Table 7 has a dash for it (DMG p.15: a bard is a human or
// a half-elf); a human to any class.
func TestRaceAllows_Step21_TheDashOfTable7(t *testing.T) {
	tables := phb(t)
	for _, tt := range []struct {
		race, class string
		want        bool
	}{
		{"human", "bard", true},
		{"half_elf", "bard", true},
		{"elf", "bard", false},
		{"dwarf", "bard", false},
		{"gnome", "bard", false},
		{"halfling", "bard", false},
		{"dwarf", "mage", false},
		{"elf", "mage", true},
		{"halfling", "thief", true},
		{"human", "cleric", true},
	} {
		got, err := tables.RaceAllows(tt.race, tt.class)
		require.NoError(t, err, "%s %s", tt.race, tt.class)
		assert.Equal(t, tt.want, got, "%s %s", tt.race, tt.class)
	}
	_, err := tables.RaceAllows("orc", "bard")
	require.Error(t, err, "a race the tables do not know")
}
