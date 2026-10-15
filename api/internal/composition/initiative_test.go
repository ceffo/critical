package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Undo after Next Round removes the initiative, a second undo the advance;
// the previous round's initiative comes back with it (ADR 0013 step 2, ADR
// 0005 step 4).
func TestInitiative_UndoPopsTheInitiativeThenTheAdvance(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d10", 3}, roll{"d10", 8})
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	first := initiativeOf(t, bs)
	require.NotNil(t, first)
	assert.Equal(t, 3, first.D10)
	_, _, round, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 2, round)
	second := initiativeOf(t, bs)
	require.NotNil(t, second)
	assert.Equal(t, 8, second.D10)

	top := latest(t, bs)
	assert.Equal(t, actionlog.KindInitiative, top.Kind)
	assert.Equal(t, second.LogEntryID, top.ID)
	assert.Equal(t, 2, top.Round)
	assert.JSONEq(t, `{"round":2,"d10":8}`, string(top.EffectivePayload()))

	_, err = bs.Log.Void(ctx, id, top.ID)
	require.NoError(t, err)
	assert.Nil(t, initiativeOf(t, bs), "the first undo removes the initiative")
	r, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, r, "the round stays")

	advance := latest(t, bs)
	require.Equal(t, actionlog.KindRoundAdvance, advance.Kind)
	_, err = bs.Log.Void(ctx, id, advance.ID)
	require.NoError(t, err)
	r, err = bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, r, "the second undo the advance")
	assert.Equal(t, first, initiativeOf(t, bs), "with the round's initiative back")
}

// A reroll replaces the newest initiative entry, and is refused (409) once
// another entry is newer (ADR 0013 step 4).
func TestInitiative_RerollOnTheRealLog(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d10", 6}, roll{"d10", 2})
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	old := initiativeOf(t, bs)

	got, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, got.D10)
	assert.Equal(t, &got, initiativeOf(t, bs))
	assert.NotEqual(t, old.LogEntryID, got.LogEntryID)
	oldEntry, err := bs.Log.List(ctx, id, 3)
	require.NoError(t, err)
	require.Len(t, oldEntry, 3, "Next Round, the first roll (voided), the reroll")
	assert.Equal(t, got.LogEntryID, oldEntry[0].ID)
	assert.Equal(t, actionlog.StatusVoided, oldEntry[1].Status(), "the first roll is popped")

	// Undo the reroll: no initiative, the advance is next.
	_, err = bs.Log.Void(ctx, id, got.LogEntryID)
	require.NoError(t, err)
	assert.Nil(t, initiativeOf(t, bs))

	dice.expect(roll{"d10", 5})
	_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	_, err = bs.HP.AppendEvent(ctx, id, -3, "orc")
	require.NoError(t, err)
	_, err = bs.Buffs.RerollInitiative(ctx, id)
	assert.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "another entry is newer")
	assert.Equal(t, 5, initiativeOf(t, bs).D10, "nothing changed")
}

// End combat ends the initiative; voiding it brings it back; nothing to
// reroll at round 0 (ADR 0013 step 5).
func TestInitiative_EndCombat(t *testing.T) {
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

	_, err = bs.Log.Void(ctx, id, latest(t, bs).ID)
	require.NoError(t, err)
	assert.Equal(t, before, initiativeOf(t, bs))
}
