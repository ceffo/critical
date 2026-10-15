package composition_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A free dice roll is the top of the action stack: undo pops it, nothing to
// restore (ADR 0010), and the roll before it is the top again.
func TestDiceRoll_UndoPopsTheRoll(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d6", 4}, roll{"d6", 2}, roll{"d100", 77})
	first, err := bs.Dice.Roll(ctx, id, "2d6")
	require.NoError(t, err)
	assert.Equal(t, 6, first.Total)
	second, err := bs.Dice.Roll(ctx, id, "d%+1")
	require.NoError(t, err)
	assert.Equal(t, 78, second.Total)

	top := latest(t, bs)
	assert.Equal(t, second.LogEntryID, top.ID)
	assert.Equal(t, actionlog.KindDice, top.Kind)
	assert.JSONEq(t, `{"formula":"d%+1","groups":[{"sides":100,"rolls":[77],"negative":false,"subtotal":77}],
		"modifier":1,"total":78}`, string(top.Payload))

	_, err = bs.Log.Undo(ctx, id, second.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, first.LogEntryID, latest(t, bs).ID)
	_, err = bs.Log.Undo(ctx, id, first.LogEntryID)
	require.NoError(t, err)
}

// ADR 0006 step 6: a dice roll is not one of the roll kinds, so it does not
// end the adjustment of the latest save (it would if it were).
func TestDiceRoll_LeavesTheSaveAdjustable(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	_, err = bs.Dice.Roll(ctx, id, "d20")
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 4})
	require.NoError(t, err)
}

func TestDiceFavourites_PersistPerCharacter(t *testing.T) {
	bs, _ := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	a, err := bs.Dice.AddFavourite(ctx, id, "d6 + d6 + 2")
	require.NoError(t, err)
	b, err := bs.Dice.AddFavourite(ctx, id, "d%")
	require.NoError(t, err)
	again, err := bs.Dice.AddFavourite(ctx, id, "2D6+2")
	require.NoError(t, err)
	assert.Equal(t, a.ID, again.ID, "the same formula is saved once")

	favs, err := bs.Dice.Favourites(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, a.Formula, "2d6+2")
	require.Len(t, favs, 2)
	assert.Equal(t, b.ID, favs[1].ID)

	require.NoError(t, bs.Dice.DeleteFavourite(ctx, id, a.ID))
	require.ErrorIs(t, bs.Dice.DeleteFavourite(ctx, id, a.ID), domain.ErrNotFound)
	favs, err = bs.Dice.Favourites(ctx, id)
	require.NoError(t, err)
	require.Len(t, favs, 1)
	assert.Equal(t, "d%", favs[0].Formula)
}

func TestDiceFavourites_ReorderPersistsAndNewOnesGoLast(t *testing.T) {
	bs, _ := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	a, err := bs.Dice.AddFavourite(ctx, id, "d4")
	require.NoError(t, err)
	b, err := bs.Dice.AddFavourite(ctx, id, "d6")
	require.NoError(t, err)

	require.NoError(t, bs.Dice.ReorderFavourites(ctx, id, []int64{b.ID, a.ID}))
	c, err := bs.Dice.AddFavourite(ctx, id, "d8")
	require.NoError(t, err)
	favs, err := bs.Dice.Favourites(ctx, id)
	require.NoError(t, err)
	require.Len(t, favs, 3)
	assert.Equal(t, []int64{b.ID, a.ID, c.ID}, []int64{favs[0].ID, favs[1].ID, favs[2].ID})

	// A refused order changes nothing.
	require.ErrorIs(t, bs.Dice.ReorderFavourites(ctx, id, []int64{a.ID, a.ID, c.ID}), domain.ErrInvalidInput)
	favs, err = bs.Dice.Favourites(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []int64{b.ID, a.ID, c.ID}, []int64{favs[0].ID, favs[1].ID, favs[2].ID})
}
