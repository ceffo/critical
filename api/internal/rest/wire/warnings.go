package wire

import "github.com/ceffo/critical/api/internal/domain/service/charactersvc"

// WarningDTO is a code and the field it is about; the web words it. The
// numbers outside their range (ADR 0032 step 5.2) carry value, min and max;
// the languages over the limit (ADR 0032 step 4.2) also carry how many
// additional languages the character knows (count) and its Intelligence
// allows (limit).
type WarningDTO struct {
	Code  string `json:"code"`
	Field string `json:"field"`
	Count *int   `json:"count,omitempty"`
	Limit *int   `json:"limit,omitempty"`
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
}

// WarningsDTO maps the character's warnings, never nil.
func WarningsDTO(ws []charactersvc.Warning) []WarningDTO {
	out := make([]WarningDTO, 0, len(ws))
	for _, w := range ws {
		out = append(out, WarningDTO(w))
	}
	return out
}
