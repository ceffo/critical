package buffdal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
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

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestActiveBuffs_MissingRowIsEmpty(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)

	got, err := buffdal.New(db).GetActiveBuffs(context.Background(), charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestActiveBuffs_RoundTrip(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	buffs := []buffsvc.BuffEffect{
		{ID: "a", Name: "Power Gem", Duration: 5, Effects: buffsvc.BuffEffects{
			WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5), MinStr: intPtr(18), SaveBonus: intPtr(1),
		}},
		{ID: "b", Name: "Mirror Image", Duration: 27, Charges: 3},
		{ID: "c", Name: "Strength", Duration: 540, Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(4), StrMax: intPtr(18), AC: intPtr(-2),
		}},
		{ID: "d", Name: "Bless", Duration: 6, Effects: buffsvc.BuffEffects{ToHit: intPtr(1), Damage: intPtr(-1)}},
	}
	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, buffs))

	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, buffs, got)
}

func TestActiveBuffs_SaveOverwrites(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, []buffsvc.BuffEffect{{ID: "a"}, {ID: "b"}}))
	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, []buffsvc.BuffEffect{{ID: "b"}}))

	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "b", got[0].ID)
}

func TestActiveBuffs_SaveEmpty(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, []buffsvc.BuffEffect{{ID: "a"}}))
	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, nil))

	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestActiveBuffs_PerCharacter(t *testing.T) {
	db := openDB(t)
	c1, c2 := seedCharacter(t, db), seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	require.NoError(t, dal.SaveActiveBuffs(ctx, c1, []buffsvc.BuffEffect{{ID: "a"}}))

	got, err := dal.GetActiveBuffs(ctx, c2)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRoundNumber(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	r, err := dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 0, r, "missing row means round 0")

	require.NoError(t, dal.SaveRoundNumber(ctx, charID, 3))
	require.NoError(t, dal.SaveRoundNumber(ctx, charID, 4))

	r, err = dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 4, r)
}

func TestInTx_CommitsOnSuccess(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	err := db.InTx(ctx, func(ctx context.Context) error {
		if err := dal.SaveRoundNumber(ctx, charID, 7); err != nil {
			return err
		}
		return dal.SaveActiveBuffs(ctx, charID, []buffsvc.BuffEffect{{ID: "a"}})
	})
	require.NoError(t, err)

	r, err := dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 7, r)
	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// Writes made with the ctx of an InTx join that transaction and roll back
// with it (a pool write would also deadlock SQLite while the tx is open).
func TestInTx_RollsBackOnError(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()
	require.NoError(t, dal.SaveRoundNumber(ctx, charID, 2))

	boom := errors.New("boom")
	err := db.InTx(ctx, func(ctx context.Context) error {
		require.NoError(t, dal.SaveRoundNumber(ctx, charID, 3))
		require.NoError(t, dal.SaveActiveBuffs(ctx, charID, []buffsvc.BuffEffect{{ID: "a"}}))
		r, err := dal.GetRoundNumber(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, 3, r, "reads inside the tx see its writes")
		return boom
	})
	assert.ErrorIs(t, err, boom)

	r, err := dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 2, r, "round must be rolled back")
	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Empty(t, got, "buffs must be rolled back")
}

// Reads work independently of the upsert bug skipped above: seed the rows by
// hand using the on-disk JSON format.
func TestActiveBuffs_ReadsStoredJSON(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO active_buffs (character_id, buffs) VALUES (?, ?)`, charID,
		`[{"id":"a","name":"Power Gem","type":"duration","charges":0,"duration":5,"effects":{"weapon_buff":5,"multiplier":2,"min_str":18}}]`)
	require.NoError(t, err)

	got, err := buffdal.New(db).GetActiveBuffs(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.BuffEffect{{
		ID: "a", Name: "Power Gem", Duration: 5,
		Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0), MinStr: intPtr(18)},
	}}, got)
}

// str_set ("set Strength to X") was removed (critical-qki.33): a row written
// before still reads, and the effect is ignored.
func TestActiveBuffs_IgnoresRemovedStrSet(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO active_buffs (character_id, buffs) VALUES (?, ?)`, charID,
		`[{"id":"o","name":"Ogre gauntlets","charges":0,"duration":0,"effects":{"str_set":18,"ac":-1}}]`)
	require.NoError(t, err)

	got, err := buffdal.New(db).GetActiveBuffs(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.BuffEffect{{
		ID: "o", Name: "Ogre gauntlets", Effects: buffsvc.BuffEffects{AC: intPtr(-1)},
	}}, got)
}

func TestActiveBuffs_CorruptJSON(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO active_buffs (character_id, buffs) VALUES (?, 'not json')`, charID)
	require.NoError(t, err)

	_, err = buffdal.New(db).GetActiveBuffs(context.Background(), charID)
	assert.Error(t, err)
}

func TestRoundNumber_ReadsStoredValue(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO round_state (character_id, round_number) VALUES (?, 9)`, charID)
	require.NoError(t, err)

	r, err := buffdal.New(db).GetRoundNumber(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, 9, r)
}

func TestActiveBuffs_NoteRoundTripsAndOldRowsReadEmpty(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `INSERT INTO active_buffs (character_id, buffs) VALUES (?, '[{"id":"o","name":"Old","charges":0,"duration":3,"effects":{}}]')`, charID)
	require.NoError(t, err)
	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Note)

	got[0].Note = "+4 saves vs fire"
	require.NoError(t, dal.SaveActiveBuffs(ctx, charID, got))
	got, err = dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, "+4 saves vs fire", got[0].Note)
}

func TestSavedBuffs_ReplaceKeepsGivenIDsAndAssignsNew(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := buffdal.New(db)
	ctx := context.Background()

	list, err := dal.ListSavedBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Empty(t, list)

	form := buffsvc.SavedBuff{
		Name: "Form", Duration: 4, Charges: 1, Note: "n",
		Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), MinStr: intPtr(18), SpellsBuff: floatPtr(2)},
	}
	require.NoError(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{form}))
	list, err = dal.ListSavedBuffs(ctx, charID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	form.ID = list[0].ID
	assert.NotZero(t, form.ID)
	assert.Equal(t, form, list[0])

	// A restore writes explicit ids back; a new entry gets the next one.
	require.NoError(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{{ID: 42, Name: "Old"}, {Name: "New"}}))
	list, err = dal.ListSavedBuffs(ctx, charID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, int64(42), list[0].ID)
	assert.Equal(t, "New", list[1].Name)
}
