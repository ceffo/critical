// The adapter runs on mocked Queriers (its own and the localized texts'): the
// tests assert the operations called, their arguments and how rows map to
// items; no database, no SQL text (ADR 0025 step 6.2). What only the SQL
// decides (sheet order, the count never below 0, which behaviour a daily
// reset refills) is checked by sqlc at generate time and by running the app
// (step 7).
package itemdal_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	"github.com/ceffo/critical/api/internal/adapters/itemdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/itemdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/adapters/textdal"
	textq "github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	textmocks "github.com/ceffo/critical/api/internal/adapters/textdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// owner is the character every call is made for: each query carries it (items belong to a
// character).
const owner = int64(4)

func mocked(t *testing.T) (*itemdal.DAL, *mocks.MockQuerier, *textmocks.MockQuerier) {
	t.Helper()
	q, tq := mocks.NewMockQuerier(t), textmocks.NewMockQuerier(t)
	return itemdal.New(q, textdal.New(tq)), q, tq
}

func powers(t *testing.T) ([]itemsvc.Power, string) {
	t.Helper()
	p := []itemsvc.Power{{ID: "glow", MinStr: new(18)}}
	raw, err := itemsvc.EncodePowers(p)
	require.NoError(t, err)
	return p, raw
}

var wandRow = queries.Item{
	ID: "wand_lightning", Position: 3, Name: 13, Type: "wand", Behaviour: "charges",
	Count: 4, MaxCount: 4, EffectDice: "6d6", EffectSave: "rod_staff_wand", Notes: "hum",
}

var wandItem = itemsvc.Item{
	ID: "wand_lightning", Position: 3, Name: localized.Text{"en": "Wand"}, Type: "wand", Behaviour: itemsvc.BehaviourCharges,
	Count: 4, MaxCount: 4, EffectDice: "6d6", EffectSave: "rod_staff_wand", Notes: "hum",
}

func wandName() []textq.GetTextRow { return []textq.GetTextRow{{Lang: "en", Body: "Wand"}} }

// The three behaviours with their type labels, the magic bonus, ammo kind
// and powers map from the rows, in the query's order, each with the name
// under its key.
func TestList_MapsBehavioursFieldsAndNames(t *testing.T) {
	ctx := context.Background()
	dal, q, _ := mocked(t)
	p, raw := powers(t)

	q.EXPECT().ListItems(ctx, owner).Return([]queries.Item{
		{ID: "arrows_plus1", Position: 0, Name: 10, Type: "arrow", Behaviour: "stock", MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18},
		{ID: "power_gem_dispel", Position: 1, Name: 11, Type: "gem", Behaviour: "daily", Count: 1, MaxCount: 1, Powers: raw},
		wandRow,
	}, nil).Once()
	q.EXPECT().ItemNames(ctx, owner).Return([]queries.LocalizedText{
		{TextID: 13, Lang: "en", Body: "Wand"},
		{TextID: 10, Lang: "en", Body: "Arrows +1"},
		{TextID: 10, Lang: "fr", Body: "Flèches +1"},
		{TextID: 11, Lang: "en", Body: "Power Gem"},
	}, nil).Once()
	got, err := dal.List(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Item{
		{
			ID: "arrows_plus1", Position: 0, Name: localized.Text{"en": "Arrows +1", "fr": "Flèches +1"}, Type: "arrow",
			Behaviour: itemsvc.BehaviourStock, MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18,
		},
		{
			ID: "power_gem_dispel", Position: 1, Name: localized.Text{"en": "Power Gem"}, Type: "gem",
			Behaviour: itemsvc.BehaviourDaily, Count: 1, MaxCount: 1, Powers: p,
		},
		wandItem,
	}, got)
}

// A stored power list that does not decode is an error, not an item without
// powers.
func TestList_UndecodablePowersAreAnError(t *testing.T) {
	ctx := context.Background()
	dal, q, _ := mocked(t)

	q.EXPECT().ListItems(ctx, owner).Return([]queries.Item{{ID: "gem", Powers: `[{"oops":1}]`}}, nil).Once()
	q.EXPECT().ItemNames(ctx, owner).Return([]queries.LocalizedText{}, nil).Once()
	_, err := dal.List(ctx, owner)
	require.Error(t, err)
}

// A spent count returns the updated row with its name; no row updated is
// ErrItemEmpty when the item exists, ErrItemNotFound when it does not.
func TestDecrement_UpdatedItemElseEmptyOrNotFound(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)

	spent := wandRow
	spent.Count = 3
	q.EXPECT().DecrementItem(ctx, queries.DecrementItemParams{CharacterID: owner, ID: "wand_lightning"}).Return(spent, nil).Once()
	tq.EXPECT().GetText(ctx, int64(13)).Return(wandName(), nil).Once()
	got, err := dal.Decrement(ctx, owner, "wand_lightning")
	require.NoError(t, err)
	want := wandItem
	want.Count = 3
	assert.Equal(t, want, got)

	mock.InOrder(
		q.EXPECT().DecrementItem(ctx, queries.DecrementItemParams{CharacterID: owner, ID: "wand_lightning"}).Return(queries.Item{}, sql.ErrNoRows).Once(),
		q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "wand_lightning"}).Return(wandRow, nil).Once(),
	)
	_, err = dal.Decrement(ctx, owner, "wand_lightning")
	require.ErrorIs(t, err, itemsvc.ErrItemEmpty)

	mock.InOrder(
		q.EXPECT().DecrementItem(ctx, queries.DecrementItemParams{CharacterID: owner, ID: "nope"}).Return(queries.Item{}, sql.ErrNoRows).Once(),
		q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "nope"}).Return(queries.Item{}, sql.ErrNoRows).Once(),
	)
	_, err = dal.Decrement(ctx, owner, "nope")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestIncrement_AddsNAndReturnsTheUpdatedItem(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)

	q.EXPECT().IncrementItem(ctx, queries.IncrementItemParams{N: 3, CharacterID: owner, ID: "wand_lightning"}).Return(wandRow, nil).Once()
	tq.EXPECT().GetText(ctx, int64(13)).Return(wandName(), nil).Once()
	got, err := dal.Increment(ctx, owner, "wand_lightning", 3)
	require.NoError(t, err)
	assert.Equal(t, wandItem, got)

	q.EXPECT().IncrementItem(ctx, queries.IncrementItemParams{N: 1, CharacterID: owner, ID: "nope"}).Return(queries.Item{}, sql.ErrNoRows).Once()
	_, err = dal.Increment(ctx, owner, "nope", 1)
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestResetDaily_RunsTheDailyRefill(t *testing.T) {
	ctx := context.Background()
	dal, q, _ := mocked(t)

	q.EXPECT().ResetDailyItems(ctx, owner).Return(nil).Once()
	require.NoError(t, dal.ResetDaily(ctx, owner))
}

// SetCount writes the count given; no row updated is ErrItemNotFound.
func TestSetCount_WritesTheCountElseNotFound(t *testing.T) {
	ctx := context.Background()
	dal, q, _ := mocked(t)

	q.EXPECT().SetItemCount(ctx, queries.SetItemCountParams{Count: 3, CharacterID: owner, ID: "arrows"}).Return(1, nil).Once()
	require.NoError(t, dal.SetCount(ctx, owner, "arrows", 3))
	q.EXPECT().SetItemCount(ctx, queries.SetItemCountParams{Count: 1, CharacterID: owner, ID: "nope"}).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetCount(ctx, owner, "nope", 1), itemsvc.ErrItemNotFound)
}

func TestGet_MapsTheRowAndItsNameElseNotFound(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)

	q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "wand_lightning"}).Return(wandRow, nil).Once()
	tq.EXPECT().GetText(ctx, int64(13)).Return(wandName(), nil).Once()
	got, err := dal.Get(ctx, owner, "wand_lightning")
	require.NoError(t, err)
	assert.Equal(t, wandItem, got)

	q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "nope"}).Return(queries.Item{}, sql.ErrNoRows).Once()
	_, err = dal.Get(ctx, owner, "nope")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

// ADR 0023 step 9: an item's name is its localized_texts rows, one per
// language. Update writes every field (powers encoded) and replaces the rows
// under the key the row holds, Insert stores them under a new key and the row
// with that key, Delete removes them with the item; an unknown id touches no
// name.
func TestName_Step9_WrittenAsLocalizedTextRows(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)
	p, raw := powers(t)
	it := itemsvc.Item{
		ID: "w", Position: 9, Name: localized.Text{"fr": "Baguette", "de": "Stab"}, Type: "staff",
		Behaviour: itemsvc.BehaviourDaily, MagicBonus: 1, AmmoKind: "bolt", Count: 2, MaxCount: 3,
		EffectDice: "1d4+1", EffectSave: "spell", Notes: "hum", Powers: p,
	}

	mock.InOrder(
		q.EXPECT().UpdateItem(ctx, queries.UpdateItemParams{
			Position: 9, Type: "staff", Behaviour: "daily", MagicBonus: 1, AmmoKind: "bolt", Count: 2, MaxCount: 3,
			EffectDice: "1d4+1", EffectSave: "spell", Notes: "hum", Powers: raw, CharacterID: owner, ID: "w",
			DamageTypes: []string{},
		}).Return(100, nil).Once(),
		tq.EXPECT().DeleteText(ctx, int64(100)).Return(nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 100, Lang: "de", Body: "Stab"}).Return(nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 100, Lang: "fr", Body: "Baguette"}).Return(nil).Once(),
	)
	require.NoError(t, dal.Update(ctx, owner, it))

	it.ID, it.Name = "x", localized.Text{"en": "Rope"}
	mock.InOrder(
		tq.EXPECT().NextTextKey(ctx).Return(101, nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 101, Lang: "en", Body: "Rope"}).Return(nil).Once(),
		q.EXPECT().InsertItem(ctx, queries.InsertItemParams{
			CharacterID: owner, ID: "x", Position: 9, Name: 101, Type: "staff", Behaviour: "daily", MagicBonus: 1, AmmoKind: "bolt",
			Count: 2, MaxCount: 3, EffectDice: "1d4+1", EffectSave: "spell", Notes: "hum", Powers: raw, DamageTypes: []string{},
		}).Return(nil).Once(),
	)
	require.NoError(t, dal.Insert(ctx, owner, it))

	mock.InOrder(
		q.EXPECT().DeleteItem(ctx, queries.DeleteItemParams{CharacterID: owner, ID: "w"}).Return(100, nil).Once(),
		tq.EXPECT().DeleteText(ctx, int64(100)).Return(nil).Once(),
	)
	require.NoError(t, dal.Delete(ctx, owner, "w"))

	q.EXPECT().DeleteItem(ctx, queries.DeleteItemParams{CharacterID: owner, ID: "w"}).Return(0, sql.ErrNoRows).Once()
	require.ErrorIs(t, dal.Delete(ctx, owner, "w"), itemsvc.ErrItemNotFound)
	q.EXPECT().UpdateItem(ctx, mock.Anything).Return(0, sql.ErrNoRows).Once()
	require.ErrorIs(t, dal.Update(ctx, owner, it), itemsvc.ErrItemNotFound)
}

// Storage errors are returned, never taken for not found or empty, and stop
// the write: no item row without its name, no name written after a failed
// update.
func TestStorageErrors_AreReturnedAndStopTheWrite(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)

	q.EXPECT().ListItems(ctx, owner).Return(nil, assert.AnError).Once()
	_, err := dal.List(ctx, owner)
	require.ErrorIs(t, err, assert.AnError)

	q.EXPECT().DecrementItem(ctx, queries.DecrementItemParams{CharacterID: owner, ID: "w"}).Return(queries.Item{}, assert.AnError).Once()
	_, err = dal.Decrement(ctx, owner, "w")
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, itemsvc.ErrItemEmpty)

	q.EXPECT().DecrementItem(ctx, queries.DecrementItemParams{CharacterID: owner, ID: "v"}).Return(queries.Item{}, sql.ErrNoRows).Once()
	q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "v"}).Return(queries.Item{}, assert.AnError).Once()
	_, err = dal.Decrement(ctx, owner, "v")
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, itemsvc.ErrItemEmpty)

	q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "w"}).Return(wandRow, nil).Once()
	tq.EXPECT().GetText(ctx, int64(13)).Return(nil, assert.AnError).Once()
	_, err = dal.Get(ctx, owner, "w")
	require.ErrorIs(t, err, assert.AnError)

	tq.EXPECT().NextTextKey(ctx).Return(0, assert.AnError).Once()
	require.ErrorIs(t, dal.Insert(ctx, owner, wandItem), assert.AnError)

	q.EXPECT().UpdateItem(ctx, mock.Anything).Return(0, assert.AnError).Once()
	err = dal.Update(ctx, owner, wandItem)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, itemsvc.ErrItemNotFound)

	q.EXPECT().SetItemCount(ctx, queries.SetItemCountParams{Count: 1, CharacterID: owner, ID: "w"}).Return(0, assert.AnError).Once()
	require.ErrorIs(t, dal.SetCount(ctx, owner, "w", 1), assert.AnError)

	q.EXPECT().ResetDailyItems(ctx, owner).Return(assert.AnError).Once()
	require.ErrorIs(t, dal.ResetDaily(ctx, owner), assert.AnError)
}

// ADR 0033 steps 1 to 6: every field of a piece round-trips through its
// row (category, state, the weapon's, the armour's and the shield's
// columns; the flags as 0/1).
func TestPiece_EveryColumnMapped(t *testing.T) {
	ctx := context.Background()
	dal, q, tq := mocked(t)
	piece := itemsvc.Item{
		ID: "bow", Position: 2, Name: localized.Text{"en": "Bow"}, Category: itemsvc.CategoryWeapon,
		CatalogueCode: "short_bow", State: itemsvc.StateWielded, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		DamageTypes: []string{"p"}, DamageSM: "1d6", DamageL: "1d6", WeaponType: "short_bow", WeaponKind: "missile",
		Size: "m", Speed: 7, MagicToHit: 2, MagicDamage: 1, TwoHanded: true, Fires: "arrow",
		BaseAC: 4, SkillColumn: "metal", ArmorRow: "chain", ShieldKind: "body", ACMelee: 1, ACMissile: 2,
		MagicKind: "misc",
	}
	row := queries.Item{
		ID: "bow", Position: 2, Name: 7, Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", CharacterID: owner,
		Category: "weapon", MagicKind: "misc", CatalogueCode: "short_bow", State: "wielded", WeaponType: "short_bow",
		WeaponKind: "missile", Size: "m", DamageTypes: []string{"p"}, Speed: 7, DamageSm: "1d6", DamageL: "1d6",
		MagicToHit: 2, MagicDamage: 1, TwoHanded: 1, Fires: "arrow", Specialized: 1, // the old flag, ignored (ADR 0037 step 9)
		BaseAc: 4, SkillColumn: "metal",
		ArmorRow: "chain", ShieldKind: "body", AcMelee: 1, AcMissile: 2,
	}
	q.EXPECT().GetItem(ctx, queries.GetItemParams{CharacterID: owner, ID: "bow"}).Return(row, nil).Once()
	tq.EXPECT().GetText(ctx, int64(7)).Return([]textq.GetTextRow{{Lang: "en", Body: "Bow"}}, nil).Once()
	got, err := dal.Get(ctx, owner, "bow")
	require.NoError(t, err)
	assert.Equal(t, piece, got)

	q.EXPECT().UpdateItem(ctx, queries.UpdateItemParams{
		Position: 2, Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", CharacterID: owner, ID: "bow",
		Category: "weapon", MagicKind: "misc", CatalogueCode: "short_bow", State: "wielded", WeaponType: "short_bow",
		WeaponKind: "missile", Size: "m", DamageTypes: []string{"p"}, Speed: 7, DamageSm: "1d6", DamageL: "1d6",
		MagicToHit: 2, MagicDamage: 1, TwoHanded: 1, Fires: "arrow", BaseAc: 4, SkillColumn: "metal",
		ArmorRow: "chain", ShieldKind: "body", AcMelee: 1, AcMissile: 2,
	}).Return(7, nil).Once()
	tq.EXPECT().DeleteText(ctx, int64(7)).Return(nil).Once()
	tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 7, Lang: "en", Body: "Bow"}).Return(nil).Once()
	require.NoError(t, dal.Update(ctx, owner, piece))
}

// ADR 0033 step 27: the review rows map to reviews; a piece's rows are
// deleted by its id.
func TestReviews(t *testing.T) {
	ctx := context.Background()
	dal, q, _ := mocked(t)
	q.EXPECT().ListReviews(ctx, owner).Return([]queries.ListReviewsRow{{ItemID: "mace", Reason: "weapon_unmatched"}}, nil).Once()
	got, err := dal.Reviews(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Review{{ItemID: "mace", Reason: "weapon_unmatched"}}, got)

	q.EXPECT().DeleteReviews(ctx, queries.DeleteReviewsParams{CharacterID: owner, ItemID: "mace"}).Return(nil).Once()
	require.NoError(t, dal.DeleteReviews(ctx, owner, "mace"))
}
