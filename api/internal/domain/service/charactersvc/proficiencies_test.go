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

func picks(ps ...charactersvc.Pick) charactersvc.Picks {
	return charactersvc.Picks{Nonweapon: append([]charactersvc.Pick{}, ps...)}
}

func p(id string, slots int) charactersvc.Pick { return charactersvc.Pick{ID: id, Slots: slots} }

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
		picks  charactersvc.Picks
		before charactersvc.Picks
		want   *domain.FieldError
	}{
		{"in group at cost", "fighter", picks(p("tracking", 2)), picks(), nil},
		{"out of group at the plus-one cost", "fighter", picks(p("tumbling", 2)), picks(), nil},
		{"out of group at the table's cost", "fighter", picks(p("tumbling", 1)), picks(), &domain.FieldError{Field: "nonweapon.tumbling", Message: "out_of_range"}},
		{"under the cost", "bard", picks(p("tracking", 1)), picks(), &domain.FieldError{Field: "nonweapon.tracking", Message: "out_of_range"}},
		{"extra slots", "bard", picks(p("tumbling", 4)), picks(), nil},
		{"over the bound", "bard", picks(p("tumbling", charactersvc.MaxPickSlots+1)), picks(), &domain.FieldError{Field: "nonweapon.tumbling", Message: "out_of_range"}},
		{"unknown id", "bard", picks(p("juggling", 1)), picks(), &domain.FieldError{Field: "nonweapon.juggling", Message: "unknown"}},
		{"duplicate", "bard", picks(p("cooking", 1), p("cooking", 2)), picks(), &domain.FieldError{Field: "nonweapon.cooking", Message: "duplicate"}},
		{"no id", "bard", picks(p("", 1)), picks(), &domain.FieldError{Field: "nonweapon", Message: "empty"}},
		{"languages modern on its own", "bard", picks(p("languages_modern", 1)), picks(), &domain.FieldError{Field: "nonweapon.languages_modern", Message: "not_allowed"}},
		{"an unknown stored pick is kept", "bard", picks(p("old_skill", 1), p("cooking", 1)), old, nil},
		{"an unknown stored pick may spend more", "bard", picks(p("old_skill", 2)), old, nil},
		{"an unknown stored pick needs a slot", "bard", picks(p("old_skill", 0)), old, &domain.FieldError{Field: "nonweapon.old_skill", Message: "out_of_range"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := charactersvc.ValidPicks(book, tc.class, tc.picks, tc.before)
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
		before, after charactersvc.Picks
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
			err := charactersvc.PicksWithinBudget(book, "bard", 1, 9, tc.before, tc.after)
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
		require.NoError(t, charactersvc.PicksWithinBudget((*ruletablesvc.Proficiencies)(nil), "bard", 1, 9, picks(), picks(p("tumbling", 50))))
	})
}

// ADR 0037 step 11: over budget is slots_over_budget {pool, spent, available}, slots to spend the
// reminder slots_to_spend {pool, left}, none when the pool is spent exactly; a pick the catalogue
// does not hold is reported (proficiency_unknown), never dropped.
func TestWarnings_Step11_OverBudgetAndToSpend(t *testing.T) {
	book := profBook(t)
	warnings := func(ps charactersvc.Picks) []charactersvc.Warning {
		f, ok := charactersvc.ProficiencyFigures(book, "bard", 1, 9, ps) // 5 slots
		require.True(t, ok)
		return charactersvc.SlotWarnings(f)
	}
	seven, five, two, three := 7, 5, 2, 3
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_over_budget", Field: "nonweapon", Pool: "nonweapon", Spent: &seven, Available: &five}},
		warnings(picks(p("tumbling", 7))))
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &three}},
		warnings(picks(p("tumbling", 2))))
	assert.Equal(t, []charactersvc.Warning{{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &five}},
		warnings(picks()), "a new character's reminder")
	assert.Empty(t, warnings(picks(p("tumbling", 5))), "spent exactly")
	assert.Equal(t, []charactersvc.Warning{
		{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &two},
		{Code: "proficiency_unknown", Field: "nonweapon.old_skill"},
	}, warnings(picks(p("tumbling", 2), p("old_skill", 1))))
}

// ADR 0037 step 5 (amending ADR 0014 step 1): a pick spends at least its cost and each slot beyond
// it adds +1 to the check target, ability score + catalogue modifier + extra slots; a pick with no
// check (or one the catalogue does not hold) has no target.
func TestNonweaponPick_Step5_ExtraSlotsRaiseTheTarget(t *testing.T) {
	book := profBook(t)
	c := kelsen() // Dex 17, Wis 8
	f, ok := charactersvc.ProficiencyFigures(book, "bard", 15, c.Int, picks(
		p("tumbling", 1), p("tumbling_more", 1), p("healing", 3), p("tracking", 5), p("blind_fighting", 2),
	))
	require.True(t, ok)
	f.Picks[1] = charactersvc.PickFigures{ID: "tumbling_more", Slots: 1, Cost: 1} // unknown to the catalogue
	f.SetTargets(c.Score)
	assert.Equal(t, []charactersvc.PickFigures{
		{ID: "tumbling", Slots: 1, Cost: 1, Extra: 0, Ability: "dex", Known: true, Target: 17, HasCheck: true},
		{ID: "tumbling_more", Slots: 1, Cost: 1},
		// Healing is a priest proficiency: 3 for a bard, so 3 slots are its cost, no extra.
		{ID: "healing", Slots: 3, Cost: 3, Extra: 0, Ability: "wis", Modifier: -2, Known: true, Target: 6, HasCheck: true},
		{ID: "tracking", Slots: 5, Cost: 2, Extra: 3, Ability: "wis", Known: true, Target: 11, HasCheck: true},
		{ID: "blind_fighting", Slots: 2, Cost: 2, Known: true},
	}, f.Picks)
	for _, tc := range []struct{ slots, target int }{{1, 17}, {2, 18}, {4, 20}} {
		g, _ := charactersvc.ProficiencyFigures(book, "fighter", 1, 10, picks(p("tumbling", tc.slots+1)))
		g.SetTargets(c.Score)
		assert.Equal(t, tc.target, g.Picks[0].Target, "a fighter's Tumbling costs 2: %d extra", tc.slots-1)
	}
}

// expectKelsenAt answers the lookups of Kelsen's read at the realized level 15 with an XP total
// whose target level is target.
func expectKelsenAt(e *env, target int) {
	e.rules.EXPECT().LevelFor("bard", mock.Anything).Return(target, nil).Once()
	e.rules.EXPECT().XPThreshold("bard", target+1).Return(1_600_000, nil).Once()
	e.rules.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
	e.rules.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	e.rules.EXPECT().BardSpellsPerDay(15).Return(level15Slots, nil).Once()
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
			k.Proficiencies = picks(p("tumbling", 1), p("cooking", 1), p("tracking", 2), p("blind_fighting", 2), p("old_skill", 2))
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
			e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
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
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenAt(e, 15)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		assert.Equal(t, 13, got.Derived.Proficiencies.Nonweapon.Available)
	})
	t.Run("without the catalogue there is no pool", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenAt(e, 15)
		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		assert.Nil(t, got.Derived.Proficiencies)
	})
}

// ADR 0037 step 12's preview: the character read with draft picks, refused as the save refuses
// (a field error, the budget), storing nothing.
func TestPreviewPicks_Step12_SameRefusalsAsTheSave(t *testing.T) {
	read := func(t *testing.T, stored charactersvc.Picks) *env {
		e := newEnvWithBook(t, profBook(t))
		k := kelsen()
		k.Proficiencies = stored
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(k, nil).Once()
		return e
	}
	t.Run("within budget", func(t *testing.T) {
		e := read(t, picks())
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenAt(e, 15)
		got, err := e.svc.PreviewProficiencies(context.Background(), 3, picks(p("tracking", 4)))
		require.NoError(t, err)
		assert.Equal(t, 6, got.Derived.Proficiencies.Nonweapon.Left)
		assert.Equal(t, 2, got.Derived.Proficiencies.Picks[0].Extra)
	})
	t.Run("over budget and growing", func(t *testing.T) {
		e := read(t, picks(p("tumbling", 10)))
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenAt(e, 15)
		_, err := e.svc.PreviewProficiencies(context.Background(), 3, picks(p("tumbling", 11)))
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: 11, Limit: 10}, *be)
	})
	t.Run("a refused pick", func(t *testing.T) {
		e := read(t, picks())
		_, err := e.svc.PreviewProficiencies(context.Background(), 3, picks(p("juggling", 1)))
		assert.Equal(t, domain.FieldError{Field: "nonweapon.juggling", Message: "unknown"}, fieldErr(t, err))
	})
}

// ADR 0037 steps 10 and 14: the creation takes picks, checked as the editor checks them with
// nothing picked before: within budget they are stored, leftovers reminded; over budget, or a
// refused pick, nothing is written. A level 1 bard with Intelligence 14 has 3 + 0 + 4 = 7 slots.
func TestCreate_Step14_PicksCheckedAsTheEditor(t *testing.T) {
	withPicks := func(ps charactersvc.Picks) charactersvc.NewCharacter {
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
		_, err := e.svc.Create(context.Background(), ownerID, withPicks(picks(p("old_skill", 1))))
		assert.Equal(t, domain.FieldError{Field: "nonweapon.old_skill", Message: "unknown"}, fieldErr(t, err), "nothing before: no unknown id kept")
		assert.Zero(t, e.tx.n)
	})
	t.Run("the preview reminds the leftovers", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Once()
		e.rules.EXPECT().XPThreshold("bard", 2).Return(1_250, nil).Once()
		e.rules.EXPECT().THAC0For("rogue", 1).Return(20, nil).Once()
		e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
		e.rules.EXPECT().BardSpellsPerDay(1).Return(map[int]int{}, nil).Once()
		got, err := e.svc.Preview(context.Background(), withPicks(picks(p("tumbling", 1))))
		require.NoError(t, err)
		six := 6
		assert.Contains(t, got.Warnings, charactersvc.Warning{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &six})
	})
	t.Run("the checks endpoint answers the pool", func(t *testing.T) {
		e := newEnvWithBook(t, profBook(t))
		e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Once()
		xp := 0
		got, err := e.svc.CreationChecks(context.Background(), charactersvc.DraftInput{
			Class: "bard", XP: &xp, Scores: map[string]int{"int": 14, "dex": 15}, Proficiencies: picks(p("tumbling", 2)),
		})
		require.NoError(t, err)
		require.NotNil(t, got.ProficiencySlots)
		assert.Equal(t, ruletablesvc.NonweaponPool{Initial: 3, PerLevels: 4, Intelligence: 4, Available: 7, Spent: 2, Left: 5}, got.ProficiencySlots.Nonweapon)
		assert.Equal(t, 16, got.ProficiencySlots.Picks[0].Target, "Dex 15 + 1 extra slot")
	})
}
