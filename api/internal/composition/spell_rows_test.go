package composition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
)

// ADR 0004 step 20 through the real services and storage: a utility spell
// (Polymorph Self, 20 rounds a level) received from a 15th-level caster puts
// a 300-round row on the list, with no effects; Next Round and passing an
// hour count it down (steps 2 and 20.3); undoing the receive removes it
// (ADR 0018). An area spell the mage casts (Sleep, 5 rounds a level) puts a
// row marked as not on him (step 20.5), which changes none of his numbers.
func TestBuffs_Step20_RowCountsDownAndUndoRemovesIt(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6}, calls: map[string]int{}}
	tb := mageWith(t, dice, "sleep")
	list := func() []buffsvc.BuffStatus {
		got, err := tb.bs.Buffs.ListStatus(tb.ctx, tb.id)
		require.NoError(t, err)
		return got
	}
	aggregate := func() buffsvc.BuffModifiers {
		got, err := tb.bs.Buffs.Aggregate(tb.ctx, tb.id, buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 9}}, buffsvc.WeaponMelee)
		require.NoError(t, err)
		return got
	}
	neutral := aggregate()

	res, err := tb.bs.Cast.Receive(tb.ctx, tb.id, castsvc.ReceiveRequest{SpellID: "polymorph_self", CasterLevel: 15})
	require.NoError(t, err)
	require.NotNil(t, res.Buff)
	assert.True(t, res.Buff.Applied)
	rows := list()
	require.Len(t, rows, 1)
	assert.Equal(t, "polymorph_self", rows[0].SpellID, "the client names it in the player's language")
	assert.Equal(t, "Polymorph Self", rows[0].Name)
	assert.Equal(t, 300, rows[0].Duration)
	assert.Equal(t, buffsvc.BuffEffects{}, rows[0].Effects)
	assert.False(t, rows[0].OnOther)

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
	assert.Empty(t, list(), "undoing the receive removes the row")

	_, err = tb.bs.Cast.Receive(tb.ctx, tb.id, castsvc.ReceiveRequest{SpellID: "polymorph_self", CasterLevel: 15})
	require.NoError(t, err)
	_, _, _, _, err = tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	assert.Equal(t, 299, list()[0].Duration, "Next Round counts it down")
	_, _, _, _, err = tb.bs.Buffs.PassTime(tb.ctx, tb.id, 1)
	require.NoError(t, err)
	assert.Equal(t, 239, list()[0].Duration, "an hour is 60 rounds")

	cast, err := tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, cast.Buff)
	assert.True(t, cast.Buff.OnOther)
	assert.False(t, cast.Buff.Applied)
	rows = list()
	require.Len(t, rows, 2)
	assert.Equal(t, "sleep", rows[1].SpellID)
	assert.Equal(t, 15, rows[1].Duration, "5 rounds a level at 3rd level")
	assert.True(t, rows[1].OnOther)
	assert.False(t, rows[1].Applies, "a row on someone else never applies")
	assert.Equal(t, neutral, aggregate(), "no row changes the mage's numbers")

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
	rows = list()
	require.Len(t, rows, 1, "undoing the cast removes its row only")
	assert.Equal(t, "polymorph_self", rows[0].SpellID)
	assert.Equal(t, 1, dice.calls["die"], "the round's d10 only")
}
