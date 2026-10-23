package skillsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// pointsFixture is a character of class at realized level 5 whose sheet
// stores points.
func pointsFixture(t *testing.T, class string, points map[string]int) *fixture {
	t.Helper()
	return pointsFixtureAt(t, class, 5, points)
}

// pointsFixtureAt is a character of class at realized level whose sheet
// stores points.
func pointsFixtureAt(t *testing.T, class string, level int, points map[string]int) *fixture {
	t.Helper()
	f := &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: mocks.NewMockPicks(t), Buffs: mocks.NewMockBuffs(t), Rulesets: f.rulesets, Book: testBook(t), Tables: &fakeTables{}, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	v := viewtest.Single(class, level, viewtest.WithIdentity(charID, "Test", ""))
	v.SkillPoints = skills.SetPoints(nil).With(class, points)
	f.chars.EXPECT().View(mock.Anything, charID).Return(v, nil).Maybe()
	return f
}

// ADR 0030 step 17 after creation: the player sets each skill's total
// points, checked as at creation; the zeros are dropped, the write and its
// skill_points entry (the points before and after, snapshotted under
// "skill_points") share one transaction (75 points, 60 in one skill, within
// a 5th-level thief's 180, 90 in one skill).
func TestSetPoints_Step17_WritesAndLogsTheSpend(t *testing.T) {
	f := pointsFixture(t, "thief", map[string]int{"open_locks": 30})
	f.records()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints{"thief": {"open_locks": 60, "pick_pockets": 15}}).Return(nil).Once()

	got, err := f.svc.SetPoints(context.Background(), charID, "", map[string]int{"open_locks": 60, "pick_pockets": 15, "climb_walls": 0})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"open_locks": 60, "pick_pockets": 15}, got)
	assert.Equal(t, 1, f.tx.calls)
	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Equal(t, actionlog.KindSkillPoints, e.Kind)
	assert.JSONEq(t, `{"set":"thief","before":{"open_locks":30},"after":{"open_locks":60,"pick_pockets":15}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, skillsvc.PointsStateKey, e.Snapshots[0].Key)
	assert.JSONEq(t, `{"set":"thief","points":{"open_locks":30}}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"set":"thief","points":{"open_locks":60,"pick_pockets":15}}`, snaptest.State(t, e.Snapshots[0].After))
}

// Step 17: points refused (out of range, an unknown skill, any point of a
// class without thieving skills) name the field; nothing is written or
// logged. The same points as stored write and log nothing either.
func TestSetPoints_Step17_RefusalsAndNoChange(t *testing.T) {
	for _, tt := range []struct {
		name, class string
		points      map[string]int
		field       string
	}{
		{"over 95", "thief", map[string]int{"open_locks": 96}, "skillPoints.open_locks"},
		{"negative", "thief", map[string]int{"open_locks": -1}, "skillPoints.open_locks"},
		{"unknown skill", "thief", map[string]int{"fly": 5}, "skillPoints.fly"},
		{"a thieving skill a bard lacks", "bard", map[string]int{"open_locks": 5}, "skillPoints.open_locks"},
		{"a fighter's points", "fighter", map[string]int{"climb_walls": 5}, "skillPoints"},
		{"no change", "thief", map[string]int{"open_locks": 30, "hide_in_shadows": 0}, ""},
		{"a fighter with none", "fighter", map[string]int{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stored := map[string]int{"open_locks": 30}
			if tt.class != "thief" {
				stored = nil
			}
			f := pointsFixture(t, tt.class, stored) // the mocks fail any write or record
			_, err := f.svc.SetPoints(context.Background(), charID, "", tt.points)
			if tt.field == "" {
				require.NoError(t, err)
				return
			}
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
		})
	}
}

// ADR 0019 on step 17: the restorer writes a snapshot's points back, an
// empty one as none, in the snapshot's skill set (a snapshot from before skill sets: the
// character's only one).
func TestSetPoints_Step17_RestorerWritesThePointsBack(t *testing.T) {
	f := pointsFixture(t, "thief", nil)
	r := f.svc.Module().Restorer(skillsvc.PointsStateKey)
	require.NotNil(t, r)
	f.points.EXPECT().SkillPoints(mock.Anything, charID).Return("thief", nil, nil).Times(3)
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints{"thief": {"open_locks": 30}}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"set":"thief","points":{"open_locks":30}}`)))
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints(nil)).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"set":"thief","points":{}}`)))
	// An entry from before skill sets has no set: the character's only one.
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints{"thief": {"open_locks": 10}}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"points":{"open_locks":10}}`)))
}

// ADR 0037 step 10 on the skill points: a spend is refused when, after it,
// the total is over the realized level's budget and above the total before,
// or (the thief's per-skill cap) one skill is over the cap and above its
// points before. The refusal names the field, the points and the limit, and
// nothing is written or logged. Any other spend is stored: within the
// budget, leftovers included, and an existing overspend that shrinks or
// stays (it keeps the derived warning). At level 1 a thief has 60 points,
// 30 in one skill; at level 2, 90 and 45; a bard at level 1 has 20.
func TestSetPoints_Step10_SkillPointsRefusedOverAndGrowing(t *testing.T) {
	type refusal struct {
		field        string
		spent, limit int
	}
	for _, tt := range []struct {
		name, class string
		level       int
		stored, set map[string]int
		refused     *refusal
	}{
		{"within the budget, points left", "thief", 1, nil, map[string]int{"open_locks": 30, "pick_pockets": 20}, nil},
		{"the budget spent exactly", "thief", 1, nil, map[string]int{"open_locks": 30, "pick_pockets": 30}, nil},
		{"the realized level's budget", "thief", 2, nil, map[string]int{"open_locks": 45, "pick_pockets": 45}, nil},
		{"a new overspend of the total", "thief", 1, map[string]int{"open_locks": 30}, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 1}, &refusal{"skillPoints", 61, 60}},
		{"a new overspend of one skill", "thief", 1, map[string]int{"open_locks": 30}, map[string]int{"open_locks": 31}, &refusal{"skillPoints.open_locks", 31, 30}},
		{"an overspent total reduced", "thief", 1, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 10}, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 5}, nil},
		{"an overspent total moved between skills", "thief", 1, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 10}, map[string]int{"open_locks": 30, "pick_pockets": 25, "climb_walls": 15}, nil},
		{"an overspent total grown", "thief", 1, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 10}, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 11}, &refusal{"skillPoints", 71, 60}},
		{"an overspent skill kept, another raised", "thief", 1, map[string]int{"open_locks": 40}, map[string]int{"open_locks": 40, "pick_pockets": 5}, nil},
		{"an overspent skill reduced", "thief", 1, map[string]int{"open_locks": 40}, map[string]int{"open_locks": 35}, nil},
		{"an overspent skill grown", "thief", 1, map[string]int{"open_locks": 40}, map[string]int{"open_locks": 41}, &refusal{"skillPoints.open_locks", 41, 30}},
		{"a bard within its total, all in one skill", "bard", 1, nil, map[string]int{"climb_walls": 20}, nil},
		{"a bard's new overspend", "bard", 1, nil, map[string]int{"climb_walls": 15, "read_languages": 6}, &refusal{"skillPoints", 21, 20}},
		{"a bard's overspend reduced", "bard", 1, map[string]int{"climb_walls": 25}, map[string]int{"climb_walls": 22}, nil},
		{"a bard's overspend grown", "bard", 1, map[string]int{"climb_walls": 25}, map[string]int{"climb_walls": 25, "detect_noise": 1}, &refusal{"skillPoints", 26, 20}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := pointsFixtureAt(t, tt.class, tt.level, tt.stored) // the mocks fail any unexpected write or record
			if tt.refused == nil {
				f.records()
				f.points.EXPECT().SetSkillPoints(mock.Anything, charID, skills.SetPoints{tt.class: tt.set}).Return(nil).Once()
			}
			got, err := f.svc.SetPoints(context.Background(), charID, "", tt.set)
			if tt.refused == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.set, got)
				assert.Len(t, f.entries, 1)
				return
			}
			require.ErrorIs(t, err, domain.ErrOverBudget)
			var be *domain.BudgetError
			require.ErrorAs(t, err, &be)
			assert.Equal(t, domain.BudgetError{Field: tt.refused.field, Spent: tt.refused.spent, Limit: tt.refused.limit}, *be)
			assert.Empty(t, f.entries, "no skill_points entry")
		})
	}
	t.Run("an overspend unchanged writes nothing and is no refusal", func(t *testing.T) {
		stored := map[string]int{"open_locks": 40, "pick_pockets": 30}
		f := pointsFixtureAt(t, "thief", 1, stored)
		got, err := f.svc.SetPoints(context.Background(), charID, "", stored)
		require.NoError(t, err)
		assert.Equal(t, stored, got)
	})
}

// ADR 0030 steps 17 and 18 in the editor's preview (ADR 0037 step 17: the skills module's): the
// tab as it would read with the draft points, the percentages from the tables at the
// character's race, Dexterity and skill armour, the budget at the realized level; refused as the
// spend refuses (an unknown skill, an overspend that grows), nothing written or logged (the
// points and log mocks fail any write or record).
func TestPreviewPoints_Step17_TheDraftReadAsTheSpendChecksIt(t *testing.T) {
	f := &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	tables := &fakeTables{}
	svc, err := skillsvc.New(skillsvc.Deps{Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: mocks.NewMockPicks(t), Buffs: mocks.NewMockBuffs(t), Rulesets: f.rulesets, Book: testBook(t), Tables: tables, Tx: f.tx})
	require.NoError(t, err)
	f.chars.EXPECT().View(mock.Anything, charID).RunAndReturn(func(context.Context, int64) (charactersvc.View, error) {
		v := viewtest.Single("thief", 1, viewtest.WithIdentity(charID, "Test", "elf"), viewtest.WithScores(0, 17, 0, 0, 0, 0))
		v.SkillPoints, v.SkillColumn = skills.SetPoints{"thief": {"open_locks": 30}}, "leather"
		return v, nil
	})
	f.log.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, mock.Anything).Return(nil, nil)

	got, err := svc.PreviewPoints(context.Background(), charID, "", map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 0})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"open_locks": 30, "pick_pockets": 30}, tables.points, "the draft without its zeros")
	assert.Equal(t, "elf", tables.race)
	assert.Equal(t, 17, tables.dex)
	assert.Equal(t, "leather", tables.armor, "the character's skill armour")
	require.Len(t, got.SkillSets, 1)
	require.NotNil(t, got.SkillSets[0].Budget)
	assert.Equal(t, 60, got.SkillSets[0].Budget.Spent)
	assert.Equal(t, 0, got.SkillSets[0].Budget.Left, "a level 1 thief's 60 points, spent")
	assert.Equal(t, "leather", got.SkillSets[0].Armor)

	_, err = svc.PreviewPoints(context.Background(), charID, "", map[string]int{"juggling": 5})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "skillPoints.juggling", fe.Field)

	_, err = svc.PreviewPoints(context.Background(), charID, "", map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 5})
	var be *domain.BudgetError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, domain.BudgetError{Field: "skillPoints", Spent: 65, Limit: 60}, *be)
}

// ADR 0042 steps 13 and 16.3 in the preview: a draft on a dual-class (a thief
// left at 9, now a fighter 5) reads the thief's set through the grant of the
// frozen entry: the multi-class flag set, so body
// armour the thief may not wear blocks its skills, which a single thief's
// draft in the same armour does not.
func TestPreviewPoints_Step13_ADraftOnADualClassKeepsTheMultiFlagAndDormancy(t *testing.T) {
	preview := func(v charactersvc.View) *skillsvc.Skills {
		chars, logm := mocks.NewMockCharacters(t), mocks.NewMockLog(t)
		svc, err := skillsvc.New(skillsvc.Deps{Characters: chars, Abilities: mocks.NewMockAbilities(t), Dice: mocks.NewMockDice(t), Log: logm, Points: mocks.NewMockPoints(t), Picks: mocks.NewMockPicks(t), Buffs: mocks.NewMockBuffs(t), Rulesets: rulesetmocks.NewMockResolver(t), Book: testBook(t), Tables: &fakeTables{}, Tx: &countingTx{}})
		require.NoError(t, err)
		v.SkillPoints, v.SkillColumn = skills.SetPoints{"thief": {"open_locks": 10}}, "chain"
		chars.EXPECT().View(mock.Anything, charID).Return(v, nil).Once()
		logm.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, mock.Anything).Return(nil, nil)
		got, err := svc.PreviewPoints(context.Background(), charID, "", map[string]int{"open_locks": 20})
		require.NoError(t, err)
		require.Len(t, got.SkillSets, 1)
		return got
	}
	grant := func(dormant bool) viewtest.Option {
		return func(v *charactersvc.View) {
			v.Features = []charactersvc.FeatureGrant{{Kind: classes.KindRogueSkills, Class: "thief", Level: 9, Dormant: dormant}}
		}
	}
	blocked := func(s *skillsvc.Skills) bool {
		for _, sk := range s.SkillSets[0].Skills {
			if sk.Skill == "pick_pockets" {
				return sk.Blocked
			}
		}
		return false
	}

	dual := preview(viewtest.Multi(viewtest.WithFrozen("thief", 9), viewtest.WithEntry("fighter", 5), viewtest.WithDormant(), grant(true)))
	assert.True(t, blocked(dual), "multi-class: the chain the thief may not wear blocks the skill")

	single := preview(viewtest.Single("thief", 9, grant(false)))
	assert.False(t, blocked(single), "single-class: not blocked")
}
