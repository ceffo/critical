package composition_test

// ADR 0044 steps 4, 5 and 8 end to end: a defence mode raised and its undo,
// a standing defence across the long rest, and a contest's undo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

var defenceModes = []psionics.Pick{
	{Kind: psionics.PickDiscipline, ID: "telepathy"},
	{Kind: psionics.PickPower, ID: "thought_shield"},
	{Kind: psionics.PickPower, ID: "mind_blank"},
}

// defenceRows are the active rows of a defence mode.
func defenceRows(t *testing.T, bs interface {
	ListActive(context.Context, int64) ([]buffsvc.BuffEffect, error)
}, id int64,
) []buffsvc.BuffEffect {
	t.Helper()
	rows, err := bs.ListActive(context.Background(), id)
	require.NoError(t, err)
	var out []buffsvc.BuffEffect
	for _, b := range rows {
		if b.Defence != buffsvc.DefenceNone {
			out = append(out, b)
		}
	}
	return out
}

// ADR 0044 step 4: thought shield raised in round 1 pays 2 PSPs, its row
// lasts the round; the undo gives both back; a use of it is refused; the
// next Next Round ends it.
func TestRaiseDefence_Step4_UndoGivesThePSPAndTheRowBack(t *testing.T) {
	ctx := context.Background()
	dice := fives(t)
	bs, id := thiefPsionicistRolling(t, defenceModes, dice)

	_, err := bs.Psionics.RaiseDefence(ctx, id, "thought_shield")
	require.ErrorIs(t, err, psionicsvc.ErrNotInCombat)
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)

	before := dice.calls["die"]
	res, err := bs.Psionics.RaiseDefence(ctx, id, "thought_shield")
	require.NoError(t, err)
	assert.Equal(t, before, dice.calls["die"], "no roll")
	assert.Equal(t, 179, currentPSP(t, bs, id))
	rows := defenceRows(t, bs.Buffs, id)
	require.Len(t, rows, 1)
	assert.Equal(t, res.BuffID, rows[0].ID)
	assert.Equal(t, buffsvc.DefenceRaised, rows[0].Defence)
	_, err = bs.Psionics.Use(ctx, id, "thought_shield", psionicsvc.UseRequest{})
	require.ErrorIs(t, err, psionicsvc.ErrDefenceModeIsRaised)

	undoTop(t, bs, id)
	assert.Equal(t, 181, currentPSP(t, bs, id))
	assert.Empty(t, defenceRows(t, bs.Buffs, id))
	redoTop(t, bs, id)
	assert.Equal(t, 179, currentPSP(t, bs, id))
	assert.Len(t, defenceRows(t, bs.Buffs, id), 1)

	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, defenceRows(t, bs.Buffs, id), "it lasts the round")
	assert.Equal(t, 179, currentPSP(t, bs, id), "never charged")
}

// ADR 0044 step 5: mind blank raised out of combat is a standing row: PSPs
// recover under it and the long rest keeps it.
func TestLongRest_Step5_KeepsAZeroCostRow(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, defenceModes, fives(t))
	_, err := bs.Psionics.RaiseDefence(ctx, id, "mind_blank")
	require.NoError(t, err)
	spendPSP(t, bs, id, 20)

	_, err = bs.Psionics.Recover(ctx, id, 1, "resting")
	require.NoError(t, err, "a zero-cost row allows recovery")
	_, err = bs.Rest.LongRest(ctx, id)
	require.NoError(t, err)
	rows := defenceRows(t, bs.Buffs, id)
	require.Len(t, rows, 1, "kept asleep")
	assert.Equal(t, buffsvc.DefenceStanding, rows[0].Defence)
	entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindLongRest, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.NotContains(t, string(entries[0].Payload), "mind_blank")
}

// ADR 0044 step 8: a contest against thought shield logs one
// psionic_defence entry with no PSP; its undo removes it and gives nothing
// back.
func TestDefend_Step8_UndoRemovesTheEntry(t *testing.T) {
	ctx := context.Background()
	dice := fives(t)
	bs, id := thiefPsionicistRolling(t, defenceModes, dice)
	_, _, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	_, err = bs.Psionics.RaiseDefence(ctx, id, "thought_shield")
	require.NoError(t, err)

	before := dice.calls["die"]
	res, err := bs.Psionics.Defend(ctx, id, psionicsvc.DefendRequest{
		Attack: "psionic_blast", Rolls: []psionics.ContestSide{{Roll: 3, Success: true}},
	})
	require.NoError(t, err)
	assert.Equal(t, "thought_shield", res.Defence.Defence)
	assert.Equal(t, 15, res.Defence.Score, "Wis 17 - 2")
	require.NotNil(t, res.Defence.Modifier, "Table 14's cell")
	assert.Equal(t, psionics.SideDefender, res.Defence.Contests[0].Winner, "3 against 5")
	assert.Equal(t, 179, currentPSP(t, bs, id), "no PSP")
	assert.Equal(t, before+1, dice.calls["die"], "one check")

	undoTop(t, bs, id)
	entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindPsionicDefence, 5)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, 179, currentPSP(t, bs, id))
	assert.Len(t, defenceRows(t, bs.Buffs, id), 1, "the raise stays")
}
