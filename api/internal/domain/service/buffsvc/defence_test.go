package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// defenceRow is a defence mode's row (ADR 0044 steps 4 and 5).
func defenceRow(id, power string, kind buffsvc.DefenceKind) buffsvc.BuffEffect {
	b := buffsvc.BuffEffect{ID: id, Name: power, PowerID: power, Defence: kind}
	if kind == buffsvc.DefenceRaised {
		b.Duration = 1
	}
	return b
}

// ADR 0044 step 4: a raised defence lasts the round it is raised in: every
// move of the round counter ends it (Next Round, End combat, Set round,
// passing time); a standing one stays.
func TestRaiseDefence_Step4_LastsTheRound(t *testing.T) {
	ctx := context.Background()
	raised, standing := defenceRow("r", "thought_shield", buffsvc.DefenceRaised), defenceRow("s", "mind_blank", buffsvc.DefenceStanding)

	t.Run("Next Round", func(t *testing.T) {
		f := newFixture(t)
		active, expired, _ := f.advances(t, []buffsvc.BuffEffect{raised, standing}, 3)
		assert.Equal(t, []buffsvc.BuffEffect{standing}, active)
		require.Len(t, expired, 1)
		assert.Equal(t, "r", expired[0].ID)
	})
	t.Run("Next Round, whatever rounds the row was edited to", func(t *testing.T) {
		f := newFixture(t)
		long := raised
		long.Duration = 5
		active, expired, _ := f.advances(t, []buffsvc.BuffEffect{long}, 3)
		assert.Empty(t, active)
		require.Len(t, expired, 1)
	})
	t.Run("passing time", func(t *testing.T) {
		f := newFixture(t)
		active, expired, _ := f.passes(t, []buffsvc.BuffEffect{raised, standing}, 3, 1)
		assert.Equal(t, []buffsvc.BuffEffect{standing}, active)
		require.Len(t, expired, 1)
	})
	t.Run("End combat", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{raised, standing}, 3)
		f.records()
		f.saves([]buffsvc.BuffEffect{standing})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)
		_, _, err := f.svc.ResetRound(ctx, charID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound}, snapKeys(f.entries[0]), "undo raises it again")
	})
	t.Run("Set round", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{raised}, 3)
		f.records()
		f.saves([]buffsvc.BuffEffect{})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 7).Return(nil).Once()
		_, _, err := f.svc.SetRound(ctx, charID, 7)
		require.NoError(t, err)
	})
	t.Run("Set round to the same round keeps it", func(t *testing.T) {
		f := newFixture(t) // SaveActiveBuffs unexpected
		f.state([]buffsvc.BuffEffect{raised}, 3)
		f.records()
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 3).Return(nil).Once()
		_, _, err := f.svc.SetRound(ctx, charID, 3)
		require.NoError(t, err)
	})
}

// ADR 0044 step 4 (Q2): one raised defence a round, a second replaces the
// first; a standing one replaces the raised and the standing ones, and a
// raised one leaves the standing one up (the current defence is the raised).
func TestRaiseDefence_Step4_ASecondReplacesTheFirst(t *testing.T) {
	ctx := context.Background()
	shield, standing := defenceRow("r", "thought_shield", buffsvc.DefenceRaised), defenceRow("s", "mind_blank", buffsvc.DefenceStanding)
	other := buffsvc.BuffEffect{ID: "x", Name: "Bless"}

	f := newFixture(t)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{shield, standing, other}, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(4, nil).Once()
	barrier := defenceRow("b", "mental_barrier", buffsvc.DefenceRaised)
	want := barrier
	want.StartRound = 4
	f.saves([]buffsvc.BuffEffect{standing, other, want})
	_, _, err := f.svc.Add(ctx, charID, barrier)
	require.NoError(t, err)
	cur, ok := buffsvc.CurrentDefence([]buffsvc.BuffEffect{standing, other, want})
	require.True(t, ok)
	assert.Equal(t, "b", cur.ID, "the raised mode over the standing one")

	f = newFixture(t)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{shield, other}, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(4, nil).Once()
	again := defenceRow("s2", "mind_blank", buffsvc.DefenceStanding)
	wantStanding := again
	wantStanding.StartRound = 4
	f.saves([]buffsvc.BuffEffect{other, wantStanding})
	_, _, err = f.svc.Add(ctx, charID, again)
	require.NoError(t, err)

	_, ok = buffsvc.CurrentDefence([]buffsvc.BuffEffect{other})
	assert.False(t, ok, "no defence row: none")
}
