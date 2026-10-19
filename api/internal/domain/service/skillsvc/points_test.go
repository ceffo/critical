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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
)

// pointsFixture is a character of class whose sheet stores points.
func pointsFixture(t *testing.T, class string, points map[string]int) *fixture {
	t.Helper()
	f := &fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{DAL: f.dal, Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Class: class, SkillPoints: points}, nil).Maybe()
	return f
}

// ADR 0030 step 17 after creation: the player sets each skill's total
// points, checked as at creation; the zeros are dropped, the write and its
// skill_points entry (the points before and after, snapshotted under
// "skill_points") share one transaction, and the budget is no refusal (60 in
// one skill is above any 1st-level budget).
func TestSetPoints_Step17_WritesAndLogsTheSpend(t *testing.T) {
	f := pointsFixture(t, "thief", map[string]int{"open_locks": 30})
	f.records()
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, map[string]int{"open_locks": 60, "pick_pockets": 15}).Return(nil).Once()

	got, err := f.svc.SetPoints(context.Background(), charID, map[string]int{"open_locks": 60, "pick_pockets": 15, "climb_walls": 0})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"open_locks": 60, "pick_pockets": 15}, got)
	assert.Equal(t, 1, f.tx.calls)
	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Equal(t, actionlog.KindSkillPoints, e.Kind)
	assert.JSONEq(t, `{"before":{"open_locks":30},"after":{"open_locks":60,"pick_pockets":15}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, skillsvc.PointsStateKey, e.Snapshots[0].Key)
	assert.JSONEq(t, `{"points":{"open_locks":30}}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"points":{"open_locks":60,"pick_pockets":15}}`, snaptest.State(t, e.Snapshots[0].After))
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
			_, err := f.svc.SetPoints(context.Background(), charID, tt.points)
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
// empty one as none.
func TestSetPoints_Step17_RestorerWritesThePointsBack(t *testing.T) {
	f := pointsFixture(t, "thief", nil)
	r := f.svc.Restorers()[skillsvc.PointsStateKey]
	require.NotNil(t, r)
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, map[string]int{"open_locks": 30}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"points":{"open_locks":30}}`)))
	f.points.EXPECT().SetSkillPoints(mock.Anything, charID, map[string]int(nil)).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"points":{}}`)))
}
