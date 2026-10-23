package composition_test

// ADR 0044 step 7 end to end: an attack mode's use and its undo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

// linkRows are the active tangent and contact rows.
func linkRows(t *testing.T, bs interface {
	ListActive(context.Context, int64) ([]buffsvc.BuffEffect, error)
}, id int64,
) []buffsvc.BuffEffect {
	t.Helper()
	rows, err := bs.ListActive(context.Background(), id)
	require.NoError(t, err)
	var out []buffsvc.BuffEffect
	for _, b := range rows {
		if b.TangentRow() || b.ContactRow() {
			out = append(out, b)
		}
	}
	return out
}

// ADR 0044 step 7.5: ego whip (Wis 17 - 3, 4 PSPs) on a closed mind with no
// defence wins two tangents for 4 PSPs; an attack on another target breaks
// them. Each undo gives back the PSPs and the rows as they were.
func TestAttack_Step7_UndoRestoresPSPAndTangents(t *testing.T) {
	ctx := context.Background()
	dice := fives(t)
	bs, id := thiefPsionicistRolling(t, []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"},
		{Kind: psionics.PickPower, ID: "ego_whip"},
	}, dice)
	_, _, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)

	before := dice.calls["die"]
	orc := psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed}
	res, err := bs.Psionics.Attack(ctx, id, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: orc})
	require.NoError(t, err)
	assert.Equal(t, before+2, dice.calls["die"], "two checks")
	assert.Equal(t, 14, res.Attack.Score)
	assert.Equal(t, 2, res.Attack.TangentsAfter)
	assert.Equal(t, 177, currentPSP(t, bs, id))
	rows := linkRows(t, bs.Buffs, id)
	require.Len(t, rows, 1)
	assert.Equal(t, "Orc", rows[0].Target)
	assert.Equal(t, 2, rows[0].Tangents)

	_, err = bs.Psionics.Attack(ctx, id, psionicsvc.AttackRequest{
		Mode: "ego_whip", Opponent: psionics.Opponent{Target: "Goblin", Mind: psionics.MindClosed},
	})
	require.NoError(t, err)
	assert.Equal(t, 173, currentPSP(t, bs, id))
	rows = linkRows(t, bs.Buffs, id)
	require.Len(t, rows, 1)
	assert.Equal(t, "Goblin", rows[0].Target, "the Orc's row is broken")

	undoTop(t, bs, id)
	assert.Equal(t, 177, currentPSP(t, bs, id))
	rows = linkRows(t, bs.Buffs, id)
	require.Len(t, rows, 1)
	assert.Equal(t, "Orc", rows[0].Target, "the Orc's row is back")
	assert.Equal(t, 2, rows[0].Tangents)

	undoTop(t, bs, id)
	assert.Equal(t, 181, currentPSP(t, bs, id))
	assert.Empty(t, linkRows(t, bs.Buffs, id))
}
