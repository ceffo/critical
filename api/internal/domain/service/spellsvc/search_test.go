package spellsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
)

// catalogue is a fixture in the Reader's order (level, name, class).
func catalogue() []spellsvc.Spell {
	return []spellsvc.Spell{
		{ID: "burning_hands", Name: localized.Text{"en": "Burning Hands", "fr": "Mains brûlantes"}, Class: "Wizard", Level: 1, School: localized.Of("Alteration")},
		{ID: "detect_magic_priest", Name: localized.Text{"en": "Detect Magic", "fr": "Détection de la magie"}, Class: "Priest", Level: 1, School: localized.Of("Divination"), Sphere: localized.Of("Divination")},
		{ID: "detect_magic", Name: localized.Text{"en": "Detect Magic", "fr": "Détection de la magie"}, Class: "Wizard", Level: 1, School: localized.Of("Divination")},
		{ID: "faerie_fire", Name: localized.Of("Faerie Fire"), Class: "Priest", Level: 1, School: localized.Of("Alteration"), Sphere: localized.Of("Elemental Fire")},
		{ID: "fire", Name: localized.Text{"en": "Fire", "fr": "Feu"}, Class: "Wizard", Level: 2, School: localized.Of("Evocation")},
		{ID: "fire_trap", Name: localized.Text{"en": "Fire Trap", "fr": "Piège de feu"}, Class: "Wizard", Level: 2, School: localized.Of("Abjuration, Evocation")},
		{ID: "hold_person_priest", Name: localized.Of("Hold Person"), Class: "Priest", Level: 2, School: localized.Of("Enchantment/Charm"), Sphere: localized.Of("Charm")},
		{ID: "fireball", Name: localized.Text{"en": "Fireball", "fr": "Boule de feu"}, Class: "Wizard", Level: 3, School: localized.Of("Invocation/Evocation")},
		{ID: "fire_shield", Name: localized.Of("Fire Shield"), Class: "Wizard", Level: 4, School: localized.Of("Evocation, Alteration")},
		{ID: "fortify_l4", Name: localized.Of("Fortify"), Class: "Priest", Level: 4, School: localized.Of("Necromantic"), Sphere: localized.Of("Wards Elemental Air")},
		{ID: "wall_of_fire", Name: localized.Text{"en": "Wall of Fire", "fr": "Mur de feu en boule"}, Class: "Wizard", Level: 4, School: localized.Of("Evocation")},
		{ID: "tensers_brawl", Name: localized.Of("Tenser's Brawl"), Class: "Wizard", Level: 5, School: localized.Of("Alteration, Geometry")},
		{ID: "sands_of_time", Name: localized.Text{"en": "Sands of Time", "fr": "Sables du temps et Œil"}, Class: "Wizard", Level: 6, School: localized.Of("Dimension")},
	}
}

// ids runs the real service over spells with the given book and returns the
// ids of the first page of 200, in order.
func ids(t *testing.T, f spellsvc.Filter, book []string, spells []spellsvc.Spell) []string {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	reader := mocks.NewMockReader(t)
	b := mocks.NewMockSpellbook(t)
	b.EXPECT().Contains(mock.Anything).RunAndReturn(func(id string) bool {
		for _, k := range book {
			if k == id {
				return true
			}
		}
		return false
	}).Maybe()
	svc, err := spellsvc.New(dal, reader, b)
	require.NoError(t, err)
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)
	expectDetails(reader, spells)
	got, err := svc.Search(context.Background(), f, spellsvc.PageRequest{PageSize: spellsvc.MaxPageSize})
	require.NoError(t, err)
	out := []string{}
	for _, s := range got.Items {
		out = append(out, s.ID)
	}
	assert.Equal(t, len(out), got.Total, "one page holds every match")
	return out
}

func TestSearch_RankingTiers(t *testing.T) {
	tests := []struct {
		name string
		f    spellsvc.Filter
		want []string
	}{
		{
			"no text keeps the reader's level-then-name order",
			spellsvc.Filter{Levels: []int{2}},
			[]string{"fire", "fire_trap", "hold_person_priest"},
		},
		{
			"exact, then prefix, then contains; level then name within a tier",
			spellsvc.Filter{Query: "fire"},
			[]string{"fire", "fire_trap", "fireball", "fire_shield", "faerie_fire", "wall_of_fire"},
		},
		{
			"case-insensitive, surrounding spaces ignored",
			spellsvc.Filter{Query: "  FIREBALL "},
			[]string{"fireball"},
		},
		{
			"the French name matches, accents and case ignored",
			spellsvc.Filter{Query: "MAINS"},
			[]string{"burning_hands"},
		},
		{
			"an exact French name ranks before French substrings",
			spellsvc.Filter{Query: "feu"},
			[]string{"fire", "fire_trap", "fireball", "wall_of_fire"},
		},
		{
			"typed without accents, a French name with accents matches",
			spellsvc.Filter{Query: "detection"},
			[]string{"detect_magic_priest", "detect_magic"},
		},
		{
			"typed with accents, too",
			spellsvc.Filter{Query: "Détection de la"},
			[]string{"detect_magic_priest", "detect_magic"},
		},
		{
			"the best tier of the two names wins",
			spellsvc.Filter{Query: "boule"},
			[]string{"fireball", "wall_of_fire"},
		},
		{
			"a ligature matches its letters",
			spellsvc.Filter{Query: "oeil"},
			[]string{"sands_of_time"},
		},
		{
			"no match",
			spellsvc.Filter{Query: "zzz"},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spells := catalogue()
			if len(tt.f.Levels) > 0 { // the Reader applies Levels; mimic it
				var kept []spellsvc.Spell
				for _, s := range spells {
					for _, l := range tt.f.Levels {
						if s.Level == l {
							kept = append(kept, s)
						}
					}
				}
				spells = kept
			}
			assert.Equal(t, tt.want, ids(t, tt.f, nil, spells))
		})
	}
}

func TestSearch_MineKeepsOnlyTheSpellbook(t *testing.T) {
	book := []string{"burning_hands", "fireball", "not_in_catalogue"}
	f := spellsvc.Filter{Class: "wizard", Mine: true}
	assert.Equal(t, []string{"burning_hands", "fireball"}, ids(t, f, book, catalogue()))

	f = spellsvc.Filter{Class: "wizard", Mine: true, Query: "fire"}
	assert.Equal(t, []string{"fireball"}, ids(t, f, book, catalogue()), "mine combines with the search")
}

func TestSearch_SchoolGroupsAndSpheres(t *testing.T) {
	tests := []struct {
		name string
		f    spellsvc.Filter
		want []string
	}{
		{
			"Invocation/Evocation includes spells listed as Evocation",
			spellsvc.Filter{School: spellsvc.SchoolEvocation},
			[]string{"fire", "fire_trap", "fireball", "fire_shield", "wall_of_fire"},
		},
		{
			"Enchantment/Charm",
			spellsvc.Filter{School: spellsvc.SchoolEnchantment},
			[]string{"hold_person_priest"},
		},
		{
			"Necromantic is Necromancy",
			spellsvc.Filter{School: spellsvc.SchoolNecromancy},
			[]string{"fortify_l4"},
		},
		{
			"Other holds the Tome of Magic schools",
			spellsvc.Filter{School: spellsvc.SchoolOther},
			[]string{"tensers_brawl", "sands_of_time"},
		},
		{
			"a sphere",
			spellsvc.Filter{Class: "priest", Sphere: "Elemental Fire"},
			[]string{"faerie_fire"},
		},
		{
			"a sphere glued to another by the scrape",
			spellsvc.Filter{Sphere: "Elemental Air"},
			[]string{"fortify_l4"},
		},
		{
			"school and search combine",
			spellsvc.Filter{School: spellsvc.SchoolAlteration, Query: "fire"},
			[]string{"fire_shield", "faerie_fire"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ids(t, tt.f, nil, catalogue()))
		})
	}
}

func TestSpellSchoolGroups(t *testing.T) {
	tests := []struct {
		school string
		want   []string
	}{
		{"Evocation", []string{"evocation"}},
		{"Invocation", []string{"evocation"}},
		{"Invocation/Evocation", []string{"evocation"}},
		{"Enchantment", []string{"enchantment"}},
		{"Conjuration, Summoning", []string{"conjuration"}},
		{"(Conjuration/Summoning", []string{"conjuration"}},
		{"Illusion", []string{"illusion"}},
		{"Alteration, Phantasm", []string{"alteration", "illusion"}},
		{"Divination/Illusion", []string{"divination", "illusion"}},
		{"Alteration Enchantment/Charm", []string{"alteration", "enchantment"}},
		{"Necromancy/Alteration", []string{"necromancy", "alteration"}},
		{"Alteration, Elemental Water, Alchemy", []string{"alteration", "other"}},
		{"Conjuration/Summoning, all Elemental, Geometry", []string{"conjuration", "other"}},
		{"Dimension", []string{"other"}},
		{"Various schools", []string{"other"}},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.school, func(t *testing.T) {
			assert.Equal(t, tt.want, spellsvc.SpellSchoolGroups(tt.school))
		})
	}
}

func TestSpellSpheres(t *testing.T) {
	tests := []struct {
		sphere string
		want   []string
	}{
		{"Charm", []string{"Charm"}},
		{"Numbers, Divination", []string{"Divination", "Numbers"}},
		{"Divination (Animal, Plant)", []string{"Animal", "Divination", "Plant"}},
		{"Wards Elemental Air,Elemental Earth", []string{"Elemental Air", "Elemental Earth", "Wards"}},
		{"Elemental Air, Water", []string{"Elemental Air", "Elemental Water"}},
		{"Divination, Elemental", []string{"Divination", "Elemental Air", "Elemental Earth", "Elemental Fire", "Elemental Water"}},
		{"Elemental Fire, Elemental Plant", []string{"Elemental Fire", "Plant"}},
		{"All", []string{"All"}},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.sphere, func(t *testing.T) {
			assert.Equal(t, tt.want, spellsvc.SpellSpheres(tt.sphere))
		})
	}
}

func TestNames_ListsEveryCatalogueSpellWithItsNames(t *testing.T) {
	reader := mocks.NewMockReader(t)
	svc, err := spellsvc.New(mocks.NewMockDAL(t), reader, mocks.NewMockSpellbook(t))
	require.NoError(t, err)
	reader.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(catalogue()[:3], nil)
	got, err := svc.Names(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.SpellName{
		{ID: "burning_hands", Name: localized.Text{"en": "Burning Hands", "fr": "Mains brûlantes"}},
		{ID: "detect_magic_priest", Name: localized.Text{"en": "Detect Magic", "fr": "Détection de la magie"}},
		{ID: "detect_magic", Name: localized.Text{"en": "Detect Magic", "fr": "Détection de la magie"}},
	}, got)
}

// The level filter's choices come from the catalogue (ADR 0027 step 13.2):
// each level once, ascending, whatever the reader's order; a level no spell
// has (0, 5 here) is absent.
func TestLevels_AreTheCatalogueLevelsAscendingOnce(t *testing.T) {
	reader := mocks.NewMockReader(t)
	svc, err := spellsvc.New(mocks.NewMockDAL(t), reader, mocks.NewMockSpellbook(t))
	require.NoError(t, err)
	reader.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return([]spellsvc.Spell{
		{ID: "d", Level: 4}, {ID: "a", Level: 2}, {ID: "b", Level: 1}, {ID: "c", Level: 2}, {ID: "e", Level: 9},
	}, nil)
	got, err := svc.Levels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 4, 9}, got)
}

func TestLevels_PassesTheReaderError(t *testing.T) {
	reader := mocks.NewMockReader(t)
	svc, err := spellsvc.New(mocks.NewMockDAL(t), reader, mocks.NewMockSpellbook(t))
	require.NoError(t, err)
	boom := errors.New("boom")
	reader.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, boom)
	_, err = svc.Levels(context.Background())
	assert.ErrorIs(t, err, boom)
}
