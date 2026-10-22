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
		{"rogue_skills without the none_allowed and the metal entries", addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
			Set: ruletablesvc.SkillSetThief, Armour: allColumns()[:4],
		}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, ruletablesvc.CheckDefinitions(editedDefs(t, tc.edit)))
		})
	}
	require.NoError(t, ruletablesvc.CheckDefinitions(editedDefs(t, addClass("test_scout", "thief", "", classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
		Set: ruletablesvc.SkillSetThief, Armour: allColumns(),
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
