package skillsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// A bard whose character service derived its four skills from its points
// (ADR 0030 step 18); it has no proficiency.
func derivedBardFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: mocks.NewMockPicks(t), Buffs: mocks.NewMockBuffs(t), Rulesets: f.rulesets, Book: testBook(t), Tables: &fakeTables{}, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	bard := viewtest.Single("bard", 1, viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(0, 16, 0, 0, 0, 0))
	bard.SkillPoints = skills.SetPoints{"bard": {"detect_noise": 17}}
	bard.SkillSets = []skills.SkillSet{{Set: "bard", Skills: []ruletablesvc.SkillPercent{
		{Skill: "climb_walls", Base: 50, Armor: -25, Percent: 25},
		{Skill: "detect_noise", Base: 20, Points: 17, Percent: 37},
		{Skill: "pick_pockets", Base: 10, Armor: -25, Percent: 0},
		{Skill: "read_languages", Base: 5, Percent: 5},
	}}}
	f.chars.EXPECT().View(mock.Anything, charID).Return(bard, nil).Maybe()
	return f
}

// ADR 0030 step 18 through ADR 0014 step 3 (as amended): a bard skill is
// one d100 at or under the percentage the character service derives from
// its points, logged as one skill check of kind rogueSkill with its set.
func TestBardSkill_Step18_D100AtOrUnderTheDerivedPercent(t *testing.T) {
	for _, tt := range []struct {
		roll    int
		success bool
	}{{37, true}, {38, false}, {1, true}} {
		f := derivedBardFixture(t)
		f.records()
		f.dice.EXPECT().RollDie(skillsvc.RogueSkillDie).Return(tt.roll).Once()
		got := f.roll(t, "detect_noise")
		assert.Equal(t, skillsvc.KindRogueSkill, got.Kind)
		assert.Equal(t, "bard", got.Set)
		assert.Equal(t, 100, got.Die)
		assert.Equal(t, 37, got.Target, "the derived percentage")
		assert.Equal(t, tt.success, got.Success, "roll %d", tt.roll)
		require.Len(t, f.entries, 1)
		var rec map[string]any
		require.NoError(t, json.Unmarshal(f.entries[0].Payload, &rec))
		assert.Equal(t, "rogueSkill", rec["kind"])
		assert.Equal(t, "bard", rec["set"])
	}
	f := derivedBardFixture(t)
	_, err := f.svc.Roll(context.Background(), charID, "open_locks", skillsvc.Adjustment{})
	require.Error(t, err, "a thieving skill the bard lacks; the dice mock fails on any roll")
}

// ADR 0030 step 18: the bard's skills are listed with their terms (the
// editor's points) and the last check of each, in Table 33's order.
func TestBardSkill_Step18_ListedWithTheirTermsAndLastCheck(t *testing.T) {
	f := derivedBardFixture(t)
	f.log.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, skillsvc.LastLookup).Return([]actionlog.Entry{
		checkEntry(t, 5, `{"id":"detect_noise","kind":"bardSkill","die":100,"roll":30,"target":37,"adjustedTarget":37,"success":true}`),
	}, nil)
	got, err := f.svc.List(context.Background(), charID)
	require.NoError(t, err)
	require.Len(t, got.SkillSets, 1)
	set := got.SkillSets[0]
	assert.Equal(t, "bard", set.Set)
	require.Len(t, set.Skills, 4)
	assert.Equal(t, []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"},
		[]string{set.Skills[0].Skill, set.Skills[1].Skill, set.Skills[2].Skill, set.Skills[3].Skill})
	assert.Equal(t, 17, set.Skills[1].Points)
	assert.Equal(t, &skillsvc.LastCheck{EntryID: 5, Roll: 30, Target: 37, AdjustedTarget: 37, Success: true}, set.Skills[1].Last)
}

// ADR 0030 step 18 after creation: a bard spends its points as a thief
// does, one skill_points entry with the points' snapshot; the bard has no
// limit in one skill (60 in one skill, 77 of its 80 points at level 5).
func TestSetPoints_Step18_ABardSpendsItsPoints(t *testing.T) {
	f := pointsFixture(t, "bard", map[string]int{"detect_noise": 17})
	f.records()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints{"bard": {"detect_noise": 17, "climb_walls": 60}}).Return(nil).Once()
	got, err := f.svc.SetPoints(context.Background(), charID, "", map[string]int{"detect_noise": 17, "climb_walls": 60, "read_languages": 0})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"detect_noise": 17, "climb_walls": 60}, got)
	require.Len(t, f.entries, 1)
	assert.Equal(t, actionlog.KindSkillPoints, f.entries[0].Kind)
	assert.JSONEq(t, `{"set":"bard","before":{"detect_noise":17},"after":{"detect_noise":17,"climb_walls":60}}`, string(f.entries[0].Payload))
}
