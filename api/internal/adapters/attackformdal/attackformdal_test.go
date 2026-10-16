// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package attackformdal_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// No stored row (none saved for this character yet) is "no form", not an
// error.
func TestGet_NoneUntilSaved(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().GetAttackForm(ctx, int64(2)).Return(queries.GetAttackFormRow{}, sql.ErrNoRows).Once()
	_, ok, err := attackformdal.New(q).Get(ctx, 2)
	require.NoError(t, err)
	assert.False(t, ok)
}

// What Put stores for a character is what Get maps back: every field, a known
// target AC (negative included) as target_ac_known 1, an unknown one (nil) as
// known 0 and AC 0, read back as unknown, not as AC 0.
func TestPut_StoresWhatGetMapsBack(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		form combatsvc.AttackForm
		row  queries.GetAttackFormRow
	}{
		"known AC": {
			form: combatsvc.AttackForm{
				WeaponID: "short_bow", PreviousWeaponID: "long_sword", AmmoItemID: "arrows",
				TargetAC: new(-3), TargetSize: combatsvc.SizeLarge,
				Situational: combatsvc.Situational{Value: 2, Label: "rear"},
			},
			row: queries.GetAttackFormRow{
				WeaponID: "short_bow", PreviousWeaponID: "long_sword", AmmoItemID: "arrows",
				TargetAc: -3, TargetAcKnown: 1, TargetSize: "l", SituationalValue: 2, SituationalLabel: "rear",
			},
		},
		"unknown AC": {
			form: combatsvc.AttackForm{WeaponID: "long_sword", TargetSize: combatsvc.SizeSmallMedium},
			row:  queries.GetAttackFormRow{WeaponID: "long_sword", TargetSize: "sm"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			q := mocks.NewMockQuerier(t)
			dal := attackformdal.New(q)
			r := tc.row
			q.EXPECT().UpsertAttackForm(ctx, queries.UpsertAttackFormParams{
				CharacterID: 1, WeaponID: r.WeaponID, PreviousWeaponID: r.PreviousWeaponID, AmmoItemID: r.AmmoItemID,
				TargetAc: r.TargetAc, TargetAcKnown: r.TargetAcKnown, TargetSize: r.TargetSize,
				SituationalValue: r.SituationalValue, SituationalLabel: r.SituationalLabel,
			}).Return(nil).Once()
			require.NoError(t, dal.Put(ctx, 1, tc.form))

			q.EXPECT().GetAttackForm(ctx, int64(1)).Return(r, nil).Once()
			got, ok, err := dal.Get(ctx, 1)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, tc.form, got)
		})
	}
}

func TestStorageErrors_AreReturned(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := attackformdal.New(q)

	q.EXPECT().GetAttackForm(ctx, int64(1)).Return(queries.GetAttackFormRow{}, assert.AnError).Once()
	_, ok, err := dal.Get(ctx, 1)
	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, ok)

	q.EXPECT().UpsertAttackForm(ctx, queries.UpsertAttackFormParams{CharacterID: 1}).Return(assert.AnError).Once()
	require.ErrorIs(t, dal.Put(ctx, 1, combatsvc.AttackForm{}), assert.AnError)
}
