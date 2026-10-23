package xpsvc_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// updateGolden rewrites the golden files from the code as it is: `go test
// ./internal/domain/service/xpsvc -run TestProgress_Step5 -update`. Review the diff before
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

// ADR 0046 step 5.3 (step 7: the values are the old code's, committed in
// testdata/golden/progress.json): ADR 0015 step 3's progress (floor, ceiling, what is left,
// the share and the cells) for every class of the fixture catalogue at every level from 1 to one
// above the table, the total one under the floor, at it, halfway, one under the ceiling, at it and
// one over.
func TestProgress_Step5_ReadsTheClassDefinition(t *testing.T) {
	type progress struct {
		Class        string
		Level, Total int
		Progress     xpsvc.Progress
	}
	var rows []any
	for _, def := range cataloguetest.Definitions(t).Classes() {
		for level := 1; level <= classes.MaxLevel+1; level++ {
			floor, err := def.XPThreshold(min(level, classes.MaxLevel))
			require.NoError(t, err)
			ceiling := floor + 1000
			if level < classes.MaxLevel {
				ceiling, err = def.XPThreshold(level + 1)
				require.NoError(t, err)
			}
			for _, total := range []int{floor - 1, floor, (floor + ceiling) / 2, ceiling - 1, ceiling, ceiling + 1} {
				got, err := xpsvc.ComputeProgress(def, level, total)
				require.NoError(t, err, "%s level %d total %d", def.ID, level, total)
				rows = append(rows, progress{def.ID, level, total, got})
			}
		}
	}
	goldenLines(t, "progress.json", rows)
}
