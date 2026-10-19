package skillsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
)

// A thief whose character service derived two thieving skills (ADR 0030
// step 17); it has no proficiency and no bard skill.
func thiefFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{DAL: f.dal, Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	f.dal.EXPECT().ListProficiencies(mock.Anything, charID).Return(nil, nil).Maybe()
	thief := &charactersvc.Character{ID: charID, Class: "thief", Dex: 17}
	thief.Derived.ThiefSkills = []ruletablesvc.SkillPercent{
		{Skill: "open_locks", Base: 10, Points: 20, Dexterity: 10, Percent: 40},
		{Skill: "climb_walls", Base: 60, Race: -15, Armor: 10, Percent: 55},
	}
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(thief, nil).Maybe()
	return f
}

// ADR 0030 step 17 through ADR 0014 step 3: a thieving skill is one d100 at
// or under the percentage the character service derives, logged as one
// skill check of kind thiefSkill.
func TestThiefSkill_Step17_D100AtOrUnderTheDerivedPercent(t *testing.T) {
	for _, tt := range []struct {
		roll    int
		success bool
	}{{40, true}, {41, false}, {1, true}} {
		f := thiefFixture(t)
		f.records()
		f.dice.EXPECT().RollDie(skillsvc.ThiefSkillDie).Return(tt.roll).Once()
		got := f.roll(t, "open_locks")
		assert.Equal(t, skillsvc.KindThiefSkill, got.Kind)
		assert.Equal(t, 100, got.Die)
		assert.Equal(t, 40, got.Target)
		assert.Equal(t, tt.success, got.Success, "roll %d", tt.roll)
		require.Len(t, f.entries, 1)
		assert.Equal(t, actionlog.KindSkillCheck, f.entries[0].Kind)
		var rec map[string]any
		require.NoError(t, json.Unmarshal(f.entries[0].Payload, &rec))
		assert.Equal(t, "thiefSkill", rec["kind"])
	}
}

// ADR 0014 steps 5 and 8 on a thieving skill: the DM's adjustment moves
// the target, in percentage points up to 100, never the roll.
func TestThiefSkill_TheAdjustmentMovesTheTarget(t *testing.T) {
	f := thiefFixture(t)
	f.records()
	f.dice.EXPECT().RollDie(100).Return(48).Once()
	got, err := f.svc.Roll(context.Background(), charID, "open_locks", skillsvc.Adjustment{Value: 10, Reason: "simple lock"})
	require.NoError(t, err)
	assert.Equal(t, 50, got.AdjustedTarget)
	assert.True(t, got.Success)
	assert.Equal(t, 100, skillsvc.MaxAdjustment(skillsvc.KindThiefSkill))

	f = thiefFixture(t)
	_, err = f.svc.Roll(context.Background(), charID, "open_locks", skillsvc.Adjustment{Value: 101})
	require.Error(t, err, "refused before the die")
}

// The thieving skills are listed with their terms and the last check of
// each; an unknown skill rolls nothing.
func TestThiefSkill_ListedWithTheirTermsAndLastCheck(t *testing.T) {
	f := thiefFixture(t)
	f.log.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, skillsvc.LastLookup).Return([]actionlog.Entry{
		checkEntry(t, 5, `{"id":"climb_walls","kind":"thiefSkill","die":100,"roll":30,"target":55,"adjustedTarget":55,"success":true}`),
	}, nil)
	got, err := f.svc.List(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []skillsvc.ThiefSkill{
		{SkillPercent: ruletablesvc.SkillPercent{Skill: "open_locks", Base: 10, Points: 20, Dexterity: 10, Percent: 40}},
		{
			SkillPercent: ruletablesvc.SkillPercent{Skill: "climb_walls", Base: 60, Race: -15, Armor: 10, Percent: 55},
			Last:         &skillsvc.LastCheck{EntryID: 5, Roll: 30, Target: 55, AdjustedTarget: 55, Success: true},
		},
	}, got.ThiefSkills)

	f = thiefFixture(t)
	_, err = f.svc.Roll(context.Background(), charID, "read_languages", skillsvc.Adjustment{})
	require.Error(t, err, "a skill the character lacks; the dice mock fails on any roll")
}

// ADR 0030 step 17 (PHB p.39): a thieving skill at 0% is refused before
// the die (the dice mock fails a roll), even with a DM bonus, until points
// raise it to 1% at least.
func TestThiefSkill_Step17_ZeroPercentRefusedBeforeTheDie(t *testing.T) {
	f := &fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{DAL: f.dal, Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Tx: f.tx})
	require.NoError(t, err)
	f.dal.EXPECT().ListProficiencies(mock.Anything, charID).Return(nil, nil)
	thief := &charactersvc.Character{ID: charID, Class: "thief"}
	thief.Derived.ThiefSkills = []ruletablesvc.SkillPercent{{Skill: "climb_walls", Base: 60, Race: -15, Armor: -50, Percent: 0}}
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(thief, nil)
	_, err = svc.Roll(context.Background(), charID, "climb_walls", skillsvc.Adjustment{Value: 30, Reason: "rope"})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, skillsvc.CodeSkillUnusable, fe.Message)
}
