package composition_test

// ADR 0010: undo pops the top of the action stack, any kind, against the top
// the client saw. Exercised on the real SQLite log with every restorer wired.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

type undoState struct {
	Stack      stackState
	Prepared   []string
	Wand       int
	Round      int
	Initiative *buffsvc.Initiative
}

func undoStateOf(t *testing.T, bs *composition.BackingServices) undoState {
	t.Helper()
	round, err := bs.Buffs.GetRound(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	slots, err := bs.Spells.GetSlots(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	prepared := []string{}
	for _, s := range slots {
		prepared = append(prepared, s.SpellID)
	}
	return undoState{Stack: stateOf(t, bs), Prepared: prepared, Wand: itemCount(t, bs, "wand_lightning"), Round: round, Initiative: initiativeOf(t, bs)}
}

func topID(t *testing.T, bs *composition.BackingServices) int64 {
	t.Helper()
	id, ok, err := bs.Log.TopID(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	require.True(t, ok, "something to pop")
	return id
}

// buildUndoStack logs one entry of each state-changing kind: spell
// preparation, a cast (slot and Stoneskin), Haste, a bow shot (one +3 arrow),
// damage, a wand charge and a Next Round (the advance and its initiative, one
// transaction, ADR 0018 step 1). It returns each transaction's first kind in
// order and the state before the first transaction and after each.
func buildUndoStack(t *testing.T) (*composition.BackingServices, []actionlog.Kind, []undoState) {
	t.Helper()
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	states := []undoState{undoStateOf(t, bs)}
	var kinds []actionlog.Kind
	step := func(err error, k ...actionlog.Kind) {
		t.Helper()
		require.NoError(t, err)
		kinds = append(kinds, k...)
		states = append(states, undoStateOf(t, bs))
	}
	_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin"})
	step(err, actionlog.KindSpellPrep)
	_, err = bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	step(err, actionlog.KindCast)
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 3})
	step(err, actionlog.KindBuffChange)
	_, err = bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: new(5)})
	step(err, actionlog.KindAttack)
	_, err = bs.HP.AppendEvent(ctx, id, -7, "troll claw")
	step(err, actionlog.KindHPChange)
	_, err = bs.Items.UseItem(ctx, id, "wand_lightning")
	step(err, actionlog.KindItemUse)
	// One call, two entries, one transaction: one step.
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	step(err, actionlog.KindRoundAdvance)
	require.NotNil(t, states[len(states)-1].Initiative)

	for i := 1; i < len(states); i++ {
		require.NotEqual(t, states[i-1], states[i], "%s changed no state", kinds[i-1])
	}
	return bs, kinds, states
}

// ADR 0010 step 2, ADR 0018 step 2: undo with the expected top pops the
// newest transaction, any kind, and reverts its whole effect (HP, buffs,
// round, initiative, ammo, slots, item charges): Next Round pops in one step,
// its initiative with it. Popping in order walks back every step, then there
// is nothing left to undo.
func TestUndo_PopsEveryKindInOrder(t *testing.T) {
	bs, kinds, states := buildUndoStack(t)
	ctx := context.Background()
	for i := len(kinds) - 1; i >= 0; i-- {
		e, err := bs.Log.Undo(ctx, bs.CharacterID, topID(t, bs))
		require.NoError(t, err, "undo %s", kinds[i])
		assert.Equal(t, kinds[i], e.Kind, "the undo answers with the transaction's first entry")
		assert.Equal(t, actionlog.StatusVoided, e.Status())
		assert.Equal(t, states[i], undoStateOf(t, bs), "state after undoing %s", kinds[i])
	}
	_, ok, err := bs.Log.TopID(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.False(t, ok, "the stack is empty")
	_, err = bs.Log.Undo(ctx, bs.CharacterID, 1)
	require.ErrorIs(t, err, actionlogsvc.ErrNothingToUndo)
	assert.Equal(t, states[0], undoStateOf(t, bs))
}

// ADR 0010 step 3: a stale expected top (another action landed since the
// client read the log) is refused and nothing changes: no state, no entry.
func TestUndo_StaleExpectedTopChangesNothing(t *testing.T) {
	bs, _, _ := buildUndoStack(t)
	ctx := context.Background()
	seen := topID(t, bs)
	_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -3, "goblin")
	require.NoError(t, err)

	before := undoStateOf(t, bs)
	entriesBefore, err := bs.Log.List(ctx, bs.CharacterID, 50)
	require.NoError(t, err)

	_, err = bs.Log.Undo(ctx, bs.CharacterID, seen)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)

	assert.Equal(t, before, undoStateOf(t, bs))
	entriesAfter, err := bs.Log.List(ctx, bs.CharacterID, 50)
	require.NoError(t, err)
	assert.Equal(t, entriesBefore, entriesAfter, "no correction written")

	// Two clients undoing the same top: the second is refused.
	top := topID(t, bs)
	_, err = bs.Log.Undo(ctx, bs.CharacterID, top)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, bs.CharacterID, top)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
}
