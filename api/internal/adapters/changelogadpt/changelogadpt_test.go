package changelogadpt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/changelogadpt"
)

// Step 6.3: the server reads the baked file; with no path (a dev build) the changelog is empty.
func TestChangelogAdpt_Step6_NoPathIsTheEmptyChangelog(t *testing.T) {
	log, err := changelogadpt.Load("")
	require.NoError(t, err)
	p, err := log.Page("", 10)
	require.NoError(t, err)
	assert.Empty(t, p.Versions)
}

func TestChangelogAdpt_Step6_ReadsTheGeneratorsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changelog.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"versions":[{"version":"1.1.0","features":["B"],"fixes":[]},{"version":"1.0.0"}]}`), 0o600))
	log, err := changelogadpt.Load(path)
	require.NoError(t, err)
	p, err := log.Page("", 10)
	require.NoError(t, err)
	require.Len(t, p.Versions, 2)
	assert.Equal(t, []string{"B"}, p.Versions[0].Features)
	assert.NotNil(t, p.Versions[1].Features, "a missing list is an empty list")
	assert.NotNil(t, p.Versions[1].Fixes)
}

// A named file that is missing or is not the generator's output stops the start: the image must carry it whole.
func TestChangelogAdpt_Step6_RefusesAMissingOrMalformedFile(t *testing.T) {
	dir := t.TempDir()
	_, err := changelogadpt.Load(filepath.Join(dir, "missing.json"))
	require.Error(t, err)
	for name, body := range map[string]string{
		"not json":      `nope`,
		"unknown field": `{"versions":[],"extra":1}`,
		"no version":    `{"versions":[{"features":[],"fixes":[]}]}`,
		"twice":         `{"versions":[{"version":"1.0.0"},{"version":"1.0.0"}]}`,
	} {
		_, err := changelogadpt.Parse([]byte(body))
		require.Error(t, err, name)
	}
}
