package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0037 step 3 on the read: the weapon pool follows the realized level, never the target
// level; Kelsen (rogue, level 15) has 2 + 3 = 5 weapon slots, a rogue gains its next at 16, which
// a pending level-up does not grant. Each pick says whether the class may pick its type.
func TestDerive_Step3_WeaponSlotsFollowTheRealizedLevel(t *testing.T) {
	for _, target := range []int{15, 16} {
		e := newEnvWithBook(t, profBook(t))
		k := kelsen()
		k.Proficiencies = weapons("other_melee", "long_sword", "short_bow", "dagger")
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
		e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
		expectKelsenAt(e, target)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		require.NotNil(t, got.Derived.Proficiencies)
		assert.Equal(t, ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: 3, Available: 5, Spent: 4, Left: 1},
			got.Derived.Proficiencies.Weapon, "target %d", target)
		one := 1
		assert.Contains(t, got.Warnings, charactersvc.Warning{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &one})
		assert.Equal(t, skills.WeaponPickFigures{Type: "other_melee", Slots: 1, Allowed: true}, got.Derived.Proficiencies.WeaponPicks[0])
	}
	t.Run("a stored type the class may not pick reads as not allowed", func(t *testing.T) {
		f, ok := skills.ProficiencyFigures(profBook(t), "mage", 1, 9, skills.Spending{Picks: weapons("dagger", "long_sword")})
		require.True(t, ok)
		assert.Equal(t, []skills.WeaponPickFigures{
			{Type: "dagger", Slots: 1, Allowed: true}, {Type: "long_sword", Slots: 1, Allowed: false},
		}, f.WeaponPicks)
	})
}

// ADR 0037 steps 7, 10 and 14: the creation takes weapon picks, checked as the editor checks them
// with nothing picked before: within budget stored; over budget, or a type the class may not
// pick, refused with nothing written. A level 1 bard has 2 weapon slots.
func TestCreate_Step14_WeaponPicksCheckedAsTheEditor(t *testing.T) {
	withPicks := func(ps skills.Picks) charactersvc.NewCharacter {
		n := lyra()
		n.Proficiencies = ps
		return n
	}
	t.Run("within budget: stored", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		want := weapons("long_sword", "short_bow")
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.MatchedBy(func(c charactersvc.Character) bool {
			return c.Proficiencies.Equal(want)
		})).Return(12, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(12)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, withPicks(want))
		require.NoError(t, err)
	})
	t.Run("over budget: refused, nothing written", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		_, err := e.svc.Create(context.Background(), ownerID, withPicks(weapons("long_sword", "short_bow", "dagger")))
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "weapon", Spent: 3, Limit: 2}, *be)
		assert.Zero(t, e.tx.n)
	})
	t.Run("a type the class may not pick: refused, nothing written", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		n := withPicks(weapons("long_sword"))
		n.Class, n.Race = "mage", "human"
		e.rules.EXPECT().RaceAllows("human", "mage").Return(true, nil).Once()
		// The picks are judged at the starting level: a level 1 mage's derivation.
		e.rules.EXPECT().LevelFor("mage", 0).Return(1, nil).Once()
		e.rules.EXPECT().XPThreshold("mage", 2).Return(2_500, nil).Once()
		e.rules.EXPECT().THAC0For("wizard", 1).Return(20, nil).Once()
		e.rules.EXPECT().SavesFor("wizard", 1).Return(ruletablesvc.SavingThrowRow{Group: "wizard", LevelMin: 1, LevelMax: 5}, nil).Once()
		e.rules.EXPECT().SpellsPerDay(classDef("mage"), 1, mock.Anything).Return(map[int]int{1: 1}, nil).Once()
		e.rules.EXPECT().IntelligenceFor(mock.Anything).Return(int14, nil).Maybe()
		_, err := e.svc.Create(context.Background(), ownerID, n)
		assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}, fieldErr(t, err))
		assert.Zero(t, e.tx.n)
	})
	t.Run("the draft answers the weapon pool", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		expectLevel1Bard(e)
		got, err := e.svc.Draft(context.Background(), withPicks(weapons("dagger")), nil)
		require.NoError(t, err)
		require.NotNil(t, got.Character)
		require.NotNil(t, got.Character.Derived.Proficiencies)
		assert.Equal(t, ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, Available: 2, Spent: 1, Left: 1}, got.Character.Derived.Proficiencies.Weapon)
	})
}

// ADR 0037 steps 7 and 9 for the editor: a class's weapon types in display order with the
// specialization's slots for a fighter (2, a bow 3), 0 for a class that does not specialize, and
// only the types the class may pick.
func TestClassWeaponTypeOptions_Step7_AllowedTypesAndTheSpecializationCost(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	def := func(id string) classes.Class {
		c, ok := defs.Class(id)
		require.True(t, ok, id)
		return c
	}
	fighter := charactersvc.ClassWeaponTypeOptions(def("fighter"))
	require.Len(t, fighter, len(allWeaponTypes))
	assert.Equal(t, charactersvc.WeaponTypeOption{Type: "long_bow", SpecializationSlots: 3}, fighter[0])
	assert.Equal(t, charactersvc.WeaponTypeOption{Type: "light_crossbow", SpecializationSlots: 2}, fighter[2])
	assert.Equal(t, []charactersvc.WeaponTypeOption{
		{Type: "sling"}, {Type: "other_melee"}, {Type: "other_missile"},
	}, charactersvc.ClassWeaponTypeOptions(def("cleric")))
	for _, o := range charactersvc.ClassWeaponTypeOptions(def("bard")) {
		assert.Zero(t, o.SpecializationSlots, "a bard picks %s but never specializes", o.Type)
	}
}
