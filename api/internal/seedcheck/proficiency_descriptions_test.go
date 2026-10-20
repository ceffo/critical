package seedcheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
)

func proficiencyIDs(t *testing.T, dir string) []string {
	t.Helper()
	book, err := ruletablesadpt.ReadProficiencies(dir)
	require.NoError(t, err)
	var ids []string
	for _, p := range book.All() {
		ids = append(ids, p.ID)
	}
	return ids
}

// ADR 0021 step 16.4: the fixture's descriptions agree with the testdata proficiency seed, both
// languages for every id.
func TestFixture_Step16_DescriptionsAgreeWithTheTestdataSeed(t *testing.T) {
	a, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	require.NoError(t, catalogue.CheckProficiencyDescriptions(a.Catalogue.ProficiencyDescriptions, proficiencyIDs(t, fixtureSeedsDir)))
}

// ADR 0021 step 16: the real artifact (format 9, `just catalogue`) has a description in English
// and French for every id of the real seeds/proficiencies.json, which is what the start checks.
// Skipped without a format-9 .catalogue/catalogue.json.
func TestRealCatalogue_Step16_EveryDescriptionPresentInBothLanguages(t *testing.T) {
	realCatalogue(t)
	a, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	require.NoError(t, catalogue.CheckProficiencyDescriptions(a.Catalogue.ProficiencyDescriptions, proficiencyIDs(t, seedsDir)))
	assert.Len(t, a.Catalogue.ProficiencyDescriptions, 65)
}

// ADR 0037 step 19.4: the descriptions are served from the catalogue, never from the seeds or the
// locale files: neither holds a description key.
func TestProficiencyDescriptions_Step19_NotInTheSeeds(t *testing.T) {
	key := regexp.MustCompile(`(?i)"description[a-z_]*"\s*:`)
	for _, dir := range []string{seedsDir, fixtureSeedsDir} {
		raw, err := os.ReadFile(filepath.Join(dir, "proficiencies.json")) //nolint:gosec // fixed test paths
		require.NoError(t, err)
		assert.False(t, key.Match(raw), "%s/proficiencies.json carries a description", dir)
	}
	locales, err := filepath.Glob(filepath.Join("..", "..", "..", "web", "src", "i18n", "locales", "*", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, locales)
	for _, f := range locales {
		raw, err := os.ReadFile(f) //nolint:gosec // globbed test paths
		require.NoError(t, err)
		assert.NotRegexp(t, `"proficienc[a-z]*"\s*:\s*\{[^}]*"description`, string(raw), "%s", f)
	}
}
