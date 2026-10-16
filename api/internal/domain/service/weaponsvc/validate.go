package weaponsvc

import (
	"fmt"
	"regexp"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// Limits on what the weapon editor accepts.
const (
	MaxBonus     = 10
	maxDiceCount = 100
	maxDiceSides = 1000
)

// The attack rolls the damage dice as NdM (the weapon's magic is its own
// field), so a "+K" is refused here rather than silently dropped.
var diceRE = regexp.MustCompile(`^(\d+)d(\d+)$`)

func validDice(dice string) bool {
	m := diceRE.FindStringSubmatch(dice)
	if m == nil {
		return false
	}
	var n, sides int
	_, _ = fmt.Sscan(m[1], &n)
	_, _ = fmt.Sscan(m[2], &sides)
	return n >= 1 && n <= maxDiceCount && sides >= 2 && sides <= maxDiceSides
}

func fieldErr(field, msg string) error { return &domain.FieldError{Field: field, Message: msg} }

// nameLanguages refuses a name typed in a code that is not a UI language
// (ADR 0023 step 11), blank bodies included.
func nameLanguages(name localized.Text) error {
	if lang, ok := name.UnknownLanguage(); ok {
		return domain.FieldErrorf("name", "Unknown language %q", lang)
	}
	return nil
}

// Validate checks a whole weapon as it would be stored, returning a
// *domain.FieldError naming the first bad field (wire names).
func (w Weapon) Validate() error {
	if len(w.Name.Trimmed()) == 0 {
		return fieldErr("name", "Name is required")
	}
	if !validDice(w.DamageSM) {
		return fieldErr("damage_sm", "Damage must look like 1d8 or 2d4")
	}
	if !validDice(w.DamageL) {
		return fieldErr("damage_l", "Damage must look like 1d8 or 2d4")
	}
	if w.MagicToHit < -MaxBonus || w.MagicToHit > MaxBonus {
		return domain.FieldErrorf("magic_to_hit", "To-hit bonus must be between -%d and %d", MaxBonus, MaxBonus)
	}
	if w.MagicDamage < -MaxBonus || w.MagicDamage > MaxBonus {
		return domain.FieldErrorf("magic_damage", "Damage bonus must be between -%d and %d", MaxBonus, MaxBonus)
	}
	if w.Kind != KindMelee && w.Kind != KindMissile {
		return fieldErr("kind", "Kind must be melee or missile")
	}
	if w.Kind == KindMelee && w.AmmoKind != "" {
		return fieldErr("ammo_kind", "Only a missile weapon fires ammunition")
	}
	return nil
}
