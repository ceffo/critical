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
		classes.KindPsionics:             {Kind: classes.KindPsionics, Psionics: psionics()},
		classes.KindSaveBonus:            {Kind: classes.KindSaveBonus, SaveBonus: saveBonus()},
		classes.KindSchoolSpecialization: {Kind: classes.KindSchoolSpecialization, SchoolSpecialization: schoolSpecialization()},
	}
	require.Len(t, valid, len(classes.FeatureKinds), "one case per kind of the closed list")
	for _, kind := range classes.FeatureKinds {
		t.Run(kind, func(t *testing.T) {
			f := valid[kind]
			fs := []classes.Feature{f}
			if kind == classes.KindSchoolSpecialization { // a specialist keeps a spellbook (ADR 0050 step 2.1)
				fs = append(fs, valid[classes.KindSpellbook])
			}
			r := registry(t, withFeatures(fs...))
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
		{"psionics with a param of another kind", []classes.Feature{{Kind: classes.KindPsionics, Psionics: psionics(), Table: "t"}}},
		{"psionics' params on another kind", []classes.Feature{{Kind: classes.KindExceptionalStrength, Psionics: psionics()}}},
		{"save_bonus with a param of another kind", []classes.Feature{{Kind: classes.KindSaveBonus, SaveBonus: saveBonus(), Ability: "wis"}}},
		{"save_bonus without params", []classes.Feature{{Kind: classes.KindSaveBonus}}},
		{"save_bonus against an unknown tag", []classes.Feature{{Kind: classes.KindSaveBonus, SaveBonus: &classes.SaveBonus{Against: "fire", Bonus: 2}}}},
		{"save_bonus of 0", []classes.Feature{{Kind: classes.KindSaveBonus, SaveBonus: &classes.SaveBonus{Against: classes.SaveAgainstMindAffecting}}}},
		{"save_bonus above the maximum", []classes.Feature{{Kind: classes.KindSaveBonus, SaveBonus: &classes.SaveBonus{Against: classes.SaveAgainstMindAffecting, Bonus: classes.MaxSaveBonus + 1}}}},
		{"save_bonus' params on another kind", []classes.Feature{{Kind: classes.KindExceptionalStrength, SaveBonus: saveBonus()}}},
		{"spellbook without params", []classes.Feature{{Kind: classes.KindSpellbook}}},
		{"spellbook above 9th level", []classes.Feature{{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 10}}}},
		{"spellbook under 1st level", []classes.Feature{{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{}}}},
	}
	for _, tc := range refused {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			_, err := classes.New([]classes.Class{withFeatures(tc.fs...)}, groups("warrior"), []classes.Race{{ID: "human", Identity: identity(), Name: localized.Text{"en": "Human"}}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test_class", "names the class")
		})
	}
}

// schoolSpecialization are valid school_specialization params (ADR 0050 step 2): a transmuter's
// of Table 22, with human the only race the test registry defines.
func schoolSpecialization() *classes.SchoolSpecialization {
	return &classes.SchoolSpecialization{
		Schools:           []string{classes.SchoolAlteration},
		OppositionSchools: []string{classes.SchoolAbjuration, classes.SchoolNecromancy},
		Races:             []string{"human"},
	}
}

// ADR 0050 step 2 and 2.1: school_specialization takes schools, opposition schools and races (and
// the races of a multi-class character), each code of the closed list or a defined race; every
// start-up check of 2.1 refuses its offence naming the class, and the kind once per class is the
// registry's "a kind listed twice" (2.2). Until the engine item the kind is not implemented: a
// class listing it is not creatable.
func TestFeatures_Step2_SchoolSpecializationParses(t *testing.T) {
	book := classes.Feature{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 9}}
	specialist := func(mutate func(s *classes.SchoolSpecialization)) []classes.Feature {
		s := schoolSpecialization()
		mutate(s)
		return []classes.Feature{book, {Kind: classes.KindSchoolSpecialization, SchoolSpecialization: s}}
	}
	races := []classes.Race{
		{ID: "human", Identity: identity(), Name: localized.Text{"en": "Human"}},
		{ID: "gnome", Identity: identity(), Name: localized.Text{"en": "Gnome"}},
	}
	build := func(fs []classes.Feature) (*classes.Registry, error) {
		return classes.New([]classes.Class{withFeatures(fs...)}, groups("warrior"), races)
	}

	r, err := build(specialist(func(s *classes.SchoolSpecialization) {
		s.Races, s.MulticlassRaces = []string{"human", "gnome"}, []string{"gnome"}
	}))
	require.NoError(t, err)
	c, ok := r.Class("test_class")
	require.True(t, ok)
	f, ok := c.Feature(classes.KindSchoolSpecialization, 1)
	require.True(t, ok)
	assert.Equal(t, &classes.SchoolSpecialization{
		Schools:           []string{"alteration"},
		OppositionSchools: []string{"abjuration", "necromancy"},
		Races:             []string{"human", "gnome"},
		MulticlassRaces:   []string{"gnome"},
	}, f.SchoolSpecialization, "the params come back as the catalogue wrote them")
	assert.False(t, c.Implemented(), "the kind has no engine yet (ADR 0050 step 17.2): the class is not creatable")

	// The diviner's reading (Q3): lesser divination among his schools, never in an opposition.
	_, err = build(specialist(func(s *classes.SchoolSpecialization) {
		s.Schools = []string{classes.SchoolGreaterDivination, classes.SchoolLesserDivination}
	}))
	require.NoError(t, err)

	refused := []struct {
		name   string
		fs     []classes.Feature
		needle string
	}{
		{"an unknown school", specialist(func(s *classes.SchoolSpecialization) { s.Schools = []string{"dimension"} }), `unknown school "dimension"`},
		{"an unknown opposition school", specialist(func(s *classes.SchoolSpecialization) {
			s.OppositionSchools = append(s.OppositionSchools, "Illusion")
		}), `unknown school "Illusion"`},
		{"no school", specialist(func(s *classes.SchoolSpecialization) { s.Schools = nil }), "a school and a race are needed"},
		{"no race", specialist(func(s *classes.SchoolSpecialization) { s.Races = nil }), "a school and a race are needed"},
		{"a school that is also opposed", specialist(func(s *classes.SchoolSpecialization) {
			s.OppositionSchools = append(s.OppositionSchools, classes.SchoolAlteration)
		}), `school "alteration" is also in the opposition`},
		{"lesser divination opposed", specialist(func(s *classes.SchoolSpecialization) {
			s.OppositionSchools = append(s.OppositionSchools, classes.SchoolLesserDivination)
		}), "open to every wizard"},
		{"a school twice", specialist(func(s *classes.SchoolSpecialization) {
			s.Schools = []string{classes.SchoolAlteration, classes.SchoolAlteration}
		}), "twice"},
		{"a race that is no race", specialist(func(s *classes.SchoolSpecialization) { s.Races = []string{"human", "orc"} }), `unknown race "orc"`},
		{"a multi-class race that is no race", specialist(func(s *classes.SchoolSpecialization) {
			s.MulticlassRaces = []string{"orc"}
		}), `multi-class race "orc" is not among the races`},
		{"a multi-class race outside the races", specialist(func(s *classes.SchoolSpecialization) {
			s.MulticlassRaces = []string{"gnome"}
		}), `multi-class race "gnome" is not among the races`},
		{"no spellbook", []classes.Feature{{Kind: classes.KindSchoolSpecialization, SchoolSpecialization: schoolSpecialization()}}, "lists no spellbook"},
		{"no params", []classes.Feature{book, {Kind: classes.KindSchoolSpecialization}}, "a missing or unexpected parameter"},
		{"its params on another kind", []classes.Feature{{Kind: classes.KindExceptionalStrength, SchoolSpecialization: schoolSpecialization()}}, "a missing or unexpected parameter"},
		{"the kind twice", append(specialist(func(*classes.SchoolSpecialization) {}), classes.Feature{Kind: classes.KindSchoolSpecialization, SchoolSpecialization: schoolSpecialization()}), "twice"},
	}
	for _, tc := range refused {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			_, err := build(tc.fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test_class", "names the class")
			assert.Contains(t, err.Error(), tc.needle)
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

// ADR 0040 step 3.5: rogue_skills is listed once per skill set, so a class may grant two sets;
// the same set twice is refused. FeaturesAt answers every set granted at a level, in the class's
// order, ListsAll every set listed.
func TestFeatures_Step3_5_ARogueSkillsPerSkillSet(t *testing.T) {
	skills := func(set string, from int) classes.Feature {
		rs := rogueSkills()
		rs.Set = set
		return classes.Feature{Kind: classes.KindRogueSkills, FromLevel: from, RogueSkills: rs}
	}
	r := registry(t, withFeatures(skills("set_a", 1), skills("set_b", 3)))
	c, ok := r.Class("test_class")
	require.True(t, ok)
	sets := func(fs []classes.Feature) []string {
		var out []string
		for _, f := range fs {
			out = append(out, f.RogueSkills.Set)
		}
		return out
	}
	assert.Equal(t, []string{"set_a"}, sets(c.FeaturesAt(classes.KindRogueSkills, 2)), "the second set starts at its level")
	assert.Equal(t, []string{"set_a", "set_b"}, sets(c.FeaturesAt(classes.KindRogueSkills, 3)))
	assert.Equal(t, []string{"set_a", "set_b"}, sets(c.ListsAll(classes.KindRogueSkills)))
	assert.Empty(t, c.FeaturesAt(classes.KindBackstab, 20))

	_, err := classes.New([]classes.Class{withFeatures(skills("set_a", 1), skills("set_a", 5))}, groups("warrior"),
		[]classes.Race{{ID: "human", Name: localized.Text{"en": "Human"}}})
	require.Error(t, err, "the same skill set twice")
}

// ADR 0040 step 8, the creation gate: a class is implemented only when every
// kind it lists is computed. save_bonus was the last kind without a reader
// (ADR 0043 step 3; psionics left with the PSP pool, step 5): every kind of the
// closed list is computed now, alone or together, so a class listing psionics
// and save_bonus is creatable. The gate reads kinds, never class ids.
func TestFeatures_Step8_AClassIsImplementedWhenEveryKindIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		fs   []classes.Feature
		want bool
	}{
		{"no feature", nil, true},
		{"only computed kinds", []classes.Feature{{Kind: classes.KindExceptionalStrength}, {Kind: classes.KindBackstab, Table: "t"}}, true},
		{"psionics", []classes.Feature{{Kind: classes.KindPsionics, Psionics: psionics()}}, true},
		{"save_bonus", []classes.Feature{{Kind: classes.KindSaveBonus, SaveBonus: saveBonus()}}, true},
		{"a computed kind and psionics", []classes.Feature{{Kind: classes.KindBackstab, Table: "t"}, {Kind: classes.KindPsionics, Psionics: psionics()}}, true},
		{"psionics and save_bonus", []classes.Feature{{Kind: classes.KindPsionics, Psionics: psionics()}, {Kind: classes.KindSaveBonus, SaveBonus: saveBonus()}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := registry(t, withFeatures(tc.fs...)).Class("test_class")
			require.True(t, ok)
			assert.Equal(t, tc.want, c.Implemented())
		})
	}
}

// saveBonus are valid save_bonus params (ADR 0043 step 3): the psionicist's +2
// against the mind-affecting tag.
func saveBonus() *classes.SaveBonus {
	return &classes.SaveBonus{Against: classes.SaveAgainstMindAffecting, Bonus: 2}
}

// psionics are valid psionics params (ADR 0043 step 2).
func psionics() *classes.Psionics {
	return &classes.Psionics{
		Progression: "test_powers",
		PSP: classes.PSPRule{
			BaseAbility: "wis", ModifierAbilities: []string{"int", "con"},
			PerLevel: classes.PSPPerLevel{Ability: "wis", Plus: 10}, Table: "test_potential",
		},
		Recovery: "test_recovery",
	}
}

// ADR 0043 step 2: the kind psionics carries its three params (the power
// progression table, the PSP rule, the recovery table); one missing, a table
// not named or an unknown ability is refused when the definitions load (ADR
// 0040 step 8).
func TestFeatures_Step2_PsionicsParsesItsParams(t *testing.T) {
	c, ok := registry(t, withFeatures(classes.Feature{Kind: classes.KindPsionics, Psionics: psionics()})).Class("test_class")
	require.True(t, ok)
	got, ok := c.Feature(classes.KindPsionics, 1)
	require.True(t, ok)
	assert.Equal(t, psionics(), got.Psionics)

	for name, change := range map[string]func(*classes.Psionics){
		"no progression table":         func(p *classes.Psionics) { p.Progression = "" },
		"no recovery table":            func(p *classes.Psionics) { p.Recovery = "" },
		"no inherent potential table":  func(p *classes.Psionics) { p.PSP.Table = "" },
		"an unknown base ability":      func(p *classes.Psionics) { p.PSP.BaseAbility = "luck" },
		"an unknown modifier ability":  func(p *classes.Psionics) { p.PSP.ModifierAbilities = []string{"int", "luck"} },
		"an unknown per-level ability": func(p *classes.Psionics) { p.PSP.PerLevel.Ability = "" },
	} {
		t.Run("refused: "+name, func(t *testing.T) {
			p := psionics()
			change(p)
			_, err := classes.New([]classes.Class{withFeatures(classes.Feature{Kind: classes.KindPsionics, Psionics: p})}, groups("warrior"),
				[]classes.Race{{ID: "human", Identity: identity(), Name: localized.Text{"en": "Human"}}})
			require.Error(t, err)
		})
	}
	_, err := classes.New([]classes.Class{withFeatures(classes.Feature{Kind: classes.KindPsionics})}, groups("warrior"),
		[]classes.Race{{ID: "human", Identity: identity(), Name: localized.Text{"en": "Human"}}})
	require.Error(t, err, "psionics without params")
}
