package composition_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An arrow shot at an unknown AC is spent once, whatever the GM's verdicts
// and the AC given after (docs/adr/0012 steps 4 and 5, ADR 0002 step 10);
// undo pops each of them, then the attack, which gives the arrow back.
func TestUnknownAC_AmmoSpentOnceAndUndoPopsTheAttack(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	before := arrowCount(t, bs, "arrows_plus3")

	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3"})
	require.NoError(t, err)
	assert.False(t, shot.ACKnown)
	assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"))

	_, err = bs.Combat.SetAttackVerdict(ctx, id, shot.LogEntryID, true)
	require.NoError(t, err)
	_, err = bs.Combat.SetAttackVerdict(ctx, id, shot.LogEntryID, false)
	require.NoError(t, err)
	given, err := bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 4)
	require.NoError(t, err)
	assert.True(t, given.ACKnown)
	assert.Equal(t, combatsvc.VerdictNone, given.Verdict)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 6)
	require.NoError(t, err)
	assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"), "one arrow, whatever was said after")

	// Each verdict and correction is its own step above the attack (ADR 0010 step 6).
	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 5)
	for _, e := range entries[:4] {
		assert.Equal(t, actionlog.KindCorrection, e.Kind)
	}
	assert.Equal(t, actionlog.KindAttack, entries[4].Kind)

	// ADR 0010: undo pops the four steps, then the attack, which gives the arrow back.
	for range 4 {
		_, err = bs.Log.Undo(ctx, id, topID(t, bs))
		require.NoError(t, err)
		assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"))
	}
	popped, err := bs.Log.Undo(ctx, id, topID(t, bs))
	require.NoError(t, err)
	assert.Equal(t, shot.LogEntryID, popped.ID)
	assert.Equal(t, before, arrowCount(t, bs, "arrows_plus3"))
}

// A verdict, like an AC correction, is only for the latest roll, never a
// popped one; a popped roll still counts as newer (the actionlog rule for
// every amend), so its predecessor does not become correctable again.
func TestUnknownAC_VerdictOnlyOnTheLatestAttack(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	sword := combatsvc.AttackRequest{WeaponID: "longsword"}

	first, err := bs.Combat.ResolveAttack(ctx, id, sword)
	require.NoError(t, err)
	second, err := bs.Combat.ResolveAttack(ctx, id, sword)
	require.NoError(t, err)

	_, err = bs.Combat.SetAttackVerdict(ctx, id, first.LogEntryID, true)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, first.LogEntryID, 5)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)

	_, err = bs.Log.Undo(ctx, id, second.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Combat.SetAttackVerdict(ctx, id, second.LogEntryID, true)
	require.ErrorIs(t, err, actionlogsvc.ErrAlreadyVoided)
	_, err = bs.Combat.SetAttackVerdict(ctx, id, first.LogEntryID, true)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
}

// The attack form starts with an unknown AC, keeps one given, and forgets it
// again ("Inconnue"), through the stored form.
func TestAttackForm_UnknownTargetACPersists(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	v, err := bs.Combat.GetAttackForm(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, v.Form.TargetAC, "unknown by default")
	assert.Nil(t, v.Preview.RollNeeded)

	v, err = bs.Combat.UpdateAttackForm(ctx, id, combatsvc.AttackFormPatch{TargetAC: new(-1)})
	require.NoError(t, err)
	assert.Equal(t, new(-1), v.Form.TargetAC)
	require.NotNil(t, v.Preview.RollNeeded)
	assert.Equal(t, v.Preview.EffectiveThac0+1, *v.Preview.RollNeeded)

	_, err = bs.Combat.UpdateAttackForm(ctx, id, combatsvc.AttackFormPatch{UnknownTargetAC: true})
	require.NoError(t, err)
	v, err = bs.Combat.GetAttackForm(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, v.Form.TargetAC, "forgotten, not AC 0")
}
