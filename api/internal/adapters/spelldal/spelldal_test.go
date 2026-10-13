package spelldal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
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
		{"Sleep", "bard", 1},
		{"Charm Person", "bard", 1},
		{"Invisibility", "bard", 2},
		{"Sleep", "wizard", 1},
		{"Fireball", "wizard", 3},
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
	want := spellsvc.Spell{Name: "Fireball", Class: "wizard", Level: 3, School: "Enchantment", Description: "desc"}
	got[0].Stats = spellstats.Stats{}
	assert.Equal(t, want, got[0])
}

// A row without stats JSON (or with garbage) is typed from its raw columns.
func TestSearch_RowWithoutStatsIsParsedFromRawText(t *testing.T) {
	db := openDB(t)
	_, err := db.ExecContext(context.Background(), `INSERT INTO spells (name, class, level, range, saving_throw, stats)
		VALUES ('A', 'wizard', 1, '10 yds./level', '1/2', NULL), ('B', 'wizard', 2, '20 yds.', 'Neg.', '{not json')`)
	require.NoError(t, err)

	got, err := spelldal.New(db).Search(context.Background(), "", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 150.0, got[0].Stats.At(15).Range.Amount)
	assert.Equal(t, []string{"half"}, got[0].Stats.Save.Kinds)
	assert.Equal(t, []string{"negates"}, got[1].Stats.Save.Kinds)
}

// Seeding stores the typed stats; the DAL returns them with the raw text.
func TestSearch_SeededSpellsCarryTypedStats(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	require.NoError(t, db.Seed(ctx, filepath.Join("..", "..", "..", "testdata", "seeds")))

	got, err := spelldal.New(db).Search(ctx, "Magic Missile", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	mm := got[0]
	assert.Equal(t, "60 yds. + 10 yds./level", mm.Range)
	assert.Equal(t, "60 yds. + 10 yds./level", mm.Stats.Range.Raw)
	assert.Equal(t, spellstats.RangeDistance, mm.Stats.Range.Kind)
	assert.Equal(t, 210.0, mm.Stats.At(15).Range.Amount)

	fb, ok, err := spelldal.New(db).Describe(ctx, "fireball")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{spellstats.SaveHalf}, fb.Stats.Save.Kinds)
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

func TestDescribe(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	for _, r := range []struct{ name, class string }{
		{"Light", "Priest"}, {"Light", "Wizard"}, {"Melf's Acid Arrow", "Wizard"}, {"Bless", "Priest"},
	} {
		_, err := db.ExecContext(ctx, `INSERT INTO spells (name, class, level, description) VALUES (?, ?, 1, ?)`,
			r.name, r.class, r.class+" text")
		require.NoError(t, err)
	}
	d := spelldal.New(db)
	tests := []struct {
		id, wantName, wantClass string
		found                   bool
	}{
		{"light", "Light", "Wizard", true},
		{"melfs_acid_arrow", "Melf's Acid Arrow", "Wizard", true},
		{"bless", "Bless", "Priest", true},
		{"no_such_spell", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got, ok, err := d.Describe(ctx, tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.found, ok)
			assert.Equal(t, tt.wantName, got.Name)
			assert.Equal(t, tt.wantClass, got.Class)
		})
	}
}
