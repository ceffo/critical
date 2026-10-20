package wire

import "github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"

// SkillPointsBudgetDTO is a thief's or a bard's skill point budget at the
// realized level (ADR 0030 steps 17 and 18), served by GET /api/skills and in
// the sheet's derived block (GET /api/character, the creation draft):
// available = first + perLevel x (level - 1) points in all, spent entered,
// left = available - spent (negative when over). perSkillCap = perSkillFirst +
// perSkillPerLevel x (level - 1) is the most one skill holds (thief), all 0
// for no such limit (bard).
type SkillPointsBudgetDTO struct {
	Level            int `json:"level"`
	First            int `json:"first"`
	PerLevel         int `json:"perLevel"`
	Available        int `json:"available"`
	Spent            int `json:"spent"`
	Left             int `json:"left"`
	PerSkillFirst    int `json:"perSkillFirst"`
	PerSkillPerLevel int `json:"perSkillPerLevel"`
	PerSkillCap      int `json:"perSkillCap"`
}

// SkillPointsBudgetToDTO maps a budget; nil (a class without skill points)
// stays nil.
func SkillPointsBudgetToDTO(b *ruletablesvc.SkillPointsBudget) *SkillPointsBudgetDTO {
	if b == nil {
		return nil
	}
	return &SkillPointsBudgetDTO{
		Level: b.Level, First: b.First, PerLevel: b.PerLevel, Available: b.Total, Spent: b.Spent, Left: b.Left,
		PerSkillFirst: b.PerSkillFirst, PerSkillPerLevel: b.PerSkillPerLevel, PerSkillCap: b.PerSkill,
	}
}
