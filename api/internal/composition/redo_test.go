package composition_test

// ADR 0019: redo replays the newest popped transaction. Exercised on the real
// SQLite log with every restorer wired.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

func redoTarget(t *testing.T, bs *composition.BackingServices) *actionlog.Entry {
	t.Helper()
	e, err := bs.Log.RedoTarget(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	return e
}

// ADR 0019 steps 1 to 3: for every table-state mutation (the list of
// TestUndo_OneUndoWalksEveryMutationFullyBack), undo then redo gives the
// state exactly as after the mutation (every stack table, HP, the log as it
// reads) with its transaction on top again and nothing left to redo; undo
// after the redo pops it again. A restorer that cannot write its own "after"
// back fails here.
func TestRedo_UndoThenRedoGivesEveryMutationBack(t *testing.T) {
	for _, m := range stackMutations() {
		t.Run(m.name, func(t *testing.T) {
			bs := bootMaxDice(t)
			ctx := context.Background()
			_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -1, "scratch")
			require.NoError(t, err)
			if m.setup != nil {
				require.NoError(t, m.setup(ctx, bs))
			}
			before := tableStateOf(t, bs)
			require.Nil(t, redoTarget(t, bs), "nothing popped yet")

			require.NoError(t, m.run(ctx, bs))
			after := tableStateOf(t, bs)
			top := topID(t, bs)

			popped, err := bs.Log.Undo(ctx, bs.CharacterID, top)
			require.NoError(t, err)
			target := redoTarget(t, bs)
			require.NotNil(t, target, "the popped transaction can be redone")
			assert.Equal(t, popped.ID, target.ID, "the redo names the transaction the undo named")

			redone, err := bs.Log.Redo(ctx, bs.CharacterID, target.ID)
			require.NoError(t, err)
			assert.NotEqual(t, actionlog.StatusVoided, redone.Status())
			assert.Equal(t, after, tableStateOf(t, bs), "undo then redo gives the state as after the mutation")
			assert.Equal(t, top, topID(t, bs), "the transaction is on top again")
			assert.Nil(t, redoTarget(t, bs), "nothing left to redo")

			_, err = bs.Log.Undo(ctx, bs.CharacterID, top)
			require.NoError(t, err)
			assert.Equal(t, before, tableStateOf(t, bs), "undo after redo pops it again")
		})
	}
}

// ADR 0019 steps 1 and 2 on Next Round: undo then redo gives the round, the
// buffs and the initiative back as after Next Round, without rolling the
// initiative again (the scripted dice fail on any extra roll); undo twice
// then redo twice replays the two Next Rounds in order, and the initiative
// can be rerolled after it.
func TestNextRound_UndoRedoReplaysRoundBuffsAndInitiative(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 3})
	require.NoError(t, err)
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Bless", Duration: 2})
	require.NoError(t, err)
	type state struct {
		Round      int
		Buffs      []buffsvc.BuffEffect
		Initiative *buffsvc.Initiative
	}
	read := func() state {
		t.Helper()
		r, err := bs.Buffs.GetRound(ctx, id)
		require.NoError(t, err)
		buffs, err := bs.Buffs.ListActive(ctx, id)
		require.NoError(t, err)
		return state{Round: r, Buffs: buffs, Initiative: initiativeOf(t, bs)}
	}
	s0 := read()
	dice.expect(roll{"d10", 3}, roll{"d10", 8})
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	s1 := read()
	_, expired, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, []string{"Bless"}, names(expired))
	s2 := read()
	require.Equal(t, 8, s2.Initiative.D10)

	undo := func() {
		t.Helper()
		_, err := bs.Log.Undo(ctx, id, topID(t, bs))
		require.NoError(t, err)
	}
	redo := func() *actionlog.Entry {
		t.Helper()
		target := redoTarget(t, bs)
		require.NotNil(t, target)
		e, err := bs.Log.Redo(ctx, id, target.ID)
		require.NoError(t, err)
		return e
	}

	undo()
	require.Equal(t, s1, read())
	e := redo()
	assert.Equal(t, actionlog.KindRoundAdvance, e.Kind, "the redo names the Next Round, not its initiative")
	assert.Equal(t, s2, read(), "round, buffs (Bless expired again) and initiative as after Next Round")

	undo()
	undo()
	require.Equal(t, s0, read())
	redo()
	assert.Equal(t, s1, read(), "the first redo replays the first Next Round")
	redo()
	assert.Equal(t, s2, read(), "the second redo replays the second")
	assert.Nil(t, redoTarget(t, bs))

	// The replayed initiative names its entry again, so the reroll guard
	// (ADR 0018 step 5) sees the redone Next Round on top.
	dice.expect(roll{"d10", 5})
	rerolled, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 5, rerolled.D10)
}

// ADR 0019 step 2: a new transaction after an undo empties the redo stack:
// redo answers nothing to redo and changes nothing. Undoing that new
// transaction makes it, and only it, redoable.
func TestRedo_NewTransactionEmptiesTheRedoStack(t *testing.T) {
	bs := bootMaxDice(t)
	ctx, id := context.Background(), bs.CharacterID

	hit := func(d int, src string) {
		t.Helper()
		_, err := bs.HP.AppendEvent(ctx, id, d, src)
		require.NoError(t, err)
	}
	hit(-3, "rat")
	hit(-5, "orc")
	orc := topID(t, bs)
	hit(-7, "troll")
	troll := topID(t, bs)
	for range 2 {
		_, err := bs.Log.Undo(ctx, id, topID(t, bs))
		require.NoError(t, err)
	}
	require.Equal(t, orc, redoTarget(t, bs).ID)

	hit(-1, "bee")
	bee := topID(t, bs)
	assert.Nil(t, redoTarget(t, bs), "the push emptied the redo stack")
	state := tableStateOf(t, bs)
	_, err := bs.Log.Redo(ctx, id, orc)
	require.ErrorIs(t, err, actionlogsvc.ErrNothingToRedo)
	assert.Equal(t, state, tableStateOf(t, bs), "nothing changes")

	_, err = bs.Log.Undo(ctx, id, bee)
	require.NoError(t, err)
	require.Equal(t, bee, redoTarget(t, bs).ID, "only the new transaction is redoable")
	_, err = bs.Log.Redo(ctx, id, bee)
	require.NoError(t, err)
	assert.Equal(t, state, tableStateOf(t, bs))
	assert.Nil(t, redoTarget(t, bs), "the orc and the troll stay popped")
	_, err = bs.Log.Redo(ctx, id, troll)
	require.ErrorIs(t, err, actionlogsvc.ErrNothingToRedo)
}

// ADR 0019 step 3: a redo sends the target the client read. When another
// client redid (or undid) in between, the stale target is refused and
// nothing changes.
func TestRedo_StaleExpectedRedoIDChangesNothing(t *testing.T) {
	bs := bootMaxDice(t)
	ctx, id := context.Background(), bs.CharacterID

	_, err := bs.HP.AppendEvent(ctx, id, -3, "rat")
	require.NoError(t, err)
	rat := topID(t, bs)
	_, err = bs.HP.AppendEvent(ctx, id, -5, "orc")
	require.NoError(t, err)
	orc := topID(t, bs)
	for _, top := range []int64{orc, rat} {
		_, err := bs.Log.Undo(ctx, id, top)
		require.NoError(t, err)
	}
	read := redoTarget(t, bs).ID // both clients read the rat
	require.Equal(t, rat, read)

	_, err = bs.Log.Redo(ctx, id, orc)
	require.ErrorIs(t, err, actionlogsvc.ErrRedoChanged, "the orc is not the next redo")

	_, err = bs.Log.Redo(ctx, id, read) // client B
	require.NoError(t, err)
	state := tableStateOf(t, bs)
	_, err = bs.Log.Redo(ctx, id, read) // client A, stale
	require.ErrorIs(t, err, actionlogsvc.ErrRedoChanged)
	assert.Equal(t, state, tableStateOf(t, bs), "nothing changes")
	assert.Equal(t, orc, redoTarget(t, bs).ID)
}
