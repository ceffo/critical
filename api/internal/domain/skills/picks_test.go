package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// profBook is a catalogue with PHB Table 34 and Table 38 as the seed holds them and a few Table 37
// rows: Tumbling (rogue, 1 slot, Dex), Tracking (warrior, 2, Wis), Healing (priest, 2, Wis -2),
// Cooking (general, 1, Int), Blind-fighting (rogue and warrior, 2, no check), Languages, Modern.
func profBook(t *testing.T) *ruletablesvc.Proficiencies {
	t.Helper()
	b, err := ruletablesvc.NewProficiencies([]ruletablesvc.Proficiency{
		{ID: "tumbling", Ability: "dex", Slots: 1, Groups: []string{"rogue"}},
		{ID: "tracking", Ability: "wis", Slots: 2, Groups: []string{"warrior"}},
		{ID: "healing", Ability: "wis", Modifier: -2, Slots: 2, Groups: []string{"priest"}},
		{ID: "cooking", Ability: "int", Slots: 1, Groups: []string{"general"}},
		{ID: "blind_fighting", Slots: 2, Groups: []string{"rogue", "warrior"}},
		{ID: "languages_modern", Ability: "int", Slots: 1, Groups: []string{"general"}},
	}, []ruletablesvc.Crossover{
		{Class: "fighter", Groups: []string{"warrior", "general"}},
		{Class: "mage", Groups: []string{"wizard", "general"}},
		{Class: "cleric", Groups: []string{"priest", "general"}},
		{Class: "thief", Groups: []string{"rogue", "general"}},
		{Class: "bard", Groups: []string{"rogue", "warrior", "wizard", "general"}},
	}, []ruletablesvc.SlotRow{
		{Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
		{Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
	})
	require.NoError(t, err)
	return b
}

func picks(ps ...skills.Pick) skills.Picks {
	return skills.Picks{Nonweapon: append([]skills.Pick{}, ps...), Weapon: []skills.WeaponPick{}}
}

// weapons are weapon picks of types, 1 slot each, and no non-weapon pick.
func weapons(types ...string) skills.Picks {
	out := skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{}}
	for _, t := range types {
		out.Weapon = append(out.Weapon, skills.WeaponPick{Type: t, Slots: 1})
	}
	return out
}

func p(id string, slots int) skills.Pick { return skills.Pick{ID: id, Slots: slots} }

func fieldErr(t *testing.T, err error) domain.FieldError {
	t.Helper()
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	return *fe
}

// ADR 0037 step 12: each id once, a catalogue id other than Languages, Modern (step 6), spending
// from its cost for the class (step 5) up to the bound; an id the catalogue does not hold is
// refused unless the stored picks already have it (step 16: older data is kept, at 1 slot or
// more).
func TestValidPicks_Step12_Refusals(t *testing.T) {
	book := profBook(t)
	old := picks(p("old_skill", 1))
	for _, tc := range []struct {
		name   string
		class  string
		picks  skills.Picks
		before skills.Picks
		want   *domain.FieldError
	}{
		{"in group at cost", "fighter", picks(p("tracking", 2)), picks(), nil},
		{"out of group at the plus-one cost", "fighter", picks(p("tumbling", 2)), picks(), nil},
		{"out of group at the table's cost", "fighter", picks(p("tumbling", 1)), picks(), &domain.FieldError{Field: "nonweapon.tumbling", Message: "out_of_range"}},
		{"under the cost", "bard", picks(p("tracking", 1)), picks(), &domain.FieldError{Field: "nonweapon.tracking", Message: "out_of_range"}},
		{"extra slots", "bard", picks(p("tumbling", 4)), picks(), nil},
		{"over the bound", "bard", picks(p("tumbling", skills.MaxPickSlots+1)), picks(), &domain.FieldError{Field: "nonweapon.tumbling", Message: "out_of_range"}},
		{"unknown id", "bard", picks(p("juggling", 1)), picks(), &domain.FieldError{Field: "nonweapon.juggling", Message: "unknown"}},
		{"duplicate", "bard", picks(p("cooking", 1), p("cooking", 2)), picks(), &domain.FieldError{Field: "nonweapon.cooking", Message: "duplicate"}},
		{"no id", "bard", picks(p("", 1)), picks(), &domain.FieldError{Field: "nonweapon", Message: "empty"}},
		{"languages modern on its own", "bard", picks(p("languages_modern", 1)), picks(), &domain.FieldError{Field: "nonweapon.languages_modern", Message: "not_allowed"}},
		{"an unknown stored pick is kept", "bard", picks(p("old_skill", 1), p("cooking", 1)), old, nil},
		{"an unknown stored pick may spend more", "bard", picks(p("old_skill", 2)), old, nil},
		{"an unknown stored pick needs a slot", "bard", picks(p("old_skill", 0)), old, &domain.FieldError{Field: "nonweapon.old_skill", Message: "out_of_range"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := skills.ValidPicks(book, tc.class, tc.picks, tc.before)
			if tc.want == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.picks, got)
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
}

// ADR 0037 step 10 on the non-weapon pool: refused when, after the save, spent > available AND
// spent > spent before; any other save passes. A level 1 bard with Intelligence 9 has 3 + 0 + 2 =
// 5 slots.
func TestBudget_Step10_RefusedWhenOverAndGrowing(t *testing.T) {
	book := profBook(t)
	five := picks(p("tumbling", 3), p("tracking", 2))
	for _, tc := range []struct {
		name          string
		before, after skills.Picks
		refused       bool
		spent         int
	}{
		{"under budget", picks(), picks(p("tumbling", 1), p("cooking", 1)), false, 2},
		{"at the budget", picks(), five, false, 5},
		{"a new overspend", five, picks(p("tumbling", 3), p("tracking", 2), p("cooking", 1)), true, 6},
		{"created over budget (nothing before)", picks(), picks(p("tumbling", 6)), true, 6},
		{"an overspend kept as it is", picks(p("tumbling", 7)), picks(p("tumbling", 7)), false, 7},
		{"an overspend moved between picks, same total", picks(p("tumbling", 7)), picks(p("tumbling", 5), p("cooking", 2)), false, 7},
		{"an overspend reduced, still over", picks(p("tumbling", 8)), picks(p("tumbling", 6)), false, 6},
		{"an overspend increased", picks(p("tumbling", 7)), picks(p("tumbling", 8)), true, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := skills.PicksWithinBudget(book, "bard", 1, 9, skills.Spending{Picks: tc.before}, skills.Spending{Picks: tc.after})
			if !tc.refused {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, domain.ErrOverBudget)
			var be *domain.BudgetError
			require.ErrorAs(t, err, &be)
			assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: tc.spent, Limit: 5}, *be)
		})
	}
	t.Run("no Table 34 row: no budget", func(t *testing.T) {
		require.NoError(t, skills.PicksWithinBudget((*ruletablesvc.Proficiencies)(nil), "bard", 1, 9, skills.Spending{Picks: picks()}, skills.Spending{Picks: picks(p("tumbling", 50))}))
	})
}

// ADR 0037 step 11: over budget is slots_over_budget {pool, spent, available}, slots to spend the
// reminder slots_to_spend {pool, left}, none when the pool is spent exactly; a pick the catalogue
// does not hold is reported (proficiency_unknown), never dropped.
func TestWarnings_Step11_OverBudgetAndToSpend(t *testing.T) {
	book := profBook(t)
	warnings := func(ps skills.Picks) []skills.Warning {
		// 2 weapon slots, spent exactly: no weapon warning.
		ps.Weapon = weapons("dagger", "dart").Weapon
		f, ok := skills.ProficiencyFigures(book, "bard", 1, 9, skills.Spending{Picks: ps}) // 5 slots
		require.True(t, ok)
		return skills.SlotWarnings(f)
	}
	seven, five, two, three := 7, 5, 2, 3
	assert.Equal(t, []skills.Warning{{Code: "slots_over_budget", Field: "nonweapon", Pool: "nonweapon", Spent: &seven, Available: &five}},
		warnings(picks(p("tumbling", 7))))
	assert.Equal(t, []skills.Warning{{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &three}},
		warnings(picks(p("tumbling", 2))))
	assert.Equal(t, []skills.Warning{{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &five}},
		warnings(picks()), "a new character's reminder")
	assert.Empty(t, warnings(picks(p("tumbling", 5))), "spent exactly")
	assert.Equal(t, []skills.Warning{
		{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &two},
		{Code: "proficiency_unknown", Field: "nonweapon.old_skill"},
	}, warnings(picks(p("tumbling", 2), p("old_skill", 1))))
}

// ADR 0037 step 5 (amending ADR 0014 step 1): a pick spends at least its cost and each slot beyond
// it adds +1 to the check target, ability score + catalogue modifier + extra slots; a pick with no
// check (or one the catalogue does not hold) has no target.
func TestNonweaponPick_Step5_ExtraSlotsRaiseTheTarget(t *testing.T) {
	book := profBook(t)
	// Kelsen's scores: Dex 17, Int 14, Wis 8.
	score := func(a string) (int, bool) {
		v, ok := map[string]int{"dex": 17, "int": 14, "wis": 8}[a]
		return v, ok
	}
	f, ok := skills.ProficiencyFigures(book, "bard", 15, 14, skills.Spending{Picks: picks(
		p("tumbling", 1), p("tumbling_more", 1), p("healing", 3), p("tracking", 5), p("blind_fighting", 2),
	)})
	require.True(t, ok)
	f.Picks[1] = skills.PickFigures{ID: "tumbling_more", Slots: 1, Cost: 1} // unknown to the catalogue
	f.SetTargets(score)
	assert.Equal(t, []skills.PickFigures{
		{ID: "tumbling", Slots: 1, Cost: 1, Extra: 0, Ability: "dex", Known: true, Target: 17, HasCheck: true},
		{ID: "tumbling_more", Slots: 1, Cost: 1},
		// Healing is a priest proficiency: 3 for a bard, so 3 slots are its cost, no extra.
		{ID: "healing", Slots: 3, Cost: 3, Extra: 0, Ability: "wis", Modifier: -2, Known: true, Target: 6, HasCheck: true},
		{ID: "tracking", Slots: 5, Cost: 2, Extra: 3, Ability: "wis", Known: true, Target: 11, HasCheck: true},
		{ID: "blind_fighting", Slots: 2, Cost: 2, Known: true},
	}, f.Picks)
	for _, tc := range []struct{ slots, target int }{{1, 17}, {2, 18}, {4, 20}} {
		g, _ := skills.ProficiencyFigures(book, "fighter", 1, 10, skills.Spending{Picks: picks(p("tumbling", tc.slots+1))})
		g.SetTargets(score)
		assert.Equal(t, tc.target, g.Picks[0].Target, "a fighter's Tumbling costs 2: %d extra", tc.slots-1)
	}
}

// ADR 0037 step 13: picks are equal whatever their order; a different slot count, id or length
// is a change.
func TestPicksEqual_Step13_OrderDoesNotCount(t *testing.T) {
	p := func(ps ...skills.Pick) skills.Picks { return skills.Picks{Nonweapon: ps} }
	a, b, c := skills.Pick{ID: "tumbling", Slots: 1}, skills.Pick{ID: "tracking", Slots: 2}, skills.Pick{ID: "tumbling", Slots: 2}
	for _, tc := range []struct {
		name string
		x, y skills.Picks
		want bool
	}{
		{"same order", p(a, b), p(a, b), true},
		{"reordered", p(a, b), p(b, a), true},
		{"both empty", p(), p(), true},
		{"other slots", p(a, b), p(c, b), false},
		{"one more", p(a), p(a, b), false},
		{"one less", p(a, b), p(b), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.x.Equal(tc.y))
			assert.Equal(t, tc.want, tc.y.Equal(tc.x))
		})
	}
}
