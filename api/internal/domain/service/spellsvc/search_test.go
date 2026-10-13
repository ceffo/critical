package spellsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// catalogue is a fixture in the Reader's order (level, name, class).
func catalogue() []spellsvc.Spell {
	return []spellsvc.Spell{
		{ID: "burning_hands", Name: "Burning Hands", Class: "Wizard", Level: 1, School: "Alteration"},
		{ID: "detect_magic_priest", Name: "Detect Magic", Class: "Priest", Level: 1, School: "Divination", Sphere: "Divination"},
		{ID: "detect_magic", Name: "Detect Magic", Class: "Wizard", Level: 1, School: "Divination"},
		{ID: "faerie_fire", Name: "Faerie Fire", Class: "Priest", Level: 1, School: "Alteration", Sphere: "Elemental Fire"},
		{ID: "fire", Name: "Fire", Class: "Wizard", Level: 2, School: "Evocation"},
		{ID: "fire_trap", Name: "Fire Trap", Class: "Wizard", Level: 2, School: "Abjuration, Evocation"},
		{ID: "hold_person_priest", Name: "Hold Person", Class: "Priest", Level: 2, School: "Enchantment/Charm", Sphere: "Charm"},
		{ID: "fireball", Name: "Fireball", Class: "Wizard", Level: 3, School: "Invocation/Evocation"},
		{ID: "fire_shield", Name: "Fire Shield", Class: "Wizard", Level: 4, School: "Evocation, Alteration"},
		{ID: "fortify_l4", Name: "Fortify", Class: "Priest", Level: 4, School: "Necromantic", Sphere: "Wards Elemental Air"},
		{ID: "wall_of_fire", Name: "Wall of Fire", Class: "Wizard", Level: 4, School: "Evocation"},
		{ID: "tensers_brawl", Name: "Tenser's Brawl", Class: "Wizard", Level: 5, School: "Alteration, Geometry"},
		{ID: "sands_of_time", Name: "Sands of Time", Class: "Wizard", Level: 6, School: "Dimension"},
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
			"a client match alone finds a spell the English text does not",
			spellsvc.Filter{Query: "mains", Names: []spellsvc.NameMatch{{Key: "burning_hands", Tier: spellsvc.TierPrefix}}},
			[]string{"burning_hands"},
		},
		{
			"client tiers rank with the English ones",
			spellsvc.Filter{Query: "fire", Names: []spellsvc.NameMatch{
				{Key: "wall_of_fire", Tier: spellsvc.TierExact},
				{Key: "burning_hands", Tier: spellsvc.TierContains},
			}},
			[]string{"fire", "wall_of_fire", "fire_trap", "fireball", "fire_shield", "burning_hands", "faerie_fire"},
		},
		{
			"the best tier wins: an English prefix beats a client contains",
			spellsvc.Filter{Query: "fire", Names: []spellsvc.NameMatch{{Key: "fireball", Tier: spellsvc.TierContains}}},
			[]string{"fire", "fire_trap", "fireball", "fire_shield", "faerie_fire", "wall_of_fire"},
		},
		{
			"a catalogue key names the _priest and _l<n> spells of that name",
			spellsvc.Filter{Query: "détection", Names: []spellsvc.NameMatch{
				{Key: "detect_magic", Tier: spellsvc.TierPrefix},
				{Key: "fortify", Tier: spellsvc.TierContains},
			}},
			[]string{"detect_magic_priest", "detect_magic", "fortify_l4"},
		},
		{
			"an unknown tier is ignored",
			spellsvc.Filter{Names: []spellsvc.NameMatch{{Key: "fireball", Tier: spellsvc.Tier(9)}}},
			[]string{},
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

func TestBaseKey(t *testing.T) {
	for in, want := range map[string]string{
		"hold_person_priest":    "hold_person",
		"fortify_l4":            "fortify",
		"cure_wounds_priest_l4": "cure_wounds",
		"fireball":              "fireball",
	} {
		assert.Equal(t, want, spellsvc.BaseKey(in), in)
	}
}
