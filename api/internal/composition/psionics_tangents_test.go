package composition_test

// ADR 0044 step 3 end to end: a mind's row stored, let go and undone, and
// ended unpaid and undone.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// mindRows are the active tangent and contact rows.
func mindRows(t *testing.T, bs buffsvc.SVC, id int64) []buffsvc.BuffEffect {
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

// ADR 0044 step 3: two tangents on a mind survive the store; letting go
// (dismiss) and an unpaid Next Round each end the row, and each undo brings
// it back with its target and tangents.
func TestTangents_Step3_UndoRestoresTheRow(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, defenceModes, fives(t))
	_, _, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	_, _, err = bs.Buffs.AddTangents(ctx, id, "Grey Ogre", 2)
	require.NoError(t, err)
	rows := mindRows(t, bs.Buffs, id)
	require.Len(t, rows, 1)
	held := rows[0]
	assert.Equal(t, "Grey Ogre", held.Target)
	assert.Equal(t, 2, held.Tangents)

	_, _, err = bs.Buffs.Dismiss(ctx, id, held.ID, "")
	require.NoError(t, err)
	assert.Empty(t, mindRows(t, bs.Buffs, id), "let go")
	undoTop(t, bs, id)
	assert.Equal(t, []buffsvc.BuffEffect{held}, mindRows(t, bs.Buffs, id))

	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id) // its round closes, nothing charged
	require.NoError(t, err)
	spendPSP(t, bs, id, currentPSP(t, bs, id))
	_, expired, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Len(t, expired, 1, "unpaid")
	assert.Empty(t, mindRows(t, bs.Buffs, id))
	undoTop(t, bs, id)
	back := mindRows(t, bs.Buffs, id)
	require.Len(t, back, 1)
	assert.Equal(t, "Grey Ogre", back[0].Target)
	assert.Equal(t, 2, back[0].Tangents)
}
