package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repoSeeds() string { return filepath.Join("..", "..", "..", "..", "seeds") }

func openMigrated(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func scalar(t *testing.T, db *DB, q string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(q).Scan(&n))
	return n
}

func TestSeed_WeaponsAndItemsMatchSheetAndAreIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	require.NoError(t, db.Seed(ctx, repoSeeds()))

	assert.Equal(t, 6, scalar(t, db, "SELECT COUNT(*) FROM weapons"))
	assert.Equal(t, 6, scalar(t, db, "SELECT COUNT(*) FROM items"))
	assert.Equal(t, 18, scalar(t, db, "SELECT count FROM items WHERE id = 'arrows_plus1'"))
	assert.Equal(t, 16, scalar(t, db, "SELECT count FROM items WHERE id = 'arrows_plus2'"))
	assert.Equal(t, 2, scalar(t, db, "SELECT count FROM items WHERE id = 'arrows_plus3'"))
	assert.Equal(t, 5, scalar(t, db, "SELECT count FROM items WHERE id = 'fire_flask'"))
	assert.Equal(t, 4, scalar(t, db, "SELECT max_count FROM items WHERE id = 'wand_lightning'"), "max_count defaults to count")
	assert.Equal(t, 1, scalar(t, db, "SELECT per_day FROM items WHERE id = 'power_gem_dispel'"))

	// A table change must survive a re-seed (a restart).
	_, err := db.Exec("UPDATE items SET count = 3 WHERE id = 'arrows_plus1'")
	require.NoError(t, err)
	require.NoError(t, db.Seed(ctx, repoSeeds()))
	assert.Equal(t, 6, scalar(t, db, "SELECT COUNT(*) FROM items"))
	assert.Equal(t, 3, scalar(t, db, "SELECT count FROM items WHERE id = 'arrows_plus1'"))
	assert.Equal(t, 6, scalar(t, db, "SELECT COUNT(*) FROM weapons"))
}

func TestSeed_InvalidCatalogsFailWithoutPartialRows(t *testing.T) {
	tests := map[string]struct{ file, content, table string }{
		"weapon missing damage": {"weapons.json", `[{"id":"a","name":"A","damage_sm":"1d4","damage_l":"1d4","kind":"melee"},{"id":"b","name":"B"}]`, "weapons"},
		"weapon bad kind":       {"weapons.json", `[{"id":"a","name":"A","damage_sm":"1d4","damage_l":"1d4","kind":"magic"}]`, "weapons"},
		"item bad kind":         {"items.json", `[{"id":"a","name":"A","kind":"relic","count":1}]`, "items"},
		"item negative count":   {"items.json", `[{"id":"a","name":"A","kind":"ammo","count":-1}]`, "items"},
		"weapon duplicate id":   {"weapons.json", `[{"id":"a","name":"A","damage_sm":"1d4","damage_l":"1d4","kind":"melee"},{"id":"a","name":"B","damage_sm":"1d4","damage_l":"1d4","kind":"melee"}]`, "weapons"},
		"item duplicate id":     {"items.json", `[{"id":"a","name":"A","kind":"ammo","count":1},{"id":"a","name":"B","kind":"ammo","count":1}]`, "items"},
		"item count over max":   {"items.json", `[{"id":"a","name":"A","kind":"ammo","count":5,"max_count":3}]`, "items"},
		"invalid json":          {"items.json", `{nope`, "items"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.content), 0o600))
			db := openMigrated(t)
			require.Error(t, db.Seed(context.Background(), dir))
			assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM "+tt.table))
		})
	}
}

func TestSeed_MissingCatalogFileIsAStartupError(t *testing.T) {
	for _, missing := range []string{"weapons.json", "items.json"} {
		t.Run(missing, func(t *testing.T) {
			dir := t.TempDir()
			present, content := "items.json", `[{"id":"a","name":"A","kind":"ammo","count":1}]`
			if missing == "items.json" {
				present, content = "weapons.json", `[{"id":"a","name":"A","damage_sm":"1d4","damage_l":"1d4","kind":"melee"}]`
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, present), []byte(content), 0o600))
			err := openMigrated(t).Seed(context.Background(), dir)
			require.ErrorContains(t, err, missing[:len(missing)-5])
			require.ErrorContains(t, err, "seed file is unreadable")
		})
	}
}

func TestSeed_MissingCatalogFileIsFineOnceTableIsFilled(t *testing.T) {
	db := openMigrated(t)
	require.NoError(t, db.Seed(context.Background(), repoSeeds()))
	require.NoError(t, db.Seed(context.Background(), t.TempDir()))
}
