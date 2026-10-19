package wire

import "github.com/ceffo/critical/api/internal/domain/service/charactersvc"

// PendingLevelDTO is one level whose hit points are owed (ADR 0030 step 7):
// `die` the hit die's sides to roll up to the name level (0 above it),
// `fixedGain` the gain above it (0 up to it).
type PendingLevelDTO struct {
	Level     int `json:"level"`
	Die       int `json:"die"`
	FixedGain int `json:"fixedGain"`
}

// PendingDTO maps the pending levels, never null.
func PendingDTO(p []charactersvc.PendingLevel) []PendingLevelDTO {
	out := make([]PendingLevelDTO, 0, len(p))
	for _, l := range p {
		out = append(out, PendingLevelDTO{Level: l.Level, Die: l.Die, FixedGain: l.FixedGain})
	}
	return out
}
