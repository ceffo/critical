package wire

import (
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// PendingLevelDTO is one level whose hit points are owed (ADR 0030 step 7):
// `die` the hit die's sides to roll up to the name level (0 above it),
// `fixedGain` the gain above it (0 up to it), `grants` what the level grants
// beyond hit points (ADR 0037 step 15).
type PendingLevelDTO struct {
	Level     int            `json:"level"`
	Die       int            `json:"die"`
	FixedGain int            `json:"fixedGain"`
	Grants    LevelGrantsDTO `json:"grants"`
}

// LevelGrantsDTO is ADR 0037 step 15: the weapon and non-weapon slots and the
// skill points (a thief's or a bard's) a level grants, or left to spend; 0
// for none.
type LevelGrantsDTO struct {
	WeaponSlots    int `json:"weaponSlots"`
	NonweaponSlots int `json:"nonWeaponSlots"`
	SkillPoints    int `json:"skillPoints"`
}

// GrantsDTO maps a level's grants.
func GrantsDTO(g skills.LevelGrants) LevelGrantsDTO { return LevelGrantsDTO(g) }

// PendingDTO maps the pending levels, never null.
func PendingDTO(p []charactersvc.PendingLevel) []PendingLevelDTO {
	out := make([]PendingLevelDTO, 0, len(p))
	for _, l := range p {
		out = append(out, PendingLevelDTO{Level: l.Level, Die: l.Die, FixedGain: l.FixedGain, Grants: GrantsDTO(l.Grants)})
	}
	return out
}
