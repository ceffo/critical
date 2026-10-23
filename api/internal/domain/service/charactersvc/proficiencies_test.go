package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// expectKelsenAt answers the lookups of Kelsen's read at the realized level 15 with an XP total
// whose target level is target.
func expectKelsenAt(e *env, target int) {
	e.rules.EXPECT().XPThreshold("bard", target+1).Return(1_600_000, nil).Once()
	e.rules.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	e.rules.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
	e.rules.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Maybe()
}

// ADR 0037 step 3 on the read: the non-weapon pool follows the realized level, never the target
// level (a pending level-up grants nothing until it is taken), with the sheet's Intelligence
// (Table 4); Kelsen (rogue, level 15, Intelligence 14) has 3 + 3 + 4 = 10 slots, his eight
// picks spend 8 and leave 2 to spend (the reminder).
func TestDerive_Step3_SlotsFollowTheRealizedLevel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target int
	}{
		{"no level-up pending", 15},
		{"level 16 pending (rogue gains a slot at 16)", 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnvWithBook(t, profBook(t))
			k := kelsen()
			k.XPTotal = xpAt(t, "bard", tc.target)
			k.Proficiencies = picks(p("tumbling", 1), p("cooking", 1), p("tracking", 2), p("blind_fighting", 2), p("old_skill", 2))
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
			e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
			expectKelsenAt(e, tc.target)
			got, err := e.svc.GetCharacter(context.Background(), 3)
			require.NoError(t, err)
			require.NotNil(t, got.Derived.Proficiencies)
			assert.Equal(t, ruletablesvc.NonweaponPool{
				Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 8, Left: 2,
			}, got.Derived.Proficiencies.Nonweapon)
			two := 2
			assert.Contains(t, got.Warnings, charactersvc.Warning{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &two})
			assert.Contains(t, got.Warnings, charactersvc.Warning{Code: "proficiency_unknown", Field: "nonweapon.old_skill"})
			assert.Equal(t, 17, got.Derived.Proficiencies.Picks[0].Target, "Tumbling: Dex 17 + 0")
		})
	}
	t.Run("the Intelligence term follows the sheet's score", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		k := kelsen()
		k.Int = 18 // Table 4: 7 languages
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
		e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
		expectKelsenAt(e, 15)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		assert.Equal(t, 13, got.Derived.Proficiencies.Nonweapon.Available)
	})
	t.Run("without the catalogue there is no pool", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
		expectKelsenAt(e, 15)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		assert.Nil(t, got.Derived.Proficiencies)
	})
}

// ADR 0037 steps 10 and 14: the creation takes picks, checked as the editor checks them with
// nothing picked before: within budget they are stored, leftovers reminded; over budget, or a
// refused pick, nothing is written. A level 1 bard with Intelligence 14 has 3 + 0 + 4 = 7 slots.
func TestCreate_Step14_PicksCheckedAsTheEditor(t *testing.T) {
	withPicks := func(ps skills.Picks) charactersvc.NewCharacter {
		n := lyra()
		n.Proficiencies = ps
		return n
	}
	t.Run("within budget: stored", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		want := picks(p("tumbling", 3), p("tracking", 2))
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
		_, err := e.svc.Create(context.Background(), ownerID, withPicks(picks(p("tumbling", 6), p("tracking", 2))))
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: 8, Limit: 7}, *be)
		assert.Zero(t, e.tx.n)
	})
	t.Run("a refused pick: nothing written", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e) // the picks are judged at the starting level
		_, err := e.svc.Create(context.Background(), ownerID, withPicks(picks(p("old_skill", 1))))
		assert.Equal(t, domain.FieldError{Field: "nonweapon.old_skill", Message: "unknown"}, fieldErr(t, err), "nothing before: no unknown id kept")
		assert.Zero(t, e.tx.n)
	})
	t.Run("the draft reminds the leftovers", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e) // the starting level
		expectLevel1Bard(e) // the read
		got, err := e.svc.Draft(context.Background(), withPicks(picks(p("tumbling", 1))), nil)
		require.NoError(t, err)
		require.NotNil(t, got.Character)
		six := 6
		assert.Contains(t, got.Character.Warnings, charactersvc.Warning{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &six})
	})
	t.Run("the draft answers the pool", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		expectLevel1Bard(e)
		got, err := e.svc.Draft(context.Background(), withPicks(picks(p("tumbling", 2))), nil)
		require.NoError(t, err)
		require.NotNil(t, got.Character)
		slots := got.Character.Derived.Proficiencies
		require.NotNil(t, slots)
		assert.Equal(t, ruletablesvc.NonweaponPool{Initial: 3, PerLevels: 4, Intelligence: 4, Available: 7, Spent: 2, Left: 5}, slots.Nonweapon)
		assert.Equal(t, 16, slots.Picks[0].Target, "Dex 15 + 1 extra slot")
	})
}
