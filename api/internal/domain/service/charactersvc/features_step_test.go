package charactersvc_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The feature-kind tests of ADR 0040 step 3: the derived character reads a
// mechanic only through its class's feature list (step 5). Each test derives
// on the fixture's real tables with an edited set of definitions: a book class
// with the kind struck from its list keeps its id and loses the mechanic,
// and a test class (an id the code has never seen) that lists the kind gets
// it. Both fail on code that grants a mechanic by class id.

// edited are the fixture's tables over its definitions with edit applied to
// the class list.
func edited(t *testing.T, edit func([]classes.Class) []classes.Class) *ruletablesvc.Tables {
	t.Helper()
	tables := cataloguetest.Tables(t)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	reg, err := classes.New(edit(defs.Classes()), defs.GroupDefs(), defs.Races())
	require.NoError(t, err)
	return tables.WithDefinitions(reg)
}

// strike is the class list with kind removed from class id's features.
func strike(id, kind string) func([]classes.Class) []classes.Class {
	return func(cs []classes.Class) []classes.Class {
		for i := range cs {
			if cs[i].ID == id {
				cs[i].Features = slices.DeleteFunc(cs[i].Features, func(f classes.Feature) bool { return f.Kind == kind })
			}
		}
		return cs
	}
}

// add is the class list with a test class: a copy of the book class like
// (its group, XP and minimums), with id, the spell progression and only
// the features fs.
func add(id, like, progression string, fs ...classes.Feature) func([]classes.Class) []classes.Class {
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

// featureCase is one derivation: a human of class at level with the
// scores, on the definitions edit leaves.
type featureCase struct {
	name  string
	edit  func([]classes.Class) []classes.Class
	class string
	level int
	set   func(*charactersvc.Character)
	check func(*testing.T, charactersvc.Derived)
}

func runFeatureCases(t *testing.T, cases []featureCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &charactersvc.Character{
				Name: "Test", Class: tc.class, Race: "human", Str: 18, Dex: 17, Con: 14, Int: 17, Wis: 17, Cha: 16,
				HPMax: 20, RealizedLevel: tc.level,
			}
			if tc.set != nil {
				tc.set(c)
			}
			d, err := charactersvc.Derive(c, 0, "", edited(t, tc.edit))
			require.NoError(t, err)
			tc.check(t, d)
		})
	}
}

func keep(cs []classes.Class) []classes.Class { return cs }

func TestFeatures_Step3_ExceptionalStrengthFromTheKind(t *testing.T) {
	runFeatureCases(t, []featureCase{
		{"the fighter has it", keep, "fighter", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.True(t, d.ExceptionalStrength)
		}},
		{"a fighter without the kind has none", strike("fighter", classes.KindExceptionalStrength), "fighter", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.False(t, d.ExceptionalStrength)
		}},
		{"a wizard-group test class listing it has it", add("test_strongmage", "mage", "", classes.Feature{Kind: classes.KindExceptionalStrength}), "test_strongmage", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.True(t, d.ExceptionalStrength)
		}},
	})
}

func TestFeatures_Step3_WeaponSpecializationFromTheKind(t *testing.T) {
	runFeatureCases(t, []featureCase{
		{"the fighter specializes", keep, "fighter", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.True(t, d.Specializes)
		}},
		{"a fighter without the kind does not", strike("fighter", classes.KindWeaponSpecialization), "fighter", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.False(t, d.Specializes)
		}},
		{"a test class listing it does", add("test_duellist", "thief", "", classes.Feature{Kind: classes.KindWeaponSpecialization}), "test_duellist", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.True(t, d.Specializes)
		}},
	})
}

// PHB Table 24 at level 1 is one 1st-level spell; Wisdom 17's bonus spells
// (Table 5) add two 1st (13, 14) and two 2nd (15, 16) levels, only the 1st
// open at level 1: 3. The slot table is the class's spell_progression.
func TestFeatures_Step3_WisdomBonusSpellsFromTheKind(t *testing.T) {
	runFeatureCases(t, []featureCase{
		{"the cleric gets Table 5's bonus spells", keep, "cleric", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, map[int]int{1: 3}, d.SpellSlots)
		}},
		{"a cleric without the kind reads Table 24 alone", strike("cleric", classes.KindWisdomBonusSpells), "cleric", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, map[int]int{1: 1}, d.SpellSlots)
		}},
		{"a test class on Table 24 listing it gets them", add("test_oracle", "thief", ruletablesvc.TablePriestSpells, classes.Feature{Kind: classes.KindWisdomBonusSpells}), "test_oracle", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, map[int]int{1: 3}, d.SpellSlots)
		}},
		{"a test class with no spell progression has no slots", add("test_plain", "cleric", ""), "test_plain", 5, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Empty(t, d.SpellSlots)
		}},
	})
}

// PHB Table 5's spell failure at a score of 10 is 15%, read on the kind's
// ability param.
func TestFeatures_Step3_SpellFailureFromTheKind(t *testing.T) {
	wis10 := func(c *charactersvc.Character) { c.Wis = 10 }
	runFeatureCases(t, []featureCase{
		{"the cleric of Wis 10 fails 15%", keep, "cleric", 1, wis10, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, 15, d.SpellFailure)
		}},
		{"a cleric without the kind never fails", strike("cleric", classes.KindSpellFailure), "cleric", 1, wis10, func(t *testing.T, d charactersvc.Derived) {
			assert.Zero(t, d.SpellFailure)
		}},
		{
			"a test class on Intelligence reads its Int, not its Wis", add("test_scholar", "fighter", "", classes.Feature{Kind: classes.KindSpellFailure, Ability: "int"}), "test_scholar", 1,
			func(c *charactersvc.Character) { c.Int = 10 }, func(t *testing.T, d charactersvc.Derived) {
				assert.Equal(t, 15, d.SpellFailure)
			},
		},
	})
}

func TestFeatures_Step3_TurnUndeadFromTheKind(t *testing.T) {
	turning := func(t *testing.T, level int) []ruletablesvc.TurningTarget {
		t.Helper()
		want, err := cataloguetest.Tables(t).TurningAt(level)
		require.NoError(t, err)
		require.NotEmpty(t, want)
		return want
	}
	runFeatureCases(t, []featureCase{
		{"the cleric turns", keep, "cleric", 3, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, turning(t, 3), d.Turning)
		}},
		{"a cleric without the kind does not", strike("cleric", classes.KindTurnUndead), "cleric", 3, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Nil(t, d.Turning)
		}},
		{"a test warrior listing it turns at its level", add("test_templar", "fighter", "", classes.Feature{Kind: classes.KindTurnUndead, Table: ruletablesvc.TableTurningUndead}), "test_templar", 3, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, turning(t, 3), d.Turning)
		}},
		{"from_level: not below the level it starts at", add("test_templar", "fighter", "", classes.Feature{Kind: classes.KindTurnUndead, FromLevel: 3, Table: ruletablesvc.TableTurningUndead}), "test_templar", 2, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Nil(t, d.Turning)
		}},
	})
}

func TestFeatures_Step3_BackstabFromTheKind(t *testing.T) {
	runFeatureCases(t, []featureCase{
		{"the thief backstabs x2 at level 1", keep, "thief", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, 2, d.Backstab)
		}},
		{"a thief without the kind does not", strike("thief", classes.KindBackstab), "thief", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Zero(t, d.Backstab)
		}},
		{"a test warrior listing it backstabs x3 at level 5", add("test_cutthroat", "fighter", "", classes.Feature{Kind: classes.KindBackstab, Table: ruletablesvc.TableBackstab}), "test_cutthroat", 5, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, 3, d.Backstab)
		}},
	})
}

func TestFeatures_Step3_SpellbookFromTheKind(t *testing.T) {
	runFeatureCases(t, []featureCase{
		{"the mage writes up to 9th level", keep, "mage", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			require.NotNil(t, d.Spellbook)
			assert.Equal(t, 9, d.Spellbook.ClassMaxSpellLevel)
		}},
		{"a mage without the kind has no spellbook", strike("mage", classes.KindSpellbook), "mage", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Nil(t, d.Spellbook)
		}},
		{"a test priest listing it writes up to its max_spell_level", add("test_scribe", "cleric", "", classes.Feature{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 4}}), "test_scribe", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			require.NotNil(t, d.Spellbook)
			assert.Equal(t, 4, d.Spellbook.ClassMaxSpellLevel)
		}},
	})
}

// rogue_skills: the skill list, base rows and budget are the params'. The
// thief's budget at level 1 is 60 points, 30 in one skill.
func TestFeatures_Step3_RogueSkillsFromTheKind(t *testing.T) {
	thiefSkills := func(t *testing.T) classes.Feature {
		t.Helper()
		def, ok := cataloguetest.Definitions(t).Class("thief")
		require.True(t, ok)
		f, ok := def.Lists(classes.KindRogueSkills)
		require.True(t, ok)
		return f
	}(t)
	runFeatureCases(t, []featureCase{
		{"the thief has skills and a budget", keep, "thief", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.NotEmpty(t, d.ThiefSkills)
			require.NotNil(t, d.SkillPoints)
			assert.Equal(t, 60, d.SkillPoints.Total)
		}},
		{"a thief without the kind has neither", strike("thief", classes.KindRogueSkills), "thief", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Nil(t, d.ThiefSkills)
			assert.Nil(t, d.BardSkills)
			assert.Nil(t, d.SkillPoints)
		}},
		{"a sixth test class, a warrior with the thief's set, has them", add("test_scout", "fighter", "", thiefSkills), "test_scout", 1, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.NotEmpty(t, d.ThiefSkills)
			assert.Nil(t, d.BardSkills)
			require.NotNil(t, d.SkillPoints)
			assert.Equal(t, 60, d.SkillPoints.Total)
			assert.Equal(t, 30, d.SkillPoints.PerSkill)
		}},
		{"from_level: no skills nor budget below it", lateSkills(thiefSkills), "test_latescout", 2, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Nil(t, d.ThiefSkills)
			assert.Nil(t, d.SkillPoints)
		}},
		{"from_level: skills and the level's budget from it", lateSkills(thiefSkills), "test_latescout", 3, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.NotEmpty(t, d.ThiefSkills)
			require.NotNil(t, d.SkillPoints)
			assert.Equal(t, 60+2*30, d.SkillPoints.Total)
		}},
	})
}

// lateSkills adds test_latescout, a warrior granting rs from level 3.
func lateSkills(rs classes.Feature) func([]classes.Class) []classes.Class {
	rs.FromLevel = 3
	return add("test_latescout", "fighter", "", rs)
}

// ADR 0037 step 10 on a rogue_skills from_level (ADR 0040 step 3): the entry
// and the budget read the same grant. test_latescout, created at level 1 (0
// XP) with points, is refused on skillPoints with a budget of 0, as its
// derive shows no skills there; created at level 3 the points are in its
// budget.
func TestDraft_Step10_SkillPointsBelowTheFromLevelAreRefused(t *testing.T) {
	def, ok := cataloguetest.Definitions(t).Class("thief")
	require.True(t, ok)
	thiefSkills, ok := def.Lists(classes.KindRogueSkills)
	require.True(t, ok)
	rules := edited(t, lateSkills(thiefSkills))
	threshold, err := rules.XPThreshold("test_latescout", 3)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		xp     int
		refuse bool
	}{
		{"at level 1", 0, true},
		{"at level 3", threshold, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			svc, err := charactersvc.New(charactersvc.Deps{
				Reader: e.reader, XP: e.xp, Rules: rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
				Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book, FreeSpells: e.free,
			})
			require.NoError(t, err)
			got, err := svc.Draft(context.Background(), charactersvc.NewCharacter{
				Details: identity(), Name: "Vex", Class: "test_latescout", Race: "human",
				Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 10, XP: tc.xp,
				SkillPoints: map[string]int{"open_locks": 10},
			}, nil)
			require.NoError(t, err)
			var budget *domain.BudgetError
			for _, r := range got.Refusals {
				if be, ok := errors.AsType[*domain.BudgetError](r); ok && be.Field == "skillPoints" {
					budget = be
				}
			}
			if !tc.refuse {
				assert.Nil(t, budget)
				return
			}
			require.NotNil(t, budget, "refusals: %v", got.Refusals)
			assert.Equal(t, 10, budget.Spent)
			assert.Zero(t, budget.Limit)
		})
	}
}

// ADR 0040 step 5: a class made of known kinds needs no code. test_skald,
// group rogue, lists rogue_skills of the bard's set (with its own budget)
// and a spellbook, and derives the bard's skills, its budget and a
// spellbook; nothing else (no thief skills, backstab or turning).
func TestLookup_Step5_ANewClassOfKnownKindsNeedsNoCode(t *testing.T) {
	skald := add("test_skald", "thief", "",
		classes.Feature{Kind: classes.KindRogueSkills, RogueSkills: &classes.RogueSkills{
			Set: ruletablesvc.SkillSetBard, Budget: classes.SkillBudget{First: 25, PerLevel: 10},
			Armour: []classes.SkillArmour{
				{Code: "none", Column: "none"}, {Code: "leather"}, {Code: "padded_studded", Column: "padded_studded_leather"},
				{Code: "elven_chain", Column: "elven_chain"}, {Code: "metal", Column: "elven_chain", Extra: -5},
				{Code: "none_allowed", Column: "padded_studded_leather"},
			},
		}},
		classes.Feature{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 3}},
	)
	runFeatureCases(t, []featureCase{
		{"the skald", skald, "test_skald", 3, nil, func(t *testing.T, d charactersvc.Derived) {
			assert.Equal(t, "rogue", d.Group)
			assert.NotEmpty(t, d.BardSkills)
			assert.Nil(t, d.ThiefSkills)
			require.NotNil(t, d.SkillPoints)
			assert.Equal(t, 25+2*10, d.SkillPoints.Total, "its own budget at level 3")
			require.NotNil(t, d.Spellbook)
			assert.Equal(t, 3, d.Spellbook.ClassMaxSpellLevel)
			assert.Zero(t, d.Backstab)
			assert.Nil(t, d.Turning)
			assert.Empty(t, d.SpellSlots, "no spell_progression")
		}},
	})
}
