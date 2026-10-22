package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0030 steps 17 and 18: the budget the UI shows, with the terms of its formula, at levels 1,
// 2 and 15. Thief: 60 + 30 x (L-1) points, 30 + 15 x (L-1) in one skill. Bard: 20 + 15 x (L-1),
// no limit in one skill. Left is available minus spent, negative when over.
func TestSkillPointsFigures_Steps17And18_FormulaTermsAtLevels(t *testing.T) {
	tests := []struct {
		name   string
		class  string
		level  int
		points map[string]int
		want   ruletablesvc.SkillPointsBudget
	}{
		{"thief 1", "thief", 1, map[string]int{"open_locks": 30, "pick_pockets": 25}, ruletablesvc.SkillPointsBudget{
			Level: 1, First: 60, PerLevel: 30, Total: 60, Spent: 55, Left: 5, PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 30,
		}},
		{"thief 2", "thief", 2, nil, ruletablesvc.SkillPointsBudget{
			Level: 2, First: 60, PerLevel: 30, Total: 90, Spent: 0, Left: 90, PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 45,
		}},
		{"thief 15", "thief", 15, map[string]int{"open_locks": 90, "climb_walls": 90, "pick_pockets": 90, "move_silently": 90, "hide_in_shadows": 90}, ruletablesvc.SkillPointsBudget{
			Level: 15, First: 60, PerLevel: 30, Total: 480, Spent: 450, Left: 30, PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 240,
		}},
		{"bard 1", "bard", 1, map[string]int{"climb_walls": 5}, ruletablesvc.SkillPointsBudget{
			Level: 1, First: 20, PerLevel: 15, Total: 20, Spent: 5, Left: 15,
		}},
		{"bard 2", "bard", 2, nil, ruletablesvc.SkillPointsBudget{
			Level: 2, First: 20, PerLevel: 15, Total: 35, Left: 35,
		}},
		{"bard 15 over", "bard", 15, map[string]int{"climb_walls": 90, "detect_noise": 90, "pick_pockets": 90}, ruletablesvc.SkillPointsBudget{
			Level: 15, First: 20, PerLevel: 15, Total: 230, Spent: 270, Left: -40,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ruletablesvc.SkillPointsFigures(rogue(t, tt.class), tt.level, tt.points)
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// A class without skill points, and a level under 1, have no budget.
func TestSkillPointsFigures_NoBudget(t *testing.T) {
	_, ok := ruletablesvc.SkillPointsFigures(rogue(t, "fighter"), 5, nil)
	assert.False(t, ok)
	_, ok = ruletablesvc.SkillPointsFigures(rogue(t, "thief"), 0, nil)
	assert.False(t, ok)
}
