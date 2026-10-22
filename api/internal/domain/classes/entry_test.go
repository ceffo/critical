package classes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
