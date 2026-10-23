package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0040 steps 1.9, 1.11 and 3: the adapter maps each class's spell
// progression and feature list, every kind with its params, as the step 3
// table lists them for the five classes (PHB chapter 3: the fighter's
// exceptional Strength, attacks and specialization; the cleric's Wisdom
// spells, spheres, turning; the thief's and bard's skills, Tables 26 and 33
// with their budgets; the thief's backstab; the mage's and bard's
// spellbooks, 9th and 6th level).
func TestDefinitions_Step3_TheFeatureListsMapWithTheirParams(t *testing.T) {
	tables, err := definitionsOf(t, func(map[string]any) {})
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	get := func(id string) classes.Class {
		c, err := defs.Get(id)
		require.NoError(t, err)
		return c
	}
	armour := func(metalColumn string, metalExtra int) []classes.SkillArmour {
		return []classes.SkillArmour{
			{Code: "none", Column: "none"},
			{Code: "leather"},
			{Code: "padded_studded", Column: "padded_studded_leather"},
			{Code: "elven_chain", Column: "elven_chain"},
			{Code: "metal", Column: metalColumn, Extra: metalExtra},
			{Code: "none_allowed", Column: "padded_studded_leather"},
		}
	}
	want := map[string]struct {
		progression string
		features    []classes.Feature
	}{
		"fighter": {"", []classes.Feature{
			{Kind: classes.KindExceptionalStrength, FromLevel: 1},
			{Kind: classes.KindMeleeAttacksPerRound, FromLevel: 1, Table: "warrior_attacks"},
			{Kind: classes.KindWeaponSpecialization, FromLevel: 1, Specialization: &classes.WeaponSpecialization{
				DefaultSlots: 2, MeleeColumn: "melee", MissileColumn: "other_missile",
				Types: []classes.SpecializationType{
					{WeaponTypes: []string{"long_bow", "short_bow"}, Slots: 3, RateOfFire: true},
					{WeaponTypes: []string{"light_crossbow"}, Slots: 2, Column: "light_crossbow"},
					{WeaponTypes: []string{"heavy_crossbow"}, Slots: 2, Column: "heavy_crossbow"},
					{WeaponTypes: []string{"dagger"}, Slots: 2, Column: "thrown_dagger"},
					{WeaponTypes: []string{"dart"}, Slots: 2, Column: "thrown_dart"},
				},
			}},
		}},
		"mage": {"wizard_spells", []classes.Feature{
			{Kind: classes.KindSpellbook, FromLevel: 1, Spellbook: &classes.Spellbook{MaxSpellLevel: 9, FreeSpellPerNewSpellLevel: true}},
		}},
		"cleric": {"priest_spells", []classes.Feature{
			{Kind: classes.KindWisdomBonusSpells, FromLevel: 1},
			{Kind: classes.KindSpellFailure, FromLevel: 1, Ability: "wis"},
			{Kind: classes.KindSphereAccess, FromLevel: 1, Spheres: &classes.SphereAccess{
				Major: []string{"all", "astral", "charm", "combat", "creation", "divination", "guardian", "healing", "necromantic", "protection", "summoning", "sun"},
				Minor: []string{"elemental_air", "elemental_earth", "elemental_fire", "elemental_water"}, MinorMaxLevel: 3,
			}},
			{Kind: classes.KindTurnUndead, FromLevel: 1, Table: "turning_undead"},
		}},
		"thief": {"", []classes.Feature{
			{Kind: classes.KindRogueSkills, FromLevel: 1, RogueSkills: &classes.RogueSkills{
				Set: "thief", Budget: classes.SkillBudget{First: 60, PerLevel: 30, PerSkillFirst: 30, PerSkillPerLevel: 15},
				Armour:     armour("padded_studded_leather", 0),
				Skills:     []string{"pick_pockets", "open_locks", "find_remove_traps", "move_silently", "hide_in_shadows", "detect_noise", "climb_walls", "read_languages"},
				ArmourFree: []string{"open_locks", "detect_noise"},
			}},
			{Kind: classes.KindBackstab, FromLevel: 1, Table: "backstab"},
		}},
		"bard": {"bard_spells", []classes.Feature{
			{Kind: classes.KindRogueSkills, FromLevel: 1, RogueSkills: &classes.RogueSkills{
				Set: "bard", Budget: classes.SkillBudget{First: 20, PerLevel: 15}, Armour: armour("elven_chain", -5),
				Skills: []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"},
			}},
			{Kind: classes.KindSpellbook, FromLevel: 1, Spellbook: &classes.Spellbook{MaxSpellLevel: 6}},
		}},
		"psionicist": {"", []classes.Feature{
			{Kind: classes.KindPsionics, FromLevel: 1, Psionics: &classes.Psionics{
				Progression: "psionicist_powers", Recovery: "psp_recovery",
				PSP: classes.PSPRule{
					BaseAbility: "wis", ModifierAbilities: []string{"int", "con"},
					PerLevel: classes.PSPPerLevel{Ability: "wis", Plus: 10}, Table: "inherent_potential",
				},
			}},
			{Kind: classes.KindSaveBonus, FromLevel: 1, SaveBonus: &classes.SaveBonus{Against: classes.SaveAgainstMindAffecting, Bonus: 2}},
		}},
	}
	assert.Len(t, defs.Classes(), len(want), "the fixture's five book classes and the psionicist")
	for id, w := range want {
		t.Run(id, func(t *testing.T) {
			c := get(id)
			assert.Equal(t, w.progression, c.SpellProgression)
			assert.Equal(t, w.features, c.Features)
		})
	}
}

// ADR 0040 step 8 on the mapped features: a broken feature in the data
// stops the import, naming the class.
func TestImport_Step8_RefusesABrokenFeature(t *testing.T) {
	feature := func(d map[string]any, class string, i int) map[string]any {
		for _, c := range d["classes"].([]any) {
			if m := c.(map[string]any); m["id"] == class {
				return m["features"].([]any)[i].(map[string]any)
			}
		}
		t.Fatalf("no class %q in the fixture", class)
		return nil
	}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"an unknown kind", func(d map[string]any) { feature(d, "fighter", 0)["kind"] = "test_flight" }, `unknown feature kind "test_flight"`},
		{"a missing table", func(d map[string]any) { delete(feature(d, "thief", 1), "params") }, `class "thief"`},
		{"a from_level above 20", func(d map[string]any) { feature(d, "fighter", 0)["from_level"] = 21 }, `class "fighter"`},
		{"params that are not an object", func(d map[string]any) { feature(d, "cleric", 1)["params"] = "wis" }, `class "cleric"`},
		{"a table naming another kind's table", func(d map[string]any) {
			feature(d, "thief", 1)["params"] = map[string]any{"table": "turning_undead"}
		}, `class "thief"`},
		{"an unknown skill set", func(d map[string]any) {
			feature(d, "bard", 0)["params"].(map[string]any)["set"] = "test_set"
		}, `unknown skill set "test_set"`},
		{"a spell progression naming no table", func(d map[string]any) {
			for _, c := range d["classes"].([]any) {
				if m := c.(map[string]any); m["id"] == "mage" {
					m["spell_progression"] = "test_spells"
				}
			}
		}, `spell_progression "test_spells"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := definitionsOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
