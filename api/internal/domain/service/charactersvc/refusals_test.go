package charactersvc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// The refusal rules run on the fixture's class and race definitions alone: no service, no store,
// no derivation, no composition root.

// codes is the "field: code" of each refusal.
func codes(t *testing.T, refusals []error) []string {
	t.Helper()
	out := []string{}
	for _, r := range refusals {
		var fe *domain.FieldError
		require.ErrorAs(t, r, &fe)
		out = append(out, fe.Field+": "+fe.Message)
	}
	return out
}

func multi(race string, es ...NewClass) NewCharacter { return NewCharacter{Race: race, Classes: es} }

// ADR 0042 step 1: the entries as a whole, in the order of the entries.
func TestClassShape_Step1_TheBooksLimitsOnTheEntries(t *testing.T) {
	fighter, thief, mage, cleric := NewClass{Class: "fighter"}, NewClass{Class: "thief"}, NewClass{Class: "mage"}, NewClass{Class: "cleric"}
	left := func(c string, at int) NewClass { return NewClass{Class: c, LeftAtLevel: at} }
	for _, tc := range []struct {
		name string
		n    NewCharacter
		want []string
	}{
		{"the one-class case has no entries to judge", NewCharacter{Class: "fighter"}, []string{}},
		{"a multi-class of three", multi("human", fighter, thief, mage), []string{}},
		{"a dual-class", multi("human", left("fighter", 9), mage), []string{}},
		{"no entry", multi("human"), []string{}}, // no entries is the one-class case
		{"only a class left", multi("human", left("fighter", 9)), []string{"classes: " + CodeNoActiveClass}},
		{"four active", multi("human", fighter, thief, mage, cleric), []string{"classes: " + CodeTooManyClasses}},
		{"multi-class and dual-class", multi("human", left("fighter", 9), thief, mage), []string{"classes: " + CodeMultiAndDualClass}},
		{"a class twice", multi("human", fighter, fighter), []string{"classes.1.class: " + CodeDuplicateClass}},
		{"the new class already left", multi("human", left("fighter", 9), fighter), []string{"classes.1.class: " + CodeClassAlreadyLeft}},
		{"the class left listed after the new one", multi("human", fighter, left("fighter", 9)), []string{"classes.0.class: " + CodeClassAlreadyLeft}},
		{"left under level 2", multi("human", left("fighter", 1), mage), []string{"classes.0.leftAtLevel: " + CodeLevelTooLow}},
		{"left above the maximum level", multi("human", left("fighter", 99), mage), []string{"classes.0.leftAtLevel: " + CodeOutOfRange}},
		{"negative left-at level", multi("human", NewClass{Class: "fighter", LeftAtLevel: -1}, mage), []string{"classes.0.leftAtLevel: " + CodeOutOfRange}},
		{"a level out of range", multi("human", NewClass{Class: "fighter", Level: 99}), []string{"classes.0.level: " + CodeOutOfRange}},
		{"a class left at a level other than its left-at level", multi("human", NewClass{Class: "fighter", LeftAtLevel: 9, Level: 5}, mage), []string{"classes.0.level: " + CodeOutOfRange}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codes(t, classShape(tc.n)))
		})
	}
}

// ADR 0042 step 2.1 (amended 2026-10-22): a class the catalogue marks `multiclass: false` is
// refused at its own entry when the character has more than one active class.
func TestMultiClassClassRefusals_Step2_1_TheCatalogueBarsTheBard(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	fighter, thief, bard := NewClass{Class: "fighter"}, NewClass{Class: "thief"}, NewClass{Class: "bard"}
	for _, tc := range []struct {
		name string
		n    NewCharacter
		want []string
	}{
		{"the bard alone", multi("half_elf", bard), []string{}},
		{"the bard with a thief", multi("half_elf", thief, bard), []string{"classes.1.class: " + CodeMultiClassClassNotAllowed}},
		{"the bard first of three", multi("half_elf", bard, fighter, thief), []string{"classes.0.class: " + CodeMultiClassClassNotAllowed}},
		{"into the bard (dual-class)", multi("human", NewClass{Class: "fighter", LeftAtLevel: 9}, bard), []string{}},
		{"out of the bard (dual-class)", multi("human", NewClass{Class: "bard", LeftAtLevel: 9}, fighter), []string{}},
		{"a class the catalogue does not know is another check's", multi("human", fighter, NewClass{Class: "nope"}), []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codes(t, multiClassClassRefusals(tc.n, defs)))
		})
	}
}

// ADR 0042 step 2 (and ADR 0041 step 2.2): the race's limit on multi-class and dual-class
// characters, waived by the ruleset's racial lift, and DMG Table 7's dash otherwise.
func TestRaceRefusals_Step2_TheLiftDecidesTheRaceLimit(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	fighter, thief, cleric, mage := NewClass{Class: "fighter"}, NewClass{Class: "thief"}, NewClass{Class: "cleric"}, NewClass{Class: "mage"}
	book, lifted := ruleset.RacialLift{}, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true}
	allowAll := func(string, string) (bool, error) { return true, nil }
	dash := func(race, class string) (bool, error) { return race != "dwarf" || class != "mage", nil }
	for _, tc := range []struct {
		name   string
		lift   ruleset.RacialLift
		n      NewCharacter
		allows func(string, string) (bool, error)
		want   []string
	}{
		{"a human multi-class, the book", book, multi("human", fighter, thief), allowAll, []string{"classes: " + CodeMultiClassNotAllowed}},
		{"a human multi-class, lifted", lifted, multi("human", fighter, thief), allowAll, []string{}},
		{"a half-elf multi-class, the book", book, multi("half_elf", fighter, cleric), allowAll, []string{}},
		{"an elf dual-class, the book", book, multi("elf", NewClass{Class: "fighter", LeftAtLevel: 9}, mage), allowAll, []string{"classes: " + CodeDualClassNotAllowed}},
		{"a human dual-class, the book", book, multi("human", NewClass{Class: "fighter", LeftAtLevel: 9}, mage), allowAll, []string{}},
		{"a dwarf dual-class, the book", book, multi("dwarf", NewClass{Class: "fighter", LeftAtLevel: 9}, mage), allowAll, []string{"classes: " + CodeDualClassNotAllowed}},
		{"a dwarf dual-class, lifted", lifted, multi("dwarf", NewClass{Class: "fighter", LeftAtLevel: 9}, mage), allowAll, []string{}},
		{"the lift waives the dash of a multi-class", lifted, multi("dwarf", fighter, mage), dash, []string{}},
		{"the dash on a single class", lifted, NewCharacter{Race: "dwarf", Class: "mage"}, dash, []string{"race: " + CodeRaceNotAllowed}},
		{"the dash on a multi-class, the book", book, multi("dwarf", fighter, mage), dash, []string{"race: " + CodeRaceNotAllowed}},
		{"a class the race admits", book, NewCharacter{Race: "dwarf", Class: "fighter"}, dash, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := raceRefusals(tc.lift, defs, tc.n, tc.allows)
			require.NoError(t, err)
			assert.Equal(t, tc.want, codes(t, got))
		})
	}
	t.Run("a failing table read is an error, not a refusal", func(t *testing.T) {
		boom := errors.New("boom")
		got, err := raceRefusals(book, defs, NewCharacter{Race: "dwarf", Class: "fighter"}, func(string, string) (bool, error) { return false, boom })
		assert.ErrorIs(t, err, boom)
		assert.Empty(t, got)
	})
}

// ADR 0042 step 2.2: a dual-class character needs 15 in every prime requisite of a class it left
// and 17 in every one of its new class.
func TestPrimeRefusals_Step2_2_DualClassPrimeRequisites(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	dual := func(oldClass, newClass string) NewCharacter {
		return multi("human", NewClass{Class: oldClass, LeftAtLevel: 9}, NewClass{Class: newClass})
	}
	scores := func(kv ...any) map[string]int {
		m := map[string]int{"str": 17, "dex": 17, "con": 17, "int": 17, "wis": 17, "cha": 17}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1].(int)
		}
		return m
	}
	for _, tc := range []struct {
		name   string
		n      NewCharacter
		scores map[string]int
		want   []string
	}{
		{"both met", dual("fighter", "mage"), scores(), []string{}},
		{"the minimums exactly", dual("fighter", "mage"), scores("str", 15, "int", 17), []string{}},
		{"under 15 in the class left", dual("fighter", "mage"), scores("str", 14), []string{"classes.0.class: " + CodePrimeRequisiteTooLow}},
		{"under 17 in the new class", dual("fighter", "mage"), scores("int", 16), []string{"classes.1.class: " + CodePrimeRequisiteTooLow}},
		{
			"both short", dual("fighter", "mage"), scores("str", 14, "int", 16),
			[]string{"classes.0.class: " + CodePrimeRequisiteTooLow, "classes.1.class: " + CodePrimeRequisiteTooLow},
		},
		{
			"one refusal per class however many requisites fail", dual("fighter", "bard"), scores("dex", 10, "cha", 10),
			[]string{"classes.1.class: " + CodePrimeRequisiteTooLow},
		},
		{"a multi-class character has no such rule", multi("half_elf", NewClass{Class: "fighter"}, NewClass{Class: "mage"}), scores("str", 3, "int", 3), []string{}},
		{"the one-class case has none", NewCharacter{Class: "fighter"}, scores("str", 3), []string{}},
		{"a score not entered is not judged", dual("fighter", "mage"), map[string]int{}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codes(t, primeRefusals(tc.n, defs, tc.scores)))
		})
	}
}
