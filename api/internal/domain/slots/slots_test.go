package slots_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// golden compares the records, one JSON line each, with testdata/golden/name: values the old
// code gave, committed before the formulas moved here (ADR 0046 step 7), so there is no -update.
func golden(t *testing.T, name string, records []any) {
	t.Helper()
	lines := make([]string, 0, len(records))
	for _, r := range records {
		b, err := json.Marshal(r)
		require.NoError(t, err)
		lines = append(lines, string(b))
	}
	want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	require.NoError(t, err)
	require.Equal(t, string(want), "[\n"+strings.Join(lines, ",\n")+"\n]\n", "%s changed", name)
}

// goldenRows are Table 34's rows as seeds/proficiencies.json holds them, and a row with no level
// rate (no slot is ever gained, never a division by 0).
var goldenRows = []slots.Row{
	{Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
	{Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
	{Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
	{Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
	{Group: "psionicist", WeaponInitial: 2, WeaponLevels: 5, Penalty: -4, NonweaponInitial: 3, NonweaponLevels: 3},
	{Group: "no_rate", WeaponInitial: 1, NonweaponInitial: 2},
}

// ADR 0046 step 4.1 (step 7: the values are the old code's, committed in testdata/golden):
// ADR 0037 step 3's weapon and non-weapon pools for every Table 34 row at every level from 0 to
// 20, the non-weapon pool for every Intelligence from 3 to 25 (Table 4's "# of Lang.", read from
// the rule tables as the callers do), the picks spending 2 slots; and step 5's extra slots for
// picks of 0 to 4 slots at costs 0 to 3.
func TestSlots_Step4_PoolsMatchTable34(t *testing.T) {
	type nonweapon struct {
		Group      string
		Level, Int int
		Pool       slots.NonweaponPool
	}
	type weapon struct {
		Group string
		Level int
		Pool  slots.WeaponPool
	}
	type extra struct{ Slots, Cost, Extra int }
	var nw, w, ex []any
	for _, row := range goldenRows {
		for level := 0; level <= 20; level++ {
			w = append(w, weapon{row.Group, level, slots.Weapon(row, level, 2)})
			for in := 3; in <= 25; in++ {
				nw = append(nw, nonweapon{row.Group, level, in, slots.Nonweapon(row, level, ruletablesvc.LanguageLimit(in), 2)})
			}
		}
	}
	for n := 0; n <= 4; n++ {
		for cost := 0; cost <= 3; cost++ {
			ex = append(ex, extra{n, cost, slots.Extra(n, cost)})
		}
	}
	golden(t, "slots_nonweapon.json", nw)
	golden(t, "slots_weapon.json", w)
	golden(t, "slots_extra.json", ex)
}

// ADR 0046 step 4.3 (step 7: the values are the old code's, committed in
// testdata/golden/slots_splits.json): ADR 0008 step 9's splits of 0 to 9 granted spell levels.
func TestSlots_Step4_SplitsMatchGolden(t *testing.T) {
	type splits struct {
		Levels int
		Splits [][]int
	}
	var rows []any
	for levels := 0; levels <= 9; levels++ {
		rows = append(rows, splits{levels, slots.Splits(levels)})
	}
	golden(t, "slots_splits.json", rows)
}

// ADR 0008 step 9 (ADR 0046 step 4.3): the splits of the granted spell levels, all level 1
// first.
func TestSlots_Step4_SplitsOfThreeLevels(t *testing.T) {
	tests := map[int][][]int{
		1: {{1}},
		3: {{1, 1, 1}, {1, 2}, {3}},
		4: {{1, 1, 1, 1}, {1, 1, 2}, {1, 3}, {2, 2}, {4}},
	}
	for levels, want := range tests {
		assert.Equal(t, want, slots.Splits(levels), "%d levels", levels)
	}
}
