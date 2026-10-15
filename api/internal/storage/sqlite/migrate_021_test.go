package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 021 adds the character details: an existing character keeps its row, gets
// the Bard class and the sheet languages once, and a later edit survives the
// next start (the migration does not run twice).
func TestMigrate021_ExistingCharacterGetsLanguagesOnce(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	entries, err := migrationFiles.ReadDir("migrations")
	require.NoError(t, err)
	var before []string
	for _, e := range entries {
		if e.Name() < "021_character_details.sql" {
			before = append(before, e.Name())
		}
	}
	applyUpTo(t, db, before...)
	_, err = db.ExecContext(ctx, `INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base)
		VALUES ('Old', 12, 12, 12, 12, 12, 12, 40, 15)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))
	var class, race, langs string
	var age int
	require.NoError(t, db.QueryRow("SELECT class, race, age, languages FROM characters").Scan(&class, &race, &age, &langs))
	assert.Equal(t, "bard", class)
	assert.Empty(t, race)
	assert.Zero(t, age)
	assert.JSONEq(t, `["Elfique","Commun","Orque","Nain"]`, langs)

	_, err = db.ExecContext(ctx, `UPDATE characters SET languages = '[]'`)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.QueryRow("SELECT languages FROM characters").Scan(&langs))
	assert.Equal(t, "[]", langs, "a second start does not restore the languages")
}

// A fresh database takes the languages from the character seed, and the seed
// is idempotent.
func TestSeed_CharacterLanguagesFromSeed(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seeds := filepath.Join("..", "..", "..", "..", "seeds")
	require.NoError(t, db.Seed(ctx, seeds))
	require.NoError(t, db.Seed(ctx, seeds))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM characters"))
	var raw string
	require.NoError(t, db.QueryRow("SELECT languages FROM characters").Scan(&raw))
	var langs []string
	require.NoError(t, json.Unmarshal([]byte(raw), &langs))
	assert.Equal(t, []string{"Elfique", "Commun", "Orque", "Nain"}, langs)
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM characters WHERE race != '' OR alignment != '' OR age != 0 OR sex != '' OR height != '' OR weight != '' OR hair != '' OR eyes != ''"))
}
