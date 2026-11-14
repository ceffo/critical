package wire

import (
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// WarningDTO is a code and the field it is about; the web words it. The
// numbers outside their range (ADR 0032 step 5.2) carry value, min and max.
// ADR 0032 step 4.2's languages_over_limit (count, limit) is no longer served:
// the languages spend non-weapon slots (ADR 0037 step 6).
type WarningDTO struct {
	Code  string `json:"code"`
	Field string `json:"field"`
	// Value, Min and Max are a height (cm), weight (kg) or age (years) outside
	// the race's table range (ADR 0032 step 5.2).
	Value *float64 `json:"value,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	// Pool, Spent, Available and Left are a slot warning's figures (ADR 0037
	// step 11): slots_over_budget {pool, spent, available}, slots_to_spend
	// {pool, left}.
	Pool      string `json:"pool,omitempty"`
	Spent     *int   `json:"spent,omitempty"`
	Available *int   `json:"available,omitempty"`
	Left      *int   `json:"left,omitempty"`
	// Class is the class entry of a level_limit warning (ADR 0042 step 7),
	// its level in value and the race's limit in max, or of a specialist_race
	// warning (ADR 0050 step 14.1).
	Class string `json:"class,omitempty"`
	// Race is a specialist_race warning's race, outside PHB Table 22's for
	// the class (ADR 0050 step 14.1).
	Race string `json:"race,omitempty"`
	// Type and Slots are a weapon_pick_to_redo warning's weapon type and the
	// slots it spent (ADR 0037 step 21.8.3).
	Type  string `json:"type,omitempty"`
	Slots *int   `json:"slots,omitempty"`
}

// WarningsDTO maps the character's warnings, never nil.
func WarningsDTO(ws []charactersvc.Warning) []WarningDTO {
	out := make([]WarningDTO, 0, len(ws))
	for _, w := range ws {
		out = append(out, WarningDTO(w))
	}
	return out
}

// SkillWarningsDTO maps the skills module's warnings (ADR 0037 step 11: the slot warnings of an
// editor's preview or save), never nil.
func SkillWarningsDTO(ws []skills.Warning) []WarningDTO {
	out := make([]WarningDTO, 0, len(ws))
	for _, w := range ws {
		out = append(out, WarningDTO{Code: w.Code, Field: w.Field, Pool: w.Pool, Spent: w.Spent, Available: w.Available, Left: w.Left, Type: w.Type, Slots: w.Slots})
	}
	return out
}
