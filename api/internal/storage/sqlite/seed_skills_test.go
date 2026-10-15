package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedsWithSkills copies the frozen fixture seeds to a temp dir and replaces
// skills.json with content.
func seedsWithSkills(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(repoSeeds())
	require.NoError(t, err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(repoSeeds(), e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600)) //nolint:gosec // test temp dir, fixture file names
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills.json"), []byte(content), 0o600))
	return dir
}

// The sheet's proficiencies (PHB Table 37 abilities and modifiers) and bard
// skills load, and a restart with the same seed changes nothing.
func TestSeed_SkillsMatchSheetAndAreIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	require.NoError(t, db.Seed(ctx, repoSeeds()))

	assert.Equal(t, 8, scalar(t, db, "SELECT COUNT(*) FROM proficiencies"))
	assert.Equal(t, 4, scalar(t, db, "SELECT COUNT(*) FROM bard_skills"))
	for id, want := range map[string]struct {
		ability  string
		modifier int
	}{
		"musical_instrument": {"dex", -1}, "tumbling": {"dex", 0}, "ventriloquism": {"int", -2},
		"riding_land": {"wis", 3}, "appraising": {"int", 0}, "singing": {"cha", 0},
		"reading_writing": {"int", 1}, "local_history": {"cha", 0},
	} {
		assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM proficiencies WHERE id = '"+id+"' AND ability = '"+want.ability+"'"), id)
		assert.Equal(t, want.modifier, scalar(t, db, "SELECT modifier FROM proficiencies WHERE id = '"+id+"'"), id)
	}
	for id, want := range map[string]int{"climb_walls": 85, "detect_noise": 90, "pick_pockets": 50, "read_languages": 75} {
		assert.Equal(t, want, scalar(t, db, "SELECT percent FROM bard_skills WHERE id = '"+id+"'"), id)
	}
	assert.Zero(t, scalar(t, db, "SELECT position FROM proficiencies WHERE id = 'musical_instrument'"), "seed order kept")

	require.NoError(t, db.Seed(ctx, repoSeeds()))
	assert.Equal(t, 8, scalar(t, db, "SELECT COUNT(*) FROM proficiencies"))
	assert.Equal(t, 4, scalar(t, db, "SELECT COUNT(*) FROM bard_skills"))
	assert.Equal(t, 50, scalar(t, db, "SELECT percent FROM bard_skills WHERE id = 'pick_pockets'"))
	assert.Equal(t, 7, scalar(t, db, "SELECT position FROM proficiencies WHERE id = 'local_history'"), "order kept")
	require.NoError(t, db.Seed(ctx, t.TempDir()), "a missing file is fine once the tables are filled")
}

// An edited seed reaches an existing database: changed values and order are
// applied by id, a new row is added, nothing is duplicated, and a row the seed
// no longer lists is kept.
func TestSeed_SkillsReconcileByID(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	require.NoError(t, db.Seed(ctx, seedsWithSkills(t, `{
		"proficiencies":[{"id":"tumbling","ability":"dex","modifier":0},{"id":"singing","ability":"cha","modifier":0}],
		"bard_skills":[{"id":"climb_walls","percent":85},{"id":"pick_pockets","percent":50}]}`)))

	edited := seedsWithSkills(t, `{
		"proficiencies":[{"id":"singing","ability":"cha","modifier":1},{"id":"tumbling","ability":"dex","modifier":0},
			{"id":"ventriloquism","ability":"int","modifier":-2}],
		"bard_skills":[{"id":"climb_walls","percent":90}]}`)
	for range 2 {
		require.NoError(t, db.Seed(ctx, edited))
		assert.Equal(t, 3, scalar(t, db, "SELECT COUNT(*) FROM proficiencies"), "new row added once")
		assert.Equal(t, 1, scalar(t, db, "SELECT modifier FROM proficiencies WHERE id = 'singing'"), "corrected modifier")
		assert.Zero(t, scalar(t, db, "SELECT position FROM proficiencies WHERE id = 'singing'"), "new order")
		assert.Equal(t, 2, scalar(t, db, "SELECT position FROM proficiencies WHERE id = 'ventriloquism'"))
		assert.Equal(t, -2, scalar(t, db, "SELECT modifier FROM proficiencies WHERE id = 'ventriloquism'"))
		assert.Equal(t, 90, scalar(t, db, "SELECT percent FROM bard_skills WHERE id = 'climb_walls'"), "raised percentage")
		assert.Equal(t, 2, scalar(t, db, "SELECT COUNT(*) FROM bard_skills"), "a row the seed dropped is kept")
	}
}

func TestSeed_InvalidSkillsFailWithoutPartialRows(t *testing.T) {
	tests := map[string]string{
		"unknown ability":        `{"proficiencies":[{"id":"a","ability":"dex"},{"id":"b","ability":"luck"}]}`,
		"empty id":               `{"proficiencies":[{"id":"","ability":"dex"}]}`,
		"duplicate across lists": `{"proficiencies":[{"id":"a","ability":"dex"}],"bard_skills":[{"id":"a","percent":5}]}`,
		"percent above 100":      `{"bard_skills":[{"id":"a","percent":101}]}`,
		"negative percent":       `{"bard_skills":[{"id":"a","percent":-1}]}`,
		"invalid json":           `{nope`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			db := openMigrated(t)
			require.Error(t, db.Seed(context.Background(), seedsWithSkills(t, content)))
			assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM proficiencies"))
			assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM bard_skills"))
		})
	}
}

func TestSeed_MissingSkillsFileIsAStartupError(t *testing.T) {
	dir := seedsWithSkills(t, "{}")
	require.NoError(t, os.Remove(filepath.Join(dir, "skills.json")))
	err := openMigrated(t).Seed(context.Background(), dir)
	require.ErrorContains(t, err, "seed skills")
	require.ErrorContains(t, err, "seed file is unreadable")
}
