package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ADR 0004 step 17: a buff from a spell carries the spell's catalogue id.
// Add (the spell cast) keeps the id it is given and a renewal takes the new
// one; Activate (a buff entered by hand) drops a caller's id; edit, absorb
// and the round keep it; the buff entries name it (spellId, and
// expiredSpellIds parallel to expired, omitted when no expired buff has
// one); the void restores it and an entry recorded before it restores "".
func TestBuffs_Step17_SpellID(t *testing.T) {
	ctx := context.Background()

	t.Run("add keeps the spell id; a renewal takes the new buff's", func(t *testing.T) {
		f := newFixture(t)
		old := buffsvc.BuffEffect{ID: "o", Name: "Stoneskin", Charges: 2, SpellID: "stoneskin_old"}
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{old}, nil).Once()
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, nil).Once()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

		got, snap, err := f.svc.Add(ctx, charID, buffsvc.BuffEffect{Name: "Stoneskin", Charges: 6, SpellID: "stoneskin"})
		require.NoError(t, err)
		assert.Equal(t, "stoneskin", got.SpellID)
		require.Len(t, saved, 1, "renewed")
		assert.Equal(t, "stoneskin", saved[0].SpellID)
		assert.Equal(t, "stoneskin", decodeSnap(t, snap.After)[0]["spell_id"], "the snapshot carries it for the void")
		assert.Equal(t, "stoneskin_old", decodeSnap(t, snap.Before)[0]["spell_id"])
	})

	t.Run("activate drops a caller's spell id: a hand-made buff is not from a spell", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 0)
		f.records()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

		got, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "Strength", Duration: 5, SpellID: "strength"})
		require.NoError(t, err)
		require.Len(t, saved, 1)
		assert.Empty(t, saved[0].SpellID)
		assert.Empty(t, got[0].SpellID)
		assert.NotContains(t, decodeSnap(t, f.entries[0].Snapshots[0].After)[0], "spell_id")
		assert.NotContains(t, string(f.entries[0].Payload), "spellId")
	})

	mirror := buffsvc.BuffEffect{ID: "m", Name: "Mirror Image", Duration: 4, Charges: 3, ChargesTotal: 3, SpellID: "mirror_image"}
	gem := buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}
	entries := []struct {
		name    string
		run     func(f *fixture) error
		want    []buffsvc.BuffEffect
		payload string
	}{
		{
			"absorb keeps it and names it", func(f *fixture) error {
				_, err := f.svc.Trigger(ctx, charID, "m")
				return err
			},
			[]buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 4, Charges: 2, ChargesTotal: 3, SpellID: "mirror_image"}, gem},
			`{"action":"absorb","buff":"Mirror Image","spellId":"mirror_image","charges":2}`,
		},
		{
			"edit keeps it and names it", func(f *fixture) error {
				_, err := f.svc.Edit(ctx, charID, "m", buffsvc.BuffEdit{Duration: intPtr(2)})
				return err
			},
			[]buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 2, Charges: 3, ChargesTotal: 3, SpellID: "mirror_image"}, gem},
			`{"action":"edit","buff":"Mirror Image","spellId":"mirror_image","fromRounds":4,"fromCharges":3,"rounds":2,"charges":3}`,
		},
		{
			"dismiss names it", func(f *fixture) error {
				_, _, err := f.svc.Dismiss(ctx, charID, "m", "")
				return err
			},
			[]buffsvc.BuffEffect{gem},
			`{"action":"dismiss","buff":"Mirror Image","spellId":"mirror_image"}`,
		},
		{
			"a buff not from a spell has no spellId", func(f *fixture) error {
				_, _, err := f.svc.Dismiss(ctx, charID, "g", "")
				return err
			},
			[]buffsvc.BuffEffect{mirror},
			`{"action":"dismiss","buff":"Power Gem"}`,
		},
	}
	for _, tt := range entries {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state([]buffsvc.BuffEffect{mirror, gem}, 1)
			f.records()
			f.saves(tt.want)
			require.NoError(t, tt.run(f))
			assert.JSONEq(t, tt.payload, string(f.entries[0].Payload))
		})
	}

	t.Run("next round keeps it and names the expired buffs' spells in order", func(t *testing.T) {
		f := newFixture(t)
		haste := buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 1, SpellID: "haste"}
		hand := buffsvc.BuffEffect{ID: "x", Name: "Rage", Duration: 1}
		f.state([]buffsvc.BuffEffect{haste, hand, mirror}, 2)
		f.records()
		f.saves([]buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 3, Charges: 3, ChargesTotal: 3, SpellID: "mirror_image"}})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 3).Return(nil).Once()
		f.initiativeIs(nil)
		f.rollsInitiative(4)

		_, _, _, _, _, err := f.svc.AdvanceRound(ctx, charID)
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"next_round","fromRound":2,"toRound":3,"expired":["Haste","Rage"],"expiredSpellIds":["haste",""]}`,
			string(f.entries[0].Payload))
	})

	t.Run("passing time with no expired spell omits expiredSpellIds", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{{ID: "x", Name: "Rage", Duration: 1}, gem}, 0)
		f.records()
		f.saves([]buffsvc.BuffEffect{gem})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)

		_, _, _, _, _, err := f.svc.PassTime(ctx, charID, 0.02)
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"pass_time","hours":0.02,"rounds":1,"fromRound":0,"toRound":0,"expired":["Rage"]}`,
			string(f.entries[0].Payload))
	})

	t.Run("passing time names the expired spells", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{mirror, gem}, 0)
		f.records()
		f.saves([]buffsvc.BuffEffect{gem})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)

		_, _, _, _, _, err := f.svc.PassTime(ctx, charID, 1)
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"pass_time","hours":1,"rounds":60,"fromRound":0,"toRound":0,"expired":["Mirror Image"],"expiredSpellIds":["mirror_image"]}`,
			string(f.entries[0].Payload))
	})

	t.Run("voiding restores it from the snapshot", func(t *testing.T) {
		f := newFixture(t)
		before := []buffsvc.BuffEffect{mirror, gem}
		f.state(before, 1)
		f.records()
		f.saves([]buffsvc.BuffEffect{gem})
		_, _, err := f.svc.Dismiss(ctx, charID, "m", "")
		require.NoError(t, err)

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyBuffs).Restore(ctx, charID, f.entries[0].Snapshots[0].Before))
	})

	t.Run("an entry recorded before the field restores no spell id", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, []buffsvc.BuffEffect{{ID: "o", Name: "Haste", Duration: 3}}).Return(nil).Once()
		raw := snaptest.Env(`[{"id":"o","name":"Haste","charges":0,"duration":3,"effects":{}}]`)
		require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyBuffs).Restore(ctx, charID, raw))
	})
}
