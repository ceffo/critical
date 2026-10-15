package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migratedTo027 is a database with every migration before 028 applied.
func migratedTo027(t *testing.T) *DB {
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
		"025_action_transaction.sql", "026_redo_correction.sql", "027_spell_catalogue.sql")
	return db
}

// buffList decodes a JSON buff list into generic objects, keeping every key.
func buffList(t *testing.T, db *DB, q string) []map[string]any {
	t.Helper()
	var blob string
	require.NoError(t, db.QueryRow(q).Scan(&blob))
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(blob), &out))
	return out
}

// legacyBuffs is a buff list stored before buffs carried a spell id: one
// buff per case of the backfill rule.
const legacyBuffs = `[
	{"id":"a","name":"Strength","charges":0,"duration":5,"effects":{"str_bonus":3},"start_round":2},
	{"id":"b","name":"Light","charges":0,"duration":10,"effects":{}},
	{"id":"c","name":"Bless","charges":0,"duration":6,"effects":{"to_hit":1}},
	{"id":"d","name":"Power Gem: Fire","charges":0,"duration":0,"effects":{"min_str":18,"weapon_buff":5}},
	{"id":"e","name":"strength","charges":0,"duration":3,"effects":{"str_bonus":1}},
	{"id":"f","name":"Stoneskin","charges":4,"duration":0,"effects":{},"charges_total":6,"spell_id":"stoneskin_homebrew"},
	{"id":"g","name":"Stoneskin","charges":2,"duration":0,"effects":{},"note":"second","charges_total":2}
]`

// 028 (ADR 0004 step 17): a stored buff takes the spell id of the one keyed
// catalogue spell whose English name equals its name exactly. A name no
// keyed spell has, a name two spells share, a different case and a buff that
// already has an id are left alone, and so is a list holding anything but
// buff objects. The active lists and the buff snapshots
// are backfilled alike (undo and redo restore them), keeping the order and
// every other key; other snapshots and log payloads are not touched.
func TestMigrate028_BackfillsBuffSpellIDsWhereTheNameIsUnambiguous(t *testing.T) {
	ctx := context.Background()
	db := migratedTo027(t)
	for _, q := range []string{
		`INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`,
		`INSERT INTO spells (id, name, class, level, key) VALUES
			(1, 'Strength', 'Wizard', 2, 'strength'),
			(2, 'Light', 'Wizard', 1, 'light'),
			(3, 'Light', 'Priest', 1, 'light_priest'),
			(4, 'Bless', 'Priest', 1, NULL),
			(5, 'Stoneskin', 'Wizard', 4, 'stoneskin')`,
		`INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (2, 'U', 10, 10, 10, 10, 10, 10, 30, 19)`,
		`INSERT INTO active_buffs (character_id, buffs) VALUES (1, '` + legacyBuffs + `'), (2, '["Strength",{"name":"Strength"}]')`,
		`INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload, transaction_id) VALUES
			(1, 1, 0, 'buff_change', '', 0, '{"action":"activate","buff":"Strength"}', 1),
			(2, 1, 0, 'spell_prep', '', 0, '{}', 2)`,
		`INSERT INTO action_snapshots (action_id, state_key, before_state, after_state) VALUES
			(1, 'buffs', '[]', '` + legacyBuffs + `'),
			(2, 'spell_slots', '[{"Level":1,"SpellID":"","PreparedSpellName":"Strength"}]', '[{"name":"Strength"}]')`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '028_buff_spell_id.sql'"))
	want := func(buffs []map[string]any) {
		t.Helper()
		ids := map[string]any{}
		order := make([]any, len(buffs))
		for i, b := range buffs {
			ids[b["id"].(string)] = b["spell_id"]
			order[i] = b["id"]
		}
		assert.Equal(t, map[string]any{
			"a": "strength",           // one keyed spell named Strength
			"b": nil,                  // Wizard and Priest Light: ambiguous
			"c": nil,                  // Bless is not keyed
			"d": nil,                  // no spell of that name
			"e": nil,                  // names compare exactly
			"f": "stoneskin_homebrew", // already set: kept
			"g": "stoneskin",
		}, ids)
		assert.Equal(t, []any{"a", "b", "c", "d", "e", "f", "g"}, order, "order kept")
		assert.Equal(t, map[string]any{
			"id": "a", "name": "Strength", "charges": float64(0), "duration": float64(5),
			"effects": map[string]any{"str_bonus": float64(3)}, "start_round": float64(2), "spell_id": "strength",
		}, buffs[0], "every other key kept")
		assert.Equal(t, "second", buffs[6]["note"])
		assert.InDelta(t, 2, buffs[6]["charges_total"], 0)
	}
	want(buffList(t, db, `SELECT buffs FROM active_buffs WHERE character_id = 1`))
	want(buffList(t, db, `SELECT after_state FROM action_snapshots WHERE action_id = 1`))
	assert.Empty(t, buffList(t, db, `SELECT before_state FROM action_snapshots WHERE action_id = 1`))

	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM active_buffs WHERE character_id = 2 AND buffs = '["Strength",{"name":"Strength"}]'`),
		"a list that is not a list of buffs is left as it is")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM action_snapshots WHERE action_id = 2
		AND before_state = '[{"Level":1,"SpellID":"","PreparedSpellName":"Strength"}]' AND after_state = '[{"name":"Strength"}]'`),
		"a snapshot of another state is not touched")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM action_log WHERE id = 1 AND payload = '{"action":"activate","buff":"Strength"}'`),
		"log payloads are not rewritten")
}

// 028 on a database whose catalogue was never imported (no keyed spell) and
// on an empty list changes nothing.
func TestMigrate028_NothingKeyedNothingChanges(t *testing.T) {
	ctx := context.Background()
	db := migratedTo027(t)
	for _, q := range []string{
		`INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES
			(1, 'T', 10, 10, 10, 10, 10, 10, 30, 19), (2, 'U', 10, 10, 10, 10, 10, 10, 30, 19)`,
		`INSERT INTO spells (id, name, class, level) VALUES (1, 'Strength', 'Wizard', 2)`,
		`INSERT INTO active_buffs (character_id, buffs) VALUES (1, '[{"id":"a","name":"Strength","charges":0,"duration":5,"effects":{}}]'), (2, '[]')`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}

	require.NoError(t, db.Migrate(ctx))

	buffs := buffList(t, db, `SELECT buffs FROM active_buffs WHERE character_id = 1`)
	require.Len(t, buffs, 1)
	assert.NotContains(t, buffs[0], "spell_id")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM active_buffs WHERE character_id = 2 AND buffs = '[]'`))
}
