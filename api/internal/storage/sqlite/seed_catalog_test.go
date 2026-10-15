package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoSeeds is the frozen test fixture, not the campaign seeds/.
func repoSeeds() string { return filepath.Join("..", "..", "..", "testdata", "seeds") }

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
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM items WHERE id = 'power_gem_dispel' AND behaviour = 'daily' AND type = 'gem'"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM items WHERE id = 'wand_lightning' AND behaviour = 'charges' AND type = 'wand'"))
	assert.Equal(t, 4, scalar(t, db, "SELECT COUNT(*) FROM items WHERE behaviour = 'stock'"), "arrows and flasks")

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
		"item bad behaviour":    {"items.json", `[{"id":"a","name":"A","behaviour":"relic","count":1}]`, "items"},
		"item old kind":         {"items.json", `[{"id":"a","name":"A","kind":"ammo","count":1}]`, "items"},
		"item negative count":   {"items.json", `[{"id":"a","name":"A","behaviour":"stock","count":-1}]`, "items"},
		"weapon duplicate id":   {"weapons.json", `[{"id":"a","name":"A","damage_sm":"1d4","damage_l":"1d4","kind":"melee"},{"id":"a","name":"B","damage_sm":"1d4","damage_l":"1d4","kind":"melee"}]`, "weapons"},
		"item duplicate id":     {"items.json", `[{"id":"a","name":"A","behaviour":"stock","count":1},{"id":"a","name":"B","behaviour":"stock","count":1}]`, "items"},
		"item count over max":   {"items.json", `[{"id":"a","name":"A","behaviour":"daily","count":5,"max_count":3}]`, "items"},
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
			present, content := "items.json", `[{"id":"a","name":"A","behaviour":"stock","count":1}]`
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

type spellRow struct {
	ID                                                int64
	Name, Class, School, Duration, Description, Stats string
}

func spellRows(t *testing.T, db *DB) []spellRow {
	t.Helper()
	rows, err := db.Query(`SELECT id, name, class, COALESCE(school,''), COALESCE(duration,''), COALESCE(description,''), COALESCE(stats,'') FROM spells ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []spellRow
	for rows.Next() {
		var r spellRow
		require.NoError(t, rows.Scan(&r.ID, &r.Name, &r.Class, &r.School, &r.Duration, &r.Description, &r.Stats))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestSeed_SpellsReconcileAnExistingCatalog(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	var fixture []spellSeed
	data, err := os.ReadFile(filepath.Join(repoSeeds(), "spells.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.Greater(t, len(fixture), 3)

	// A database seeded earlier: a subset of the catalog, one stale value, an
	// untyped row, a spell the seed does not know, and a duplicated name.
	first, second := fixture[0], fixture[1]
	ins := `INSERT INTO spells (id, name, class, level, school, duration, description) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(ins, 500, first.Name, first.Class, first.Level, "Stale", "Stale", "stale text")
	require.NoError(t, err)
	_, err = db.Exec(ins, 600, second.Name, second.Class, second.Level, second.School, second.Duration, second.Description)
	require.NoError(t, err)
	_, err = db.Exec(ins, 700, "Homebrew Bolt", "Wizard", 1, "Evocation", "1 rd.", "not in the seed")
	require.NoError(t, err)
	_, err = db.Exec(ins, 800, first.Name, first.Class, first.Level, "Stale dup", "Stale", "stale dup")
	require.NoError(t, err)

	require.NoError(t, db.Seed(ctx, repoSeeds()))

	got := spellRows(t, db)
	byID := map[int64]spellRow{}
	for _, r := range got {
		byID[r.ID] = r
	}
	assert.Equal(t, len(fixture)+2, len(got), "every seed spell once, plus the unknown and the duplicate row, none deleted")
	assert.Equal(t, first.Name, byID[500].Name, "ids of existing rows are unchanged")
	assert.Equal(t, first.School, byID[500].School, "stale value updated")
	assert.Equal(t, first.Duration, byID[500].Duration)
	assert.Equal(t, first.Description, byID[500].Description)
	assert.NotEmpty(t, byID[500].Stats, "typed stats written")
	assert.Equal(t, second.Name, byID[600].Name)
	assert.NotEmpty(t, byID[600].Stats, "untyped row gets its stats")
	assert.Equal(t, "Homebrew Bolt", byID[700].Name, "a spell outside the seed is kept")
	assert.Equal(t, "Stale dup", byID[800].School, "only the oldest row of a duplicated name is reconciled")
	for _, s := range fixture {
		n := scalar(t, db, "SELECT COUNT(*) FROM spells WHERE name = '"+strings.ReplaceAll(s.Name, "'", "''")+"' AND class = '"+s.Class+"'")
		want := 1
		if s.Name == first.Name && s.Class == first.Class {
			want = 2
		}
		assert.Equal(t, want, n, "%s %s", s.Class, s.Name)
	}

	// A restart changes nothing.
	require.NoError(t, db.Seed(ctx, repoSeeds()))
	assert.Equal(t, got, spellRows(t, db))
}

func TestSeed_SpellsFreshDatabaseLoadsTheWholeCatalog(t *testing.T) {
	db := openMigrated(t)
	require.NoError(t, db.Seed(context.Background(), repoSeeds()))
	var fixture []spellSeed
	data, err := os.ReadFile(filepath.Join(repoSeeds(), "spells.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &fixture))
	assert.Equal(t, len(fixture), scalar(t, db, "SELECT COUNT(*) FROM spells"))
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM spells WHERE stats IS NULL"))
}

// Two spells may share name and class when their levels differ; the old row
// of one of them reconciles to its own level instead of being orphaned.
func TestSeed_SpellsKeyedByNameClassAndLevel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	row := func(level int, sphere, desc string) string {
		return fmt.Sprintf(`{"name":"Twin","class":"Priest","level":%d,"school":"Alteration","sphere":%q,`+
			`"casting_time":"6","range":"0","duration":"Special","area_of_effect":"Creature touched",`+
			`"components":"V, S, M","saving_throw":"None","description":%q}`, level, sphere, desc)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spells.json"),
		[]byte("["+row(3, "War", "level three")+","+row(4, "Healing", "level four")+"]"), 0o600))

	for _, f := range []string{"weapons.json", "items.json", "skills.json"} {
		b, err := os.ReadFile(filepath.Clean(filepath.Join(repoSeeds(), f)))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), b, 0o600)) //nolint:gosec // test temp dir, fixed file names
	}

	db := openMigrated(t)
	_, err := db.Exec(`INSERT INTO spells (id, name, class, level, sphere, description) VALUES (40, 'Twin', 'Priest', 4, 'Old', 'stale four')`)
	require.NoError(t, err)

	require.NoError(t, db.Seed(ctx, dir))
	got := spellRows(t, db)
	require.Len(t, got, 2, "the level-4 row is reconciled, not duplicated; level 3 is inserted")
	var l4 string
	require.NoError(t, db.QueryRow(`SELECT description FROM spells WHERE id = 40`).Scan(&l4))
	assert.Equal(t, "level four", l4)
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM spells WHERE level = 3 AND description = 'level three'"))

	require.NoError(t, db.Seed(ctx, dir))
	assert.Equal(t, got, spellRows(t, db), "a restart changes nothing")
}
