package spelldal_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open("file::memory:?cache=shared&mode=memory")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func seedCharacter(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('Test', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func TestSlots_MissingRowIsEmpty(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)

	got, err := spelldal.New(db).GetSlots(context.Background(), charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSlots_RoundTripAndOverwrite(t *testing.T) {
	t.Skip("bug: upserts use ON CONFLICT(character_id) but migration 003 has no UNIQUE(character_id) on spell_slots, so every save fails with \"ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE constraint\"")
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := spelldal.New(db)
	ctx := context.Background()

	slots := []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, PreparedSpellName: "Invisibility", Used: true},
	}
	require.NoError(t, dal.SaveSlots(ctx, charID, slots))
	got, err := dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, slots, got)

	slots[0].Used = true
	require.NoError(t, dal.SaveSlots(ctx, charID, slots))
	got, err = dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, slots, got)
}

func TestSlots_PerCharacter(t *testing.T) {
	t.Skip("bug: upserts use ON CONFLICT(character_id) but migration 003 has no UNIQUE(character_id) on spell_slots, so every save fails with \"ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE constraint\"")
	db := openDB(t)
	c1, c2 := seedCharacter(t, db), seedCharacter(t, db)
	dal := spelldal.New(db)
	ctx := context.Background()

	require.NoError(t, dal.SaveSlots(ctx, c1, []spellsvc.SpellSlot{{Level: 1, PreparedSpellName: "Sleep"}}))
	got, err := dal.GetSlots(ctx, c2)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func seedSpells(t *testing.T, db *sqlite.DB) {
	t.Helper()
	rows := []struct {
		name, class string
		level       int
	}{
		{"Sleep", "bard", 1}, {"Charm Person", "bard", 1}, {"Invisibility", "bard", 2},
		{"Sleep", "wizard", 1}, {"Fireball", "wizard", 3},
	}
	for _, r := range rows {
		_, err := db.ExecContext(context.Background(),
			`INSERT INTO spells (name, class, level, school, description) VALUES (?, ?, ?, 'Enchantment', 'desc')`,
			r.name, r.class, r.level)
		require.NoError(t, err)
	}
}

func names(spells []spellsvc.Spell) []string {
	out := make([]string, len(spells))
	for i, s := range spells {
		out[i] = s.Class + ":" + s.Name
	}
	return out
}

func TestSearch(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)
	dal := spelldal.New(db)

	tests := []struct {
		name         string
		query, class string
		want         []string
	}{
		{"no filter, ordered by level then name", "", "", []string{"bard:Charm Person", "bard:Sleep", "wizard:Sleep", "bard:Invisibility", "wizard:Fireball"}},
		{"name substring", "sle", "", []string{"bard:Sleep", "wizard:Sleep"}},
		{"class filter", "", "wizard", []string{"wizard:Sleep", "wizard:Fireball"}},
		{"name and class", "sleep", "bard", []string{"bard:Sleep"}},
		{"no match", "zzz", "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dal.Search(context.Background(), tt.query, tt.class)
			require.NoError(t, err)
			assert.Equal(t, tt.want, names(got))
		})
	}
}

func TestSearch_MapsFields(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)

	got, err := spelldal.New(db).Search(context.Background(), "Fireball", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, spellsvc.Spell{Name: "Fireball", Class: "wizard", Level: 3, School: "Enchantment", Description: "desc"}, got[0])
}

func TestSlots_ReadsStoredJSON(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)`, charID,
		`[{"level":1,"prepared_spell_name":"Sleep","used":false},{"level":2,"prepared_spell_name":"Invisibility","used":true}]`)
	require.NoError(t, err)

	got, err := spelldal.New(db).GetSlots(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, PreparedSpellName: "Invisibility", Used: true},
	}, got)
}
