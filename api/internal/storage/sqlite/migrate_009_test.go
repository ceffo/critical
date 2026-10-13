package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 009 adds a nullable stats column: spells seeded before it keep their rows,
// and the next Seed types them from their raw text without duplicating them.
func TestMigrate009_ExistingSpellsAreBackfilledBySeed(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql", "006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO spells (name, class, level, casting_time, range, duration, area_of_effect, components, saving_throw)
		VALUES ('Old', 'Wizard', 1, '1', '10 yds./level', '1 rd./level', '20-ft. radius', 'V, S', '1/2')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM spells"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM spells WHERE stats IS NULL"))

	// Weapons/items seeds are required; the fixture has them.
	require.NoError(t, db.Seed(ctx, repoSeeds()))
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM spells WHERE stats IS NULL"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM spells WHERE name = 'Old'"), "no duplicate rows")
	var blob string
	require.NoError(t, db.QueryRow("SELECT stats FROM spells WHERE name = 'Old'").Scan(&blob))
	var got struct {
		Range struct {
			Raw  string `json:"raw"`
			Kind string `json:"kind"`
		} `json:"range"`
	}
	require.NoError(t, json.Unmarshal([]byte(blob), &got))
	assert.Equal(t, "10 yds./level", got.Range.Raw)
	assert.Equal(t, "distance", got.Range.Kind)
}
