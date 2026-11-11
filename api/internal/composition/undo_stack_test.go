package composition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
)

// ADR 0018 step 2 through the real services and storage: the log is a stack.
// A cast, then a long rest and an hour passing on top of it: the cast cannot
// be popped while later entries sit above it, each undo pops exactly the
// newest entry, and the slot and the buff come back one step at a time.
func TestUndo_Step2_ACastUnderLaterEntriesIsPoppedLast(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{1}, calls: map[string]int{}}
	tb := mageWith(t, dice, "sleep")
	used := func() bool {
		slots, err := tb.bs.Spells.GetSlots(tb.ctx, tb.id)
		require.NoError(t, err)
		require.Len(t, slots, 1)
		return slots[0].Used
	}
	rows := func() []buffsvc.BuffStatus {
		got, err := tb.bs.Buffs.ListStatus(tb.ctx, tb.id)
		require.NoError(t, err)
		return got
	}

	_, err := tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	cast := tb.top(t)
	assert.True(t, used())
	require.Len(t, rows(), 1)

	_, err = tb.bs.Rest.LongRest(tb.ctx, tb.id)
	require.NoError(t, err)
	rest := tb.top(t)
	assert.False(t, used(), "the rest refills the slot")

	_, _, _, _, _, err = tb.bs.Buffs.PassTime(tb.ctx, tb.id, 1)
	require.NoError(t, err)
	pass := tb.top(t)
	assert.Empty(t, rows(), "an hour expires Sleep's row")

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, cast.ID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged, "the cast is not on top")
	assert.Empty(t, rows(), "a refused undo changes nothing")

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, pass.ID)
	require.NoError(t, err)
	assert.Len(t, rows(), 1, "popping the hour brings the row back")
	assert.False(t, used())

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, rest.ID)
	require.NoError(t, err)
	assert.True(t, used(), "popping the rest spends the slot again")

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, cast.ID)
	require.NoError(t, err)
	assert.False(t, used(), "popping the cast gives the slot back")
	assert.Empty(t, rows())
}
