package wire

import "github.com/ceffo/critical/api/internal/domain/service/buffsvc"

// BuffEffectsDTO is the modifier payload.
type BuffEffectsDTO struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
	BaseAC     *int     `json:"base_ac,omitempty"`
	// WeaponKind is the weapon-kind condition of the other effects, melee or
	// missile (ADR 0004 step 18); omitted: every roll.
	WeaponKind buffsvc.WeaponKind `json:"weapon_kind,omitempty"`
	// WithoutMagicArmour is the AC effects' condition (ADR 0033 step 12).
	WithoutMagicArmour bool `json:"without_magic_armour,omitempty"`
}
