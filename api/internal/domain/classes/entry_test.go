package classes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

func active(class string, level int) classes.Entry {
	return classes.Entry{Class: class, RealizedLevel: level, State: classes.EntryActive}
}

func frozen(class string, level int) classes.Entry {
	return classes.Entry{Class: class, RealizedLevel: level, State: classes.EntryFrozen, LeftAtLevel: level}
}

// ADR 0042 step 1: a single-class, a multi-class and a dual-class character are accepted; each
// shape the step refuses is a field error on "classes".
func TestClassEntries_Step1_ShapesAndRefusals(t *testing.T) {
	accepted := map[string][]classes.Entry{
		"single":              {active("bard", 15)},
		"multi two":           {active("fighter", 13), active("cleric", 14)},
		"multi three":         {active("fighter", 4), active("mage", 4), active("thief", 4)},
		"dual":                {frozen("fighter", 9), active("mage", 10)},
		"dual, three classes": {frozen("fighter", 5), frozen("thief", 6), active("mage", 7)},
	}
	for name, entries := range accepted {
		assert.NoError(t, classes.ValidateEntries(entries), name)
	}

	refused := map[string][]classes.Entry{
		"no entry":               {},
		"no active entry":        {frozen("fighter", 5)},
		"a class twice":          {active("fighter", 3), active("fighter", 4)},
		"frozen under level 2":   {frozen("fighter", 1), active("mage", 1)},
		"frozen without left-at": {{Class: "fighter", RealizedLevel: 5, State: classes.EntryFrozen}, active("mage", 1)},
		"active with a left-at":  {{Class: "fighter", RealizedLevel: 5, State: classes.EntryActive, LeftAtLevel: 5}},
		"four active classes":    {active("fighter", 1), active("mage", 1), active("thief", 1), active("cleric", 1)},
		"multi-class and dual":   {frozen("fighter", 5), active("mage", 6), active("thief", 6)},
		"an unknown state":       {{Class: "fighter", RealizedLevel: 1, State: "retired"}},
		"left at another level":  {{Class: "fighter", RealizedLevel: 6, State: classes.EntryFrozen, LeftAtLevel: 5}, active("mage", 1)},
	}
	for name, entries := range refused {
		err := classes.ValidateEntries(entries)
		require.Error(t, err, name)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, name)
		assert.Equal(t, "classes", fe.Field, name)
	}
}

// A single-class character's one entry is its primary; the first active entry is the primary of a
// dual-class character whose old class comes first.
func TestClassEntries_Step1_ThePrimaryIsTheFirstActiveEntry(t *testing.T) {
	e, ok := classes.Primary([]classes.Entry{frozen("fighter", 9), active("mage", 10)})
	require.True(t, ok)
	assert.Equal(t, "mage", e.Class)
	_, ok = classes.Primary([]classes.Entry{frozen("fighter", 9)})
	assert.False(t, ok)
	assert.Equal(t, classes.Entry{Class: "bard", RealizedLevel: 15, StartingXP: 1200, State: classes.EntryActive}, classes.SingleEntry("bard", 15, 1200))
}

// ADR 0042 step 12: "A character has feature kind K when an entry's class
// lists K at or under that entry's realized level (a frozen entry: its
// left_at_level); the rule reads that entry's parameters and level [...]. Two
// entries granting the same kind each keep their own grant, read with their
// own level." A fighter 7/thief 9 backstabs at the thief's 9, not the first
// entry's 7; a dual-class's frozen thief at its left-at level; a thief/bard
// keeps two rogue_skills grants, each at its own level.
func TestLookup_Step12_EachEntryReadsItsOwnLevel(t *testing.T) {
	def := cataloguetest.Tables(t).Class
	for _, tc := range []struct {
		name    string
		entries []classes.Entry
		kind    string
		want    map[string]int
	}{
		{"a fighter 7/thief 9 backstabs at 9", []classes.Entry{active("fighter", 7), active("thief", 9)}, classes.KindBackstab, map[string]int{"thief": 9}},
		{"a frozen thief at its left-at level", []classes.Entry{frozen("thief", 5), active("fighter", 7)}, classes.KindBackstab, map[string]int{"thief": 5}},
		{"two entries granting the kind each keep their own", []classes.Entry{active("thief", 6), active("bard", 4)}, classes.KindRogueSkills, map[string]int{"thief": 6, "bard": 4}},
		{"no entry granting it", []classes.Entry{active("fighter", 7), active("mage", 6)}, classes.KindBackstab, map[string]int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs, err := classes.Grants(tc.entries, def, tc.kind)
			require.NoError(t, err)
			got := map[string]int{}
			for _, g := range gs {
				assert.Equal(t, tc.kind, g.Feature.Kind)
				got[g.Class.ID] = g.Level
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// ADR 0042 step 13: the crossover is the active entry's realized level above every frozen
// entry's left-at level; equal is still before it. A frozen entry is dormant until then.
func TestDualClass_Step13_CrossoverAboveEveryLeftAtLevel(t *testing.T) {
	cases := []struct {
		name    string
		entries []classes.Entry
		crossed bool
	}{
		{"mage 9 after fighter 9: before", []classes.Entry{frozen("fighter", 9), active("mage", 9)}, false},
		{"mage 10 after fighter 9: after", []classes.Entry{frozen("fighter", 9), active("mage", 10)}, true},
		{"mage 1 after fighter 2: before", []classes.Entry{frozen("fighter", 2), active("mage", 1)}, false},
		{"above one frozen entry, not the other", []classes.Entry{frozen("fighter", 5), frozen("thief", 8), active("mage", 7)}, false},
		{"above both frozen entries", []classes.Entry{frozen("fighter", 5), frozen("thief", 8), active("mage", 9)}, true},
		{"single class: nothing to cross", []classes.Entry{active("bard", 1)}, true},
		{"multi-class: nothing to cross", []classes.Entry{active("fighter", 2), active("cleric", 3)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.crossed, classes.Crossed(tc.entries))
			assert.Equal(t, !tc.crossed, classes.Dormant(tc.entries, tc.entries[0]), "the first entry dormant")
			last := tc.entries[len(tc.entries)-1]
			assert.False(t, classes.Dormant(tc.entries, last), "an active entry is never dormant")
		})
	}
}
