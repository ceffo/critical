package spelleffectsvc_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// updateGolden rewrites the golden files from the code as it is: `go test
// ./internal/domain/service/spelleffectsvc -run TestSlots_Step4 -update`. Review the diff before
// committing it.
var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// ADR 0046 step 4.3 (step 7: the values are the old code's, committed in
// testdata/golden/slots_splits.json): ADR 0008 step 9's splits of 0 to 9 granted spell levels,
// one line per number of levels, all level 1 first.
func TestSlots_Step4_SplitsMatchGolden(t *testing.T) {
	type splits struct {
		Levels int
		Splits [][]int
	}
	lines := []string{}
	for levels := 0; levels <= 9; levels++ {
		b, err := json.Marshal(splits{levels, spelleffectsvc.GrantSlots{Levels: levels}.SlotSplits()})
		require.NoError(t, err)
		lines = append(lines, string(b))
	}
	got := "[\n" + strings.Join(lines, ",\n") + "\n]\n"
	path := filepath.Join("testdata", "golden", "slots_splits.json")
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}
