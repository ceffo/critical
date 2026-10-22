package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0030 steps 17 and 18: the warnings flip exactly at the limits SkillBudgetLimits reports,
// at levels 1 to 5, for the thief (total and per skill) and the bard (total only).
func TestSkillBudgetLimits_Steps17And18_WarningsFlipExactlyAtTheLimits(t *testing.T) {
	for _, class := range []string{"thief", "bard"} {
		skills := ruletablesvc.SkillsOf(rogue(t, class))
		for level := 1; level <= 5; level++ {
			total, per, ok := ruletablesvc.SkillBudgetLimits(rogue(t, class), level)
			assert.True(t, ok)
			at := map[string]int{skills[0]: total}
			if per > 0 {
				at = map[string]int{skills[0]: per, skills[1]: total - per}
			}
			assert.Empty(t, ruletablesvc.SkillBudgetOf(rogue(t, class), level, at), "%s level %d at the limits", class, level)
			over := map[string]int{skills[0]: at[skills[0]], skills[1]: at[skills[1]] + 1}
			assert.NotEmpty(t, ruletablesvc.SkillBudgetOf(rogue(t, class), level, over), "%s level %d one over in all", class, level)
			if per > 0 {
				assert.NotEmpty(t, ruletablesvc.SkillBudgetOf(rogue(t, class), level, map[string]int{skills[0]: per + 1}), "%s level %d one over in a skill", class, level)
			}
		}
	}
}
