package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// campaignSeeds is the campaign's seeds/ (the real Power Gem), not the test
// fixture.
func campaignSeeds() string { return filepath.Join("..", "..", "..", "..", "seeds") }

// migratedTo031 is a database with every migration before 032 applied.
func migratedTo031(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql", "023_correction_step.sql", "024_dice_favourites_position.sql",
		"025_action_transaction.sql", "026_redo_correction.sql", "027_spell_catalogue.sql", "028_buff_spell_id.sql",
		"029_spell_stats_fr.sql", "030_drop_spell_stats_fr.sql", "031_localized_texts.sql")
	return db
}

// gemRow is the Power Gem's row: what 032 and the seed set, and what they
// keep.
type gemRow struct {
	name, notes, powers string
	position, count     int
}

func readGem(t *testing.T, db *DB) gemRow {
	t.Helper()
	var g gemRow
	require.NoError(t, db.QueryRow(`SELECT name, notes, powers, position, count FROM items WHERE id = 'power_gem_dispel'`).
		Scan(&g.name, &g.notes, &g.powers, &g.position, &g.count))
	return g
}

// thunderHammer is the gem's one listed power (ADR 0016 step 16).
var thunderHammer = []itemsvc.Power{{
	ID: "thunder_hammer", MinStr: new(18),
	Effects: buffsvc.BuffEffects{WeaponBuff: new(5), Multiplier: new(2.0), WeaponKind: buffsvc.WeaponMelee},
}}

func decodedPowers(t *testing.T, raw string) []itemsvc.Power {
	t.Helper()
	ps, err := itemsvc.DecodePowers(raw)
	require.NoError(t, err)
	return ps
}

const (
	oldGemName  = "Power Gem: Dispel Magic"
	oldGemNotes = "1/day big Dispel Magic. The weapon becomes +5 while the gem is used (STR 18 gate, handled by buffs)."
)

// ADR 0016 step 16, migration 032: on the row power_gem_dispel (the id stays)
// it sets the thunder hammer, renames the seed name to "Power Gem" and
// replaces the seed's old note, keeping position and count; the result is
// the item a new database gets from seeds/items.json. A name or a note the
// player changed is kept; the other items get no powers; a database without
// the row is untouched; running the statements again changes nothing.
func TestMigrate_Step16_GivesTheGemItsPowersAndKeepsItsID(t *testing.T) {
	ctx := context.Background()
	insertGem := func(t *testing.T, db *DB, name, notes string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO items (id, position, name, type, behaviour, count, max_count, notes) VALUES
			('arrows_plus1', 0, 'Arrows +1', 'arrow', 'stock', 11, 18, ''),
			('power_gem_dispel', 5, ?, 'gem', 'daily', 0, 1, ?)`, name, notes)
		require.NoError(t, err)
	}

	t.Run("the seeded gem as played", func(t *testing.T) {
		db := migratedTo031(t)
		insertGem(t, db, oldGemName, oldGemNotes)
		require.NoError(t, db.Migrate(ctx))

		got := readGem(t, db)
		assert.Equal(t, "Power Gem", got.name)
		assert.Equal(t, thunderHammer, decodedPowers(t, got.powers))
		assert.Contains(t, got.notes, "Dispel Magic once a day (10 m radius)")
		assert.Equal(t, 5, got.position, "position kept")
		assert.Equal(t, 0, got.count, "count kept (the dispel is spent)")
		assert.Equal(t, "[]", func() string {
			var p string
			require.NoError(t, db.QueryRow(`SELECT powers FROM items WHERE id = 'arrows_plus1'`).Scan(&p))
			return p
		}(), "other items get no powers")

		// The same item as a new database seeded from seeds/items.json.
		fresh := openMigrated(t)
		require.NoError(t, fresh.Seed(ctx, campaignSeeds()))
		seeded := readGem(t, fresh)
		assert.Equal(t, seeded.name, got.name)
		assert.Equal(t, seeded.notes, got.notes)
		assert.Equal(t, decodedPowers(t, seeded.powers), decodedPowers(t, got.powers))

		// Idempotent: the statements run again change nothing (the column
		// exists: its ALTER is skipped).
		content, err := migrationFiles.ReadFile("migrations/032_item_powers.sql")
		require.NoError(t, err)
		require.NoError(t, db.execStatements(ctx, "032_item_powers.sql", string(content)))
		assert.Equal(t, got, readGem(t, db))
	})

	t.Run("a name and a note the player changed are kept", func(t *testing.T) {
		db := migratedTo031(t)
		insertGem(t, db, "Gemme de puissance", "mine")
		require.NoError(t, db.Migrate(ctx))
		got := readGem(t, db)
		assert.Equal(t, "Gemme de puissance", got.name)
		assert.Equal(t, "mine", got.notes)
		assert.Equal(t, thunderHammer, decodedPowers(t, got.powers), "the powers are set all the same")
	})

	t.Run("a database without the gem is untouched", func(t *testing.T) {
		db := migratedTo031(t)
		_, err := db.ExecContext(ctx, `INSERT INTO items (id, position, name, type, behaviour, count, max_count, notes)
			VALUES ('wand', 0, 'Power Gem: Dispel Magic', 'wand', 'charges', 3, 4, '')`)
		require.NoError(t, err)
		require.NoError(t, db.Migrate(ctx))
		assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM items`))
		assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM items WHERE id = 'wand' AND name = 'Power Gem: Dispel Magic' AND powers = '[]'`),
			"only the row power_gem_dispel is the gem")
	})
}

// ADR 0016 step 16: a new database seeded from seeds/items.json (and the
// test fixture alike) has the one "Power Gem": daily, 1 a day, with the
// thunder hammer; the other items have no powers.
func TestSeed_Step16_PowerGemHasTheHammer(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": campaignSeeds(), "test fixture": repoSeeds()} {
		t.Run(name, func(t *testing.T) {
			db := openMigrated(t)
			require.NoError(t, db.Seed(context.Background(), dir))
			got := readGem(t, db)
			assert.Equal(t, "Power Gem", got.name)
			assert.Equal(t, 1, got.count)
			assert.Equal(t, thunderHammer, decodedPowers(t, got.powers))
			assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM items WHERE id = 'power_gem_dispel' AND behaviour = 'daily' AND max_count = 1`))
			assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM items WHERE powers <> '[]'`), "only the gem has powers")
		})
	}
}

// ADR 0016 step 9: a seed item whose power has a Strength effect (or any
// invalid power) refuses the seed, and no item is stored.
func TestSeed_Step9_RejectsAPowerWithAStrengthEffect(t *testing.T) {
	tests := map[string]string{
		"str_bonus":      `{"id":"belt","effects":{"str_bonus":4}}`,
		"str_max":        `{"id":"belt","effects":{"str_max":19}}`,
		"unknown effect": `{"id":"belt","effects":{"weapon_bluff":5}}`,
		"bad weapon":     `{"id":"belt","effects":{"weapon_kind":"thrown"}}`,
	}
	for name, power := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			content := `[{"id":"a","name":"A","behaviour":"stock","count":1},
				{"id":"belt","name":"Belt","behaviour":"stock","count":1,"powers":[` + power + `]}]`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "items.json"), []byte(content), 0o600))
			weapons, err := os.ReadFile(filepath.Join(repoSeeds(), "weapons.json"))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "weapons.json"), weapons, 0o600))
			db := openMigrated(t)
			err = db.Seed(context.Background(), dir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"belt"`)
			assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM items"))
		})
	}
}
