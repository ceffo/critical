package skillsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// swashbuckler is a test class granting the thief's and the bard's skill sets, which share
// pick_pockets, climb_walls, detect_noise and read_languages (ADR 0040 step 3.5).
const swashbuckler = "test_swashbuckler"

// dualFixture is a swashbuckler at level 5 whose character service derived both skill sets, the
// two holding a pick_pockets of their own, and whose sheet stores the points of both.
func dualFixture(t *testing.T) *fixture {
	t.Helper()
	defs := cataloguetest.Definitions(t)
	var feats []classes.Feature
	for _, id := range []string{"thief", "bard"} {
		def, ok := defs.Class(id)
		require.True(t, ok)
		f, ok := def.Lists(classes.KindRogueSkills)
		require.True(t, ok)
		feats = append(feats, f)
	}
	fighter, ok := defs.Class("fighter")
	require.True(t, ok)
	fighter.ID, fighter.SpellProgression, fighter.Features = swashbuckler, "", feats
	reg, err := classes.New(append(defs.Classes(), fighter), defs.GroupDefs(), defs.Races())
	require.NoError(t, err)
	var cross []ruletablesvc.Crossover
	for _, c := range reg.ClassIDs() {
		cross = append(cross, ruletablesvc.Crossover{Class: c, Groups: []string{"general"}})
	}
	var slots []ruletablesvc.SlotRow
	for _, g := range reg.Groups() {
		slots = append(slots, ruletablesvc.SlotRow{Group: g, WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4})
	}
	book, err := ruletablesvc.NewProficiencies(nil, cross, slots, reg)
	require.NoError(t, err)

	f := &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	f.svc, err = skillsvc.New(skillsvc.Deps{Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: mocks.NewMockPicks(t), Buffs: mocks.NewMockBuffs(t), Rulesets: f.rulesets, Book: book, Tables: &fakeTables{}, Tx: f.tx})
	require.NoError(t, err)
	c := &charactersvc.Character{
		ID: charID, Class: swashbuckler, Dex: 16, RealizedLevel: 5,
		SkillPoints: skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"pick_pockets": 5}},
	}
	c.Derived.RealizedLevel = 5
	c.Derived.SkillSets = []skills.SkillSet{
		{Set: "thief", Skills: []ruletablesvc.SkillPercent{{Skill: "pick_pockets", Base: 15, Points: 20, Percent: 35}}},
		{Set: "bard", Skills: []ruletablesvc.SkillPercent{{Skill: "pick_pockets", Base: 10, Points: 5, Percent: 15}}},
	}
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(c, nil).Maybe()
	return f
}

// ADR 0040 step 3.5: a character with two skill sets reads each set with its own percentages
// and its own last check, even for a skill the sets share: the last check of a thief's pick
// pockets is not the bard's.
func TestList_Step3_5_EachSkillSetHasItsOwnLastCheck(t *testing.T) {
	f := dualFixture(t)
	f.log.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, skillsvc.LastLookup).Return([]actionlog.Entry{
		checkEntry(t, 7, `{"id":"pick_pockets","kind":"bardSkill","die":100,"roll":9,"target":15,"adjustedTarget":15,"success":true}`),
	}, nil)
	got, err := f.svc.List(context.Background(), charID)
	require.NoError(t, err)
	require.Len(t, got.SkillSets, 2)
	assert.Equal(t, "thief", got.SkillSets[0].Set)
	assert.Equal(t, skillsvc.KindThiefSkill, got.SkillSets[0].Kind)
	assert.Equal(t, 35, got.SkillSets[0].Skills[0].Percent)
	assert.Nil(t, got.SkillSets[0].Skills[0].Last, "the bard's check is not the thief's")
	assert.Equal(t, skillsvc.KindBardSkill, got.SkillSets[1].Kind)
	assert.Equal(t, 15, got.SkillSets[1].Skills[0].Percent)
	require.NotNil(t, got.SkillSets[1].Skills[0].Last)
	assert.Equal(t, int64(7), got.SkillSets[1].Skills[0].Last.EntryID)
}

// ADR 0040 steps 3.2 and 3.5: a roll names the skill set when two share the skill, and is judged
// against that set's percentage and logged as that set's kind; without one it is the first set
// that holds the skill.
func TestRoll_Step3_5_TheSkillSetPicksThePercentAndTheKind(t *testing.T) {
	for _, tt := range []struct {
		name, set string
		kind      skillsvc.Kind
		target    int
	}{
		{"the bard's", "bard", skillsvc.KindBardSkill, 15},
		{"the thief's", "thief", skillsvc.KindThiefSkill, 35},
		{"the first set without a name", "", skillsvc.KindThiefSkill, 35},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := dualFixture(t)
			f.records()
			f.dice.EXPECT().RollDie(100).Return(20).Once()
			got, err := f.svc.RollInSet(context.Background(), charID, tt.set, "pick_pockets", skillsvc.Adjustment{})
			require.NoError(t, err)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.target, got.Target)
		})
	}
	f := dualFixture(t)
	_, err := f.svc.RollInSet(context.Background(), charID, "psionic", "pick_pockets", skillsvc.Adjustment{})
	require.ErrorIs(t, err, domain.ErrNotFound, "a set the character has not; the dice mock fails any roll")
}

// ADR 0040 step 3.5 on ADR 0037 step 10: a spend addresses one skill set and is checked against
// that set's budget, the other set's points kept as they are; with two sets the set is required.
func TestSetPoints_Step3_5_ASpendAddressesOneSkillSet(t *testing.T) {
	f := dualFixture(t)
	f.records()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID,
		skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"pick_pockets": 5, "climb_walls": 15}}).Return(nil).Once()
	got, err := f.svc.SetPoints(context.Background(), charID, "bard", map[string]int{"pick_pockets": 5, "climb_walls": 15})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"pick_pockets": 5, "climb_walls": 15}, got)
	require.Len(t, f.entries, 1)
	assert.JSONEq(t, `{"set":"bard","before":{"pick_pockets":5},"after":{"pick_pockets":5,"climb_walls":15}}`, string(f.entries[0].Payload))

	// Level 5: the bard has 20+4*15 = 80 points, the thief 60+4*30 = 180, each its own.
	_, err = f.svc.SetPoints(context.Background(), charID, "bard", map[string]int{"pick_pockets": 5, "climb_walls": 76})
	var be *domain.BudgetError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, domain.BudgetError{Field: "skillPoints", Spent: 81, Limit: 80}, *be, "the bard's budget, not the thief's")

	_, err = f.svc.SetPoints(context.Background(), charID, "", map[string]int{"pick_pockets": 5})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, skills.FieldSet, fe.Field, "two sets: which one is not a guess")

	_, err = f.svc.SetPoints(context.Background(), charID, "psionic", map[string]int{"pick_pockets": 5})
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, skills.CodeNotAllowed, fe.Message)
}

// ADR 0019 on ADR 0040 step 3.5: undo writes back the points of the snapshot's skill set only,
// the other set keeps what it has stored.
func TestSetPoints_Step3_5_UndoRestoresOneSkillSet(t *testing.T) {
	f := dualFixture(t)
	r := f.svc.Module().Restorer(skillsvc.PointsStateKey)
	require.NotNil(t, r)
	f.points.EXPECT().SkillPoints(mock.Anything, charID).Return(swashbuckler,
		skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"pick_pockets": 5}}, nil).Once()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID,
		skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"climb_walls": 3}}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"set":"bard","points":{"climb_walls":3}}`)))
}

// ADR 0019 on ADR 0040 step 3.5: an entry from before skill sets names no set; for a class that
// lists two it restores the class's first, not a refusal.
func TestSetPoints_Step3_5_ALegacyEntryRestoresTheFirstSetOfATwoSetClass(t *testing.T) {
	f := dualFixture(t)
	r := f.svc.Module().Restorer(skillsvc.PointsStateKey)
	require.NotNil(t, r)
	f.points.EXPECT().SkillPoints(mock.Anything, charID).Return(swashbuckler,
		skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"pick_pockets": 5}}, nil).Once()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID,
		skills.SetPoints{"thief": {"open_locks": 7}, "bard": {"pick_pockets": 5}}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"points":{"open_locks":7}}`)))
}

// A legacy entry whose points are still filed under no set (a release from before skill sets
// kept writing the flat column) restores into the first set without losing them.
func TestSetPoints_Step3_5_ALegacyEntryResolvesPointsFiledUnderNoSet(t *testing.T) {
	f := dualFixture(t)
	r := f.svc.Module().Restorer(skillsvc.PointsStateKey)
	require.NotNil(t, r)
	f.points.EXPECT().SkillPoints(mock.Anything, charID).Return(swashbuckler,
		skills.SetPoints{skills.LegacySet: {"pick_pockets": 20}}, nil).Once()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID,
		skills.SetPoints{"thief": {"open_locks": 7}}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"points":{"open_locks":7}}`)))
}
