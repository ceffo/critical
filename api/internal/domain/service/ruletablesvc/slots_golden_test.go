package ruletablesvc_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// updateGolden rewrites the golden files from the code as it is: `go test
// ./internal/domain/service/ruletablesvc -run TestSlots_Step4 -update`. Review the diff before
// committing it.
var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// goldenLines compares the records, one JSON line each, with testdata/golden/name.
func goldenLines(t *testing.T, name string, records []any) {
	t.Helper()
	lines := make([]string, 0, len(records))
	for _, r := range records {
		b, err := json.Marshal(r)
		require.NoError(t, err)
		lines = append(lines, string(b))
	}
	got := "[\n" + strings.Join(lines, ",\n") + "\n]\n"
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got, "%s changed", name)
}

// goldenRows are Table 34's rows as seeds/proficiencies.json holds them, and a row with no level
// rate (no slot is ever gained, never a division by 0).
var goldenRows = []ruletablesvc.SlotRow{
	{Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
	{Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
	{Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
	{Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
	{Group: "psionicist", WeaponInitial: 2, WeaponLevels: 5, Penalty: -4, NonweaponInitial: 3, NonweaponLevels: 3},
	{Group: "no_rate", WeaponInitial: 1, NonweaponInitial: 2},
}

// ADR 0046 step 4.1 (step 7: the values are the old code's, committed in
// testdata/golden/slots_pools.json): ADR 0037 step 3's weapon and non-weapon pools for every
// Table 34 row at every level from 0 to 20, the non-weapon pool for every Intelligence from 3 to
// 25 (Table 4's "# of Lang."), the picks spending 2 slots; and step 5's extra slots for picks of
// 0 to 4 slots at costs 0 to 3.
func TestSlots_Step4_PoolsMatchTable34(t *testing.T) {
	type nonweapon struct {
		Group      string
		Level, Int int
		Pool       ruletablesvc.NonweaponPool
	}
	type weapon struct {
		Group string
		Level int
		Pool  ruletablesvc.WeaponPool
	}
	type extra struct{ Slots, Cost, Extra int }
	var nw, w, ex []any
	for _, row := range goldenRows {
		for level := 0; level <= 20; level++ {
			w = append(w, weapon{row.Group, level, ruletablesvc.WeaponSlots(row, level, 2)})
			for in := 3; in <= 25; in++ {
				nw = append(nw, nonweapon{row.Group, level, in, ruletablesvc.NonweaponSlots(row, level, in, 2)})
			}
		}
	}
	for slots := 0; slots <= 4; slots++ {
		for cost := 0; cost <= 3; cost++ {
			ex = append(ex, extra{slots, cost, ruletablesvc.ExtraSlots(slots, cost)})
		}
	}
	goldenLines(t, "slots_nonweapon.json", nw)
	goldenLines(t, "slots_weapon.json", w)
	goldenLines(t, "slots_extra.json", ex)
}
