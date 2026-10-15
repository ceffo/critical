package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

func newDiceBacking(t *testing.T) (*composition.BackingServices, *scriptedDice) {
	t.Helper()
	dice := newScriptedDice(t)
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: filepath.Join("..", "..", "testdata", "seeds"),
		Dice:     dice,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	return bs, dice
}

func initiativeOf(t *testing.T, bs *composition.BackingServices) *buffsvc.Initiative {
	t.Helper()
	i, err := bs.Buffs.GetInitiative(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	return i
}

// latest is the newest non-voided entry (the one an undo pops).
func latest(t *testing.T, bs *composition.BackingServices) actionlog.Entry {
	t.Helper()
	entries, err := bs.Log.List(context.Background(), bs.CharacterID, 20)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Status() != actionlog.StatusVoided {
			return e
		}
	}
	require.FailNow(t, "no active entry")
	return actionlog.Entry{}
}

// ADR 0018 steps 1 and 2: Next Round records the advance and the new round's
// initiative as one transaction, so one undo walks back to the previous round
// with its buffs (the expired one back, the durations up again) and its
// initiative. Fails on one-entry pops: the first undo would leave the round
// advanced with no initiative.
func TestNextRound_OneUndoRestoresRoundBuffsAndInitiative(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 3})
	require.NoError(t, err)
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Bless", Duration: 2})
	require.NoError(t, err)
	dice.expect(roll{"d10", 3}, roll{"d10", 8})
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	beforeRound, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	beforeBuffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	beforeInitiative := initiativeOf(t, bs)
	require.NotNil(t, beforeInitiative)
	require.Equal(t, 3, beforeInitiative.D10)

	_, expired, round, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 2, round)
	require.Equal(t, []string{"Bless"}, names(expired))
	require.Equal(t, 8, initiativeOf(t, bs).D10)

	top := latest(t, bs)
	require.Equal(t, actionlog.KindInitiative, top.Kind, "the initiative is on top")
	popped, err := bs.Log.Undo(ctx, id, top.ID)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindRoundAdvance, popped.Kind, "the undo names the Next Round")
	assert.Equal(t, popped.ID, top.TransactionID)

	r, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, beforeRound, r, "the round is back")
	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, beforeBuffs, buffs, "Bless is back, durations up again")
	assert.Equal(t, beforeInitiative, initiativeOf(t, bs), "the previous round's initiative is back")
	assert.Equal(t, actionlog.KindInitiative, latest(t, bs).Kind, "the previous Next Round is on top again")
}

// ADR 0018 step 5: a reroll is its own transaction. Undo right after it gives
// back the roll it replaced (the Next Round's); undo again pops the Next
// Round (the round and initiative before it come back).
func TestInitiative_RerollThenUndoThenUndo(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d10", 3}, roll{"d10", 6}, roll{"d10", 2})
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	roundOne := initiativeOf(t, bs)
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	nextRound := initiativeOf(t, bs)
	require.Equal(t, 6, nextRound.D10)

	rerolled, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, rerolled.D10)

	popped, err := bs.Log.Undo(ctx, id, topID(t, bs))
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindInitiative, popped.Kind, "the first undo pops the reroll alone")
	assert.Equal(t, nextRound, initiativeOf(t, bs), "the Next Round's roll is back")
	r, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, r, "the round stays")

	popped, err = bs.Log.Undo(ctx, id, topID(t, bs))
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindRoundAdvance, popped.Kind, "the second undo pops the Next Round")
	r, err = bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, r)
	assert.Equal(t, roundOne, initiativeOf(t, bs), "with the round's initiative before it")
}

// A reroll replaces the newest initiative without voiding it, and is refused
// (409) once another transaction is newer (ADR 0013 step 4, ADR 0018 step 5).
func TestInitiative_RerollOnTheRealLog(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d10", 6}, roll{"d10", 2}, roll{"d10", 7})
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	old := initiativeOf(t, bs)

	got, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, got.D10)
	assert.Equal(t, &got, initiativeOf(t, bs))
	assert.NotEqual(t, old.LogEntryID, got.LogEntryID)
	entries, err := bs.Log.List(ctx, id, 3)
	require.NoError(t, err)
	require.Len(t, entries, 3, "Next Round (advance, first roll), the reroll")
	assert.Equal(t, got.LogEntryID, entries[0].ID)
	assert.Equal(t, entries[0].ID, entries[0].TransactionID, "the reroll is its own transaction")
	assert.Equal(t, actionlog.StatusActive, entries[1].Status(), "the first roll is not voided")

	// A second reroll replaces the first one's roll.
	again, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 7, again.D10)

	dice.expect(roll{"d10", 5})
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	_, err = bs.HP.AppendEvent(ctx, id, -3, "orc")
	require.NoError(t, err)
	_, err = bs.Buffs.RerollInitiative(ctx, id)
	assert.ErrorIs(t, err, domain.ErrConflict)
	assert.ErrorIs(t, err, buffsvc.ErrInitiativeNotOnTop, "another transaction is newer")
	assert.Equal(t, 5, initiativeOf(t, bs).D10, "nothing changed")
}

// End combat and passing time end the initiative; voiding them brings it
// back; nothing to reroll at round 0; Set round 0 -> 3 rolls none (ADR 0013
// step 5).
func TestInitiative_OutOfCombatOnTheRealLog(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d10", 4})
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	before := initiativeOf(t, bs)

	_, _, err = bs.Buffs.ResetRound(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, initiativeOf(t, bs))
	_, err = bs.Buffs.RerollInitiative(ctx, id)
	assert.ErrorIs(t, err, domain.ErrConflict)
	assert.ErrorIs(t, err, buffsvc.ErrNoInitiative)

	_, err = bs.Log.Void(ctx, id, latest(t, bs).ID)
	require.NoError(t, err)
	assert.Equal(t, before, initiativeOf(t, bs))

	// Time passes, then undo: the same.
	_, _, _, _, err = bs.Buffs.PassTime(ctx, id, 1)
	require.NoError(t, err)
	assert.Nil(t, initiativeOf(t, bs))
	_, err = bs.Log.Void(ctx, id, latest(t, bs).ID)
	require.NoError(t, err)
	assert.Equal(t, before, initiativeOf(t, bs))

	// Set round from 0 to 3 rolls nothing: no initiative until Next Round.
	_, _, err = bs.Buffs.ResetRound(ctx, id)
	require.NoError(t, err)
	_, _, err = bs.Buffs.SetRound(ctx, id, 3)
	require.NoError(t, err)
	assert.Nil(t, initiativeOf(t, bs))
}
