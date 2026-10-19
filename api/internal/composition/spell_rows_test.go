package composition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
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

// ADR 0004 step 20.5 through the real services and storage: the mage casts
// Haste on someone else, which changes none of his numbers, then on himself:
// the row on him applies, and the row on the other stays where it was (it is
// not renewed by, nor does it replace, his own).
func TestBuffs_Step20_HasteOnAnotherThenOnTheBard(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{1, 1, 1, 1}, calls: map[string]int{}}
	dice.d100s = []int{1}
	tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Name: "Elminster", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 11, Int: 17, Wis: 12, Cha: 10, HPMax: 20, XP: 40000,
	})
	got, err := tb.bs.Learn.Learn(tb.ctx, tb.id, "haste")
	require.NoError(t, err)
	require.True(t, got.Learned)
	_, err = tb.bs.Prep.Prepare(tb.ctx, tb.id, []string{"haste", "haste"})
	require.NoError(t, err)
	aggregate := func() buffsvc.BuffModifiers {
		got, err := tb.bs.Buffs.Aggregate(tb.ctx, tb.id, buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 9}}, buffsvc.WeaponMelee)
		require.NoError(t, err)
		return got
	}
	neutral := aggregate()
	neutralInit := tb.initiative(t)

	other, onSelf := false, true
	res, err := tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{OnSelf: &other})
	require.NoError(t, err)
	require.NotNil(t, res.Buff)
	assert.True(t, res.Buff.OnOther)
	assert.Equal(t, neutral, aggregate(), "a cast on someone else changes no aggregate")
	assert.Equal(t, neutralInit, tb.initiative(t), "nor his initiative")

	_, err = tb.bs.Cast.Cast(tb.ctx, tb.id, 1, castsvc.CastRequest{OnSelf: &onSelf})
	require.NoError(t, err)
	rows, err := tb.bs.Buffs.ListStatus(tb.ctx, tb.id)
	require.NoError(t, err)
	require.Len(t, rows, 2, "the row on him does not replace the row on the other")
	assert.True(t, rows[0].OnOther)
	assert.False(t, rows[0].Applies)
	assert.False(t, rows[1].OnOther)
	assert.True(t, rows[1].Applies)
	assert.Equal(t, "haste", rows[0].SpellID)
	assert.Equal(t, "haste", rows[1].SpellID)
}

// ADR 0004 step 20.3 and ADR 0018: receiving Polymorph Self while it already
// runs on the Bard renews the row; voiding that receive brings the previous
// row back with the rounds it had left.
func TestBuffs_Step20_VoidingARenewalRestoresTheOldRounds(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6, 6, 6, 6}, calls: map[string]int{}}
	tb := mageWith(t, dice)
	list := func() []buffsvc.BuffStatus {
		got, err := tb.bs.Buffs.ListStatus(tb.ctx, tb.id)
		require.NoError(t, err)
		return got
	}
	_, err := tb.bs.Cast.Receive(tb.ctx, tb.id, castsvc.ReceiveRequest{SpellID: "polymorph_self", CasterLevel: 15})
	require.NoError(t, err)
	for range 3 {
		_, _, _, _, err = tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
		require.NoError(t, err)
	}
	require.Equal(t, 297, list()[0].Duration)

	_, err = tb.bs.Cast.Receive(tb.ctx, tb.id, castsvc.ReceiveRequest{SpellID: "polymorph_self", CasterLevel: 15})
	require.NoError(t, err)
	require.Len(t, list(), 1, "renewed, not duplicated")
	require.Equal(t, 300, list()[0].Duration)

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
	rows := list()
	require.Len(t, rows, 1)
	assert.Equal(t, 297, rows[0].Duration, "the previous row, with its old remaining rounds")
}
