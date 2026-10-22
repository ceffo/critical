package classes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// withFeatures is a warrior test class listing fs.
func withFeatures(fs ...classes.Feature) classes.Class {
	return classes.Class{ID: "test_class", Group: "warrior", PrimeRequisites: []string{"str"}, Experience: xp(1000), Features: fs}
}

func rogueSkills() *classes.RogueSkills {
	return &classes.RogueSkills{Set: "test_set", Budget: classes.SkillBudget{First: 10, PerLevel: 5}, Armour: []classes.SkillArmour{{Code: "none", Column: "none"}}}
}

// ADR 0040 steps 3 and 8: every kind of the closed list is accepted with
// exactly its params; a missing param, a param of another kind, an unknown
// kind, a kind listed twice and a from_level outside 1..20 are refused when
// the registry is built.
func TestFeatures_Step3_EveryKindParsesItsParams(t *testing.T) {
	spheres := &classes.SphereAccess{Major: []string{"all"}, Minor: []string{"sun"}, MinorMaxLevel: 3}
	book := &classes.Spellbook{MaxSpellLevel: 9, FreeSpellPerNewSpellLevel: true}
	valid := map[string]classes.Feature{
		classes.KindExceptionalStrength:  {Kind: classes.KindExceptionalStrength},
		classes.KindMeleeAttacksPerRound: {Kind: classes.KindMeleeAttacksPerRound, Table: "any_table"},
		classes.KindWeaponSpecialization: {Kind: classes.KindWeaponSpecialization},
		classes.KindWisdomBonusSpells:    {Kind: classes.KindWisdomBonusSpells},
		classes.KindSpellFailure:         {Kind: classes.KindSpellFailure, Ability: "wis"},
		classes.KindSphereAccess:         {Kind: classes.KindSphereAccess, Spheres: spheres},
		classes.KindTurnUndead:           {Kind: classes.KindTurnUndead, Table: "any_table"},
		classes.KindRogueSkills:          {Kind: classes.KindRogueSkills, RogueSkills: rogueSkills()},
		classes.KindBackstab:             {Kind: classes.KindBackstab, Table: "any_table"},
		classes.KindSpellbook:            {Kind: classes.KindSpellbook, Spellbook: book},
	}
	require.Len(t, valid, len(classes.FeatureKinds), "one case per kind of the closed list")
	for _, kind := range classes.FeatureKinds {
		t.Run(kind, func(t *testing.T) {
			f := valid[kind]
			r := registry(t, withFeatures(f))
			c, ok := r.Class("test_class")
			require.True(t, ok)
			got, ok := c.Feature(kind, 1)
			require.True(t, ok)
			f.FromLevel = 1
			assert.Equal(t, f, got, "kept with its params, from_level 1 when left out")
		})
	}

	refused := []struct {
		name string
		fs   []classes.Feature
	}{
		{"an unknown kind", []classes.Feature{{Kind: "test_flight"}}},
		{"a kind listed twice", []classes.Feature{{Kind: classes.KindBackstab, Table: "t"}, {Kind: classes.KindBackstab, Table: "t"}}},
		{"from_level above 20", []classes.Feature{{Kind: classes.KindExceptionalStrength, FromLevel: 21}}},
		{"a negative from_level", []classes.Feature{{Kind: classes.KindExceptionalStrength, FromLevel: -1}}},
		{"a table missing", []classes.Feature{{Kind: classes.KindTurnUndead}}},
		{"a param on a kind without params", []classes.Feature{{Kind: classes.KindExceptionalStrength, Table: "t"}}},
		{"another kind's param", []classes.Feature{{Kind: classes.KindBackstab, Table: "t", Spellbook: book}}},
		{"spell_failure without its ability", []classes.Feature{{Kind: classes.KindSpellFailure}}},
		{"spell_failure on an unknown ability", []classes.Feature{{Kind: classes.KindSpellFailure, Ability: "luck"}}},
		{"sphere_access without spheres", []classes.Feature{{Kind: classes.KindSphereAccess}}},
		{"sphere_access with minor_max_level 0", []classes.Feature{{Kind: classes.KindSphereAccess, Spheres: &classes.SphereAccess{}}}},
		{"rogue_skills without params", []classes.Feature{{Kind: classes.KindRogueSkills}}},
		{"rogue_skills without a set", []classes.Feature{{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{Armour: rogueSkills().Armour}}}},
		{"rogue_skills with a negative budget", []classes.Feature{{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{Set: "s", Budget: classes.SkillBudget{PerLevel: -1}, Armour: rogueSkills().Armour}}}},
		{"rogue_skills without armour", []classes.Feature{{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{Set: "s"}}}},
		{"rogue_skills with an armour twice", []classes.Feature{{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{Set: "s", Armour: []classes.SkillArmour{{Code: "none"}, {Code: "none"}}}}}},
		{"spellbook without params", []classes.Feature{{Kind: classes.KindSpellbook}}},
		{"spellbook above 9th level", []classes.Feature{{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 10}}}},
		{"spellbook under 1st level", []classes.Feature{{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{}}}},
	}
	for _, tc := range refused {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			_, err := classes.New([]classes.Class{withFeatures(tc.fs...)}, groups("warrior"), []classes.Race{{ID: "human", Name: localized.Text{"en": "Human"}}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test_class", "names the class")
		})
	}
}

// ADR 0040 step 5: a class has a kind from the from_level it lists, not
// below; Lists answers before a level is known (the creation options); the
// params come back as copies, so a reader cannot change the definition.
func TestLookup_Step5_AFeatureThroughTheClassEntry(t *testing.T) {
	r := registry(t, withFeatures(
		classes.Feature{Kind: classes.KindBackstab, FromLevel: 4, Table: "t"},
		classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: rogueSkills()},
	))
	c, err := r.Get("test_class")
	require.NoError(t, err)

	for _, tc := range []struct {
		level int
		want  bool
	}{{1, false}, {3, false}, {4, true}, {20, true}} {
		_, ok := c.Feature(classes.KindBackstab, tc.level)
		assert.Equal(t, tc.want, ok, "backstab at level %d", tc.level)
		assert.Equal(t, tc.want, c.Has(classes.KindBackstab, tc.level), "Has at level %d", tc.level)
	}
	f, ok := c.Lists(classes.KindBackstab)
	require.True(t, ok, "listed whatever the level")
	assert.Equal(t, 4, f.FromLevel)
	assert.False(t, c.Has(classes.KindTurnUndead, 20), "a kind the class does not list")

	rs, ok := c.Feature(classes.KindRogueSkills, 1)
	require.True(t, ok)
	rs.RogueSkills.Armour[0].Column = "changed"
	rs.RogueSkills.Budget.First = 99
	again, err := r.Get("test_class")
	require.NoError(t, err)
	got, _ := again.Lists(classes.KindRogueSkills)
	assert.Equal(t, rogueSkills(), got.RogueSkills, "the registry's params are untouched")
}

// ADR 0030 step 15.2: a major sphere opens every spell level, a minor one
// only up to minor_max_level, any other sphere is closed; one open sphere
// among several opens the spell.
func TestSphereAccess_Step15_Opens(t *testing.T) {
	a := classes.SphereAccess{Major: []string{"combat"}, Minor: []string{"fire"}, MinorMaxLevel: 2}
	assert.True(t, a.Opens(7, []string{"combat"}), "major at 7th")
	assert.True(t, a.Opens(2, []string{"fire"}), "minor at the cap")
	assert.False(t, a.Opens(3, []string{"fire"}), "minor over the cap")
	assert.False(t, a.Opens(1, []string{"water"}), "closed")
	assert.True(t, a.Opens(5, []string{"fire", "combat"}), "open through the major one")
	assert.Equal(t, []string{"combat", "fire"}, a.Codes())
}
