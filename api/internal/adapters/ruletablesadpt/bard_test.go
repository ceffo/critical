package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The bard's skills (ADR 0030 step 18) over the fixture's class tables.
// Expected values are read from PHB Tables 27 to 29 (p.39) and 33 (p.42), not
// from the artifact. Each test isolates one term: the others are 0 (a human,
// Dexterity 13, leather, no points).

// bardSkill is skill's row of the bard's RogueSkills.
func bardSkill(t *testing.T, race string, dex int, armor string, points map[string]int, skill string) ruletablesvc.SkillPercent {
	t.Helper()
	got, err := rogueSkills(t, phb(t), "bard", race, dex, armor, points)
	require.NoError(t, err)
	for _, s := range got {
		if s.Skill == skill {
			return s
		}
	}
	t.Fatalf("no %s", skill)
	return ruletablesvc.SkillPercent{}
}

// ADR 0030 step 18: Table 33's base, the four skills in its order.
func TestBardSkills_Step18_Base(t *testing.T) {
	got, err := rogueSkills(t, phb(t), "bard", "human", 13, "leather", nil)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.SkillPercent{
		{Skill: "climb_walls", Base: 50, Raw: 50, Percent: 50},
		{Skill: "detect_noise", Base: 20, Raw: 20, Percent: 20},
		{Skill: "pick_pockets", Base: 10, Raw: 10, Percent: 10},
		{Skill: "read_languages", Base: 5, Raw: 5, Percent: 5},
	}, got)
}

// ADR 0030 step 18: the entered points add to their skill only.
func TestBardSkills_Step18_Points(t *testing.T) {
	got, err := rogueSkills(t, phb(t), "bard", "human", 13, "leather", map[string]int{"detect_noise": 15, "read_languages": 5})
	require.NoError(t, err)
	assert.Equal(t, []int{50, 35, 10, 10}, percents(got))
	assert.Equal(t, 15, got[1].Points)
	assert.Equal(t, 0, got[0].Points)
}

// ADR 0030 step 18: Table 27 for the race, the thief's rows.
func TestBardSkills_Step18_Race(t *testing.T) {
	half := bardSkill(t, "half_elf", 13, "leather", nil, "pick_pockets")
	assert.Equal(t, 10, half.Race)
	assert.Equal(t, 20, half.Percent)
	assert.Equal(t, 0, bardSkill(t, "half_elf", 13, "leather", nil, "climb_walls").Race)
	// Another race, another row: a halfling's -15 Climb Walls, -5 Read Languages.
	got, err := rogueSkills(t, phb(t), "bard", "halfling", 13, "leather", nil)
	require.NoError(t, err)
	assert.Equal(t, []int{-15, 5, 5, -5}, races(got))
	assert.Equal(t, 0, bardSkill(t, "", 13, "leather", nil, "pick_pockets").Race, "no race code: 0")
}

// ADR 0030 step 18: Table 28 adjusts Pick Pockets only.
func TestBardSkills_Step18_DexterityOnPickPocketsOnly(t *testing.T) {
	got, err := rogueSkills(t, phb(t), "bard", "human", 19, "leather", nil)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 0, 15, 0}, dexterities(got), "Dexterity 19: +15 on Pick Pockets, nothing else")
	assert.Equal(t, 25, got[2].Percent)
	assert.Equal(t, -15, bardSkill(t, "human", 9, "leather", nil, "pick_pockets").Dexterity)
}

// ADR 0030 step 18: Table 29's columns, leather the baseline, and metal
// armour up to chain mail as the elven chain column plus the note's -5.
func TestBardSkills_Step18_Armour(t *testing.T) {
	tests := []struct {
		armor string
		want  []int // the Armor term of climb_walls, detect_noise, pick_pockets, read_languages
	}{
		{"leather", []int{0, 0, 0, 0}},
		{"none", []int{10, 0, 5, 0}},
		{"padded_studded_leather", []int{-30, -10, -30, 0}},
		{"elven_chain", []int{-20, -5, -20, 0}},
		{ruletablesvc.ArmorMetal, []int{-25, -10, -25, -5}},
	}
	for _, tt := range tests {
		t.Run(tt.armor, func(t *testing.T) {
			got, err := rogueSkills(t, phb(t), "bard", "human", 13, tt.armor, map[string]int{"read_languages": 10})
			require.NoError(t, err)
			terms := make([]int, len(got))
			for i, s := range got {
				terms[i] = s.Armor
			}
			assert.Equal(t, tt.want, terms)
		})
	}
	metal := bardSkill(t, "human", 13, ruletablesvc.ArmorMetal, map[string]int{"read_languages": 10}, "read_languages")
	assert.Equal(t, 10, metal.Percent, "5 + 10 points - 5 for metal armour")
	_, err := rogueSkills(t, phb(t), "bard", "human", 13, "plate", nil)
	assert.Error(t, err, "an armour that is no entry of the bard's rogue_skills")
	_, err = rogueSkills(t, phb(t), "thief", "human", 13, ruletablesvc.ArmorMetal, nil)
	assert.Error(t, err, "metal armour is the bard's, not a thief's")
}

// ADR 0030 step 18: capped at 95 with every term, floored at 0.
func TestBardSkills_Step18_CapAndFloor(t *testing.T) {
	capped := bardSkill(t, "human", 13, "none", map[string]int{"climb_walls": 40}, "climb_walls")
	assert.Equal(t, 95, capped.Percent, "50 + 40 + 10 = 100, capped")
	at := bardSkill(t, "human", 13, "leather", map[string]int{"climb_walls": 45}, "climb_walls")
	assert.Equal(t, 95, at.Percent, "exactly 95 stays")
	floored := bardSkill(t, "dwarf", 13, ruletablesvc.ArmorMetal, nil, "read_languages")
	assert.Equal(t, 0, floored.Percent, "5 - 5 - 5 = -5, floored")
	assert.Equal(t, -5, floored.Race)
	assert.Equal(t, -5, floored.Armor)
}

// ADR 0030 step 18 through the bard's rogue_skills budget (ADR 0040 step
// 3): 20 points at level 1 and 15 a level after, a warning on the total
// only; no per-skill limit.
func TestBardSkillPointBudget_Step18(t *testing.T) {
	bard := rogueOf(t, "bard")
	assert.Empty(t, ruletablesvc.SkillBudgetOf(bard, 1, map[string]int{"climb_walls": 20}), "the whole budget in one skill")
	assert.Equal(t, []ruletablesvc.BudgetWarning{{Code: ruletablesvc.BudgetTotal, Points: 21, Limit: 20}},
		ruletablesvc.SkillBudgetOf(bard, 1, map[string]int{"climb_walls": 20, "pick_pockets": 1}))
	// Level 3: 20 + 2 × 15 = 50.
	assert.Empty(t, ruletablesvc.SkillBudgetOf(bard, 3, map[string]int{"detect_noise": 50}))
	assert.Equal(t, []ruletablesvc.BudgetWarning{{Code: ruletablesvc.BudgetTotal, Points: 51, Limit: 50}},
		ruletablesvc.SkillBudgetOf(bard, 3, map[string]int{"detect_noise": 50, "read_languages": 1}))
	assert.Empty(t, ruletablesvc.SkillBudgetOf(rogueOf(t, "fighter"), 1, map[string]int{"climb_walls": 99}))
	assert.Equal(t, ruletablesvc.SkillSetBard, bard.Set)
	assert.Equal(t, ruletablesvc.SkillSetThief, rogueOf(t, "thief").Set)
	for _, class := range []string{"fighter", "mage", "cleric"} {
		assert.Nil(t, rogueOf(t, class), class)
	}
}

func percents(s []ruletablesvc.SkillPercent) []int {
	out := make([]int, len(s))
	for i, p := range s {
		out[i] = p.Percent
	}
	return out
}

func races(s []ruletablesvc.SkillPercent) []int {
	out := make([]int, len(s))
	for i, p := range s {
		out[i] = p.Race
	}
	return out
}

func dexterities(s []ruletablesvc.SkillPercent) []int {
	out := make([]int, len(s))
	for i, p := range s {
		out[i] = p.Dexterity
	}
	return out
}

// rogueOf is the fixture class's rogue_skills params (ADR 0040 step 3), nil
// for a class without them.
func rogueOf(t *testing.T, class string) *classes.RogueSkills {
	t.Helper()
	f, ok := fixtureClass(t, class).Lists(classes.KindRogueSkills)
	if !ok {
		return nil
	}
	return f.RogueSkills
}

// fixtureClass is the fixture's definition of class.
func fixtureClass(t *testing.T, class string) classes.Class {
	t.Helper()
	def, ok := cataloguetest.Definitions(t).Class(class)
	require.True(t, ok, class)
	return def
}

// rogueSkills is tbl's RogueSkills with the fixture class's rogue_skills.
func rogueSkills(t *testing.T, tbl *ruletablesvc.Tables, class, race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error) {
	t.Helper()
	rs := rogueOf(t, class)
	require.NotNil(t, rs, class)
	return tbl.RogueSkills(*rs, race, dex, armor, points)
}
