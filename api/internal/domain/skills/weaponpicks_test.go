package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// catalogueCodes are every weapon code of the fixture catalogue, in offer order.
func catalogueCodes(t *testing.T) []string {
	var out []string
	for _, w := range profBook(t).Weapons() {
		out = append(out, w.Code)
	}
	return out
}

// ADR 0037 step 21.1: a weapon pick names one catalogue weapon code, once, 1 slot: the
// quarterstaff and the heavy horse lance, both other_melee, are two picks spending 2 slots (one
// other_melee type pick covered both before); a code the catalogue does not hold is unknown, a
// second pick of a code duplicate. A stored code the catalogue does not hold is kept (older data
// is never dropped), never added. A pick without a code (an older client's type pick) is empty on
// the pool's field.
func TestWeaponPick_Step21_1_OnePickPerCatalogueCode(t *testing.T) {
	book := profBook(t)
	refused := func(field, code string) *domain.FieldError { return &domain.FieldError{Field: field, Message: code} }
	for _, tc := range []struct {
		name   string
		picks  skills.Picks
		before skills.Picks
		want   *domain.FieldError
	}{
		{"the quarterstaff and the heavy horse lance are two picks", weapons("quarterstaff", "heavy_horse_lance"), weapons(), nil},
		{"every catalogue weapon", weapons(catalogueCodes(t)...), weapons(), nil},
		{"a weapon type is no code", weapons("other_melee"), weapons(), refused("weapon.other_melee", "unknown")},
		{"an unknown code", weapons("katana"), weapons(), refused("weapon.katana", "unknown")},
		{"a stored unknown code is kept", weapons("dagger", "katana"), weapons("katana"), nil},
		{"a stored unknown code spends 1 slot", skills.Picks{Weapon: []skills.WeaponPick{{Code: "katana", Slots: 2}}}, weapons("katana"), refused("weapon.katana", "out_of_range")},
		{"no code", weapons(""), weapons(), refused("weapon", "empty")},
		{"a duplicate code", weapons("quarterstaff", "quarterstaff"), weapons(), refused("weapon.quarterstaff", "duplicate")},
		{"no slot", skills.Picks{Weapon: []skills.WeaponPick{{Code: "dagger"}}}, weapons(), refused("weapon.dagger", "out_of_range")},
		{"three slots for a dagger (not its specialization's 2)", skills.Picks{Weapon: []skills.WeaponPick{{Code: "dagger", Slots: 3}}}, weapons(), refused("weapon.dagger", "out_of_range")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := skills.ValidPicks(book, "fighter", tc.picks, tc.before)
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks.Weapon, got.Weapon)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
	t.Run("two picks spend two slots", func(t *testing.T) {
		f, ok := skills.ProficiencyFigures(book, "fighter", 1, 9, skills.Spending{Picks: weapons("quarterstaff", "heavy_horse_lance")})
		require.True(t, ok)
		assert.Equal(t, 2, f.Weapon.Spent)
	})
}

// ADR 0037 step 21.2: a class may pick a catalogue weapon its equipment allows by the wielding
// warnings' rule, any other code weapon_not_allowed. For each fixture class an allowed and a
// refused code: a mage the quarterstaff, not the club nor the long sword; a cleric the lucern
// hammer (a blunt other_melee row) and the sling by its bullets, not the halberd (an other_melee
// row that is not blunt, which an other_melee type pick made proficient before) nor a bow; a thief
// the broad sword, not the long bow; a fighter and a bard any weapon. A code outside the list that
// the stored picks already have is kept (the migration keeps every owned weapon), never added.
// Wielding is not this check's (ADR 0033 step 20 only warns).
func TestWeaponPick_Step21_2_TheClassEquipmentDecides(t *testing.T) {
	book := profBook(t)
	refused := func(code string) *domain.FieldError {
		return &domain.FieldError{Field: "weapon." + code, Message: "weapon_not_allowed"}
	}
	for _, tc := range []struct {
		name   string
		class  string
		picks  skills.Picks
		before skills.Picks
		want   *domain.FieldError
	}{
		{"a mage: its daggers, darts, slings, knife and quarterstaff", "mage", weapons("dagger", "dart", "sling", "knife", "quarterstaff"), weapons(), nil},
		{"a mage refused a club", "mage", weapons("club"), weapons(), refused("club")},
		{"a mage refused a long sword", "mage", weapons("dagger", "long_sword"), weapons(), refused("long_sword")},
		{"a cleric: the lucern hammer, the slings", "cleric", weapons("lucern_hammer", "sling", "staff_sling", "footmans_mace"), weapons(), nil},
		{"a cleric refused the halberd", "cleric", weapons("halberd"), weapons(), refused("halberd")},
		{"a cleric refused a bow", "cleric", weapons("long_bow"), weapons(), refused("long_bow")},
		{"a thief: its list", "thief", weapons("short_bow", "composite_short_bow", "hand_crossbow", "long_sword", "broad_sword"), weapons(), nil},
		{"a thief refused a long bow", "thief", weapons("long_bow"), weapons(), refused("long_bow")},
		{"a thief refused a scimitar", "thief", weapons("scimitar"), weapons(), refused("scimitar")},
		{"a fighter: any weapon", "fighter", weapons(catalogueCodes(t)...), weapons(), nil},
		{"a bard: any weapon", "bard", weapons(catalogueCodes(t)...), weapons(), nil},
		{"a stored code outside the list is kept", "mage", weapons("dagger", "long_sword"), weapons("long_sword"), nil},
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
	t.Run("the figures serve a kept refused code as not allowed", func(t *testing.T) {
		f, ok := skills.ProficiencyFigures(book, "cleric", 3, 12, skills.Spending{Picks: weapons("halberd", "lucern_hammer")})
		require.True(t, ok)
		assert.Equal(t, []skills.WeaponPickFigures{{Code: "halberd", Slots: 1}, {Code: "lucern_hammer", Slots: 1, Allowed: true}}, f.WeaponPicks)
	})
}

// spec is a weapon pick of code at slots.
func spec(code string, slots int) skills.WeaponPick {
	return skills.WeaponPick{Code: code, Slots: slots}
}

// ADR 0037 step 21.5 (amending step 9, PHB p.52-53): specialization is a fighter's pick of one
// catalogue weapon at the cost its type gives, 2 slots for a melee weapon or a crossbow, 3 for a
// bow (2 for darts, slings and the other thrown and missile types, step 9's proposed reading); a
// composite long bow costs 3 as a long bow does. Any other count is out_of_range, another class's
// specialization and a second weapon are not_allowed. Its slots count in the weapon pool's budget
// (step 10). The weapon specialized is the pick's code: never for a plain pick, never for a class
// that does not specialize.
func TestSpecialization_Step21_5_OneWeaponAtItsTypesCost(t *testing.T) {
	book := profBook(t)
	refused := func(field, code string) *domain.FieldError { return &domain.FieldError{Field: field, Message: code} }
	picks := func(ps ...skills.WeaponPick) skills.Picks { return skills.Picks{Weapon: ps} }
	tests := []struct {
		name  string
		class string
		picks skills.Picks
		want  *domain.FieldError
	}{
		{"a composite long bow, 3 slots", "fighter", picks(spec("composite_long_bow", 3), spec("dagger", 1)), nil},
		{"a short bow, 3 slots", "fighter", picks(spec("short_bow", 3)), nil},
		{"a long sword, 2 slots", "fighter", picks(spec("long_sword", 2)), nil},
		{"a heavy horse lance, 2 slots", "fighter", picks(spec("heavy_horse_lance", 2)), nil},
		{"a long bow, 2 slots", "fighter", picks(spec("long_bow", 2)), refused("weapon.long_bow", "out_of_range")},
		{"a melee weapon, 3 slots", "fighter", picks(spec("long_sword", 3)), refused("weapon.long_sword", "out_of_range")},
		{"4 slots", "fighter", picks(spec("long_sword", 4)), refused("weapon.long_sword", "out_of_range")},
		{"a mage refused", "mage", picks(spec("dagger", 2)), refused("weapon.dagger", "not_allowed")},
		{"a bard refused", "bard", picks(spec("long_sword", 2)), refused("weapon.long_sword", "not_allowed")},
		{"a second weapon refused", "fighter", picks(spec("long_sword", 2), spec("broad_sword", 2)), refused("weapon.broad_sword", "not_allowed")},
	}
	twoSlots := []string{
		"dagger", "short_sword", "long_sword", "quarterstaff", "light_crossbow", "heavy_crossbow", "hand_crossbow",
		"dart", "sling", "staff_sling", "javelin", "blowgun",
	}
	for _, code := range twoSlots {
		tests = append(tests, struct {
			name  string
			class string
			picks skills.Picks
			want  *domain.FieldError
		}{code + ", 2 slots", "fighter", picks(spec(code, 2)), nil})
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
		within := picks(spec("composite_long_bow", 3), spec("dagger", 1))
		require.NoError(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: weapons()}, skills.Spending{Picks: within}))
		over := picks(spec("composite_long_bow", 3), spec("dagger", 1), spec("dart", 1))
		var be *domain.BudgetError
		require.ErrorAs(t, skills.PicksWithinBudget(book, "fighter", 1, 9, skills.Spending{Picks: weapons()}, skills.Spending{Picks: over}), &be)
		assert.Equal(t, domain.BudgetError{Field: "weapon", Spent: 5, Limit: 4}, *be)
	})

	t.Run("the weapon specialized is the pick's code", func(t *testing.T) {
		p := picks(spec("long_sword", 2), spec("broad_sword", 1))
		assert.Equal(t, "long_sword", skills.SpecializedCode(book, "fighter", p))
		assert.Empty(t, skills.SpecializedCode(book, "fighter", picks(spec("dagger", 1))), "a plain pick")
		assert.Empty(t, skills.SpecializedCode(book, "fighter", skills.Picks{}), "no pick")
		assert.Empty(t, skills.SpecializedCode(book, "bard", p), "a class that does not specialize")
		assert.Equal(t, "composite_long_bow", skills.SpecializedCode(book, "fighter", picks(spec("composite_long_bow", 3))))
	})
}

// ADR 0037 step 12: an editor of one pool leaves the other as stored. A nil pool in the edit is
// the stored one; an empty list removes every pick of the pool.
func TestValidPicks_Step12_AnAbsentPoolKeepsTheStoredOne(t *testing.T) {
	book := profBook(t)
	stored := skills.Picks{
		Nonweapon: []skills.Pick{p("tumbling", 1)},
		Weapon:    []skills.WeaponPick{{Code: "dagger", Slots: 1}},
	}
	got, err := skills.ValidPicks(book, "bard", skills.Picks{Nonweapon: []skills.Pick{p("cooking", 1)}}, stored)
	require.NoError(t, err)
	assert.Equal(t, skills.Picks{Nonweapon: []skills.Pick{p("cooking", 1)}, Weapon: stored.Weapon}, got)

	got, err = skills.ValidPicks(book, "bard", skills.Picks{Weapon: []skills.WeaponPick{{Code: "dart", Slots: 1}}}, stored)
	require.NoError(t, err)
	assert.Equal(t, skills.Picks{Nonweapon: stored.Nonweapon, Weapon: []skills.WeaponPick{{Code: "dart", Slots: 1}}}, got)

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
	warnings := func(codes ...string) []skills.Warning {
		ps := weapons(codes...)
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

// ADR 0037 step 13: weapon picks are equal whatever their order; another weapon is a change.
func TestPicksEqual_Step13_WeaponOrderDoesNotCount(t *testing.T) {
	assert.True(t, weapons("dagger", "dart").Equal(weapons("dart", "dagger")))
	assert.False(t, weapons("dagger", "dart").Equal(weapons("dagger", "sling")))
	assert.False(t, weapons("dagger").Equal(weapons("dagger", "dart")))
}

// ADR 0037 step 21.3 read from the character row: a weapon whose catalogue code no pick names
// takes Table 34's penalty for the class group, a picked code (a kept one the class may not take
// included) none; a weapon with no code is non-proficient (step 21.4). The specialization is step
// 21.5's, by code: the long bow is no composite long bow.
func TestWeaponProficiency_Step21_3_PenaltyByTheWeaponsCode(t *testing.T) {
	book := profBook(t)
	mage := skills.WeaponProficiencyOf(book, skills.One("mage", 1), skills.Picks{Weapon: []skills.WeaponPick{
		{Code: "dagger", Slots: 1}, {Code: "long_sword", Slots: 1},
	}})
	assert.Equal(t, -5, mage.NonproficientPenalty("sling"))
	assert.Zero(t, mage.NonproficientPenalty("dagger"))
	assert.Zero(t, mage.NonproficientPenalty("long_sword"), "a kept pick the class may not take still counts")
	assert.Equal(t, -5, mage.NonproficientPenalty(""), "a weapon with no code is non-proficient")
	assert.Empty(t, mage.Specialized)

	fighter := skills.WeaponProficiencyOf(book, skills.One("fighter", 1), skills.Picks{Weapon: []skills.WeaponPick{
		{Code: "quarterstaff", Slots: 1}, {Code: "composite_long_bow", Slots: 3},
	}})
	assert.Equal(t, "composite_long_bow", fighter.Specialized)
	assert.Equal(t, -2, fighter.NonproficientPenalty("heavy_horse_lance"), "another other_melee weapon")
	assert.Zero(t, fighter.NonproficientPenalty("quarterstaff"))
	assert.Equal(t, -2, fighter.NonproficientPenalty("long_bow"), "another long_bow weapon")

	none := skills.WeaponProficiencyOf((*ruletablesvc.Proficiencies)(nil), skills.One("fighter", 1), skills.Picks{})
	assert.Zero(t, none.NonproficientPenalty("dagger"), "without Table 34 no penalty")
	assert.Zero(t, none.NonproficientPenalty(""), "without Table 34 no penalty")

	t.Run("the derived figures judge the same", func(t *testing.T) {
		f, ok := skills.ProficiencyFigures(book, "fighter", 1, 9, skills.Spending{Picks: weapons("quarterstaff")})
		require.True(t, ok)
		assert.Zero(t, f.NonproficientPenalty("quarterstaff"))
		assert.Equal(t, -2, f.NonproficientPenalty("heavy_horse_lance"))
		assert.Equal(t, -2, f.NonproficientPenalty(""))
		assert.Zero(t, (*skills.ProficiencySlots)(nil).NonproficientPenalty(""), "no figures, no penalty")
	})
}

// ADR 0042 step 15 (Q12) on the attack: a multi-class character's non-proficiency penalty is the
// smallest of its active classes', whichever class comes first (a mage/fighter takes the
// fighter's -2, not the mage's -5); the specialization stays the first class's.
func TestWeaponProficiency_Step15_SmallestPenaltyOfTheActiveClasses(t *testing.T) {
	book := profBook(t)
	picks := skills.Picks{Weapon: []skills.WeaponPick{{Code: "dagger", Slots: 1}}}
	mageFighter := skills.WeaponProficiencyOf(book, skills.Classes{{Class: "mage", Level: 3}, {Class: "fighter", Level: 4}}, picks)
	assert.Equal(t, -2, mageFighter.NonproficientPenalty("long_sword"))
	assert.Zero(t, mageFighter.NonproficientPenalty("dagger"))
	mageThief := skills.WeaponProficiencyOf(book, skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, picks)
	assert.Equal(t, -3, mageThief.NonproficientPenalty("long_sword"), "the rogue's -3, not the wizard's -5")
}

// ADR 0042 step 16.1 as amended by ADR 0037 steps 21.2 and 21.5: a multi-class character may
// pick a weapon any of its active classes may, except that a blunt_only class restricts the picks
// to its own weapons whatever the others, unless the ruleset's weapon lift lifts it. A
// cleric/fighter is refused the long sword and picks the footman's mace, and both under the lift;
// a thief/fighter picks a long bow (the thief alone is refused it); a mage/thief is refused a long
// bow (neither class allows it). The figures judge the stored picks the same way.
func TestWeaponPick_Step16_1_ABluntOnlyEntryRestrictsTheWeapons(t *testing.T) {
	book := profBook(t)
	refused := func(field string) *domain.FieldError {
		return &domain.FieldError{Field: field, Message: "weapon_not_allowed"}
	}
	clericFighter := skills.Classes{{Class: "cleric", Level: 3}, {Class: "fighter", Level: 3}}
	for _, tc := range []struct {
		name    string
		classes skills.Classes
		lifted  bool
		picks   skills.Picks
		want    *domain.FieldError
	}{
		{"a cleric/fighter refused the long sword", clericFighter, false, weapons("long_sword"), refused("weapon.long_sword")},
		{"a fighter/cleric refused the long sword", skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, false, weapons("long_sword"), refused("weapon.long_sword")},
		{"a cleric/fighter picks the footman's mace and a sling", clericFighter, false, weapons("footmans_mace", "sling"), nil},
		{"a cleric/fighter under the lift picks both", clericFighter, true, weapons("long_sword", "footmans_mace"), nil},
		{"a thief/fighter picks a long bow", skills.Classes{{Class: "thief", Level: 3}, {Class: "fighter", Level: 3}}, false, weapons("long_bow", "long_sword"), nil},
		{"a mage/thief picks a long sword", skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, false, weapons("long_sword"), nil},
		{"a mage/thief refused a long bow", skills.Classes{{Class: "mage", Level: 3}, {Class: "thief", Level: 3}}, false, weapons("long_bow"), refused("weapon.long_bow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: tc.lifted}
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
	spec := skills.Picks{Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: fighterSlots(t, book, "long_sword")}}}
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

// ADR 0042 step 16.1 as amended by ADR 0037 step 21.6: the figures list the catalogue weapons the
// character may pick, in offer order, by the rule a save is judged by, so a cleric-first
// cleric/fighter is offered the long sword only under the ruleset's weapon lift, whichever class
// comes first; a single-class cleric never is. A specialist's weapons carry their type's cost, a
// mage's none.
func TestWeapons_Step16_1_TheOfferedWeaponsFollowTheAcceptedPicks(t *testing.T) {
	book := profBook(t)
	offered := func(cls skills.Classes, lift bool) []string {
		got, _ := skills.Read(book, skills.Sheet{Classes: cls, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: lift})
		require.NotNil(t, got)
		var out []string
		for _, w := range got.Weapons {
			out = append(out, w.Code)
		}
		return out
	}
	clericFighter := skills.Classes{{Class: "cleric", Level: 3}, {Class: "fighter", Level: 3}}
	assert.Contains(t, offered(clericFighter, true), "long_sword")
	assert.NotContains(t, offered(clericFighter, false), "long_sword")
	assert.Contains(t, offered(clericFighter, false), "lucern_hammer", "the cleric's own weapons stay")
	assert.NotContains(t, offered(skills.One("cleric", 3), true), "long_sword", "the lift is for several classes")
	assert.Equal(t, []string{"dagger", "dart", "sling", "knife", "quarterstaff"}, offered(skills.One("mage", 1), false),
		"the mage's five, in the weapon types' order then the catalogue's")
	_, err := skills.Apply(book, nil, "human", skills.Sheet{Classes: clericFighter, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: true},
		skills.Edit{Picks: weapons("long_sword")})
	require.NoError(t, err, "what is offered is accepted")

	fighter, _ := skills.Read(book, skills.Sheet{Classes: skills.One("fighter", 1), Intelligence: 12, Picks: weapons()})
	require.NotNil(t, fighter)
	assert.Equal(t, skills.WeaponFigure{Code: "composite_long_bow", SpecializationSlots: 3}, fighter.Weapons[0])
	assert.Contains(t, fighter.Weapons, skills.WeaponFigure{Code: "heavy_horse_lance", SpecializationSlots: 2})
	mage, _ := skills.Read(book, skills.Sheet{Classes: skills.One("mage", 1), Intelligence: 12, Picks: weapons()})
	assert.Equal(t, skills.WeaponFigure{Code: "dagger"}, mage.Weapons[0], "no specialization for a mage")
}

// ADR 0037 step 21.8.3: a type pick the migration found no owned weapon for is listed to redo,
// warned weapon_pick_to_redo {type, slots} on field weapon.<type>, its slots back to spend (not
// spent); a save of the non-weapon pool alone keeps the list, any save of the weapon pool clears
// it (its undo, skillsvc's, brings it back).
func TestWarnings_Step21_8_3_RedoListedUntilTheNextWeaponSave(t *testing.T) {
	book := profBook(t)
	stored := weapons("dagger")
	stored.WeaponRedo = []skills.WeaponRedo{{Type: "other_melee", Slots: 1}}
	sheet := skills.Sheet{Class: "bard", Level: 1, Intelligence: 9, Picks: stored}
	f, ws := skills.Read(book, sheet)
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Weapon.Spent, "the type to redo spends nothing")
	one := 1
	assert.Contains(t, ws, skills.Warning{Code: "weapon_pick_to_redo", Field: "weapon.other_melee", Type: "other_melee", Slots: &one})
	assert.Contains(t, ws, skills.Warning{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &one}, "its slot back to spend")

	kept, err := skills.Apply(book, nil, "human", sheet, skills.Edit{Picks: skills.Picks{Nonweapon: []skills.Pick{p("tumbling", 1)}}})
	require.NoError(t, err)
	assert.Equal(t, stored.WeaponRedo, kept.Picks.WeaponRedo, "a non-weapon save keeps the list")

	saved, err := skills.Apply(book, nil, "human", sheet, skills.Edit{Picks: skills.Picks{Weapon: weapons("dagger", "quarterstaff").Weapon}})
	require.NoError(t, err)
	assert.Empty(t, saved.Picks.WeaponRedo, "a weapon save clears it")
	_, ws = skills.Read(book, saved)
	for _, w := range ws {
		assert.NotEqual(t, "weapon_pick_to_redo", w.Code)
	}
	same, err := skills.Apply(book, nil, "human", sheet, skills.Edit{Picks: skills.Picks{Weapon: weapons("dagger").Weapon}})
	require.NoError(t, err)
	assert.Empty(t, same.Picks.WeaponRedo)
	assert.False(t, stored.Equal(same.Picks), "the same weapons with the list cleared is a change")
}
