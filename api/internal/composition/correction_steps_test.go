package composition_test

// ADR 0010 step 6: a correction after the roll (target AC, save adjustment,
// GM verdict) is its own step on the action stack. Undo pops the newest
// correction first, giving the entry its previous payload back, then the
// entry. Exercised on the real SQLite log.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// entryByID reads entry id from the log (with its corrections).
func entryByID(t *testing.T, bs *composition.BackingServices, id int64) actionlog.Entry {
	t.Helper()
	entries, err := bs.Log.List(context.Background(), bs.CharacterID, 50)
	require.NoError(t, err)
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	require.FailNow(t, "entry not listed", "#%d", id)
	return actionlog.Entry{}
}

// attackView is the part of an attack payload the corrections change.
type attackView struct {
	TargetAC       *int   `json:"targetAc"`
	RolledTargetAC *int   `json:"rolledTargetAc"`
	ACKnown        *bool  `json:"acKnown"`
	Verdict        string `json:"verdict"`
}

func attackOf(t *testing.T, bs *composition.BackingServices, id int64) (attackView, actionlog.Status) {
	t.Helper()
	e := entryByID(t, bs, id)
	var v attackView
	require.NoError(t, json.Unmarshal(e.EffectivePayload(), &v))
	return v, e.Status()
}

// undoTop pops the top, which must be a correction step on entry target, and
// returns what it named.
func undoCorrection(t *testing.T, bs *composition.BackingServices, target int64) actionlog.CorrectionStep {
	t.Helper()
	popped, err := bs.Log.Undo(context.Background(), bs.CharacterID, topID(t, bs))
	require.NoError(t, err)
	require.Equal(t, actionlog.KindCorrection, popped.Kind)
	assert.Equal(t, actionlog.StatusVoided, popped.Status())
	var step actionlog.CorrectionStep
	require.NoError(t, json.Unmarshal(popped.Payload, &step))
	assert.Equal(t, target, step.EntryID)
	return step
}

// ADR 0010 step 6: correct the target AC twice, undo twice: the attack is
// back to the AC it was rolled against, still on the log and on top; one
// more undo pops the attack itself.
func TestCorrectionStep_TargetACTwiceUndoneTwice(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 7)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 9)
	require.NoError(t, err)
	v, status := attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 9, *v.TargetAC)
	assert.Equal(t, actionlog.StatusAmended, status)

	step := undoCorrection(t, bs, shot.LogEntryID)
	assert.Equal(t, actionlog.CorrectionStep{EntryID: shot.LogEntryID, EntryKind: actionlog.KindAttack, Field: combatsvc.ChangeTargetAC, From: 7.0, To: 9.0}, step)
	v, status = attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 7, *v.TargetAC)
	assert.Equal(t, 5, *v.RolledTargetAC)
	assert.Equal(t, actionlog.StatusAmended, status)

	step = undoCorrection(t, bs, shot.LogEntryID)
	assert.Equal(t, 5.0, step.From)
	v, status = attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 5, *v.TargetAC, "the rolled AC")
	assert.Nil(t, v.RolledTargetAC, "read as rolled")
	assert.Equal(t, actionlog.StatusActive, status, "no correction in effect")
	assert.Equal(t, shot.LogEntryID, topID(t, bs), "the attack is still there, on top")

	popped, err := bs.Log.Undo(ctx, id, shot.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindAttack, popped.Kind)
	_, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok)
}

// ADR 0010 step 6: adjust a save twice, undo twice: back to the save as
// rolled, still on the log and on top.
func TestCorrectionStep_SaveAdjustmentTwiceUndoneTwice(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 2, Reason: "ring"})
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: -3})
	require.NoError(t, err)

	type saveView struct {
		Adjustment       int    `json:"adjustment"`
		AdjustmentReason string `json:"adjustmentReason"`
		Passed           bool   `json:"passed"`
		RolledPassed     bool   `json:"rolledPassed"`
		PreviousPassed   *bool  `json:"previousPassed"`
	}
	saveOf := func() (saveView, actionlog.Status) {
		e := entryByID(t, bs, save.LogEntryID)
		var v saveView
		require.NoError(t, json.Unmarshal(e.EffectivePayload(), &v))
		return v, e.Status()
	}
	v, _ := saveOf()
	assert.Equal(t, -3, v.Adjustment)

	step := undoCorrection(t, bs, save.LogEntryID)
	assert.Equal(t, actionlog.CorrectionStep{EntryID: save.LogEntryID, EntryKind: actionlog.KindSave, Field: combatsvc.ChangeSaveAdjustment, From: 2.0, To: -3.0}, step)
	v, status := saveOf()
	assert.Equal(t, 2, v.Adjustment)
	assert.Equal(t, "ring", v.AdjustmentReason)
	assert.Equal(t, actionlog.StatusAmended, status)

	step = undoCorrection(t, bs, save.LogEntryID)
	assert.Equal(t, 0.0, step.From)
	v, status = saveOf()
	assert.Equal(t, 0, v.Adjustment, "as rolled")
	assert.Nil(t, v.PreviousPassed)
	assert.Equal(t, v.RolledPassed, v.Passed)
	assert.Equal(t, actionlog.StatusActive, status)
	assert.Equal(t, save.LogEntryID, topID(t, bs), "the save is still there, on top")

	// Still the latest roll: it can be adjusted again.
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 1})
	require.NoError(t, err)
}

// ADR 0010 step 6 with ADR 0012 steps 4 and 5: two GM verdicts then the AC
// given, undone one by one, walk back to the attack against an unknown AC;
// the arrow is spent once and the last undo pops the attack and gives it back.
func TestCorrectionStep_VerdictTwiceUndoneTwice(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	before := arrowCount(t, bs, "arrows_plus3")
	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3"})
	require.NoError(t, err)
	_, err = bs.Combat.SetAttackVerdict(ctx, id, shot.LogEntryID, true)
	require.NoError(t, err)
	_, err = bs.Combat.SetAttackVerdict(ctx, id, shot.LogEntryID, false)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 4)
	require.NoError(t, err)

	step := undoCorrection(t, bs, shot.LogEntryID)
	assert.Equal(t, actionlog.CorrectionStep{EntryID: shot.LogEntryID, EntryKind: actionlog.KindAttack, Field: combatsvc.ChangeTargetAC, From: nil, To: 4.0}, step)
	v, _ := attackOf(t, bs, shot.LogEntryID)
	assert.Nil(t, v.TargetAC, "the AC is unknown again")
	assert.Equal(t, "miss", v.Verdict)

	step = undoCorrection(t, bs, shot.LogEntryID)
	assert.Equal(t, actionlog.CorrectionStep{EntryID: shot.LogEntryID, EntryKind: actionlog.KindAttack, Field: combatsvc.ChangeVerdict, From: "hit", To: "miss"}, step)
	v, _ = attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, "hit", v.Verdict)

	step = undoCorrection(t, bs, shot.LogEntryID)
	assert.Nil(t, step.From)
	v, status := attackOf(t, bs, shot.LogEntryID)
	assert.Empty(t, v.Verdict, "no verdict, as rolled")
	assert.Equal(t, actionlog.StatusActive, status)
	assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"), "a correction gives no arrow back")
	assert.Equal(t, shot.LogEntryID, topID(t, bs))

	_, err = bs.Log.Undo(ctx, id, shot.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, before, arrowCount(t, bs, "arrows_plus3"))
}

// ADR 0010 step 3 holds for corrections: a correction that lands after the
// client read the top makes its undo stale, and nothing changes.
func TestCorrectionStep_IsTheExpectedTop(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	seen := topID(t, bs)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 2)
	require.NoError(t, err)

	_, err = bs.Log.Undo(ctx, id, seen)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	v, status := attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 2, *v.TargetAC, "the correction stays")
	assert.Equal(t, actionlog.StatusAmended, status)
}

// Migration 023: amends written before correction steps (step_id NULL), on an
// attack and on a save, still give their corrected payload, sit under any new
// correction step, and pop only with their roll (ADR 0010 step 6, older amends).
func TestCorrectionStep_PreStepAmendsPopWithTheirRoll(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	legacyAmend := func(entryID int64, edit func(map[string]any)) {
		t.Helper()
		var p map[string]any
		require.NoError(t, json.Unmarshal(entryByID(t, bs, entryID).Payload, &p))
		edit(p)
		raw, err := json.Marshal(p)
		require.NoError(t, err)
		_, err = bs.DB.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind, payload) VALUES (?, 'amend', ?)`, entryID, string(raw))
		require.NoError(t, err)
	}

	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	legacyAmend(save.LogEntryID, func(p map[string]any) { p["adjustment"] = 4 })
	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	legacyAmend(shot.LogEntryID, func(p map[string]any) { p["targetAc"] = 9; p["rolledTargetAc"] = 5 })

	e := entryByID(t, bs, save.LogEntryID)
	assert.Equal(t, actionlog.StatusAmended, e.Status())
	assert.JSONEq(t, "4", string(mustField(t, e.EffectivePayload(), "adjustment")))
	v, status := attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 9, *v.TargetAC)
	assert.Equal(t, actionlog.StatusAmended, status)

	// A new correction stacks above the old amend; popping it gives the old amend back.
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 2)
	require.NoError(t, err)
	undoCorrection(t, bs, shot.LogEntryID)
	v, _ = attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 9, *v.TargetAC, "the pre-step amend is still in effect")

	// The old amends are no steps: the next undo pops the attack, then the save.
	for _, want := range []int64{shot.LogEntryID, save.LogEntryID} {
		popped, err := bs.Log.Undo(ctx, id, topID(t, bs))
		require.NoError(t, err)
		assert.Equal(t, want, popped.ID)
		assert.Equal(t, actionlog.StatusVoided, popped.Status())
	}
	_, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok)
}

func mustField(t *testing.T, payload json.RawMessage, key string) json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &m))
	return m[key]
}

// A save adjusted, then an attack whose AC is corrected: the attack is the
// latest roll (the save can no longer be adjusted), the top is the AC
// correction naming the attack, and an undo still expecting the save's
// correction is stale (ADR 0006 step 6, ADR 0010 steps 3 and 6).
func TestCorrectionStep_SaveThenAttackCorrection(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID
	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 2})
	require.NoError(t, err)
	saveStep := topID(t, bs)

	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 3)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 1})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)

	top := entryByID(t, bs, topID(t, bs))
	require.Equal(t, actionlog.KindCorrection, top.Kind)
	var step actionlog.CorrectionStep
	require.NoError(t, json.Unmarshal(top.Payload, &step))
	assert.Equal(t, shot.LogEntryID, step.EntryID)
	assert.Equal(t, actionlog.KindAttack, step.EntryKind)

	_, err = bs.Log.Undo(ctx, id, saveStep)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	v, _ := attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 3, *v.TargetAC, "nothing popped")
	assert.JSONEq(t, "2", string(mustField(t, entryByID(t, bs, save.LogEntryID).EffectivePayload(), "adjustment")))

	undoCorrection(t, bs, shot.LogEntryID)
	v, _ = attackOf(t, bs, shot.LogEntryID)
	assert.Equal(t, 5, *v.TargetAC)
}
