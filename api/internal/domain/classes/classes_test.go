package classes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// xp is an experience table of MaxLevel rows from 0, step apart.
func xp(step int) []int {
	out := make([]int, classes.MaxLevel)
	for i := range out {
		out[i] = i * step
	}
	return out
}

// skald is a class the code has never seen: a rogue with two prime
// requisites, a minimum in each and a narrowed alignment list.
func skald() classes.Class {
	return classes.Class{
		ID: "test_skald", Name: localized.Text{"en": "Skald"}, Group: "rogue",
		PrimeRequisites: []string{"cha", "str"},
		AbilityMinimums: []classes.AbilityMinimum{{Ability: "cha", Min: 14}, {Ability: "str", Min: 11}},
		Alignments:      []string{"chaotic_neutral", "chaotic_good"},
		Experience:      xp(1000),
	}
}

func registry(t *testing.T, cls ...classes.Class) *classes.Registry {
	t.Helper()
	r, err := classes.New(cls, []string{"warrior", "rogue"}, []classes.Race{{ID: "human", Name: localized.Text{"en": "Human"}}})
	require.NoError(t, err)
	return r
}

// ADR 0040 step 5: every answer about a class comes from its definition
// through the registry, so a class the code has never seen (a rogue skald)
// gets its group, prime requisites, minimums, alignments and levels with no
// code change, and a class the catalogue does not define is an
// ErrUnknownClass naming it.
func TestLookup_Step5_ANewClassOfKnownFieldsNeedsNoCode(t *testing.T) {
	plain := classes.Class{ID: "test_brute", Group: "warrior", PrimeRequisites: []string{"str"}, Experience: xp(2000)}
	r := registry(t, plain, skald())

	assert.Equal(t, []string{"test_brute", "test_skald"}, r.ClassIDs(), "catalogue order")
	g, err := r.Group("test_skald")
	require.NoError(t, err)
	assert.Equal(t, "rogue", g)

	c, err := r.Get("test_skald")
	require.NoError(t, err)
	assert.Equal(t, []string{"cha", "str"}, c.PrimeRequisites)
	assert.Equal(t, []classes.AbilityMinimum{{Ability: "str", Min: 11}, {Ability: "cha", Min: 14}}, c.AbilityMinimums,
		"the minimums in the sheet's ability order")
	assert.Equal(t, []string{"chaotic_good", "chaotic_neutral"}, c.Alignments, "the book's alignment order")
	assert.True(t, c.AlignmentAllowed("chaotic_good"))
	assert.False(t, c.AlignmentAllowed("lawful_good"))
	assert.Equal(t, []string{"str"}, c.UnmetMinimums(map[string]int{"str": 10, "cha": 14}), "cha 14 meets its minimum")
	assert.Empty(t, c.UnmetMinimums(map[string]int{"dex": 3}), "an ability with no score yet is not unmet")
	assert.Equal(t, 1, c.LevelFor(999))
	assert.Equal(t, 2, c.LevelFor(1000))
	assert.Equal(t, classes.MaxLevel, c.LevelFor(1_000_000))
	th, err := c.XPThreshold(5)
	require.NoError(t, err)
	assert.Equal(t, 4000, th)
	_, err = c.XPThreshold(classes.MaxLevel + 1)
	require.Error(t, err)

	b, ok := r.Class("test_brute")
	require.True(t, ok)
	assert.Equal(t, classes.Alignments, b.Alignments, "no list: every alignment")

	_, err = r.Get("paladin")
	require.ErrorIs(t, err, classes.ErrUnknownClass)
	require.ErrorContains(t, err, `"paladin"`)
	_, err = r.Group("paladin")
	require.ErrorIs(t, err, classes.ErrUnknownClass)
}

// ADR 0040 step 5: the registry hands out copies; a reader editing a list
// changes nothing for the next reader.
func TestLookup_Step5_TheDefinitionsAreCopies(t *testing.T) {
	r := registry(t, skald())
	c, _ := r.Class("test_skald")
	c.PrimeRequisites[0], c.Experience[1] = "dex", 7
	again, _ := r.Class("test_skald")
	assert.Equal(t, []string{"cha", "str"}, again.PrimeRequisites)
	assert.Equal(t, 1000, again.Experience[1])
}

// ADR 0040 step 8, the rules New holds (the importer runs it at start-up):
// each refusal names the class and what is wrong.
func TestImport_Step8_TheRegistryRefusesBrokenDefinitions(t *testing.T) {
	with := func(edit func(*classes.Class)) classes.Class {
		c := skald()
		edit(&c)
		return c
	}
	tests := []struct {
		name string
		cls  []classes.Class
		want string
	}{
		{"an unknown group", []classes.Class{with(func(c *classes.Class) { c.Group = "psionicist" })}, `class "test_skald": unknown group "psionicist"`},
		{"no prime requisite", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = nil })}, "no prime requisite"},
		{"an unknown ability", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = []string{"luck"} })}, `unknown ability "luck"`},
		{"a prime requisite twice", []classes.Class{with(func(c *classes.Class) { c.PrimeRequisites = []string{"str", "str"} })}, `"str" twice`},
		{"a minimum of an unknown ability", []classes.Class{with(func(c *classes.Class) {
			c.AbilityMinimums = []classes.AbilityMinimum{{Ability: "luck", Min: 9}}
		})}, "ability minimums: an unknown ability"},
		{"an unknown alignment", []classes.Class{with(func(c *classes.Class) { c.Alignments = []string{"neutral"} })}, `unknown alignment "neutral"`},
		{"19 experience rows", []classes.Class{with(func(c *classes.Class) { c.Experience = c.Experience[:19] })}, "got 19 rows"},
		{"a class twice", []classes.Class{skald(), skald()}, `classes: "test_skald" twice`},
		{"a class without an id", []classes.Class{with(func(c *classes.Class) { c.ID = "" })}, "an empty id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classes.New(tc.cls, []string{"warrior", "rogue"}, []classes.Race{{ID: "human"}})
			require.ErrorContains(t, err, tc.want)
		})
	}
	_, err := classes.New(nil, []string{"rogue"}, []classes.Race{{ID: "human"}})
	require.ErrorContains(t, err, "needs classes, groups and races")
	_, err = classes.New([]classes.Class{skald()}, []string{"rogue"}, []classes.Race{{ID: "elf"}, {ID: "elf"}})
	require.ErrorContains(t, err, `races: "elf" twice`)
}
