package itemsvc

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// Limits on what the item editor accepts.
const (
	MaxItemCount = 9999
	MaxBonus     = 10
	// MaxSpeed is the highest speed factor (ADR 0033 step 25).
	MaxSpeed = 20
	// MaxAC bounds a base AC (ADR 0033 step 25).
	MaxAC = 10
	// MaxShieldBonus bounds a shield's AC bonus (ADR 0033 step 25).
	MaxShieldBonus = 5
	maxDiceCount   = 100
	maxDiceSides   = 1000
	maxDiceMod     = 1000
)

// SaveCategories are the save categories an item's effect may allow, as
// they appear on the wire (the same names as the character's saves).
var SaveCategories = []string{"para_poison", "rod_staff_wand", "petri_poly", "breath", "spell"}

var diceRE = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

// ParseDice parses "NdM" or "NdM+K" (case-insensitive, K may be negative).
func ParseDice(dice string) (n, sides, mod int, err error) {
	m := diceRE.FindStringSubmatch(strings.ToLower(strings.TrimSpace(dice)))
	if m == nil {
		return 0, 0, 0, fmt.Errorf("invalid effect dice %q", dice)
	}
	n, _ = strconv.Atoi(m[1])
	sides, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		mod, _ = strconv.Atoi(m[3])
	}
	if n < 1 || n > maxDiceCount || sides < 2 || sides > maxDiceSides || mod > maxDiceMod || mod < -maxDiceMod {
		return 0, 0, 0, fmt.Errorf("invalid effect dice %q", dice)
	}
	return n, sides, mod, nil
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

// Validate checks a whole piece as it would be stored, returning a
// *domain.FieldError naming the first bad field (wire names): the shared
// fields (ADR 0016 step 5), the category and its fields (ADR 0033 steps 2 to
// 6), each a known code or in range.
func (i Item) Validate() error {
	if len(i.Name.Trimmed()) == 0 {
		return fieldErr("name", "Name is required")
	}
	if !i.Category.Valid() {
		return fieldErr("category", "Unknown category")
	}
	if i.Category == CategoryMagic {
		if !slices.Contains(MagicKinds, i.MagicKind) {
			return fieldErr("magicKind", "Unknown magic kind")
		}
	} else if i.MagicKind != "" {
		return fieldErr("magicKind", "Only a magic piece has a magic kind")
	}
	if i.Component != "" && !slices.Contains(Components, i.Component) {
		return fieldErr("component", "Unknown component")
	}
	if !i.State.Valid() || !Allows(i.Category, i.MagicKind, i.State) {
		return fieldErr("state", CodeNotAllowed)
	}
	if !i.Behaviour.Valid() {
		return fieldErr("behaviour", "Behaviour must be stock, charges or daily")
	}
	if i.Count < 0 {
		return fieldErr("count", "Count must be 0 or more")
	}
	if i.Count > MaxItemCount {
		return domain.FieldErrorf("count", "Count must be at most %d", MaxItemCount)
	}
	// A charges item at 0 is gone (Removed, Use): removing it is what remove is for.
	if i.Behaviour.RemovedAtZero() && i.Count == 0 {
		return fieldErr("count", "A charges item needs at least 1 charge (remove it instead)")
	}
	if i.Behaviour == BehaviourDaily {
		if i.MaxCount < 1 {
			return fieldErr("maxCount", "Daily number must be at least 1")
		}
		if i.MaxCount > MaxItemCount {
			return domain.FieldErrorf("maxCount", "Daily number must be at most %d", MaxItemCount)
		}
		if i.Count > i.MaxCount {
			return fieldErr("count", "Count cannot exceed the daily number")
		}
	}
	if i.EffectDice != "" {
		if _, _, _, err := ParseDice(i.EffectDice); err != nil {
			return fieldErr("effectDice", "Effect dice must look like 2d6 or 1d8+2")
		}
	}
	if i.EffectSave != "" && !slices.Contains(SaveCategories, i.EffectSave) {
		return fieldErr("effectSave", "Unknown save category")
	}
	switch i.Category {
	case CategoryWeapon:
		return i.validateWeapon()
	case CategoryAmmunition:
		return i.validateAmmunition()
	case CategoryArmour:
		return i.validateArmour()
	case CategoryShield:
		return i.validateShield()
	}
	return nil
}

// validateWeapon is ADR 0016 step 8 and ADR 0033 step 3: NdM damage, magic
// within ±MaxBonus, a kind, a launcher is a missile weapon, known codes, a
// speed factor of 0 to 20.
func (i Item) validateWeapon() error {
	if !validWeaponDice(i.DamageSM) {
		return fieldErr("damageSm", "Damage must look like 1d8 or 2d4")
	}
	if !validWeaponDice(i.DamageL) {
		return fieldErr("damageL", "Damage must look like 1d8 or 2d4")
	}
	if i.MagicToHit < -MaxBonus || i.MagicToHit > MaxBonus {
		return domain.FieldErrorf("magicToHit", "To-hit bonus must be between -%d and %d", MaxBonus, MaxBonus)
	}
	if i.MagicDamage < -MaxBonus || i.MagicDamage > MaxBonus {
		return domain.FieldErrorf("magicDamage", "Damage bonus must be between -%d and %d", MaxBonus, MaxBonus)
	}
	if i.WeaponKind != WeaponKindMelee && i.WeaponKind != WeaponKindMissile {
		return fieldErr("weaponKind", "Kind must be melee or missile")
	}
	if i.Fires != "" {
		if i.WeaponKind == WeaponKindMelee {
			return fieldErr("fires", "Only a missile weapon fires ammunition")
		}
		if !slices.Contains(AmmoKinds, i.Fires) {
			return fieldErr("fires", "Unknown ammunition kind")
		}
	}
	if !ValidWeaponType(i.WeaponType) {
		return fieldErr("weaponType", "Unknown weapon type")
	}
	if i.Size != "" && !slices.Contains(Sizes, i.Size) {
		return fieldErr("size", "Size must be s, m or l")
	}
	if i.Speed < 0 || i.Speed > MaxSpeed {
		return domain.FieldErrorf("speed", "Speed factor must be between 0 and %d", MaxSpeed)
	}
	return validDamageTypes(i.DamageTypes)
}

// validateAmmunition is ADR 0033 step 4: a known ammo kind, its magic, and
// its damage dice, both or none.
func (i Item) validateAmmunition() error {
	if !slices.Contains(AmmoKinds, i.AmmoKind) {
		return fieldErr("ammoKind", "Unknown ammunition kind")
	}
	if i.MagicBonus < -MaxBonus || i.MagicBonus > MaxBonus {
		return domain.FieldErrorf("magicalBonus", "Magical bonus must be between -%d and %d", MaxBonus, MaxBonus)
	}
	if (i.DamageSM == "") != (i.DamageL == "") {
		return fieldErr("damageL", "Give the damage against both sizes, or neither")
	}
	if i.DamageSM != "" && !validWeaponDice(i.DamageSM) {
		return fieldErr("damageSm", "Damage must look like 1d8 or 2d4")
	}
	if i.DamageL != "" && !validWeaponDice(i.DamageL) {
		return fieldErr("damageL", "Damage must look like 1d8 or 2d4")
	}
	return validDamageTypes(i.DamageTypes)
}

// validateArmour is ADR 0033 step 5 (and step 25's ranges): a base AC of -10
// to 10, a known Table 29 column and Table 52 row.
func (i Item) validateArmour() error {
	if i.BaseAC < -MaxAC || i.BaseAC > MaxAC {
		return domain.FieldErrorf("baseAc", "Base AC must be between -%d and %d", MaxAC, MaxAC)
	}
	if !slices.Contains(SkillColumns, i.SkillColumn) {
		return fieldErr("skillColumn", "Unknown skills column")
	}
	if !slices.Contains(ArmorRows, i.ArmorRow) {
		return fieldErr("armorRow", "Unknown armour type")
	}
	return nil
}

// validateShield is ADR 0033 step 5: a known kind, bonuses of 0 to 5.
func (i Item) validateShield() error {
	if !slices.Contains(ShieldKinds, i.ShieldKind) {
		return fieldErr("shieldKind", "Unknown shield kind")
	}
	if i.ACMelee < 0 || i.ACMelee > MaxShieldBonus {
		return domain.FieldErrorf("acMelee", "Shield bonus must be between 0 and %d", MaxShieldBonus)
	}
	if i.ACMissile < 0 || i.ACMissile > MaxShieldBonus {
		return domain.FieldErrorf("acMissile", "Shield bonus must be between 0 and %d", MaxShieldBonus)
	}
	return nil
}

func validDamageTypes(types []string) error {
	if len(types) > 2 {
		return fieldErr("damageTypes", "At most two damage types")
	}
	seen := map[string]bool{}
	for _, t := range types {
		if !slices.Contains(DamageTypes, t) || seen[t] {
			return fieldErr("damageTypes", "Damage types are one or two of b, p, s")
		}
		seen[t] = true
	}
	return nil
}

// The attack rolls a weapon's damage dice as NdM (the weapon's magic is its
// own field), so a "+K" is refused here rather than silently dropped.
var weaponDiceRE = regexp.MustCompile(`^(\d+)d(\d+)$`)

func validWeaponDice(dice string) bool {
	m := weaponDiceRE.FindStringSubmatch(dice)
	if m == nil {
		return false
	}
	n, _ := strconv.Atoi(m[1])
	sides, _ := strconv.Atoi(m[2])
	return n >= 1 && n <= maxDiceCount && sides >= 2 && sides <= maxDiceSides
}
