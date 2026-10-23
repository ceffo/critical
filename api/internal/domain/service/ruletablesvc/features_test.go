package ruletablesvc_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The rule tables' side of ADR 0040 steps 3, 5 and 8: the tables read a
// class's spell_progression and feature params, never its id. Each test
// edits the fixture's definitions: a book class with a kind struck keeps its
// id and loses the mechanic, a test class listing it gets it.

// editedDefs are the fixture's definitions with edit applied to the class
// list.
func editedDefs(t *testing.T, edit func([]classes.Class) []classes.Class) *classes.Registry {
	t.Helper()
	defs := cataloguetest.Definitions(t)
	reg, err := classes.New(edit(defs.Classes()), defs.GroupDefs(), defs.Races())
	require.NoError(t, err)
	return reg
}

// classOf is the class id of reg.
func classOf(t *testing.T, reg *classes.Registry, id string) classes.Class {
	t.Helper()
	c, err := reg.Get(id)
	require.NoError(t, err)
	return c
}

// editClass applies change to the class id of the list.
func editClass(id string, change func(*classes.Class)) func([]classes.Class) []classes.Class {
	return func(cs []classes.Class) []classes.Class {
		for i := range cs {
			if cs[i].ID == id {
				change(&cs[i])
			}
		}
		return cs
	}
}

// strikeKind removes kind from the class id's features.
func strikeKind(id, kind string) func([]classes.Class) []classes.Class {
	return editClass(id, func(c *classes.Class) {
		c.Features = slices.DeleteFunc(c.Features, func(f classes.Feature) bool { return f.Kind == kind })
	})
}

// addClass appends a test class: a copy of the book class like with id,
// progression and only the features fs.
func addClass(id, like, progression string, fs ...classes.Feature) func([]classes.Class) []classes.Class {
	return func(cs []classes.Class) []classes.Class {
		for _, c := range cs {
			if c.ID == like {
				c.ID, c.SpellProgression, c.Features = id, progression, fs
				return append(cs, c)
			}
		}
		panic("no fixture class " + like)
	}
}

// ADR 0040 step 8: the start-up check refuses a spell_progression naming no
// slot table, a feature table param naming another table than its kind's,
// an unknown rogue_skills set, and a rogue_skills armour list missing a
// Table 29 column (it would read as leather, the best column); the fixture's
// five classes pass, and a full list with a known set passes.
func TestImport_Step8_TheTablesRefuseWhatTheyCannotRead(t *testing.T) {
	require.NoError(t, ruletablesvc.CheckDefinitions(cataloguetest.Definitions(t)), "the fixture's definitions")
	for _, tc := range []struct {
		name string
		edit func([]classes.Class) []classes.Class
	}{
		{"a spell_progression naming no slot table", editClass("mage", func(c *classes.Class) { c.SpellProgression = "test_spells" })},
		{"turn_undead reading the backstab table", addClass("test_templar", "fighter", "", classes.Feature{Kind: classes.KindTurnUndead, Table: ruletablesvc.TableBackstab})},
		{"backstab reading an unknown table", addClass("test_cutthroat", "fighter", "", classes.Feature{Kind: classes.KindBackstab, Table: "test_table"})},
		{"melee_attacks_per_round reading the turning table", addClass("test_brute", "fighter", "", classes.Feature{Kind: classes.KindMeleeAttacksPerRound, Table: ruletablesvc.TableTurningUndead})},
		{"an unknown skill set", addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
			Set: "test_set", Armour: allColumns(),
		}})},
		{"a set that resolved to no skill", addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
			Set: "thief", Armour: allColumns(),
		}})},
		{"rogue_skills without the none_allowed and the metal entries", addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
			Set: "thief", Skills: []string{"climb_walls"}, Armour: allColumns()[:4],
		}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, ruletablesvc.CheckDefinitions(editedDefs(t, tc.edit)))
		})
	}
	require.NoError(t, ruletablesvc.CheckDefinitions(editedDefs(t, addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
		Set: "thief", Skills: []string{"climb_walls"}, Armour: allColumns(),
	}}))), "every column listed")
}

// allColumns is a rogue_skills armour list with one entry per Table 29
// column, in SkillColumns order (none_allowed and metal last).
func allColumns() []classes.SkillArmour {
	out := make([]classes.SkillArmour, 0, len(ruletablesvc.SkillColumns))
	for _, c := range ruletablesvc.SkillColumns {
		out = append(out, classes.SkillArmour{Code: c})
	}
	return out
}

// ADR 0040 step 1.9 and 0030 steps 8 and 13: the slot table is the one the
// class's spell_progression names, whatever its id, with the Wisdom bonus
// spells only for a class listing wisdom_bonus_spells. At level 3: Table 21
// 2 1, Table 32 2, Table 24 2 1, Table 24 with Wisdom 15's two 1st and one
// 2nd 4 2.
func TestSpellsPerDay_Step1_9_TheTableTheProgressionNames(t *testing.T) {
	tables := cataloguetest.Tables(t)
	reg := editedDefs(t, func(cs []classes.Class) []classes.Class {
		cs = addClass("test_wizard_slots", "fighter", ruletablesvc.TableWizardSpells)(cs)
		cs = addClass("test_bard_slots", "fighter", ruletablesvc.TableBardSpells)(cs)
		cs = addClass("test_priest_slots", "fighter", ruletablesvc.TablePriestSpells)(cs)
		cs = addClass("test_wise_slots", "fighter", ruletablesvc.TablePriestSpells, classes.Feature{Kind: classes.KindWisdomBonusSpells})(cs)
		return strikeKind("cleric", classes.KindWisdomBonusSpells)(cs)
	})
	for _, tc := range []struct {
		class string
		want  map[int]int
	}{
		{"test_wizard_slots", map[int]int{1: 2, 2: 1}},
		{"test_bard_slots", map[int]int{1: 2}},
		{"test_priest_slots", map[int]int{1: 2, 2: 1}},
		{"test_wise_slots", map[int]int{1: 4, 2: 2}},
		{"cleric", map[int]int{1: 2, 2: 1}}, // its wisdom_bonus_spells struck
		{"fighter", map[int]int{}},
	} {
		t.Run(tc.class, func(t *testing.T) {
			got, err := tables.SpellsPerDay(classOf(t, reg, tc.class), 3, 15)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	_, err := tables.SpellsPerDay(classes.Class{ID: "test_odd", SpellProgression: "test_spells"}, 3, 15)
	assert.Error(t, err, "a progression naming no table")
}

// ADR 0030 step 11.1 through melee_attacks_per_round: a long sword at level
// 7 is 3/2 (Table 15) for a class listing the kind, none for a fighter with
// it struck.
func TestFeatures_Step3_MeleeAttacksPerRoundFromTheKind(t *testing.T) {
	reg := editedDefs(t, func(cs []classes.Class) []classes.Class {
		cs = addClass("test_battlemage", "mage", ruletablesvc.TableWizardSpells,
			classes.Feature{Kind: classes.KindMeleeAttacksPerRound, Table: ruletablesvc.TableWarriorAttacks})(cs)
		cs = addClass("test_late", "fighter", "",
			classes.Feature{Kind: classes.KindMeleeAttacksPerRound, FromLevel: 8, Table: ruletablesvc.TableWarriorAttacks})(cs)
		return strikeKind("fighter", classes.KindMeleeAttacksPerRound)(cs)
	})
	tables := cataloguetest.Tables(t).WithDefinitions(reg)
	for _, tc := range []struct {
		class string
		want  bool
	}{{"test_battlemage", true}, {"fighter", false}, {"test_late", false}} {
		t.Run(tc.class, func(t *testing.T) {
			got, ok, err := tables.AttackRateFor(tc.class, 7, "melee", "long_sword", false)
			require.NoError(t, err)
			require.Equal(t, tc.want, ok)
			if ok {
				assert.Equal(t, ruletablesvc.AttackRate{Attacks: 3, Rounds: 2}, got)
			}
		})
	}
}

// The creation's gates read the kind on the definition (ADR 0030 steps 9 and
// 11, ADR 0040 step 3): exceptional Strength at 18 and the specialization
// picks, for any class that lists them and no class that does not.
func TestFeatures_Step3_TheCreationGatesReadTheKind(t *testing.T) {
	reg := editedDefs(t, func(cs []classes.Class) []classes.Class {
		cs = addClass("test_strong", "mage", "", classes.Feature{Kind: classes.KindExceptionalStrength}, classes.Feature{Kind: classes.KindWeaponSpecialization})(cs)
		cs = strikeKind("fighter", classes.KindExceptionalStrength)(cs)
		return strikeKind("fighter", classes.KindWeaponSpecialization)(cs)
	})
	strong, fighter := classOf(t, reg, "test_strong"), classOf(t, reg, "fighter")
	human := classes.Race{ID: "human"}
	assert.True(t, ruletablesvc.ExceptionalStrengthAllowed(strong, human, 18))
	assert.False(t, ruletablesvc.ExceptionalStrengthAllowed(strong, human, 17), "only at 18")
	assert.False(t, ruletablesvc.ExceptionalStrengthAllowed(fighter, human, 18), "a fighter without the kind")
	assert.True(t, ruletablesvc.Specializes(strong))
	assert.False(t, ruletablesvc.Specializes(fighter), "a fighter without the kind")
}

// ADR 0040 step 4: a race that lists no_exceptional_strength never rolls an exceptional
// Strength, whatever its id; one that does not, does.
func TestRaceTraits_Step4_NoExceptionalStrengthFromTheTrait(t *testing.T) {
	fighter := classOf(t, cataloguetest.Definitions(t), "fighter")
	barred := classes.Race{ID: "test_barred", Traits: []classes.Trait{{Kind: classes.TraitNoExceptionalStrength}}}
	free := classes.Race{ID: "halfling"}
	assert.False(t, ruletablesvc.ExceptionalStrengthAllowed(fighter, barred, 18))
	assert.True(t, ruletablesvc.ExceptionalStrengthAllowed(fighter, free, 18), "the id decides nothing, the trait does")
}

// ADR 0040 step 13: the slots a specialization costs and the Table 35 column it reads are the
// class's weapon_specialization params, whatever its id. A test class with other params spends
// and reads other figures than the book fighter on the same weapons.
func TestWeaponSpecialization_Step13_ParamsDecideSlotsAndColumns(t *testing.T) {
	melee := classes.Feature{Kind: classes.KindMeleeAttacksPerRound, Table: ruletablesvc.TableWarriorAttacks}
	reg := editedDefs(t, addClass("test_specialist", "fighter", "", melee, classes.Feature{
		Kind: classes.KindWeaponSpecialization,
		Specialization: &classes.WeaponSpecialization{
			DefaultSlots: 3, MeleeColumn: "melee", MissileColumn: "other_missile",
			Types: []classes.SpecializationType{
				{WeaponTypes: []string{"long_bow"}, Slots: 2, Column: "thrown_dart"},
				{WeaponTypes: []string{"light_crossbow"}, Slots: 4, Column: "other_missile"},
			},
		},
	}))
	tables := cataloguetest.Tables(t).WithDefinitions(reg)
	fighter, odd := classOf(t, reg, "fighter"), classOf(t, reg, "test_specialist")

	for _, tc := range []struct {
		weaponType     string
		fighter, other int
	}{{"long_bow", 3, 2}, {"long_sword", 2, 3}, {"light_crossbow", 2, 4}} {
		assert.Equal(t, tc.fighter, ruletablesvc.SpecializationSlots(fighter, tc.weaponType), "fighter, %s", tc.weaponType)
		assert.Equal(t, tc.other, ruletablesvc.SpecializationSlots(odd, tc.weaponType), "test class, %s", tc.weaponType)
	}
	assert.Zero(t, ruletablesvc.SpecializationSlots(classOf(t, reg, "mage"), "long_sword"), "a class that does not specialize")

	rate := func(class, kind, weaponType string) ruletablesvc.AttackRate {
		t.Helper()
		got, ok, err := tables.AttackRateFor(class, 7, kind, weaponType, true)
		require.NoError(t, err)
		require.True(t, ok)
		return got
	}
	// The crossbow: the fighter reads its own column, the test class other_missile (the fighter's sling).
	assert.Equal(t, rate("fighter", "missile", "sling"), rate("test_specialist", "missile", "light_crossbow"))
	assert.NotEqual(t, rate("fighter", "missile", "light_crossbow"), rate("test_specialist", "missile", "light_crossbow"))
	// The bow: the fighter keeps the rate of fire, the test class reads a column.
	assert.Equal(t, rate("fighter", "missile", "dart"), rate("test_specialist", "missile", "long_bow"))
	assert.NotEqual(t, rate("fighter", "missile", "long_bow"), rate("test_specialist", "missile", "long_bow"))
}

// ADR 0040 step 13: the book fighter's params give back today's numbers. Costs: 3 slots in all for
// a bow, 2 for everything else. Rates (PHB Tables 15, 35 and 45) for a specialist, by level.
func TestWeaponSpecialization_Step13_TheFighterKeepsTodaysNumbers(t *testing.T) {
	tables := cataloguetest.Tables(t)
	fighter, err := tables.Class("fighter")
	require.NoError(t, err)
	for typ, want := range map[string]int{
		"long_bow": 3, "short_bow": 3, "long_sword": 2, "dagger": 2, "light_crossbow": 2, "heavy_crossbow": 2, "dart": 2, "sling": 2,
	} {
		assert.Equal(t, want, ruletablesvc.SpecializationSlots(fighter, typ), typ)
	}
	rate := func(level int, kind, typ string, specialized bool) ruletablesvc.AttackRate {
		t.Helper()
		got, ok, err := tables.AttackRateFor("fighter", level, kind, typ, specialized)
		require.NoError(t, err)
		require.True(t, ok)
		return got
	}
	r := func(a, n int) ruletablesvc.AttackRate { return ruletablesvc.AttackRate{Attacks: a, Rounds: n} }
	// Melee: Table 15 by level, Table 35's melee column when specialized.
	assert.Equal(t, r(1, 1), rate(1, "melee", "long_sword", false))
	assert.Equal(t, r(3, 2), rate(1, "melee", "long_sword", true))
	assert.Equal(t, r(2, 1), rate(7, "melee", "long_sword", true))
	// A bow specialist keeps Table 45's rate of fire.
	assert.Equal(t, rate(7, "missile", "long_bow", false), rate(7, "missile", "long_bow", true))
	assert.Equal(t, rate(7, "missile", "short_bow", false), rate(7, "missile", "short_bow", true))
	// The other missile types read their Table 35 columns.
	for _, typ := range []string{"light_crossbow", "heavy_crossbow", "dagger", "dart", "sling"} {
		assert.NotEmpty(t, rate(7, "missile", typ, true), typ)
	}
	assert.Equal(t, rate(7, "missile", "sling", true), rate(7, "missile", "other_missile", true), "other_missile column")
	assert.NotEqual(t, rate(1, "missile", "dart", true), rate(1, "missile", "sling", true), "thrown_dart is its own column")
}
