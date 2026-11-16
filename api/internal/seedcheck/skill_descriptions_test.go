package seedcheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
)

// ADR 0021 step 33.4: the fixture's skill descriptions agree with its skill sets, both languages
// for every (set, skill) pair.
func TestFixture_Step33_SkillDescriptionsAgreeWithTheSkillSets(t *testing.T) {
	a, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	require.NoError(t, catalogue.CheckSkillDescriptions(a.Catalogue.SkillDescriptions, a.SkillSets))
}

// ADR 0021 step 33: the real artifact (format 25, `just catalogue`) has a description in English
// and French for every skill of every skill set, which is what the start checks: the thief's
// eight skills and the bard's four. Skipped without a real .catalogue/catalogue.json.
func TestRealCatalogue_Step33_EverySkillDescribedInBothLanguages(t *testing.T) {
	realCatalogue(t)
	a, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	require.NoError(t, catalogue.CheckSkillDescriptions(a.Catalogue.SkillDescriptions, a.SkillSets))
	assert.Len(t, a.Catalogue.SkillDescriptions, 12)
	byPair := map[catalogue.SkillPair]string{}
	for _, d := range a.Catalogue.SkillDescriptions {
		byPair[catalogue.SkillPair{Set: d.Set, Skill: d.Skill}] = d.Description["en"]
	}
	for _, s := range a.SkillSets {
		for _, k := range s.Skills {
			assert.NotEmpty(t, byPair[catalogue.SkillPair{Set: s.ID, Skill: k.ID}], "%s/%s", s.ID, k.ID)
		}
	}
	// The set decides the text: a skill two sets have is not described twice with one text.
	seen := map[string]catalogue.SkillPair{}
	for pair, en := range byPair {
		if other, dup := seen[en]; dup {
			assert.Failf(t, "one text for two skills", "%v and %v share their text", pair, other)
		}
		seen[en] = pair
	}
}

// ADR 0037 step 19.6: the descriptions are served from the catalogue, never from the seeds or the
// locale files: no seed file and no locale file holds a skill description key.
func TestSkillDescriptions_Step19_6_NotInTheSeeds(t *testing.T) {
	key := regexp.MustCompile(`(?i)"skill[a-z_]*description[a-z_]*"\s*:`)
	for _, dir := range []string{seedsDir, fixtureSeedsDir} {
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		require.NoError(t, err)
		require.NotEmpty(t, files)
		for _, f := range files {
			raw, err := os.ReadFile(f) //nolint:gosec // globbed test paths
			require.NoError(t, err)
			assert.False(t, key.Match(raw), "%s carries a skill description", f)
		}
	}
	locales, err := filepath.Glob(filepath.Join("..", "..", "..", "web", "src", "i18n", "locales", "*", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, locales)
	for _, f := range locales {
		raw, err := os.ReadFile(f) //nolint:gosec // globbed test paths
		require.NoError(t, err)
		assert.False(t, key.Match(raw), "%s carries a skill description", f)
		assert.NotRegexp(t, `"skill[a-z]*"\s*:\s*\{[^}]*"description"`, string(raw), "%s", f)
	}
}
