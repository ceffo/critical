package wire

import (
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
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
// for none. FreeSpells are a mage's free spells (ADR 0030 step 20.8): the
// level's, or the ones owed; never null.
type LevelGrantsDTO struct {
	WeaponSlots    int            `json:"weaponSlots"`
	NonweaponSlots int            `json:"nonWeaponSlots"`
	SkillPoints    int            `json:"skillPoints"`
	FreeSpells     []FreeSpellDTO `json:"freeSpells"`
}

// FreeSpellDTO is one free spell (ADR 0030 step 20.8): the realized level
// that grants it (the add's grantLevel) and the spell level it is of.
type FreeSpellDTO struct {
	Level      int `json:"level"`
	SpellLevel int `json:"spellLevel"`
}

// GrantsDTO maps a level's grants, or what is left to spend: the skills
// module's slots and points, and the spellbook's free spells.
func GrantsDTO(g skills.LevelGrants, free []spellbook.FreeSpell) LevelGrantsDTO {
	out := LevelGrantsDTO{
		WeaponSlots: g.WeaponSlots, NonweaponSlots: g.NonweaponSlots, SkillPoints: g.SkillPoints,
		FreeSpells: make([]FreeSpellDTO, 0, len(free)),
	}
	for _, f := range free {
		out.FreeSpells = append(out.FreeSpells, FreeSpellDTO{Level: f.Level, SpellLevel: f.SpellLevel})
	}
	return out
}

// PendingDTO maps the pending levels, never null.
func PendingDTO(p []charactersvc.PendingLevel) []PendingLevelDTO {
	out := make([]PendingLevelDTO, 0, len(p))
	for _, l := range p {
		out = append(out, PendingLevelDTO{Level: l.Level, Die: l.Die, FixedGain: l.FixedGain, Grants: GrantsDTO(l.Grants, l.FreeSpells)})
	}
	return out
}
