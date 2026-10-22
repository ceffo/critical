package spheresvc_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spheresvc"
)

type book struct{}

func (book) All(context.Context, int64) ([]prepsvc.SpellbookEntry, error) {
	return []prepsvc.SpellbookEntry{{ID: "sleep", Name: "Sleep", Level: 1}}, nil
}

func (b book) Of(ctx context.Context, id int64, _ string) ([]prepsvc.SpellbookEntry, error) {
	return b.All(ctx, id)
}

func (book) SpellKeys(context.Context, int64) ([]string, error) { return []string{"sleep"}, nil }

type chars struct{ c *charactersvc.Character }

func (c chars) GetCharacter(context.Context, int64) (*charactersvc.Character, error) { return c.c, nil }

func priest(id string, level int, spheres ...string) catalogue.Spell {
	return catalogue.Spell{ID: id, Class: "Priest", Level: level, Spheres: spheres}
}

var cat = catalogue.Catalogue{Spells: []catalogue.Spell{
	priest("flame_strike", 5, "combat"),
	priest("blade_barrier", 6, "combat"),
	priest("bless", 1, "all"),
	priest("chant", 2, "combat"),
	priest("produce_flame", 2, "elemental_fire"),
	priest("produce_fire", 4, "elemental_fire"),
	priest("thought_capture", 1, "thought"),
	{ID: "sleep", Class: "Wizard", Level: 1},
}}

func access(t *testing.T, rows ...ruletablesvc.SphereAccessRow) *ruletablesvc.SphereAccess {
	t.Helper()
	a, err := ruletablesvc.NewSphereAccess(rows, cataloguetest.Definitions(t))
	require.NoError(t, err)
	return a
}

func cleric(t *testing.T) *ruletablesvc.SphereAccess {
	return access(t,
		ruletablesvc.SphereAccessRow{Class: "cleric", Sphere: "all", Access: ruletablesvc.AccessMajor},
		ruletablesvc.SphereAccessRow{Class: "cleric", Sphere: "combat", Access: ruletablesvc.AccessMajor},
		ruletablesvc.SphereAccessRow{Class: "cleric", Sphere: "elemental_fire", Access: ruletablesvc.AccessMinor},
	)
}

func ids(es []prepsvc.SpellbookEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

// ADR 0030 step 15: a cleric prepares from the priest spells its spheres
// open, only at the spell levels it has slots of (the ruling), level then id
// order; another class from its spellbook; without a sphere access a cleric
// prepares nothing.
func TestSpheres_Step15_WhatAClassPreparesFrom(t *testing.T) {
	ctx := context.Background()
	c := &charactersvc.Character{Class: "cleric", Derived: charactersvc.Derived{SpellSlots: map[int]int{1: 3, 2: 2, 4: 1}}}
	s, err := spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{c}, Access: cleric(t), Catalogue: cat})
	require.NoError(t, err)
	all, err := s.All(ctx, 1)
	require.NoError(t, err)
	// No 5th-level slot: no Flame Strike. Elemental (minor) stops at 3rd.
	assert.Equal(t, []string{"bless", "chant", "produce_flame"}, ids(all))
	keys, err := s.SpellKeys(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"bless", "chant", "produce_flame"}, keys)

	mage := &charactersvc.Character{Class: "mage", Derived: charactersvc.Derived{SpellSlots: map[int]int{1: 1}}}
	s, err = spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{mage}, Access: cleric(t), Catalogue: cat})
	require.NoError(t, err)
	all, err = s.All(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"sleep"}, ids(all), "the stored spellbook")

	s, err = spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{c}, Catalogue: cat})
	require.NoError(t, err)
	all, err = s.All(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"sleep"}, ids(all), "no access (format 5): the stored spellbook, empty for a real cleric")
}

// A sphere access row whose code is on no spell (a typo) is refused at load:
// it would silently close the sphere.
func TestSpheres_RefusesAnUnknownSphereCode(t *testing.T) {
	typo := access(t, ruletablesvc.SphereAccessRow{Class: "cleric", Sphere: "combatt", Access: ruletablesvc.AccessMajor})
	_, err := spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{}, Access: typo, Catalogue: cat})
	require.ErrorContains(t, err, `"combatt" is on no spell`)
}

// ADR 0030 steps 13 and 15: at level 11 Wisdom 17 opens a 6th-level slot
// (PHB Table 5), so 6th-level spells are offered; Wisdom 16 has none, so
// none are (the slots as ruletablesvc.PriestSpellsPerDay gives them).
func TestSpheres_Step15_SixthLevelNeedsWisdom17(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		slots map[int]int
		want  bool
	}{
		{"Wisdom 16", map[int]int{1: 7, 2: 6, 3: 4, 4: 3, 5: 2}, false},
		{"Wisdom 17", map[int]int{1: 7, 2: 6, 3: 5, 4: 3, 5: 2, 6: 1}, true},
	} {
		c := &charactersvc.Character{Class: "cleric", Derived: charactersvc.Derived{RealizedLevel: 11, SpellSlots: tc.slots}}
		s, err := spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{c}, Access: cleric(t), Catalogue: cat})
		require.NoError(t, err)
		all, err := s.All(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, tc.want, slices.Contains(ids(all), "blade_barrier"), tc.name)
	}
}

// ADR 0042 step 14: each spellcasting entry prepares its own spells. A
// mage 6/cleric 5 (the mage first) prepares the mage's from the stored
// spellbook and the cleric's from the spheres, within the cleric entry's
// slots (no 4th level: no Produce Fire), whichever entry is first.
func TestSpheres_Step14_EachEntryPreparesFromItsOwnSource(t *testing.T) {
	ctx := context.Background()
	c := &charactersvc.Character{Class: "mage", Derived: charactersvc.Derived{
		SpellSlots: map[int]int{1: 4, 2: 2, 3: 2},
		Classes: []charactersvc.EntryDerived{
			{Class: "mage", State: classes.EntryActive, RealizedLevel: 6, Casts: true, SpellSlots: map[int]int{1: 4, 2: 2, 3: 2}},
			{Class: "cleric", State: classes.EntryActive, RealizedLevel: 5, Casts: true, SpellSlots: map[int]int{1: 3, 2: 3, 3: 1}},
		},
	}}
	s, err := spheresvc.New(spheresvc.Deps{Book: book{}, Characters: chars{c}, Access: cleric(t), Catalogue: cat})
	require.NoError(t, err)
	mage, err := s.Of(ctx, 1, "mage")
	require.NoError(t, err)
	assert.Equal(t, []string{"sleep"}, ids(mage), "the stored spellbook")
	priest, err := s.Of(ctx, 1, "cleric")
	require.NoError(t, err)
	assert.Equal(t, []string{"bless", "chant", "produce_flame"}, ids(priest))
}
