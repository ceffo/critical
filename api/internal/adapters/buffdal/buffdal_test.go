// The adapter runs on a mocked Querier: the tests assert the operations
// called, their arguments (the stored JSON blobs) and how rows map to buffs;
// no database, no SQL text (ADR 0025 step 6.2). What only the SQL decides
// (the upsert's conflict target, the saved buffs' id order, the rollback of a
// transaction) is checked by sqlc at generate time and by running the app
// (step 7).
package buffdal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/adapters/buffdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/buffdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

const charID = int64(3)

var errBoom = errors.New("boom")

func mocked(t *testing.T) (*buffdal.DAL, *mocks.MockQuerier) {
	t.Helper()
	q := mocks.NewMockQuerier(t)
	return buffdal.New(q), q
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

// stored returns the active buffs blob SaveActiveBuffs writes for buffs.
func stored(t *testing.T, buffs []buffsvc.BuffEffect) string {
	t.Helper()
	dal, q := mocked(t)
	var blob string
	q.EXPECT().UpsertActiveBuffs(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p queries.UpsertActiveBuffsParams) error {
		assert.Equal(t, charID, p.CharacterID)
		blob = p.Buffs
		return nil
	}).Once()
	require.NoError(t, dal.SaveActiveBuffs(context.Background(), charID, buffs))
	return blob
}

// read returns what GetActiveBuffs makes of a stored blob.
func read(t *testing.T, blob string) []buffsvc.BuffEffect {
	t.Helper()
	dal, q := mocked(t)
	q.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(blob, nil).Once()
	got, err := dal.GetActiveBuffs(context.Background(), charID)
	require.NoError(t, err)
	return got
}

// No row, or an empty blob, is an empty list (never nil); a read error and a
// corrupt blob are errors.
func TestActiveBuffs_MissingRowIsEmpty(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().GetActiveBuffs(ctx, charID).Return("", sql.ErrNoRows).Once()
	got, err := dal.GetActiveBuffs(ctx, charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	assert.Equal(t, []buffsvc.BuffEffect{}, read(t, ""))

	q.EXPECT().GetActiveBuffs(ctx, charID).Return("", errBoom).Once()
	_, err = dal.GetActiveBuffs(ctx, charID)
	require.ErrorIs(t, err, errBoom)
}

func TestActiveBuffs_CorruptJSON(t *testing.T) {
	dal, q := mocked(t)
	q.EXPECT().GetActiveBuffs(mock.Anything, charID).Return("not json", nil).Once()
	_, err := dal.GetActiveBuffs(context.Background(), charID)
	assert.Error(t, err)
}

// Every field and effect written is read back; the list replaces the stored
// one whole (one upsert of the character's row), an empty list included.
func TestActiveBuffs_RoundTrip(t *testing.T) {
	buffs := []buffsvc.BuffEffect{
		{ID: "a", Name: "Power Gem", Duration: 5, Effects: buffsvc.BuffEffects{
			WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5), MinStr: intPtr(18), SaveBonus: intPtr(1),
		}},
		{ID: "b", Name: "Mirror Image", Duration: 27, Charges: 3, ChargesTotal: 4, StartRound: 2, Note: "n"},
		{ID: "c", Name: "Strength", Duration: 540, Effects: buffsvc.BuffEffects{StrBonus: intPtr(4), StrMax: intPtr(18), AC: intPtr(-2)}},
		{ID: "d", Name: "Bless", Duration: 6, SpellID: "bless", Effects: buffsvc.BuffEffects{
			ToHit: intPtr(1), Damage: intPtr(-1), WeaponKind: buffsvc.WeaponKind("melee"),
		}},
	}
	assert.Equal(t, buffs, read(t, stored(t, buffs)))
	assert.JSONEq(t, `[]`, stored(t, nil), "saving no buff stores an empty list")

	dal, q := mocked(t)
	q.EXPECT().UpsertActiveBuffs(mock.Anything, mock.Anything).Return(errBoom).Once()
	require.ErrorIs(t, dal.SaveActiveBuffs(context.Background(), charID, buffs), errBoom)
}

// The on-disk format: rows written by older versions read as before.
func TestActiveBuffs_ReadsStoredJSON(t *testing.T) {
	got := read(t, `[{"id":"a","name":"Power Gem","type":"duration","charges":0,"duration":5,"effects":{"weapon_buff":5,"multiplier":2,"min_str":18}}]`)
	assert.Equal(t, []buffsvc.BuffEffect{{
		ID: "a", Name: "Power Gem", Duration: 5,
		Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0), MinStr: intPtr(18)},
	}}, got, "the old type key is ignored")
}

// str_set ("set Strength to X") was removed: a row written before still
// reads, and the effect is ignored.
func TestActiveBuffs_IgnoresRemovedStrSet(t *testing.T) {
	got := read(t, `[{"id":"o","name":"Ogre gauntlets","charges":0,"duration":0,"effects":{"str_set":18,"ac":-1}}]`)
	assert.Equal(t, []buffsvc.BuffEffect{{ID: "o", Name: "Ogre gauntlets", Effects: buffsvc.BuffEffects{AC: intPtr(-1)}}}, got)
}

const oldRow = `[{"id":"o","name":"Old","charges":2,"duration":3,"effects":{}}]`

func TestActiveBuffs_NoteRoundTripsAndOldRowsReadEmpty(t *testing.T) {
	got := read(t, oldRow)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Note)
	got[0].Note = "+4 saves vs fire"
	assert.Equal(t, "+4 saves vs fire", read(t, stored(t, got))[0].Note)
}

func TestActiveBuffs_StartRoundRoundTripsAndOldRowsReadZero(t *testing.T) {
	got := read(t, oldRow)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].StartRound, "a buff stored before the field reads as round 0")
	got[0].StartRound = 4
	assert.Equal(t, 4, read(t, stored(t, got))[0].StartRound)
}

func TestActiveBuffs_ChargesTotalRoundTripsAndOldRowsReadZero(t *testing.T) {
	got := read(t, oldRow)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].Charges)
	assert.Zero(t, got[0].ChargesTotal, "a buff stored before the field has an unknown total")
	got[0].ChargesTotal = 9
	assert.Equal(t, 9, read(t, stored(t, got))[0].ChargesTotal)
}

// ADR 0004 step 17: the spell id round-trips; a row stored before the field
// (and a buff not from a spell) reads "" and is written without the key.
func TestActiveBuffs_SpellIDRoundTripsAndOldRowsReadEmpty(t *testing.T) {
	got := read(t, oldRow)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].SpellID, "a buff stored before the field has no spell")

	got = append(got, buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 5, SpellID: "haste"})
	blob := stored(t, got)
	var raw []map[string]any
	require.NoError(t, json.Unmarshal([]byte(blob), &raw))
	require.Len(t, raw, 2)
	assert.NotContains(t, raw[0], "spell_id", "omitted for a buff not from a spell")
	assert.Equal(t, "haste", raw[1]["spell_id"])
	back := read(t, blob)
	assert.Empty(t, back[0].SpellID)
	assert.Equal(t, "haste", back[1].SpellID)
}

// The round counter: no row is round 0; a save upserts the character's
// counter.
func TestRoundNumber(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)

	q.EXPECT().GetRoundNumber(ctx, charID).Return(0, sql.ErrNoRows).Once()
	r, err := dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 0, r, "missing row means round 0")

	q.EXPECT().GetRoundNumber(ctx, charID).Return(9, nil).Once()
	r, err = dal.GetRoundNumber(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 9, r)

	q.EXPECT().GetRoundNumber(ctx, charID).Return(0, errBoom).Once()
	_, err = dal.GetRoundNumber(ctx, charID)
	require.ErrorIs(t, err, errBoom)

	q.EXPECT().UpsertRoundNumber(ctx, queries.UpsertRoundNumberParams{CharacterID: charID, RoundNumber: 4}).Return(nil).Once()
	require.NoError(t, dal.SaveRoundNumber(ctx, charID, 4))
	q.EXPECT().UpsertRoundNumber(ctx, mock.Anything).Return(errBoom).Once()
	require.ErrorIs(t, dal.SaveRoundNumber(ctx, charID, 5), errBoom)
}

// The round's initiative: no row, or a row without a roll, is none; nil
// writes NULL (none), through its own upsert that leaves the round alone.
func TestInitiative(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)

	q.EXPECT().GetInitiative(ctx, charID).Return(queries.GetInitiativeRow{}, sql.ErrNoRows).Once()
	got, err := dal.GetInitiative(ctx, charID)
	require.NoError(t, err)
	assert.Nil(t, got, "missing row means none")

	q.EXPECT().GetInitiative(ctx, charID).Return(queries.GetInitiativeRow{}, nil).Once()
	got, err = dal.GetInitiative(ctx, charID)
	require.NoError(t, err)
	assert.Nil(t, got, "a round without a roll has none")

	q.EXPECT().GetInitiative(ctx, charID).Return(queries.GetInitiativeRow{
		InitiativeD10: sql.NullInt64{Int64: 7, Valid: true}, InitiativeEntryID: sql.NullInt64{Int64: 21, Valid: true},
	}, nil).Once()
	got, err = dal.GetInitiative(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, &buffsvc.Initiative{D10: 7, LogEntryID: 21}, got)

	q.EXPECT().GetInitiative(ctx, charID).Return(queries.GetInitiativeRow{}, errBoom).Once()
	_, err = dal.GetInitiative(ctx, charID)
	require.ErrorIs(t, err, errBoom)

	q.EXPECT().UpsertInitiative(ctx, queries.UpsertInitiativeParams{
		CharacterID: charID, InitiativeD10: sql.NullInt64{Int64: 7, Valid: true}, InitiativeEntryID: sql.NullInt64{Int64: 21, Valid: true},
	}).Return(nil).Once()
	require.NoError(t, dal.SaveInitiative(ctx, charID, &buffsvc.Initiative{D10: 7, LogEntryID: 21}))
	q.EXPECT().UpsertInitiative(ctx, queries.UpsertInitiativeParams{CharacterID: charID}).Return(nil).Once()
	require.NoError(t, dal.SaveInitiative(ctx, charID, nil), "nil clears it")
	q.EXPECT().UpsertInitiative(ctx, mock.Anything).Return(errBoom).Once()
	require.ErrorIs(t, dal.SaveInitiative(ctx, charID, nil), errBoom)
}

// The saved buffs (templates) read with their effects and creation time, in
// the query's order; unreadable effects are an error.
func TestSavedBuffs_List(t *testing.T) {
	ctx := context.Background()
	created := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	dal, q := mocked(t)
	q.EXPECT().ListSavedBuffs(ctx, charID).Return([]queries.ListSavedBuffsRow{
		{ID: 4, Name: "Form", Duration: 4, Charges: 1, Effects: `{"weapon_buff":5,"min_str":18,"spells_buff":2}`, Note: "n", CreatedAt: created},
		{ID: 9, Name: "Plain", Effects: `{}`, CreatedAt: created},
	}, nil).Once()
	got, err := dal.ListSavedBuffs(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.SavedBuff{
		{
			ID: 4, Name: "Form", Duration: 4, Charges: 1, Note: "n", CreatedAt: created,
			Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), MinStr: intPtr(18), SpellsBuff: floatPtr(2)},
		},
		{ID: 9, Name: "Plain", CreatedAt: created},
	}, got)

	q.EXPECT().ListSavedBuffs(ctx, charID).Return([]queries.ListSavedBuffsRow{}, nil).Once()
	got, err = dal.ListSavedBuffs(ctx, charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	q.EXPECT().ListSavedBuffs(ctx, charID).Return([]queries.ListSavedBuffsRow{{ID: 1, Effects: "{bad"}}, nil).Once()
	_, err = dal.ListSavedBuffs(ctx, charID)
	require.Error(t, err)

	q.EXPECT().ListSavedBuffs(ctx, charID).Return(nil, errBoom).Once()
	_, err = dal.ListSavedBuffs(ctx, charID)
	require.ErrorIs(t, err, errBoom)
}

// Replace deletes the character's templates, then inserts the list in order:
// a template with an id (a restore) keeps its id and creation time, in the
// column default's layout; a new one is inserted without, for the database to
// number and stamp it.
func TestSavedBuffs_ReplaceKeepsGivenIDsAndAssignsNew(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	longAgo := time.Date(2001, 2, 3, 4, 5, 6, 0, time.FixedZone("x", 3600))
	mock.InOrder(
		q.EXPECT().DeleteSavedBuffs(ctx, charID).Return(nil).Once(),
		q.EXPECT().InsertSavedBuffWithID(ctx, queries.InsertSavedBuffWithIDParams{
			ID: 42, CharacterID: charID, Name: "Old", Duration: 3, Charges: 1, Effects: `{"ac":-1}`, Note: "n",
			CreatedAt: "2001-02-03 03:05:06",
		}).Return(nil).Once(),
		q.EXPECT().InsertSavedBuff(ctx, queries.InsertSavedBuffParams{
			CharacterID: charID, Name: "New", Effects: `{}`,
		}).Return(nil).Once(),
	)
	require.NoError(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{
		{ID: 42, Name: "Old", Duration: 3, Charges: 1, Note: "n", CreatedAt: longAgo, Effects: buffsvc.BuffEffects{AC: intPtr(-1)}},
		{Name: "New"},
	}))

	q.EXPECT().DeleteSavedBuffs(ctx, charID).Return(nil).Once()
	require.NoError(t, dal.ReplaceSavedBuffs(ctx, charID, nil), "an empty list deletes them all")
}

// A template restored from a snapshot taken before created_at was recorded
// has no creation time: it is stamped now, never stored as the zero time.
func TestSavedBuffs_ReplaceStampsARowRestoredWithoutCreatedAt(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	var stamp string
	q.EXPECT().DeleteSavedBuffs(ctx, charID).Return(nil).Once()
	q.EXPECT().InsertSavedBuffWithID(ctx, mock.Anything).RunAndReturn(func(_ context.Context, p queries.InsertSavedBuffWithIDParams) error {
		stamp = p.CreatedAt
		return nil
	}).Once()
	before := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{{ID: 5, Name: "Legacy"}}))
	after := time.Now().UTC()

	got, err := time.Parse("2006-01-02 15:04:05", stamp)
	require.NoError(t, err, "stored in the column default's layout")
	assert.False(t, got.Before(before) || got.After(after), "stamped now, got %s", stamp)
}

// A failing write stops the replace; the caller's transaction rolls it back.
func TestSavedBuffs_ReplaceStopsAtTheFailingWrite(t *testing.T) {
	ctx := context.Background()
	t.Run("delete", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().DeleteSavedBuffs(ctx, charID).Return(errBoom).Once()
		require.ErrorIs(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{{Name: "New"}}), errBoom)
	})
	t.Run("insert", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().DeleteSavedBuffs(ctx, charID).Return(nil).Once()
		q.EXPECT().InsertSavedBuff(ctx, mock.Anything).Return(errBoom).Once()
		require.ErrorIs(t, dal.ReplaceSavedBuffs(ctx, charID, []buffsvc.SavedBuff{{Name: "New"}, {Name: "Never"}}), errBoom)
	})
}
