package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

func lastOf(t *testing.T, s *skillsvc.Skills, id string) *skillsvc.LastCheck {
	t.Helper()
	for _, p := range s.Proficiencies {
		if p.ID == id {
			return p.Last
		}
	}
	for _, b := range s.BardSkills {
		if b.ID == id {
			return b.Last
		}
	}
	require.FailNow(t, "unknown skill", id)
	return nil
}

// ADR 0014 step 4 on the real stack (fixture Bard, DEX 17): each check is the
// top of the action stack, its row shows it as the last check, and undo pops
// it (nothing to restore), the row falling back to the check before it.
func TestSkillCheck_UndoPopsTheCheck(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	skills, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	require.Len(t, skills.Proficiencies, 8)
	require.Len(t, skills.BardSkills, 4)
	assert.Equal(t, 16, skills.Proficiencies[0].Target, "musical instrument: DEX 17 - 1")
	assert.Nil(t, lastOf(t, skills, "musical_instrument"))

	dice.expect(roll{"d20", 9}, roll{"d20", 17}, roll{"d100", 86})
	first, err := bs.Skills.Roll(ctx, id, "musical_instrument", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.True(t, first.Success)
	second, err := bs.Skills.Roll(ctx, id, "musical_instrument", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.False(t, second.Success)
	climb, err := bs.Skills.Roll(ctx, id, "climb_walls", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.False(t, climb.Success, "86 over 85")

	top := latest(t, bs)
	assert.Equal(t, climb.LogEntryID, top.ID)
	assert.Equal(t, actionlog.KindSkillCheck, top.Kind)

	_, err = bs.Log.Undo(ctx, id, climb.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, second.LogEntryID)
	require.NoError(t, err)

	skills, err = bs.Skills.List(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, lastOf(t, skills, "climb_walls"), "the popped check is gone")
	assert.Equal(t, &skillsvc.LastCheck{EntryID: first.LogEntryID, Roll: 9, Target: 16, AdjustedTarget: 16, Success: true},
		lastOf(t, skills, "musical_instrument"), "the check before the popped one comes back")
	assert.Equal(t, first.LogEntryID, latest(t, bs).ID)
}

// A skill check is a roll kind (ADR 0014, ADR 0006 step 6): after an attack at
// an unknown AC, a check ends the attack's corrections (verdict and AC are
// refused, 409 via ErrNotLatest), and the check is the latest result.
func TestSkillCheck_EndsTheCorrectionOfAnUnknownACAttack(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	shot, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword"})
	require.NoError(t, err)
	require.False(t, shot.ACKnown)
	check, err := bs.Skills.Roll(ctx, id, "tumbling", skillsvc.Adjustment{})
	require.NoError(t, err)

	_, err = bs.Combat.SetAttackVerdict(ctx, id, shot.LogEntryID, true)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Combat.AdjustAttackAC(ctx, id, shot.LogEntryID, 5)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	assert.Equal(t, check.LogEntryID, latest(t, bs).ID)
}

// Save, then a skill check: the save can no longer be adjusted. Popping the
// check does not make the save adjustable again: a voided roll still counts
// as the newest (ADR 0006 step 6, as for every roll kind).
func TestSkillCheck_EndsTheAdjustmentOfASave(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	check, err := bs.Skills.Roll(ctx, id, "climb_walls", skillsvc.Adjustment{})
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 4})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)

	_, err = bs.Log.Undo(ctx, id, check.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 4})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "the popped check still counts as the newest roll")
}

// ADR 0014 step 7 on the real stack (fixture Bard, DEX 17, musical
// instrument 16): each post-roll adjustment is its own correction step on
// top of the check; undo pops the newest one and the check reads the
// adjustment before it, down to the pre-roll one, the roll never changing.
// The row's last check follows.
func TestSkillCheck_Step7_UndoRestoresThePreviousAdjustment(t *testing.T) {
	bs, dice := newDiceBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	dice.expect(roll{"d20", 15})
	check, err := bs.Skills.Roll(ctx, id, "musical_instrument", skillsvc.Adjustment{Value: -2, Reason: "slippery"})
	require.NoError(t, err)
	assert.Equal(t, 14, check.AdjustedTarget)
	assert.False(t, check.Success, "15 over 16 - 2")

	first, err := bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: 1})
	require.NoError(t, err)
	assert.True(t, first.Success)
	second, err := bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: -5, Reason: "storm"})
	require.NoError(t, err)
	assert.False(t, second.Success)
	assert.Equal(t, 15, second.Roll)

	lastCheck := func() *skillsvc.LastCheck {
		skills, err := bs.Skills.List(ctx, id)
		require.NoError(t, err)
		return lastOf(t, skills, "musical_instrument")
	}
	assert.Equal(t, &skillsvc.LastCheck{EntryID: check.LogEntryID, Roll: 15, Target: 16, Adjustment: -5, AdjustedTarget: 11},
		lastCheck())

	top := latest(t, bs)
	require.Equal(t, actionlog.KindCorrection, top.Kind, "the adjustment is the top of the stack")
	_, err = bs.Log.Undo(ctx, id, top.ID)
	require.NoError(t, err)
	assert.Equal(t, &skillsvc.LastCheck{EntryID: check.LogEntryID, Roll: 15, Target: 16, Adjustment: 1, AdjustedTarget: 17, Success: true},
		lastCheck(), "the previous adjustment is back")

	top = latest(t, bs)
	require.Equal(t, actionlog.KindCorrection, top.Kind)
	_, err = bs.Log.Undo(ctx, id, top.ID)
	require.NoError(t, err)
	assert.Equal(t, &skillsvc.LastCheck{EntryID: check.LogEntryID, Roll: 15, Target: 16, Adjustment: -2, AdjustedTarget: 14},
		lastCheck(), "the check as rolled, pre-roll adjustment included")
	assert.Equal(t, check.LogEntryID, latest(t, bs).ID)

	again, err := bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: 0})
	require.NoError(t, err)
	assert.True(t, again.Success, "still the latest roll: it can be adjusted again")
	assert.False(t, again.RolledSuccess)
}

// ADR 0014 step 6 on the real stack: a check rolled before a save can no
// longer be adjusted (409), and a save is not a skill check (400).
func TestSkillCheck_Step6_OnlyTheLatestRollIsAdjusted(t *testing.T) {
	bs := newBacking(t)
	ctx, id := context.Background(), bs.CharacterID

	check, err := bs.Skills.Roll(ctx, id, "climb_walls", skillsvc.Adjustment{})
	require.NoError(t, err)
	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)

	_, err = bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: 10})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Skills.Adjust(ctx, id, save.LogEntryID, skillsvc.Adjustment{Value: 10})
	require.ErrorIs(t, err, actionlogsvc.ErrNotAmendable)

	_, err = bs.Log.Undo(ctx, id, save.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: 10})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "the popped save still counts as the newest roll")
}
