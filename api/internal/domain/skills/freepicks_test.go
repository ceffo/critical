package skills_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// The proficiencies a class gets free (ADR 0037 step 22), on the fixture bard, who lists
// free_proficiencies at level 1 with reading_writing, local_history, singing and
// musical_instrument (each 1 slot in his groups, Table 37).

// freeFour are the ids the fixture bard gets free, in the catalogue's list order.
var freeFour = []string{"reading_writing", "local_history", "singing", "musical_instrument"}

// figuresOf are the pools of the class entries cls with Intelligence 9 (Table 4: 2) spending the
// non-weapon picks ps, no weapon pick.
func figuresOf(t *testing.T, cls skills.Classes, ps ...skills.Pick) skills.ProficiencySlots {
	t.Helper()
	f, ok := skills.FiguresOf(profBook(t), cls, 9, skills.Spending{Picks: picks(ps...)})
	require.True(t, ok)
	return f
}

// figure is the pick of id among f's.
func figure(t *testing.T, f skills.ProficiencySlots, id string) skills.PickFigures {
	t.Helper()
	for _, p := range f.Picks {
		if p.ID == id {
			return p
		}
	}
	require.FailNow(t, "no pick", id)
	return skills.PickFigures{}
}

// offered are the ids f's warnings offer (free_proficiency_to_take), in order, each on field
// nonweapon.<id>.
func offered(t *testing.T, f skills.ProficiencySlots) []string {
	t.Helper()
	var out []string
	for _, w := range skills.SlotWarnings(f) {
		if w.Code == skills.CodeFreeProficiencyToTake {
			assert.Equal(t, "nonweapon."+w.ID, w.Field)
			out = append(out, w.ID)
		}
	}
	return out
}

// ADR 0037 step 22.2 (amending step 4): an id the character gets free costs 0, whatever Table 37
// and Table 38 give, and its figures say it is free; another pick keeps step 4's cost. A bard's
// four at 0 (each 1 slot in his groups otherwise), his Tumbling at 1, a fighter's singing at 1.
func TestNonweaponCost_Step22_2_AGrantedIdCostsNothing(t *testing.T) {
	bard := figuresOf(t, skills.One("bard", 1), p("reading_writing", 0), p("local_history", 0), p("singing", 0),
		p("musical_instrument", 0), p("tumbling", 1))
	for _, id := range freeFour {
		got := figure(t, bard, id)
		assert.Equal(t, 0, got.Cost, id)
		assert.True(t, got.Free, id)
		assert.Zero(t, got.Extra, id)
	}
	tumbling := figure(t, bard, "tumbling")
	assert.Equal(t, 1, tumbling.Cost)
	assert.False(t, tumbling.Free)
	assert.Equal(t, 1, bard.Nonweapon.Spent, "only Tumbling spends")

	singing := figure(t, figuresOf(t, skills.One("fighter", 1), p("singing", 1)), "singing")
	assert.Equal(t, 1, singing.Cost, "a fighter gets nothing free")
	assert.False(t, singing.Free)

	row, ok := profBook(t).Get("singing")
	require.True(t, ok)
	cost, granted := skills.NonweaponCostOf(profBook(t), skills.One("bard", 1), row)
	assert.Equal(t, 0, cost)
	assert.True(t, granted)
	cost, granted = skills.NonweaponCostOf(profBook(t), skills.One("fighter", 1), row)
	assert.Equal(t, 1, cost)
	assert.False(t, granted)
}

// ADR 0037 step 22.2 with steps 5 and 10: a free pick stores the slots spent on it, 0 or more, and
// every one is an extra slot: it spends the non-weapon pool and adds +1 to the check. At 0 slots
// it spends 0 with no extra and rolls at the score plus the modifier (Charisma 16: 16); at 2 it
// spends 2 and its target is 18. A level 1 bard of Intelligence 9 has 5 slots: with Tumbling at 5
// a singing raised to 2 leaves the pool over and growing, refused (step 10); a fighter may not
// store singing at 0 (its cost is 1, out_of_range).
func TestNonweaponPick_Step22_2_ExtraSlotsOnAFreePickCostAndCount(t *testing.T) {
	book := profBook(t)
	score := func(string) (int, bool) { return 16, true }
	for slots, want := range map[int]struct{ spent, extra, target int }{0: {0, 0, 16}, 2: {2, 2, 18}} {
		f := figuresOf(t, skills.One("bard", 1), p("singing", slots))
		f.SetTargets(score)
		got := figure(t, f, "singing")
		assert.Equal(t, want.spent, f.Nonweapon.Spent, "%d slots", slots)
		assert.Equal(t, want.extra, got.Extra, "%d slots", slots)
		assert.Equal(t, want.target, got.Target, "%d slots", slots)
		_, err := skills.ValidPicks(book, "bard", 1, picks(p("singing", slots)), skills.Picks{})
		require.NoError(t, err, "%d slots", slots)
	}

	before := skills.Spending{Picks: picks(p("tumbling", 5), p("singing", 0))}
	require.NoError(t, skills.PicksWithinBudget(book, "bard", 1, 9, skills.Spending{}, before), "5 of 5: the free pick spends 0")
	err := skills.PicksWithinBudget(book, "bard", 1, 9, before, skills.Spending{Picks: picks(p("tumbling", 5), p("singing", 2))})
	var be *domain.BudgetError
	require.True(t, errors.As(err, &be), "%v", err)
	assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: 7, Limit: 5}, *be)

	_, err = skills.ValidPicks(book, "fighter", 1, picks(p("singing", 0)), skills.Picks{})
	assert.Equal(t, domain.FieldError{Field: "nonweapon.singing", Message: skills.CodeOutOfRange}, fieldErr(t, err))
}

// ADR 0037 step 22.3: the bard's free instrument is the one musical_instrument pick, free at 0
// slots; a second pick of the id stays refused duplicate (step 12).
func TestFreePicks_Step22_3_TheInstrumentIsOnePick(t *testing.T) {
	book := profBook(t)
	got, err := skills.ValidPicks(book, "bard", 1, picks(p("musical_instrument", 0)), skills.Picks{})
	require.NoError(t, err)
	assert.Equal(t, []skills.Pick{p("musical_instrument", 0)}, got.Nonweapon)
	instrument := figure(t, figuresOf(t, skills.One("bard", 1), p("musical_instrument", 0)), "musical_instrument")
	assert.True(t, instrument.Free)
	assert.Equal(t, 0, instrument.Cost)

	_, err = skills.ValidPicks(book, "bard", 1, picks(p("musical_instrument", 0), p("musical_instrument", 1)), skills.Picks{})
	assert.Equal(t, domain.FieldError{Field: "nonweapon.musical_instrument", Message: skills.CodeDuplicate}, fieldErr(t, err))
}

// ADR 0037 step 22.4: a granted id the character does not hold is offered, the reminder
// free_proficiency_to_take {id} on field nonweapon.<id>, never a refusal and never a pick written
// for him: the bard holding two of the four is offered the other two, in the grant's order; none
// once he holds all four (at any slots); a fighter is offered none.
func TestWarnings_Step22_4_AGrantNotHeldIsOffered(t *testing.T) {
	two := figuresOf(t, skills.One("bard", 1), p("singing", 0), p("reading_writing", 1))
	assert.Equal(t, []string{"local_history", "musical_instrument"}, offered(t, two))
	assert.Len(t, two.Picks, 2, "nothing written for him")
	assert.Contains(t, skills.SlotWarnings(two), skills.Warning{
		Code: skills.CodeFreeProficiencyToTake, Field: "nonweapon.local_history", ID: "local_history",
	})

	all := figuresOf(t, skills.One("bard", 1), p("reading_writing", 0), p("local_history", 2), p("singing", 0), p("musical_instrument", 0))
	assert.Empty(t, offered(t, all))
	assert.Equal(t, freeFour, offered(t, figuresOf(t, skills.One("bard", 1))), "a bard holding none is offered the four")
	assert.Empty(t, offered(t, figuresOf(t, skills.One("fighter", 1))))
}

// ADR 0037 step 22.5 (reading ADR 0042 steps 12, 13 and 15): an id is free when any class entry
// grants it, an active one at its realized level, a frozen one at its left-at level. A fighter
// left at 5 now bard 1 gets the four free (and is offered them); a bard left at 5 now fighter 2
// (before the crossover) and fighter 6 (after) keeps them free; a fighter/thief pays (no entry
// grants them) and is offered nothing.
func TestFreePicks_Step22_5_AnyEntryGrants(t *testing.T) {
	for name, cls := range map[string]skills.Classes{
		"fighter left at 5, bard 1": {{Class: "fighter", Level: 5, Frozen: true}, {Class: "bard", Level: 1}},
		"bard left at 5, fighter 2": {{Class: "bard", Level: 5, Frozen: true}, {Class: "fighter", Level: 2}},
		"bard left at 5, fighter 6": {{Class: "bard", Level: 5, Frozen: true}, {Class: "fighter", Level: 6}},
	} {
		f := figuresOf(t, cls, p("reading_writing", 0), p("singing", 1))
		for _, id := range []string{"reading_writing", "singing"} {
			got := figure(t, f, id)
			assert.True(t, got.Free, "%s: %s", name, id)
			assert.Equal(t, 0, got.Cost, "%s: %s", name, id)
		}
		assert.Equal(t, 1, figure(t, f, "singing").Extra, "%s: the slot on singing is an extra slot", name)
		assert.Equal(t, []string{"local_history", "musical_instrument"}, offered(t, f), name)
		_, err := skills.Apply(profBook(t), nil, "", skills.Sheet{Classes: cls, Intelligence: 9},
			skills.Edit{Picks: picks(p("reading_writing", 0), p("singing", 1))})
		require.NoError(t, err, "%s: a save at 0 slots", name)
	}

	multi := skills.Classes{{Class: "fighter", Level: 1}, {Class: "thief", Level: 1}}
	f := figuresOf(t, multi, p("singing", 1))
	assert.False(t, figure(t, f, "singing").Free)
	assert.Equal(t, 1, figure(t, f, "singing").Cost, "a fighter/thief pays")
	assert.Empty(t, offered(t, f))
}

// ADR 0037 step 22.6: when no entry grants an id any more (here a fighter, as an undone class
// change leaves a pick his bard entry held), its stored pick costs step 4's cost again: kept as
// stored, a count under the cost spends the cost with no extra slot, a count at or above it spends
// what it stores. A free pick at 0 slots spends 1 once the grant is gone; at 2 slots, 2 with 1
// extra. The pool over budget is warned, never refused on read; a save that leaves the pick
// unchanged is not refused for it, a new pick under the cost is.
func TestFreePicks_Step22_6_ALostGrantChargesTheCostAgain(t *testing.T) {
	book := profBook(t)
	gone := figuresOf(t, skills.One("fighter", 1), p("singing", 0))
	singing := figure(t, gone, "singing")
	assert.Equal(t, 1, singing.Cost)
	assert.False(t, singing.Free)
	assert.Zero(t, singing.Extra)
	assert.Equal(t, 1, gone.Nonweapon.Spent, "a count under the cost spends the cost")

	two := figuresOf(t, skills.One("fighter", 1), p("singing", 2))
	assert.Equal(t, 2, two.Nonweapon.Spent)
	assert.Equal(t, 1, figure(t, two, "singing").Extra)

	over := figuresOf(t, skills.One("fighter", 1), p("singing", 0), p("tumbling", 6))
	assert.Equal(t, 7, over.Nonweapon.Spent, "1 for singing and the 6 Tumbling stores, of 5")
	assert.Equal(t, skills.CodeSlotsOverBudget, skills.SlotWarnings(over)[0].Code, "warned, not refused")

	stored := picks(p("singing", 0), p("cooking", 1))
	_, err := skills.ValidPicks(book, "fighter", 1, picks(p("singing", 0), p("cooking", 2)), stored)
	require.NoError(t, err, "the pick unchanged is kept")
	_, err = skills.ValidPicks(book, "fighter", 1, picks(p("singing", 1)), stored)
	require.NoError(t, err, "raised to its cost")
	_, err = skills.ValidPicks(book, "fighter", 1, picks(p("cooking", 1)), stored)
	require.NoError(t, err, "removed")
	_, err = skills.ValidPicks(book, "fighter", 1, picks(p("singing", 0)), skills.Picks{})
	assert.Equal(t, domain.FieldError{Field: "nonweapon.singing", Message: skills.CodeOutOfRange}, fieldErr(t, err), "a new pick under the cost")
}

// ADR 0037 step 22.7 on the rule the data step and an older action-log entry use: each pick of an
// id an entry grants free spends the cost it paid before less, never below 0; others stay.
func TestReturnFreeSlots_Step22_7_TheCostPaidGoesBack(t *testing.T) {
	book := profBook(t)
	out, returned := skills.ReturnFreeSlots(book, skills.One("bard", 3),
		[]skills.Pick{p("singing", 1), p("musical_instrument", 3), p("tumbling", 1), p("local_history", 0)})
	assert.Equal(t, []skills.Pick{p("singing", 0), p("musical_instrument", 2), p("tumbling", 1), p("local_history", 0)}, out)
	assert.Equal(t, map[string]int{"singing": 1, "musical_instrument": 1}, returned)

	out, returned = skills.ReturnFreeSlots(book, skills.One("fighter", 3), []skills.Pick{p("singing", 1)})
	assert.Equal(t, []skills.Pick{p("singing", 1)}, out)
	assert.Nil(t, returned)

	// The bard left at 5 now fighter 2 paid the fighter's cost: reading_writing is outside his groups.
	out, _ = skills.ReturnFreeSlots(book, skills.Classes{{Class: "bard", Level: 5, Frozen: true}, {Class: "fighter", Level: 2}},
		[]skills.Pick{p("reading_writing", 3)})
	assert.Equal(t, []skills.Pick{p("reading_writing", 1)}, out)
}
