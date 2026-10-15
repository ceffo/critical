package buffsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ADR 0004 step 16: a buff carries the charge count it started with; the
// server stamps it on activate and add (a caller's value is ignored), absorb
// and edit keep it (edit raises it with the charges), the void restores it
// and an old entry reads 0 (unknown).
func TestBuffs_Step16_ChargesTotal(t *testing.T) {
	ctx := context.Background()

	t.Run("activate stamps the starting charges, ignoring the caller's", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 0)
		f.records()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

		got, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "Mirror Image", Charges: 9, ChargesTotal: 2})
		require.NoError(t, err)
		require.Len(t, saved, 1)
		assert.Equal(t, 9, saved[0].ChargesTotal)
		assert.Equal(t, 9, got[0].ChargesTotal)
		after := decodeSnap(t, f.entries[0].Snapshots[0].After)
		assert.EqualValues(t, 9, after[0]["charges_total"], "the snapshot carries it for the void")
	})

	t.Run("add stamps it; a buff without charges has none", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, nil).Twice()
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(1, nil).Twice()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Twice()

		got, _, err := f.svc.Add(ctx, charID, buffsvc.BuffEffect{Name: "Stoneskin", Charges: 6, ChargesTotal: 1})
		require.NoError(t, err)
		assert.Equal(t, 6, got.ChargesTotal)
		assert.Equal(t, 6, saved[0].ChargesTotal)
		got, _, err = f.svc.Add(ctx, charID, buffsvc.BuffEffect{Name: "Haste", Duration: 3, ChargesTotal: 4})
		require.NoError(t, err)
		assert.Zero(t, got.ChargesTotal)
	})

	mirror := buffsvc.BuffEffect{ID: "m", Name: "Mirror Image", Charges: 6, ChargesTotal: 9}
	old := buffsvc.BuffEffect{ID: "o", Name: "Old", Charges: 3}
	keeps := []struct {
		name string
		run  func(f *fixture) error
		want []buffsvc.BuffEffect
	}{
		{"absorb keeps it", func(f *fixture) error {
			_, err := f.svc.Trigger(ctx, charID, "m")
			return err
		}, []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Charges: 5, ChargesTotal: 9}, old}},
		{"edit down keeps it", func(f *fixture) error {
			_, err := f.svc.Edit(ctx, charID, "m", buffsvc.BuffEdit{Charges: intPtr(2)})
			return err
		}, []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Charges: 2, ChargesTotal: 9}, old}},
		{"edit above the total raises it", func(f *fixture) error {
			_, err := f.svc.Edit(ctx, charID, "m", buffsvc.BuffEdit{Charges: intPtr(11)})
			return err
		}, []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Charges: 11, ChargesTotal: 11}, old}},
		{"an unknown total stays unknown", func(f *fixture) error {
			_, err := f.svc.Edit(ctx, charID, "o", buffsvc.BuffEdit{Charges: intPtr(7)})
			return err
		}, []buffsvc.BuffEffect{mirror, {ID: "o", Name: "Old", Charges: 7}}},
	}
	for _, tt := range keeps {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state([]buffsvc.BuffEffect{mirror, old}, 1)
			f.records()
			f.saves(tt.want)
			require.NoError(t, tt.run(f))
		})
	}

	t.Run("voiding restores it from the snapshot", func(t *testing.T) {
		f := newFixture(t)
		before := []buffsvc.BuffEffect{mirror, old}
		f.state(before, 1)
		f.records()
		f.saves([]buffsvc.BuffEffect{old})
		_, err := f.svc.Dismiss(ctx, charID, "m")
		require.NoError(t, err)

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyBuffs].Restore(ctx, charID, f.entries[0].Snapshots[0].Before))
	})

	t.Run("an entry recorded before the field restores 0", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, []buffsvc.BuffEffect{{ID: "o", Name: "Old", Charges: 3}}).Return(nil).Once()
		raw := json.RawMessage(`[{"id":"o","name":"Old","charges":3,"duration":0,"effects":{}}]`)
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyBuffs].Restore(ctx, charID, raw))
	})
}
