package skills_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// updateGolden rewrites the golden files from the code as it is: `go test
// ./internal/domain/skills -run TestSlots_Step4 -update`. Review the diff before committing it.
var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// ADR 0046 step 4.2 (step 7: the values are the old code's, committed in
// testdata/golden/slots_grants.json): what each level from 0 to 20 grants a character of each
// class of the fixture catalogue (ADR 0037 step 15, profBook's Table 34), and of a class with no
// row, read through GrantsOf for the one entry of a single-class character (the one path).
func TestSlots_Step4_GrantsAtEqualsGrantsOfForOneEntry(t *testing.T) {
	book := profBook(t)
	type grant struct {
		Class  string
		Level  int
		Grants skills.LevelGrants
	}
	lines := []string{}
	for _, class := range append(cataloguetest.Definitions(t).ClassIDs(), "no_such_class") {
		for level := 0; level <= 20; level++ {
			g := skills.GrantsOf(book, skills.Classes{{Class: class, Level: level}}, class, level)
			b, err := json.Marshal(grant{class, level, g})
			require.NoError(t, err)
			lines = append(lines, string(b))
		}
	}
	got := "[\n" + strings.Join(lines, ",\n") + "\n]\n"
	path := filepath.Join("testdata", "golden", "slots_grants.json")
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}
