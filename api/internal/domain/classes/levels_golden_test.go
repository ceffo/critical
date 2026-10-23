package classes_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

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
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got, "%s changed", name)
}

// ADR 0046 step 5.3 (step 7: the values are the old code's, the rule tables' thresholds
// committed in testdata/golden/levels_thresholds.json before the class definition became their
// one owner): every class of the fixture catalogue, levels 1 to MaxLevel, the class definition's
// threshold (ADR 0030 step 3) is the committed one, and LevelFor of that threshold is the level.
func TestLevels_Step5_ThresholdsEqualTheTable(t *testing.T) {
	type threshold struct {
		Class string
		Level int
		XP    int
	}
	var rows []any
	for _, def := range cataloguetest.Definitions(t).Classes() {
		for level := 1; level <= classes.MaxLevel; level++ {
			xp, err := def.XPThreshold(level)
			require.NoError(t, err, "%s level %d", def.ID, level)
			assert.Equal(t, level, def.LevelFor(xp), "%s: LevelFor(XPThreshold(%d))", def.ID, level)
			rows = append(rows, threshold{def.ID, level, xp})
		}
	}
	goldenLines(t, "levels_thresholds.json", rows)
}
