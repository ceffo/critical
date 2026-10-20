package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ADR 0037 step 20.1: the buff a successful proficiency check puts on carries the proficiency's
// id, as a spell's buff its spell id (ADR 0004 step 17): Add keeps it, the snapshot carries it,
// Activate drops a caller's, and the buff entries name it (proficiencyId; expiredProficiencyIds
// parallel to expired, omitted when no expired buff has one). Remove (step 20.8) takes a row off
// without an entry and hands back the snapshot; a row no longer on the list is no error.
func TestBuffs_Step20_ProficiencyID(t *testing.T) {
	ctx := context.Background()
	ac := -4
	tumbling := buffsvc.BuffEffect{ID: "t", Name: "tumbling", Duration: 1, ProficiencyID: "tumbling", Effects: buffsvc.BuffEffects{AC: &ac}}
	gem := buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}

	t.Run("add keeps it and the snapshot carries it", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, nil).Once()
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(2, nil).Once()
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()
		in := tumbling
		in.ID = ""
		got, snap, err := f.svc.Add(ctx, charID, in)
		require.NoError(t, err)
		assert.Equal(t, "tumbling", got.ProficiencyID)
		assert.Equal(t, "tumbling", decodeSnap(t, snap.After)[0]["proficiency_id"])
	})

	t.Run("activate drops a caller's", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 0)
		f.records()
		var saved []buffsvc.BuffEffect
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()
		_, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "Acrobat", Duration: 1, ProficiencyID: "tumbling"})
		require.NoError(t, err)
		require.Len(t, saved, 1)
		assert.Empty(t, saved[0].ProficiencyID)
	})

	t.Run("dismiss names it", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{tumbling, gem}, 1)
		f.records()
		f.saves([]buffsvc.BuffEffect{gem})
		_, _, err := f.svc.Dismiss(ctx, charID, "t", "")
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"dismiss","buff":"tumbling","proficiencyId":"tumbling"}`, string(f.entries[0].Payload))
	})

	t.Run("next round names the expired buffs' proficiencies in order", func(t *testing.T) {
		f := newFixture(t)
		haste := buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 1, SpellID: "haste"}
		f.state([]buffsvc.BuffEffect{haste, tumbling, gem}, 2)
		f.records()
		f.saves([]buffsvc.BuffEffect{gem})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 3).Return(nil).Once()
		f.initiativeIs(nil)
		f.rollsInitiative(4)
		_, _, _, _, _, err := f.svc.AdvanceRound(ctx, charID)
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"next_round","fromRound":2,"toRound":3,"expired":["Haste","tumbling"],
			"expiredSpellIds":["haste",""],"expiredProficiencyIds":["","tumbling"]}`, string(f.entries[0].Payload))
	})

	t.Run("remove takes the row off and records nothing", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{tumbling, gem}, nil).Once()
		f.saves([]buffsvc.BuffEffect{gem})
		removed, snap, err := f.svc.Remove(ctx, charID, "t", nil)
		require.NoError(t, err)
		assert.True(t, removed)
		assert.Equal(t, buffsvc.StateKeyBuffs, snap.Key)
		assert.Len(t, decodeSnap(t, snap.Before), 2)
		assert.Len(t, decodeSnap(t, snap.After), 1)
		assert.Empty(t, f.entries)
	})

	// ADR 0037 step 20.8: the rows a renewal replaced go back in the place of the row taken off,
	// as they were (the codec keeps every field).
	t.Run("remove puts the rows given back in the row's place", func(t *testing.T) {
		f := newFixture(t)
		ac := -4
		old := buffsvc.BuffEffect{
			ID: "t0", Name: "tumbling", ProficiencyID: "tumbling", Duration: 3, StartRound: 2,
			Note: "by hand", Effects: buffsvc.BuffEffects{AC: &ac},
		}
		raw, err := buffsvc.EncodeRows([]buffsvc.BuffEffect{old})
		require.NoError(t, err)
		back, err := buffsvc.DecodeRows(raw)
		require.NoError(t, err)
		require.Equal(t, []buffsvc.BuffEffect{old}, back)
		haste := buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 1, SpellID: "haste"}
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{haste, tumbling, gem}, nil).Once()
		f.saves([]buffsvc.BuffEffect{haste, old, gem})
		removed, snap, err := f.svc.Remove(ctx, charID, "t", back)
		require.NoError(t, err)
		assert.True(t, removed)
		assert.Len(t, decodeSnap(t, snap.After), 3)
		assert.Empty(t, f.entries)
	})

	t.Run("remove of a row no longer on the list writes nothing", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{gem}, nil).Once()
		removed, _, err := f.svc.Remove(ctx, charID, "t", []buffsvc.BuffEffect{gem})
		require.NoError(t, err)
		assert.False(t, removed)
	})
}
