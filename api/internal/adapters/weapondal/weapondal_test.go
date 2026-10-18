// The adapter runs on mocked Queriers (its own and the localized texts'): the
// tests assert the operations called, their arguments and how rows map to
// weapons; no database, no SQL text (ADR 0025 step 6.2).
package weapondal_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/textdal"
	textq "github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	textmocks "github.com/ceffo/critical/api/internal/adapters/textdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	"github.com/ceffo/critical/api/internal/adapters/weapondal/queries"
	"github.com/ceffo/critical/api/internal/adapters/weapondal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
)

// owner is the character every call is made for: each query carries it (weapons belong to a
// character).
const owner = int64(4)

func mocked(t *testing.T) (*mocks.MockQuerier, *textmocks.MockQuerier, textdal.Store) {
	t.Helper()
	q, tq := mocks.NewMockQuerier(t), textmocks.NewMockQuerier(t)
	return q, tq, textdal.New(tq)
}

var bowRow = queries.Weapon{
	ID: "shortbow", Position: 2, Name: 12, DamageSm: "1d6", DamageL: "1d6",
	MagicToHit: 2, MagicDamage: 2, Kind: "missile", TwoHanded: 1, AmmoKind: "arrow", WeaponType: "short_bow",
}

var bowName = localized.Text{"en": "Short Bow +2", "fr": "Arc court +2"}

func bowNameRows() []textq.GetTextRow {
	return []textq.GetTextRow{{Lang: "en", Body: "Short Bow +2"}, {Lang: "fr", Body: "Arc court +2"}}
}

// A row maps to combat's weapon, its name read under the row's key.
func TestGet_MapsTheRowAndItsName(t *testing.T) {
	ctx := context.Background()
	q, tq, texts := mocked(t)

	q.EXPECT().GetWeapon(ctx, queries.GetWeaponParams{CharacterID: owner, ID: "shortbow"}).Return(bowRow, nil).Once()
	tq.EXPECT().GetText(ctx, int64(12)).Return(bowNameRows(), nil).Once()
	w, err := weapondal.New(q, texts).Get(ctx, owner, "shortbow")
	require.NoError(t, err)
	assert.Equal(t, combatsvc.Weapon{
		ID: "shortbow", Name: bowName, BaseDamage: "1d6", DamageLarge: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
	}, w)
}

// The rows map in the query's order (sheet order), each with the name under
// its key; a one-handed weapon has two_handed 0.
func TestList_MapsRowsInOrderWithTheirNames(t *testing.T) {
	ctx := context.Background()
	q, _, texts := mocked(t)
	sword := queries.Weapon{ID: "longsword", Position: 1, Name: 11, DamageSm: "1d8", DamageL: "1d12", Kind: "melee"}

	q.EXPECT().ListWeapons(ctx, owner).Return([]queries.Weapon{sword, bowRow}, nil).Times(2)
	q.EXPECT().WeaponNames(ctx, owner).Return([]queries.LocalizedText{
		{TextID: 12, Lang: "en", Body: "Short Bow +2"},
		{TextID: 11, Lang: "en", Body: "Long Sword"},
		{TextID: 12, Lang: "fr", Body: "Arc court +2"},
	}, nil).Times(2)

	got, err := weapondal.New(q, texts).List(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Weapon{
		{ID: "longsword", Name: localized.Text{"en": "Long Sword"}, BaseDamage: "1d8", DamageLarge: "1d12", Kind: combatsvc.WeaponMelee},
		{
			ID: "shortbow", Name: bowName, BaseDamage: "1d6", DamageLarge: "1d6",
			MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
		},
	}, got)

	edited, err := weapondal.NewEditor(q, texts).List(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, []weaponsvc.Weapon{
		{ID: "longsword", Position: 1, Name: localized.Text{"en": "Long Sword"}, DamageSM: "1d8", DamageL: "1d12", Kind: "melee"},
		{
			ID: "shortbow", Position: 2, Name: bowName, DamageSM: "1d6", DamageL: "1d6",
			MagicToHit: 2, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
		},
	}, edited)
}

// No row is the not-found error of the caller's port, and no name is read.
func TestGet_UnknownIsNotFound(t *testing.T) {
	ctx := context.Background()
	q, _, texts := mocked(t)

	q.EXPECT().GetWeapon(ctx, queries.GetWeaponParams{CharacterID: owner, ID: "nope"}).Return(queries.Weapon{}, sql.ErrNoRows).Times(2)
	_, err := weapondal.New(q, texts).Get(ctx, owner, "nope")
	require.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)
	_, err = weapondal.NewEditor(q, texts).Get(ctx, owner, "nope")
	require.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
}

func TestEditor_GetMapsPosition(t *testing.T) {
	ctx := context.Background()
	q, tq, texts := mocked(t)

	q.EXPECT().GetWeapon(ctx, queries.GetWeaponParams{CharacterID: owner, ID: "shortbow"}).Return(bowRow, nil).Once()
	tq.EXPECT().GetText(ctx, int64(12)).Return(bowNameRows(), nil).Once()
	got, err := weapondal.NewEditor(q, texts).Get(ctx, owner, "shortbow")
	require.NoError(t, err)
	assert.Equal(t, weaponsvc.Weapon{
		ID: "shortbow", Position: 2, Name: bowName, DamageSM: "1d6", DamageL: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
	}, got)
}

var longBow = weaponsvc.Weapon{
	ID: "long_bow", Position: 9, Name: localized.Text{"en": "Long Bow", "fr": "Arc long"}, DamageSM: "1d8", DamageL: "1d8",
	MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow", Type: "long_bow",
}

// ADR 0023 step 9: Insert writes the name under a new key, then the weapon
// row holding that key.
func TestEditor_InsertWritesTheNameUnderANewKeyThenTheRow(t *testing.T) {
	ctx := context.Background()
	q, tq, texts := mocked(t)

	mock.InOrder(
		tq.EXPECT().NextTextKey(ctx).Return(30, nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 30, Lang: "en", Body: "Long Bow"}).Return(nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 30, Lang: "fr", Body: "Arc long"}).Return(nil).Once(),
		q.EXPECT().InsertWeapon(ctx, queries.InsertWeaponParams{
			CharacterID: owner, ID: "long_bow", Position: 9, Name: 30, DamageSm: "1d8", DamageL: "1d8",
			MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: 1, AmmoKind: "arrow", WeaponType: "long_bow",
		}).Return(nil).Once(),
	)
	require.NoError(t, weapondal.NewEditor(q, texts).Insert(ctx, owner, longBow))
}

// ADR 0023 step 9: Update writes every field, then replaces the name's rows
// under the key the row holds; Delete removes the row, then its name's rows.
func TestEditor_UpdateAndDeleteKeepTheNameWithTheRow(t *testing.T) {
	ctx := context.Background()
	q, tq, texts := mocked(t)
	e := weapondal.NewEditor(q, texts)
	w := longBow
	w.TwoHanded, w.Name = false, localized.Text{"en": "Bow"}

	mock.InOrder(
		q.EXPECT().UpdateWeapon(ctx, queries.UpdateWeaponParams{
			Position: 9, DamageSm: "1d8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2,
			Kind: "missile", TwoHanded: 0, AmmoKind: "arrow", WeaponType: "long_bow", CharacterID: owner, ID: "long_bow",
		}).Return(30, nil).Once(),
		tq.EXPECT().DeleteText(ctx, int64(30)).Return(nil).Once(),
		tq.EXPECT().InsertText(ctx, textq.InsertTextParams{TextID: 30, Lang: "en", Body: "Bow"}).Return(nil).Once(),
	)
	require.NoError(t, e.Update(ctx, owner, w))

	mock.InOrder(
		q.EXPECT().DeleteWeapon(ctx, queries.DeleteWeaponParams{CharacterID: owner, ID: "long_bow"}).Return(30, nil).Once(),
		tq.EXPECT().DeleteText(ctx, int64(30)).Return(nil).Once(),
	)
	require.NoError(t, e.Delete(ctx, owner, "long_bow"))
}

// No row updated or deleted is ErrWeaponNotFound, and no name is touched.
func TestEditor_UnknownIsNotFound(t *testing.T) {
	ctx := context.Background()
	q, _, texts := mocked(t)
	e := weapondal.NewEditor(q, texts)

	q.EXPECT().UpdateWeapon(ctx, mock.Anything).Return(0, sql.ErrNoRows).Once()
	require.ErrorIs(t, e.Update(ctx, owner, longBow), weaponsvc.ErrWeaponNotFound)
	q.EXPECT().DeleteWeapon(ctx, queries.DeleteWeaponParams{CharacterID: owner, ID: "long_bow"}).Return(0, sql.ErrNoRows).Once()
	require.ErrorIs(t, e.Delete(ctx, owner, "long_bow"), weaponsvc.ErrWeaponNotFound)
}

// Storage errors are returned, never taken for not found, and stop the
// write: no row without its name, no name written after a failed update.
func TestStorageErrors_AreReturnedAndStopTheWrite(t *testing.T) {
	ctx := context.Background()
	q, tq, texts := mocked(t)
	e := weapondal.NewEditor(q, texts)

	q.EXPECT().GetWeapon(ctx, queries.GetWeaponParams{CharacterID: owner, ID: "x"}).Return(queries.Weapon{}, assert.AnError).Once()
	_, err := weapondal.New(q, texts).Get(ctx, owner, "x")
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, combatsvc.ErrWeaponNotFound)

	q.EXPECT().ListWeapons(ctx, owner).Return([]queries.Weapon{bowRow}, nil).Once()
	q.EXPECT().WeaponNames(ctx, owner).Return(nil, assert.AnError).Once()
	_, err = e.List(ctx, owner)
	require.ErrorIs(t, err, assert.AnError)

	tq.EXPECT().NextTextKey(ctx).Return(0, assert.AnError).Once()
	require.ErrorIs(t, e.Insert(ctx, owner, longBow), assert.AnError)

	q.EXPECT().UpdateWeapon(ctx, mock.Anything).Return(0, assert.AnError).Once()
	err = e.Update(ctx, owner, longBow)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, weaponsvc.ErrWeaponNotFound)

	q.EXPECT().DeleteWeapon(ctx, queries.DeleteWeaponParams{CharacterID: owner, ID: "long_bow"}).Return(0, assert.AnError).Once()
	require.ErrorIs(t, e.Delete(ctx, owner, "long_bow"), assert.AnError)
}
