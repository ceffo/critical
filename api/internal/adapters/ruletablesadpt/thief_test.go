package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The thief's rules (ADR 0030 steps 17 and 19) over the fixture's class
// tables. Expected values are read from PHB Tables 26 to 30 (p.39-40), not
// from the artifact.

// ADR 0030 step 17: base + points + race + Dexterity + armour, every term
// kept, capped at 95 and floored at 0.
func TestThiefSkills_Step17_TheTermsOfEachSkill(t *testing.T) {
	tbl := phb(t)
	got, err := tbl.ThiefSkills("halfling", 18, "none", map[string]int{"pick_pockets": 20, "open_locks": 30})
	require.NoError(t, err)
	want := []ruletablesvc.SkillPercent{
		{Skill: "pick_pockets", Base: 15, Points: 20, Race: 5, Dexterity: 10, Armor: 5, Raw: 55, Percent: 55},
		{Skill: "open_locks", Base: 10, Points: 30, Race: 5, Dexterity: 15, Armor: 0, Raw: 60, Percent: 60},
		{Skill: "find_remove_traps", Base: 5, Race: 5, Dexterity: 5, Raw: 15, Percent: 15},
		{Skill: "move_silently", Base: 10, Race: 10, Dexterity: 10, Armor: 10, Raw: 40, Percent: 40},
		{Skill: "hide_in_shadows", Base: 5, Race: 15, Dexterity: 10, Armor: 5, Raw: 35, Percent: 35},
		{Skill: "detect_noise", Base: 15, Race: 5, Raw: 20, Percent: 20},
		{Skill: "climb_walls", Base: 60, Race: -15, Armor: 10, Raw: 55, Percent: 55},
		// 0 - 5: floored at 0.
		{Skill: "read_languages", Race: -5, Raw: -5, Percent: 0},
	}
	assert.Equal(t, want, got)
}

// ADR 0030 step 17: the cap, the floor, each armour column and leather's
// baseline, a Dexterity without a row.
func TestThiefSkills_Step17_CapFloorAndArmour(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		name, race, armor, skill string
		dex, points, want        int
	}{
		{"capped at 95", "human", "none", "climb_walls", 19, 40, 95},
		{"floored at 0", "human", "padded_studded_leather", "move_silently", 12, 0, 0},
		{"elven chain", "elf", "elven_chain", "pick_pockets", 17, 10, 15},
		{"padded or studded leather", "human", "padded_studded_leather", "climb_walls", 14, 0, 30},
		{"leather is the baseline", "human", "leather", "open_locks", 16, 0, 15},
		{"no race code", "", "leather", "find_remove_traps", 13, 5, 10},
		{"a Dexterity under Table 28", "human", "leather", "pick_pockets", 8, 0, 15},
		{"dwarf", "dwarf", "leather", "find_remove_traps", 13, 0, 20},
		{"gnome", "gnome", "leather", "climb_walls", 13, 0, 45},
		{"half-elf", "half_elf", "leather", "pick_pockets", 13, 0, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tbl.ThiefSkills(tt.race, tt.dex, tt.armor, map[string]int{tt.skill: tt.points})
			require.NoError(t, err)
			for _, s := range got {
				if s.Skill == tt.skill {
					assert.Equal(t, tt.want, s.Percent)
					return
				}
			}
			t.Fatalf("no %s", tt.skill)
		})
	}
	_, err := tbl.ThiefSkills("human", 13, "plate", nil)
	assert.Error(t, err, "an armour that is none of ThiefArmors")
}

// ADR 0030 step 17: the budget of PHB p.39 is a warning per breach, on the
// totals the realized level allows.
func TestSkillPointBudget_Step17(t *testing.T) {
	assert.Empty(t, ruletablesvc.SkillPointBudget(1, map[string]int{"open_locks": 30, "pick_pockets": 30}))
	assert.Equal(t, []ruletablesvc.BudgetWarning{
		{Code: ruletablesvc.BudgetTotal, Points: 65, Limit: 60},
		{Code: ruletablesvc.BudgetPerSkill, Skill: "open_locks", Points: 31, Limit: 30},
	}, ruletablesvc.SkillPointBudget(1, map[string]int{"open_locks": 31, "pick_pockets": 30, "climb_walls": 4}))
	// Level 3: 60 + 2 × 30 = 120 in all, 30 + 2 × 15 = 60 in one skill.
	assert.Empty(t, ruletablesvc.SkillPointBudget(3, map[string]int{"open_locks": 60, "pick_pockets": 60}))
	assert.Equal(t, []ruletablesvc.BudgetWarning{
		{Code: ruletablesvc.BudgetPerSkill, Skill: "pick_pockets", Points: 61, Limit: 60},
	}, ruletablesvc.SkillPointBudget(3, map[string]int{"pick_pockets": 61}))
}

// ADR 0030 step 19: PHB Table 30, x2 at 1-4, x3 at 5-8, x4 at 9-12, x5 from
// 13.
func TestBackstabFor_Step19_Table30(t *testing.T) {
	tbl := phb(t)
	for level, want := range map[int]int{1: 2, 4: 2, 5: 3, 8: 3, 9: 4, 12: 4, 13: 5, 20: 5} {
		got, err := tbl.BackstabFor(level)
		require.NoError(t, err)
		assert.Equal(t, want, got, "level %d", level)
	}
	_, err := tbl.BackstabFor(21)
	assert.Error(t, err)
	assert.True(t, ruletablesvc.Backstabs("thief"))
	assert.True(t, ruletablesvc.HasThiefSkills("thief"))
	for _, class := range []string{"bard", "fighter", "mage", "cleric"} {
		assert.False(t, ruletablesvc.Backstabs(class), class)
		assert.False(t, ruletablesvc.HasThiefSkills(class), class)
	}
}
