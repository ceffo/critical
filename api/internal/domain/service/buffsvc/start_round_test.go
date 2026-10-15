package buffsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ADR 0004 step 15: a buff carries the round it was put on the list in;
// the server stamps it on activate and add (a caller's value is ignored)
// and nothing else changes it; voiding an entry brings it back.
func TestBuffs_Step15_StartRound(t *testing.T) {
	ctx := context.Background()

	t.Run("activate stamps the current round, ignoring the caller's", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 4)
		f.records()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

		got, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "Haste", Duration: 3, StartRound: 99})
		require.NoError(t, err)
		require.Len(t, saved, 1)
		assert.Equal(t, 4, saved[0].StartRound)
		assert.Equal(t, 4, got[0].StartRound)
		after := decodeSnap(t, f.entries[0].Snapshots[0].After)
		assert.EqualValues(t, 4, after[0]["start_round"], "the snapshot carries it for the void")
	})

	t.Run("add (a cast or a received spell) stamps the current round; a renewed buff starts again", func(t *testing.T) {
		f := newFixture(t)
		old := buffsvc.BuffEffect{ID: "s1", Name: "Stoneskin", Charges: 2, StartRound: 1}
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{old}, nil).Once()
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(6, nil).Once()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

		got, _, err := f.svc.Add(ctx, charID, buffsvc.BuffEffect{ID: "s2", Name: "Stoneskin", Charges: 9, StartRound: 2})
		require.NoError(t, err)
		assert.Equal(t, 6, got.StartRound)
		require.Len(t, saved, 1)
		assert.Equal(t, 6, saved[0].StartRound)
	})

	haste := buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 5, StartRound: 2}
	mirror := buffsvc.BuffEffect{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 3, StartRound: 1}
	keeps := []struct {
		name string
		run  func(f *fixture) error
		want []buffsvc.BuffEffect
	}{
		{"edit keeps it", func(f *fixture) error {
			_, err := f.svc.Edit(ctx, charID, "h", buffsvc.BuffEdit{Duration: intPtr(8)})
			return err
		}, []buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 8, StartRound: 2}, mirror}},
		{"absorb keeps it", func(f *fixture) error {
			_, err := f.svc.Trigger(ctx, charID, "m")
			return err
		}, []buffsvc.BuffEffect{haste, {ID: "m", Name: "Mirror Image", Duration: 9, Charges: 2, StartRound: 1}}},
		{"Next Round keeps it", func(f *fixture) error {
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 6).Return(nil).Once()
			_, _, _, err := f.svc.AdvanceRound(ctx, charID)
			return err
		}, []buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 4, StartRound: 2}, {ID: "m", Name: "Mirror Image", Duration: 8, Charges: 3, StartRound: 1}}},
		{"passing time keeps it", func(f *fixture) error {
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
			_, _, _, _, err := f.svc.PassTime(ctx, charID, 0.05)
			return err
		}, []buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 2, StartRound: 2}, {ID: "m", Name: "Mirror Image", Duration: 6, Charges: 3, StartRound: 1}}},
	}
	for _, tt := range keeps {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state([]buffsvc.BuffEffect{haste, mirror}, 5)
			f.records()
			f.saves(tt.want)
			require.NoError(t, tt.run(f))
		})
	}

	t.Run("voiding restores it from the snapshot", func(t *testing.T) {
		f := newFixture(t)
		before := []buffsvc.BuffEffect{haste, mirror}
		f.state(before, 5)
		f.records()
		f.saves([]buffsvc.BuffEffect{mirror})
		_, err := f.svc.Dismiss(ctx, charID, "h")
		require.NoError(t, err)

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyBuffs].Restore(ctx, charID, f.entries[0].Snapshots[0].Before))
	})

	t.Run("an entry recorded before the field restores round 0", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, []buffsvc.BuffEffect{{ID: "o", Name: "Old", Duration: 3}}).Return(nil).Once()
		old := json.RawMessage(`[{"id":"o","name":"Old","charges":0,"duration":3,"effects":{}}]`)
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyBuffs].Restore(ctx, charID, old))
	})
}
