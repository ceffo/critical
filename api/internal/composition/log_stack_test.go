package composition_test

// The action log is a stack with note-only amends. User,
// 2026-10-14: "no amending values. just the note.. can only pop elements",
// for every entry kind. Exercised on a cast (spell slot + Stoneskin), a buff
// (Haste), a bow shot (one +3 arrow) and damage.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

type stackState struct {
	HP, Slots, Arrows int
	Buffs             []string
}

func stateOf(t *testing.T, bs *composition.BackingServices) stackState {
	t.Helper()
	ctx := context.Background()
	hp, err := bs.HP.GetState(ctx, bs.CharacterID)
	require.NoError(t, err)
	buffs, err := bs.Buffs.ListActive(ctx, bs.CharacterID)
	require.NoError(t, err)
	return stackState{HP: hp.CurrentHp, Slots: usedSlots(t, bs), Arrows: arrowCount(t, bs, "arrows_plus3"), Buffs: names(buffs)}
}

func topEntry(t *testing.T, bs *composition.BackingServices) actionlog.Entry {
	t.Helper()
	entries, err := bs.Log.List(context.Background(), bs.CharacterID, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return entries[0]
}

// buildStack logs a cast, a buff, a bow shot and damage. It returns the
// entry ids oldest first and the state before the first entry and after each.
func buildStack(t *testing.T) (*composition.BackingServices, []int64, []stackState) {
	t.Helper()
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin"})
	require.NoError(t, err)

	states := []stackState{stateOf(t, bs)}
	var ids []int64
	step := func(entryID int64) {
		ids = append(ids, entryID)
		states = append(states, stateOf(t, bs))
	}
	cast, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	step(cast.EntryID)
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 3})
	require.NoError(t, err)
	step(topEntry(t, bs).ID)
	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: new(5)})
	require.NoError(t, err)
	step(shot.LogEntryID)
	_, err = bs.HP.AppendEvent(ctx, id, -7, "troll claw")
	require.NoError(t, err)
	step(topEntry(t, bs).ID)

	for i := 1; i < len(states); i++ {
		require.NotEqual(t, states[i-1], states[i], "entry %d changed no state", i)
	}
	return bs, ids, states
}

// Void latest OK, for every kind: the top entry pops and its state comes back.
// Pop all four in order: each pop restores exactly the state before that entry.
func TestLogStack_PopInOrderRestoresEachStep(t *testing.T) {
	bs, ids, states := buildStack(t)
	ctx := context.Background()
	for i := len(ids) - 1; i >= 0; i-- {
		e, err := bs.Log.Void(ctx, bs.CharacterID, ids[i])
		require.NoError(t, err, "pop entry %d", i)
		assert.Equal(t, actionlog.StatusVoided, e.Status())
		assert.Equal(t, states[i], stateOf(t, bs), "state after popping entry %d", i)
	}
}

// Void older rejected, for every kind: only the top may go, nothing changes.
func TestLogStack_VoidOlderEntryIsRefused(t *testing.T) {
	bs, ids, states := buildStack(t)
	ctx := context.Background()
	for i, id := range ids[:len(ids)-1] {
		_, err := bs.Log.Void(ctx, bs.CharacterID, id)
		require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "entry %d", i)
		assert.Contains(t, err.Error(), fmt.Sprintf("void #%d (hp_change", ids[len(ids)-1]))
	}
	assert.Equal(t, states[len(states)-1], stateOf(t, bs), "nothing restored")
	entries, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, actionlog.StatusVoided, e.Status())
	}
}

// Amend note OK, for every kind: the note changes, values and state don't.
func TestLogStack_AmendNoteOnly(t *testing.T) {
	bs, ids, states := buildStack(t)
	ctx := context.Background()
	before, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	for i, id := range ids {
		note := "DM note " + string(rune('A'+i))
		e, err := bs.Log.Amend(ctx, bs.CharacterID, id, actionlogsvc.Amendment{Note: note})
		require.NoError(t, err)
		assert.Equal(t, note, e.Note())
		assert.Equal(t, actionlog.StatusActive, e.Status(), "a note is not a value amend")
	}
	assert.Equal(t, states[len(states)-1], stateOf(t, bs))
	after, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	require.Len(t, after, len(before))
	for i := range before {
		assert.Equal(t, before[i].EffectiveHPDelta(), after[i].EffectiveHPDelta())
		assert.Equal(t, before[i].EffectiveSummary(), after[i].EffectiveSummary())
		assert.JSONEq(t, string(before[i].EffectivePayload()), string(after[i].EffectivePayload()))
	}
}

// Checklist 4: a note on a voided entry is refused
// (ErrAlreadyVoided, 409) and a note over 500 characters too (ErrInvalidInput,
// 400); exactly 500 is fine. Status mapping: router_errors_test.
func TestLogStack_NoteRefusals(t *testing.T) {
	bs, ids, _ := buildStack(t)
	ctx := context.Background()
	top := ids[len(ids)-1]

	_, err := bs.Log.Amend(ctx, bs.CharacterID, top, actionlogsvc.Amendment{Note: strings.Repeat("é", 501)})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	e, err := bs.Log.Amend(ctx, bs.CharacterID, top, actionlogsvc.Amendment{Note: strings.Repeat("é", 500)})
	require.NoError(t, err)
	assert.Len(t, []rune(e.Note()), 500)

	_, err = bs.Log.Void(ctx, bs.CharacterID, top)
	require.NoError(t, err)
	_, err = bs.Log.Amend(ctx, bs.CharacterID, top, actionlogsvc.Amendment{Note: "too late"})
	require.ErrorIs(t, err, actionlogsvc.ErrAlreadyVoided)
	e, err = bs.Log.Amend(ctx, bs.CharacterID, ids[0], actionlogsvc.Amendment{Note: "older entries still take a note"})
	require.NoError(t, err)
	assert.Equal(t, "older entries still take a note", e.Note())
}
