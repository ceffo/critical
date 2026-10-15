package composition_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	first, err := bs.Skills.Roll(ctx, id, "musical_instrument")
	require.NoError(t, err)
	assert.True(t, first.Success)
	second, err := bs.Skills.Roll(ctx, id, "musical_instrument")
	require.NoError(t, err)
	assert.False(t, second.Success)
	climb, err := bs.Skills.Roll(ctx, id, "climb_walls")
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
	assert.Equal(t, &skillsvc.LastCheck{EntryID: first.LogEntryID, Roll: 9, Target: 16, Success: true},
		lastOf(t, skills, "musical_instrument"), "the check before the popped one comes back")
	assert.Equal(t, first.LogEntryID, latest(t, bs).ID)
}
