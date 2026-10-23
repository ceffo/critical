package catalogue_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
)

// The release build reads the formats the app accepts from the source with
// scripts/catalogue-formats.sh; it must say what the API itself reads.
func TestFormatsScriptMatchesFormats(t *testing.T) {
	out, err := exec.Command("../../../../scripts/catalogue-formats.sh", "../../../..").Output()
	require.NoError(t, err)
	var want []string
	for _, f := range catalogue.Formats {
		want = append(want, strconv.Itoa(f))
	}
	require.Equal(t, want, strings.Fields(string(out)))
}
