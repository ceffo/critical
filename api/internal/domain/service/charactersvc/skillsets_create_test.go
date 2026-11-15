package charactersvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// swashbucklerEnv is a creation service on definitions where test_swashbuckler, a warrior, grants
// the thief's skill set and the bard's from level 3 (ADR 0040 step 3.5): a character with two
// skill sets.
func swashbucklerEnv(t *testing.T) (*env, charactersvc.NewCharacter) {
	t.Helper()
	defs := cataloguetest.Definitions(t)
	listed := func(class string) classes.Feature {
		def, ok := defs.Class(class)
		require.True(t, ok)
		f, ok := def.Lists(classes.KindRogueSkills)
		require.True(t, ok)
		return f
	}
	bard := listed("bard")
	bard.FromLevel = 3
	e := newEnv(t)
	res := rulesetmocks.NewMockResolver(t)
	res.EXPECT().ForCharacter(mock.Anything, mock.Anything).Return(ruleset.Default(), nil).Maybe()
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: edited(t, add("test_swashbuckler", "fighter", "", listed("thief"), bard)),
		Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book,
		FreeSpells: e.free, Rulesets: res,
	})
	require.NoError(t, err)
	e.svc = svc
	return e, charactersvc.NewCharacter{
		Name: "Dash", Class: "test_swashbuckler", Race: "human", Details: identity(),
		Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 20,
	}
}

// xpOfLevel is the XP that reaches level for test_swashbuckler, a copy of the fighter's table.
func xpOfLevel(t *testing.T, level int) int {
	t.Helper()
	fighter, ok := cataloguetest.Definitions(t).Class("fighter")
	require.True(t, ok)
	xp, err := fighter.XPThreshold(level)
	require.NoError(t, err)
	return xp
}

// ADR 0037 step 10 and ADR 0030 steps 17 and 18 per skill set (ADR 0040 step 3.5): a creation
// with two skill sets takes the points entered for each and checks each against its own budget;
// the draft answers one budget per set.
func TestCreate_Step10_EachSkillSetHasItsOwnBudget(t *testing.T) {
	e, n := swashbucklerEnv(t)
	n.XP = xpOfLevel(t, 3) // the thief's 60+2*30 = 120 points, the bard's 20+2*15 = 50
	n.SkillPoints = skills.SetPoints{
		"thief": {"pick_pockets": 45, "open_locks": 45, "climb_walls": 30},
		"bard":  {"pick_pockets": 50},
	}

	t.Run("the draft figures each set", func(t *testing.T) {
		d, err := e.svc.Draft(context.Background(), n, nil)
		require.NoError(t, err)
		assert.Empty(t, d.Refusals)
		require.Len(t, d.SkillSets, 2)
		thief, bard := d.SkillSets[0], d.SkillSets[1]
		assert.Equal(t, "thief", thief.Set)
		assert.Equal(t, "bard", bard.Set)
		assert.Contains(t, thief.Skills, "open_locks")
		assert.NotContains(t, bard.Skills, "open_locks", "a set lists its own skills")
		require.NotNil(t, thief.Budget)
		require.NotNil(t, bard.Budget)
		assert.Equal(t, 3, thief.Budget.Level)
		assert.Equal(t, [3]int{120, 120, 0}, [3]int{thief.Budget.Total, thief.Budget.Spent, thief.Budget.Left})
		assert.Equal(t, [3]int{50, 50, 0}, [3]int{bard.Budget.Total, bard.Budget.Spent, bard.Budget.Left})
		require.NotNil(t, d.Character)
		derived := d.Character.Derived.SkillSets
		require.Len(t, derived, 2)
		assert.Equal(t, thief.Budget, derived[0].Points, "the draft's budget is the derived set's")
		assert.Equal(t, bard.Budget, derived[1].Points)
	})

	t.Run("stored per set", func(t *testing.T) {
		var stored charactersvc.Character
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).
			Run(func(_ context.Context, _ uuid.UUID, c charactersvc.Character) { stored = c }).Return(9, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(9)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, n)
		require.NoError(t, err)
		assert.Equal(t, n.SkillPoints, stored.SkillPoints, "the second set's points are stored with the first's")
	})

	t.Run("the second set over its own budget is refused by name", func(t *testing.T) {
		over := n
		over.SkillPoints = skills.SetPoints{"thief": {"pick_pockets": 10}, "bard": {"pick_pockets": 51}}
		_, err := e.svc.Create(context.Background(), ownerID, over)
		require.ErrorIs(t, err, domain.ErrOverBudget)
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "skillPoints.bard", Spent: 51, Limit: 50}, *be)
		d, err := e.svc.Draft(context.Background(), over, nil)
		require.NoError(t, err)
		assert.Equal(t, []error{be}, d.Refusals, "the draft lists the same refusal")
	})

	t.Run("none entered stores none", func(t *testing.T) {
		var stored charactersvc.Character
		none := n
		none.SkillPoints = nil
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).
			Run(func(_ context.Context, _ uuid.UUID, c charactersvc.Character) { stored = c }).Return(10, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(10)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, none)
		require.NoError(t, err)
		assert.Empty(t, stored.SkillPoints)
	})
}

// ADR 0040 step 3.5 (from_level): at level 1 only the thief's set is granted, so the draft figures
// a budget for it and none for the bard's, whose points are refused by its name with a budget of 0.
func TestCreate_Step10_ASetNotGrantedYetHasNoBudgetAndRefusesPoints(t *testing.T) {
	e, n := swashbucklerEnv(t)
	n.SkillPoints = skills.SetPoints{"thief": {"pick_pockets": 20}, "bard": {"pick_pockets": 5}}
	d, err := e.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	require.Len(t, d.SkillSets, 2)
	require.NotNil(t, d.SkillSets[0].Budget)
	assert.Equal(t, 60, d.SkillSets[0].Budget.Total)
	assert.Nil(t, d.SkillSets[1].Budget, "the bard's set starts at level 3")
	assert.Equal(t, []error{&domain.BudgetError{Field: "skillPoints.bard", Spent: 5, Limit: 0}}, d.Refusals)
}
