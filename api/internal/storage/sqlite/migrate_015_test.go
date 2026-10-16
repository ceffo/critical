package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 015 replaces items.kind and items.per_day with behaviour and type.
// Every old row is kept with its count, max, bonus, ammo
// kind, effect, note and position: ammo and consumable become stock, charged
// with per_day daily, other charged charges, and the type comes from the ammo
// kind, else the name, else the old kind.
func TestMigrate015_ItemsKeepEveryRowWithBehaviourAndType(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql")
	// The seeded rows as played (counts spent), plus a charged item already at
	// 0 and a consumable whose name gives no type.
	_, err = db.ExecContext(ctx, `INSERT INTO items (id, position, name, kind, magic_bonus, ammo_kind, count, max_count, per_day, effect_dice, effect_save, notes) VALUES
		('arrows_plus1', 0, 'Arrows +1', 'ammo', 1, 'arrow', 11, 18, 0, '', '', ''),
		('fire_flask', 1, 'Fire flasks', 'consumable', 0, '', 3, 5, 0, '5d6', '', ''),
		('wand_lightning', 2, 'Wand of lightning', 'charged', 0, '', 2, 4, 0, '6d6', 'rod_staff_wand', ''),
		('power_gem_dispel', 3, 'Power Gem: Dispel Magic', 'charged', 0, '', 0, 1, 1, '', '', '1/day'),
		('rod_empty', 4, 'Rod of nothing', 'charged', 0, '', 0, 3, 0, '', '', ''),
		('salve', 5, 'Healing salve', 'consumable', 0, '', 2, 2, 0, '1d8', '', 'apply')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '015_item_behaviour.sql'"))
	type row struct {
		id, name, typ, behaviour, ammo, dice, save, notes string
		pos, bonus, count, max                            int
	}
	rows, err := db.QueryContext(ctx, `SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes FROM items ORDER BY position`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.id, &r.pos, &r.name, &r.typ, &r.behaviour, &r.bonus, &r.ammo, &r.count, &r.max, &r.dice, &r.save, &r.notes))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []row{
		{"arrows_plus1", "Arrows +1", "arrow", "stock", "arrow", "", "", "", 0, 1, 11, 18},
		{"fire_flask", "Fire flasks", "flask", "stock", "", "5d6", "", "", 1, 0, 3, 5},
		{"wand_lightning", "Wand of lightning", "wand", "charges", "", "6d6", "rod_staff_wand", "", 2, 0, 2, 4},
		// Renamed since by 032 (ADR 0016 step 16); its note is not the seed's, so it stays.
		{"power_gem_dispel", "Power Gem", "gem", "daily", "", "", "", "1/day", 3, 0, 0, 1},
		{"rod_empty", "Rod of nothing", "charged", "charges", "", "", "", "", 4, 0, 0, 3},
		{"salve", "Healing salve", "consumable", "stock", "", "1d8", "", "apply", 5, 0, 2, 2},
	}, got)

	// The new CHECK holds and the old columns are gone.
	_, err = db.ExecContext(ctx, `UPDATE items SET behaviour = 'charged' WHERE id = 'salve'`)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `SELECT per_day FROM items`)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `SELECT kind FROM items`)
	require.Error(t, err)
}
