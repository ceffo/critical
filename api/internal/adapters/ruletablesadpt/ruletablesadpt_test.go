package ruletablesadpt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/pkg/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var seedsDir = filepath.Join("..", "..", "..", "testdata", "seeds")

// Expected values below are read from the PHB 2e pages (Table 1 and 2 on
// p.14, Table 32 on p.42), not derived from the seed file.

func TestStrengthMods(t *testing.T) {
	tbl := must.Do(ruletablesadpt.New(seedsDir))
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
	tbl := must.Do(ruletablesadpt.New(seedsDir))
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
	tbl := must.Do(ruletablesadpt.New(seedsDir))
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

func TestNew_MissingFile(t *testing.T) {
	_, err := ruletablesadpt.New(t.TempDir())
	require.ErrorContains(t, err, ruletablesadpt.FileName)
}

func TestParse_RejectsBrokenTables(t *testing.T) {
	good, err := os.ReadFile(filepath.Join(seedsDir, ruletablesadpt.FileName)) //nolint:gosec // test path
	require.NoError(t, err)
	tests := map[string]struct{ from, to string }{
		"gap in strength":    {`{"str_min": 8, "str_max": 15,`, `{"str_min": 9, "str_max": 15,`},
		"gap in 18/xx":       {`"exc_min": 51, "exc_max": 75`, `"exc_min": 52, "exc_max": 75`},
		"dex overlap":        {`"dex_min": 16, "dex_max": 16`, `"dex_min": 15, "dex_max": 16`},
		"bard row short":     {`"per_spell_level": [0, 0, 0, 0, 0, 0]`, `"per_spell_level": [0, 0]`},
		"unknown field":      {`"_source"`, `"extra": 1, "_source"`},
		"duplicate bard lvl": {`"level": 2,`, `"level": 1,`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, string(good), tt.from)
			_, err := ruletablesadpt.Parse([]byte(strings.Replace(string(good), tt.from, tt.to, 1)))
			require.Error(t, err)
		})
	}
	_, err = ruletablesadpt.Parse(good)
	require.NoError(t, err)
}
