package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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
		picks  skills.Picks
		before skills.Picks
		want   *domain.FieldError
	}{
		{"a mage: dagger, dart, sling, other_melee", "mage", weapons("dagger", "dart", "sling", "other_melee"), weapons(), nil},
		{"a mage refused a long sword", "mage", weapons("dagger", "long_sword"), weapons(), refused("weapon.long_sword", "weapon_not_allowed")},
		{"a cleric: sling, other_melee, other_missile", "cleric", weapons("sling", "other_melee", "other_missile"), weapons(), nil},
		{"a cleric refused an edged weapon", "cleric", weapons("short_sword"), weapons(), refused("weapon.short_sword", "weapon_not_allowed")},
		{"a cleric refused a bow", "cleric", weapons("long_bow"), weapons(), refused("weapon.long_bow", "weapon_not_allowed")},
		{"a thief: its list", "thief", weapons("short_bow", "hand_crossbow", "long_sword", "other_melee"), weapons(), nil},
		{"a thief refused a type outside its list", "thief", weapons("long_bow"), weapons(), refused("weapon.long_bow", "weapon_not_allowed")},
		{"a thief refused a heavy crossbow", "thief", weapons("heavy_crossbow"), weapons(), refused("weapon.heavy_crossbow", "weapon_not_allowed")},
		{"a fighter: any type", "fighter", weapons(allWeaponTypes...), weapons(), nil},
		{"a bard: any type", "bard", weapons(allWeaponTypes...), weapons(), nil},
		{"a stored type outside the list is kept", "mage", weapons("dagger", "long_sword"), weapons("long_sword"), nil},
		{"an unknown type", "fighter", weapons("katana"), weapons(), refused("weapon.katana", "unknown")},
		{"a stored unknown type is kept", "mage", weapons("dagger", "katana"), weapons("katana"), nil},
		{"a stored unknown type spends 1 slot", "fighter", skills.Picks{Weapon: []skills.WeaponPick{{Type: "katana", Slots: 2}}}, weapons("katana"), refused("weapon.katana", "out_of_range")},
		{"no type", "fighter", weapons(""), weapons(), refused("weapon", "empty")},
		{"a duplicate type", "fighter", weapons("dagger", "dagger"), weapons(), refused("weapon.dagger", "duplicate")},
		{"no slot", "fighter", skills.Picks{Weapon: []skills.WeaponPick{{Type: "dagger"}}}, weapons(), refused("weapon.dagger", "out_of_range")},
		{"three slots for a dagger (not its specialization's 2)", "fighter", skills.Picks{Weapon: []skills.WeaponPick{{Type: "dagger", Slots: 3}}}, weapons(), refused("weapon.dagger", "out_of_range")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := skills.ValidPicks(book, tc.class, tc.picks, tc.before)
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks.Weapon, got.Weapon)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
}

// spec is a weapon pick of slots.
func spec(weaponType string, slots int) skills.WeaponPick {
	return skills.WeaponPick{Type: weaponType, Slots: slots}
}

// ADR 0037 step 9 (PHB p.52-53): specialization is a fighter's weapon pick of one weapon type at
// the PHB's cost in all, 2 slots for a melee weapon or a crossbow, 3 for a bow (2 for darts,
// slings and the other thrown and missile types, the step's proposed reading); any other count
// is out_of_range, another class's specialization and a second one are not_allowed. The pick is
// the proficiency: a type is specialized only through its own pick, so a type not picked cannot
// be. Its slots count in the weapon pool's budget (step 10): a level 1 fighter's 4 slots take a
// bow's specialization and one plain pick, not two. The weapon is specialized when the fighter's
// picks hold its type's specialization (Character.Specialized): never for a plain pick, never
// for a class that does not specialize.
func TestSpecialization_Step9_FighterOnlyOneTypeAtThePhbCost(t *testing.T) {
	book := profBook(t)
	refused := func(field, code string) *domain.FieldError { return &domain.FieldError{Field: field, Message: code} }
	picks := func(ps ...skills.WeaponPick) skills.Picks { return skills.Picks{Weapon: ps} }
	tests := []struct {
		name  string
		class string
		picks skills.Picks
		want  *domain.FieldError
	}{
		{"a bow, 3 slots", "fighter", picks(spec("long_bow", 3), spec("dagger", 1)), nil},
		{"a short bow, 3 slots", "fighter", picks(spec("short_bow", 3)), nil},
		{"a bow, 2 slots", "fighter", picks(spec("long_bow", 2)), refused("weapon.long_bow", "out_of_range")},
		{"a melee weapon, 3 slots", "fighter", picks(spec("long_sword", 3)), refused("weapon.long_sword", "out_of_range")},
		{"4 slots", "fighter", picks(spec("long_sword", 4)), refused("weapon.long_sword", "out_of_range")},
		{"a mage refused", "mage", picks(spec("dagger", 2)), refused("weapon.dagger", "not_allowed")},
		{"a bard refused", "bard", picks(spec("long_sword", 2)), refused("weapon.long_sword", "not_allowed")},
		{"a second type refused", "fighter", picks(spec("long_sword", 2), spec("dagger", 2)), refused("weapon.dagger", "not_allowed")},
	}
	twoSlots := []string{
		"dagger", "short_sword", "long_sword", "other_melee", "light_crossbow", "heavy_crossbow", "hand_crossbow",
		"dart", "sling", "other_thrown", "other_missile",
	}
	for _, typ := range twoSlots {
		tests = append(tests, struct {
			name  string
			class string
			picks skills.Picks
			want  *domain.FieldError
		}{typ + ", 2 slots", "fighter", picks(spec(typ, 2)), nil})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := skills.ValidPicks(book, tc.class, tc.picks, weapons())
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks.Weapon, got.Weapon)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}

	t.Run("the budget counts its slots", func(t *testing.T) {
		within := picks(spec("long_bow", 3), spec("dagger", 1))
		require.NoError(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: weapons()}, skills.Spending{Picks: within}))
		over := picks(spec("long_bow", 3), spec("dagger", 1), spec("dart", 1))
		var be *domain.BudgetError
		require.ErrorAs(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: weapons()}, skills.Spending{Picks: over}), &be)
		assert.Equal(t, domain.BudgetError{Field: "weapon", Spent: 5, Limit: 4}, *be)
	})

	t.Run("the weapon is specialized from the pick", func(t *testing.T) {
		p := picks(spec("long_sword", 2), spec("dagger", 1))
		assert.Equal(t, "long_sword", skills.SpecializedType(book, "fighter", p))
		assert.Empty(t, skills.SpecializedType(book, "fighter", picks(spec("dagger", 1))), "a plain pick")
		assert.Empty(t, skills.SpecializedType(book, "fighter", skills.Picks{}), "no pick")
		assert.Empty(t, skills.SpecializedType(book, "bard", p), "a class that does not specialize")
	})
}

// ADR 0037 step 12: an editor of one pool leaves the other as stored. A nil pool in the edit is
// the stored one; an empty list removes every pick of the pool.
func TestValidPicks_Step12_AnAbsentPoolKeepsTheStoredOne(t *testing.T) {
	book := profBook(t)
	stored := skills.Picks{
		Nonweapon: []skills.Pick{p("tumbling", 1)},
		Weapon:    []skills.WeaponPick{{Type: "dagger", Slots: 1}},
	}
	got, err := skills.ValidPicks(book, "bard", skills.Picks{Nonweapon: []skills.Pick{p("cooking", 1)}}, stored)
	require.NoError(t, err)
	assert.Equal(t, skills.Picks{Nonweapon: []skills.Pick{p("cooking", 1)}, Weapon: stored.Weapon}, got)

	got, err = skills.ValidPicks(book, "bard", skills.Picks{Weapon: []skills.WeaponPick{{Type: "dart", Slots: 1}}}, stored)
	require.NoError(t, err)
	assert.Equal(t, skills.Picks{Nonweapon: stored.Nonweapon, Weapon: []skills.WeaponPick{{Type: "dart", Slots: 1}}}, got)

	got, err = skills.ValidPicks(book, "bard", skills.Picks{Weapon: []skills.WeaponPick{}}, stored)
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
		before, after skills.Picks
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
			err := skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: tc.before}, skills.Spending{Picks: tc.after})
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
		before := skills.Picks{Nonweapon: []skills.Pick{p("tracking", 7)}, Weapon: five.Weapon}
		after := skills.Picks{Nonweapon: []skills.Pick{p("tracking", 2)}, Weapon: five.Weapon}
		require.NoError(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: before}, skills.Spending{Picks: after}))
		after = skills.Picks{Nonweapon: []skills.Pick{p("tracking", 7)}, Weapon: six.Weapon}
		var be *domain.BudgetError
		require.ErrorAs(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: before}, skills.Spending{Picks: after}), &be)
		assert.Equal(t, "weapon", be.Field)
	})
}

// ADR 0037 step 11 on the weapon pool: slots_over_budget {pool weapon, spent, available}, the
// reminder slots_to_spend {pool weapon, left}, none when spent exactly. A level 1 bard has 2
// weapon slots and, Intelligence 9, 5 non-weapon slots (spent exactly here).
func TestWarnings_Step11_WeaponPool(t *testing.T) {
	book := profBook(t)
	warnings := func(types ...string) []skills.Warning {
		ps := weapons(types...)
		ps.Nonweapon = []skills.Pick{p("tumbling", 5)}
		f, ok := skills.ProficiencyFigures(book, "bard", 1, 9, skills.Spending{Picks: ps})
		require.True(t, ok)
		return skills.SlotWarnings(f)
	}
	three, two, one := 3, 2, 1
	assert.Equal(t, []skills.Warning{{Code: "slots_over_budget", Field: "weapon", Pool: "weapon", Spent: &three, Available: &two}},
		warnings("dagger", "dart", "sling"))
	assert.Equal(t, []skills.Warning{{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &one}}, warnings("dagger"))
	assert.Equal(t, []skills.Warning{{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &two}}, warnings())
	assert.Empty(t, warnings("dagger", "dart"), "spent exactly")
}

// ADR 0037 step 13: weapon picks are equal whatever their order; another type is a change.
func TestPicksEqual_Step13_WeaponOrderDoesNotCount(t *testing.T) {
	assert.True(t, weapons("dagger", "dart").Equal(weapons("dart", "dagger")))
	assert.False(t, weapons("dagger", "dart").Equal(weapons("dagger", "sling")))
	assert.False(t, weapons("dagger").Equal(weapons("dagger", "dart")))
}

// ADR 0037 step 8 read from the character row: a weapon type without a pick takes Table 34's
// penalty for the class group, a picked type (a kept one the class may not take included) and a
// weapon with no type none; the specialization is step 9's.
func TestWeaponProficiency_Step8_PenaltyFromThePicks(t *testing.T) {
	book := profBook(t)
	mage := skills.WeaponProficiencyOf(book, skills.One("mage", 1), skills.Picks{Weapon: []skills.WeaponPick{
		{Type: "dagger", Slots: 1}, {Type: "long_sword", Slots: 1},
	}})
	assert.Equal(t, -5, mage.NonproficientPenalty("sling"))
	assert.Zero(t, mage.NonproficientPenalty("dagger"))
	assert.Zero(t, mage.NonproficientPenalty("long_sword"), "a kept pick the class may not take still counts")
	assert.Zero(t, mage.NonproficientPenalty(""), "a weapon with no type takes none")
	assert.Empty(t, mage.Specialized)

	fighter := skills.WeaponProficiencyOf(book, skills.One("fighter", 1), skills.Picks{Weapon: []skills.WeaponPick{{Type: "long_bow", Slots: 3}}})
	assert.Equal(t, "long_bow", fighter.Specialized)
	assert.Equal(t, -2, fighter.NonproficientPenalty("dagger"))

	none := skills.WeaponProficiencyOf((*ruletablesvc.Proficiencies)(nil), skills.One("fighter", 1), skills.Picks{})
	assert.Zero(t, none.NonproficientPenalty("dagger"), "without Table 34 no penalty")
}

// ADR 0042 step 15 (Q12) on the attack: a multi-class character's non-proficiency penalty is the
// smallest of its active classes', whichever class comes first (a mage/fighter takes the
// fighter's -2, not the mage's -5); the specialization stays the first class's.
func TestWeaponProficiency_Step15_SmallestPenaltyOfTheActiveClasses(t *testing.T) {
	book := profBook(t)
	picks := skills.Picks{Weapon: []skills.WeaponPick{{Type: "dagger", Slots: 1}}}
	mageFighter := skills.WeaponProficiencyOf(book, skills.Classes{{Class: "mage", Level: 3}, {Class: "fighter", Level: 4}}, picks)
	assert.Equal(t, -2, mageFighter.NonproficientPenalty("long_sword"))
	assert.Zero(t, mageFighter.NonproficientPenalty("dagger"))
	mageThief := skills.WeaponProficiencyOf(book, skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, picks)
	assert.Equal(t, -3, mageThief.NonproficientPenalty("long_sword"), "the rogue's -3, not the wizard's -5")
}

// ADR 0042 step 16.1 on the weapon picks (ADR 0037 step 7): a multi-class character may pick a
// type any of its active classes may, except that a blunt_only class restricts the picks to its
// own list whatever the others. A fighter/cleric is refused a long sword (the fighter listed
// first: a check of the first class alone allows it) and picks a sling; a thief/fighter picks a
// long bow (the thief alone is refused it); a mage/thief is refused a long bow (neither class
// lists it). The figures judge the stored picks the same way.
func TestWeaponPick_Step16_1_ABluntOnlyEntryRestrictsThePicks(t *testing.T) {
	book := profBook(t)
	refused := func(field string) *domain.FieldError {
		return &domain.FieldError{Field: field, Message: "weapon_not_allowed"}
	}
	for _, tc := range []struct {
		name    string
		classes skills.Classes
		picks   skills.Picks
		want    *domain.FieldError
	}{
		{"a fighter/cleric refused a long sword", skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, weapons("long_sword"), refused("weapon.long_sword")},
		{"a fighter/cleric picks a sling", skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, weapons("sling", "other_melee"), nil},
		{"a thief/fighter picks a long bow", skills.Classes{{Class: "thief", Level: 3}, {Class: "fighter", Level: 3}}, weapons("long_bow", "long_sword"), nil},
		{"a mage/thief picks a long sword", skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, weapons("long_sword"), nil},
		{"a mage/thief refused a long bow", skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, weapons("long_bow"), refused("weapon.long_bow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons()}
			got, err := skills.Apply(book, nil, "human", before, skills.Edit{Picks: skills.Picks{Weapon: tc.picks.Weapon}})
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks.Weapon, got.Picks.Weapon)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
	t.Run("the figures judge the stored picks together", func(t *testing.T) {
		got, ok := skills.FiguresOf(book, skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, 12,
			skills.Spending{Picks: weapons("long_sword", "sling")})
		require.True(t, ok)
		require.Len(t, got.WeaponPicks, 2)
		assert.False(t, got.WeaponPicks[0].Allowed, "the cleric's blunt weapons only")
		assert.True(t, got.WeaponPicks[1].Allowed)
		thief, ok := skills.FiguresOf(book, skills.Classes{{Class: "thief", Level: 3}, {Class: "fighter", Level: 3}}, 12,
			skills.Spending{Picks: weapons("long_bow")})
		require.True(t, ok)
		assert.True(t, thief.WeaponPicks[0].Allowed, "the fighter's any weapon")
	})
}

// ADR 0042 as built (PHB p.52, Weapon Specialization: "multi-class characters cannot use weapon
// specialization; it is available only to single-class fighters"): a fighter/thief, the fighter
// first, is refused a specialization pick and a stored one specializes nothing; the fighter alone
// takes it.
func TestWeaponPick_Step15_OnlyASingleClassFighterSpecializes(t *testing.T) {
	book := profBook(t)
	spec := skills.Picks{Weapon: []skills.WeaponPick{{Type: "long_sword", Slots: ruletablesvc.SpecializationSlots("long_sword")}}}
	fighterThief := skills.Classes{{Class: "fighter", Level: 3}, {Class: "thief", Level: 3}}

	_, err := skills.Apply(book, nil, "human", skills.Sheet{Classes: fighterThief, Intelligence: 12, Picks: weapons()}, skills.Edit{Picks: spec})
	assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: skills.CodeNotAllowed}, fieldErr(t, err))
	assert.Empty(t, skills.WeaponProficiencyOf(book, fighterThief, spec).Specialized)

	fighter := skills.One("fighter", 3)
	got, err := skills.Apply(book, nil, "human", skills.Sheet{Classes: fighter, Intelligence: 12, Picks: weapons()}, skills.Edit{Picks: spec})
	require.NoError(t, err)
	assert.Equal(t, spec.Weapon, got.Picks.Weapon)
	assert.Equal(t, "long_sword", skills.WeaponProficiencyOf(book, fighter, spec).Specialized)
}

// ADR 0042 step 16.1 as built: the figures list the types the character may pick, by the rule a
// save is judged by, so a cleric-first cleric/fighter is offered the long sword only under the
// ruleset's weapon lift, whichever class comes first; a single-class cleric never is.
func TestWeaponTypes_Step16_1_TheOfferedTypesFollowTheAcceptedPicks(t *testing.T) {
	book := profBook(t)
	offered := func(cls skills.Classes, lift bool) []string {
		got, _ := skills.Read(book, skills.Sheet{Classes: cls, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: lift})
		require.NotNil(t, got)
		var out []string
		for _, w := range got.WeaponTypes {
			out = append(out, w.Type)
		}
		return out
	}
	clericFighter := skills.Classes{{Class: "cleric", Level: 3}, {Class: "fighter", Level: 3}}
	assert.Contains(t, offered(clericFighter, true), "long_sword")
	assert.NotContains(t, offered(clericFighter, false), "long_sword")
	assert.Contains(t, offered(clericFighter, false), "sling", "the cleric's own types stay")
	assert.NotContains(t, offered(skills.One("cleric", 3), true), "long_sword", "the lift is for several classes")
	_, err := skills.Apply(book, nil, "human", skills.Sheet{Classes: clericFighter, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: true},
		skills.Edit{Picks: weapons("long_sword")})
	require.NoError(t, err, "what is offered is accepted")
}
