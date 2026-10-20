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
)

// allWeaponTypes are every weapon type of ADR 0030 step 23.
var allWeaponTypes = ruletablesvc.WeaponTypes

// ADR 0037 step 7: a weapon pick is one weapon type, once, 1 slot, of a type the class's PHB
// list allows: a mage is refused a long sword, a cleric an edged weapon, a thief a type outside
// its list; a fighter and a bard pick any type. A type outside the list that the stored picks
// already have is kept (the migration grants every owned type), never added; so is a stored type
// that is no weapon type (older data is never dropped, as in the non-weapon pool). Wielding is not
// this check's (ADR 0033 step 20 only warns).
func TestWeaponPick_Step7_ClassLimitsRefuseThePick(t *testing.T) {
	book := profBook(t)
	refused := func(field, code string) *domain.FieldError { return &domain.FieldError{Field: field, Message: code} }
	for _, tc := range []struct {
		name   string
		class  string
		picks  charactersvc.Picks
		before charactersvc.Picks
		want   *domain.FieldError
	}{
		{"a mage: dagger, dart, sling, other_melee", "mage", weapons("dagger", "dart", "sling", "other_melee"), weapons(), nil},
		{"a mage refused a long sword", "mage", weapons("dagger", "long_sword"), weapons(), refused("weapon.long_sword", "weapon_not_allowed")},
		{"a cleric: sling, other_melee, other_missile", "cleric", weapons("sling", "other_melee", "other_missile"), weapons(), nil},
		{"a cleric refused an edged weapon", "cleric", weapons("short_sword"), weapons(), refused("weapon.short_sword", "weapon_not_allowed")},
		{"a cleric refused a bow", "cleric", weapons("long_bow"), weapons(), refused("weapon.long_bow", "weapon_not_allowed")},
		{"a thief: its list", "thief", weapons("short_bow", "hand_crossbow", "long_sword", "other_thrown"), weapons(), nil},
		{"a thief refused a type outside its list", "thief", weapons("long_bow"), weapons(), refused("weapon.long_bow", "weapon_not_allowed")},
		{"a thief refused a heavy crossbow", "thief", weapons("heavy_crossbow"), weapons(), refused("weapon.heavy_crossbow", "weapon_not_allowed")},
		{"a fighter: any type", "fighter", weapons(allWeaponTypes...), weapons(), nil},
		{"a bard: any type", "bard", weapons(allWeaponTypes...), weapons(), nil},
		{"a stored type outside the list is kept", "mage", weapons("dagger", "long_sword"), weapons("long_sword"), nil},
		{"an unknown type", "fighter", weapons("katana"), weapons(), refused("weapon.katana", "unknown")},
		{"a stored unknown type is kept", "mage", weapons("dagger", "katana"), weapons("katana"), nil},
		{"a stored unknown type spends 1 slot", "fighter", charactersvc.Picks{Weapon: []charactersvc.WeaponPick{{Type: "katana", Slots: 2}}}, weapons("katana"), refused("weapon.katana", "out_of_range")},
		{"no type", "fighter", weapons(""), weapons(), refused("weapon", "empty")},
		{"a duplicate type", "fighter", weapons("dagger", "dagger"), weapons(), refused("weapon.dagger", "duplicate")},
		{"no slot", "fighter", charactersvc.Picks{Weapon: []charactersvc.WeaponPick{{Type: "dagger"}}}, weapons(), refused("weapon.dagger", "out_of_range")},
		{"two slots (no specialization yet)", "fighter", charactersvc.Picks{Weapon: []charactersvc.WeaponPick{{Type: "dagger", Slots: 2}}}, weapons(), refused("weapon.dagger", "out_of_range")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := charactersvc.ValidPicks(book, tc.class, tc.picks, tc.before)
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks.Weapon, got.Weapon)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
}

// ADR 0037 step 12: an editor of one pool leaves the other as stored. A nil pool in the edit is
// the stored one; an empty list removes every pick of the pool.
func TestValidPicks_Step12_AnAbsentPoolKeepsTheStoredOne(t *testing.T) {
	book := profBook(t)
	stored := charactersvc.Picks{
		Nonweapon: []charactersvc.Pick{p("tumbling", 1)},
		Weapon:    []charactersvc.WeaponPick{{Type: "dagger", Slots: 1}},
	}
	got, err := charactersvc.ValidPicks(book, "bard", charactersvc.Picks{Nonweapon: []charactersvc.Pick{p("cooking", 1)}}, stored)
	require.NoError(t, err)
	assert.Equal(t, charactersvc.Picks{Nonweapon: []charactersvc.Pick{p("cooking", 1)}, Weapon: stored.Weapon}, got)

	got, err = charactersvc.ValidPicks(book, "bard", charactersvc.Picks{Weapon: []charactersvc.WeaponPick{{Type: "dart", Slots: 1}}}, stored)
	require.NoError(t, err)
	assert.Equal(t, charactersvc.Picks{Nonweapon: stored.Nonweapon, Weapon: []charactersvc.WeaponPick{{Type: "dart", Slots: 1}}}, got)

	got, err = charactersvc.ValidPicks(book, "bard", charactersvc.Picks{Weapon: []charactersvc.WeaponPick{}}, stored)
	require.NoError(t, err)
	assert.Empty(t, got.Weapon, "an empty list removes every weapon pick")
	assert.Equal(t, stored.Nonweapon, got.Nonweapon)
}

// ADR 0037 step 10 on the weapon pool: refused when, after the save, spent > available AND spent >
// spent before; any other save passes. A level 1 fighter has 4 weapon slots (Table 34).
func TestBudget_Step10_WeaponPoolRefusedWhenOverAndGrowing(t *testing.T) {
	book := profBook(t)
	four := weapons("dagger", "dart", "sling", "long_sword")
	five := weapons("dagger", "dart", "sling", "long_sword", "short_sword")
	six := weapons("dagger", "dart", "sling", "long_sword", "short_sword", "long_bow")
	for _, tc := range []struct {
		name          string
		before, after charactersvc.Picks
		refused       bool
		spent         int
	}{
		{"under budget", weapons(), weapons("dagger"), false, 1},
		{"at the budget", weapons(), four, false, 4},
		{"a new overspend", four, five, true, 5},
		{"created over budget (nothing before)", weapons(), five, true, 5},
		{"an overspend kept as it is", five, five, false, 5},
		{"an overspend swapped, same total", five, weapons("dagger", "dart", "sling", "long_sword", "long_bow"), false, 5},
		{"an overspend reduced, still over", six, five, false, 5},
		{"an overspend increased", five, six, true, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := charactersvc.PicksWithinBudget(book, "fighter", 1, 9, charactersvc.Spending{Picks: tc.before}, charactersvc.Spending{Picks: tc.after})
			if !tc.refused {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, domain.ErrOverBudget)
			var be *domain.BudgetError
			require.ErrorAs(t, err, &be)
			assert.Equal(t, domain.BudgetError{Field: "weapon", Spent: tc.spent, Limit: 4}, *be)
		})
	}
	t.Run("each pool is judged on its own", func(t *testing.T) {
		// The non-weapon pool over budget and kept does not let the weapon pool grow over,
		// and a weapon overspend kept does not block a non-weapon save within budget.
		before := charactersvc.Picks{Nonweapon: []charactersvc.Pick{p("tracking", 7)}, Weapon: five.Weapon}
		after := charactersvc.Picks{Nonweapon: []charactersvc.Pick{p("tracking", 2)}, Weapon: five.Weapon}
		require.NoError(t, charactersvc.PicksWithinBudget(book, "fighter", 1, 9, charactersvc.Spending{Picks: before}, charactersvc.Spending{Picks: after}))
		after = charactersvc.Picks{Nonweapon: []charactersvc.Pick{p("tracking", 7)}, Weapon: six.Weapon}
		var be *domain.BudgetError
		require.ErrorAs(t, charactersvc.PicksWithinBudget(book, "fighter", 1, 9, charactersvc.Spending{Picks: before}, charactersvc.Spending{Picks: after}), &be)
		assert.Equal(t, "weapon", be.Field)
	})
}

// ADR 0037 step 11 on the weapon pool: slots_over_budget {pool weapon, spent, available}, the
// reminder slots_to_spend {pool weapon, left}, none when spent exactly. A level 1 bard has 2
// weapon slots and, Intelligence 9, 5 non-weapon slots (spent exactly here).
func TestWarnings_Step11_WeaponPool(t *testing.T) {
	book := profBook(t)
	warnings := func(types ...string) []charactersvc.Warning {
		ps := weapons(types...)
		ps.Nonweapon = []charactersvc.Pick{p("tumbling", 5)}
		f, ok := charactersvc.ProficiencyFigures(book, "bard", 1, 9, charactersvc.Spending{Picks: ps})
		require.True(t, ok)
		return charactersvc.SlotWarnings(f)
	}
	three, two, one := 3, 2, 1
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_over_budget", Field: "weapon", Pool: "weapon", Spent: &three, Available: &two}},
		warnings("dagger", "dart", "sling"))
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &one}}, warnings("dagger"))
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &two}}, warnings())
	assert.Empty(t, warnings("dagger", "dart"), "spent exactly")
}

// ADR 0037 step 3 on the read: the weapon pool follows the realized level, never the target
// level; Kelsen (rogue, level 15) has 2 + 3 = 5 weapon slots, a rogue gains its next at 16, which
// a pending level-up does not grant. Each pick says whether the class may pick its type.
func TestDerive_Step3_WeaponSlotsFollowTheRealizedLevel(t *testing.T) {
	for _, target := range []int{15, 16} {
		e := newEnvWithBook(t, profBook(t))
		k := kelsen()
		k.Proficiencies = weapons("other_melee", "long_sword", "short_bow", "dagger")
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenAt(e, target)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		require.NotNil(t, got.Derived.Proficiencies)
		assert.Equal(t, ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: 3, Available: 5, Spent: 4, Left: 1},
			got.Derived.Proficiencies.Weapon, "target %d", target)
		one := 1
		assert.Contains(t, got.Warnings, charactersvc.Warning{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &one})
		assert.Equal(t, charactersvc.WeaponPickFigures{Type: "other_melee", Slots: 1, Allowed: true}, got.Derived.Proficiencies.WeaponPicks[0])
	}
	t.Run("a stored type the class may not pick reads as not allowed", func(t *testing.T) {
		f, ok := charactersvc.ProficiencyFigures(profBook(t), "mage", 1, 9, charactersvc.Spending{Picks: weapons("dagger", "long_sword")})
		require.True(t, ok)
		assert.Equal(t, []charactersvc.WeaponPickFigures{
			{Type: "dagger", Slots: 1, Allowed: true}, {Type: "long_sword", Slots: 1, Allowed: false},
		}, f.WeaponPicks)
	})
}

// ADR 0037 steps 7, 10 and 14: the creation takes weapon picks, checked as the editor checks them
// with nothing picked before: within budget stored; over budget, or a type the class may not
// pick, refused with nothing written. A level 1 bard has 2 weapon slots.
func TestCreate_Step14_WeaponPicksCheckedAsTheEditor(t *testing.T) {
	withPicks := func(ps charactersvc.Picks) charactersvc.NewCharacter {
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
		e.rules.EXPECT().RaceAllows("human", "mage").Return(true, nil).Maybe()
		_, err := e.svc.Create(context.Background(), ownerID, n)
		assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}, fieldErr(t, err))
		assert.Zero(t, e.tx.n)
	})
	t.Run("the checks endpoint answers the weapon pool", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Once()
		xp := 0
		got, err := e.svc.CreationChecks(context.Background(), charactersvc.DraftInput{
			Class: "bard", XP: &xp, Scores: map[string]int{"int": 14, "dex": 15}, Proficiencies: weapons("dagger"),
		})
		require.NoError(t, err)
		require.NotNil(t, got.ProficiencySlots)
		assert.Equal(t, ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, Available: 2, Spent: 1, Left: 1}, got.ProficiencySlots.Weapon)
	})
}

// ADR 0037 step 13: weapon picks are equal whatever their order; another type is a change.
func TestPicksEqual_Step13_WeaponOrderDoesNotCount(t *testing.T) {
	assert.True(t, weapons("dagger", "dart").Equal(weapons("dart", "dagger")))
	assert.False(t, weapons("dagger", "dart").Equal(weapons("dagger", "sling")))
	assert.False(t, weapons("dagger").Equal(weapons("dagger", "dart")))
}
