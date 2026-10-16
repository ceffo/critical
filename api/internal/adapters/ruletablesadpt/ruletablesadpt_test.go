package ruletablesadpt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/pkg/must"
)

var (
	// fixture is the catalogue artifact CI runs on: format 3, carrying the
	// rule tables (ADR 0021 step 9.7).
	fixture = filepath.Join("..", "..", "..", "testdata", "catalogue.json")
	// campaignSeeds holds the seed a format-1 artifact takes its tables
	// from (step 9.6.1).
	campaignSeeds = filepath.Join("..", "..", "..", "..", "seeds")
)

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

// phb parses the fixture's rule tables, as the catalogue adapter does for a
// format-3 artifact.
func phb(t *testing.T) *ruletablesvc.Tables {
	t.Helper()
	return must.Do(ruletablesadpt.Parse(fixtureRuleTables(t)))
}

// Expected values below are read from the PHB 2e pages (Table 1 and 2 on
// p.14, Table 32 on p.42), not derived from the artifact or the seed file.

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

// ADR 0021 step 9.7: the fixture's rule tables hold the same numbers as the
// campaign seed a format-1 artifact still reads (step 9.6.1), so every lookup
// (attack adjustments, bard spells per day, XP thresholds) gives the same
// result from either source.
func TestRuleTables_Step9_7_TheFixtureCarriesTheSeedsNumbers(t *testing.T) {
	seedRaw, err := os.ReadFile(filepath.Join(campaignSeeds, ruletablesadpt.FileName)) //nolint:gosec // test path
	require.NoError(t, err)
	var seed, artifact map[string]any
	require.NoError(t, json.Unmarshal(seedRaw, &seed))
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &artifact))
	delete(seed, "_source")
	assert.Equal(t, seed, artifact, "the artifact's tables are the seed's, without its _source comment")

	fromSeed := must.Do(ruletablesadpt.New(campaignSeeds))
	fromArtifact := phb(t)
	for str := 1; str <= 25; str++ {
		for exc := 0; exc <= ruletablesvc.ExceptionalMax; exc++ {
			sh, sd, serr := fromSeed.StrengthMods(str, exc)
			ah, ad, aerr := fromArtifact.StrengthMods(str, exc)
			require.NoError(t, serr)
			require.NoError(t, aerr)
			assert.Equal(t, [2]int{sh, sd}, [2]int{ah, ad}, "STR %d/%d", str, exc)
		}
	}
	for dex := 1; dex <= 25; dex++ {
		sm, sa, serr := fromSeed.DexMods(dex)
		am, aa, aerr := fromArtifact.DexMods(dex)
		require.NoError(t, serr)
		require.NoError(t, aerr)
		assert.Equal(t, [2]int{sm, sa}, [2]int{am, aa}, "DEX %d", dex)
	}
	for level := 1; level <= ruletablesvc.MaxBardLevel; level++ {
		assert.Equal(t, must.Do(fromSeed.BardSpellsPerDay(level)), must.Do(fromArtifact.BardSpellsPerDay(level)), "bard level %d", level)
		assert.Equal(t, must.Do(fromSeed.BardXPThreshold(level)), must.Do(fromArtifact.BardXPThreshold(level)), "XP level %d", level)
	}
}

// ADR 0021 step 9.6.1: a format-1 artifact takes its tables from the seeds
// directory, and a seeds directory without the file refuses, naming it.
func TestNew_Step9_6_1_Format1ReadsTheSeedFile(t *testing.T) {
	_, err := ruletablesadpt.New(campaignSeeds)
	require.NoError(t, err)
	_, err = ruletablesadpt.New(t.TempDir())
	require.ErrorContains(t, err, ruletablesadpt.FileName)
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
			_, err := ruletablesadpt.Parse([]byte(strings.Replace(string(good), tt.from, tt.to, 1)))
			require.ErrorContains(t, err, tt.table)
		})
	}
	_, err := ruletablesadpt.Parse(good)
	require.NoError(t, err)
}
