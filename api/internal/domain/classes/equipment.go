package classes

import (
	"errors"
	"fmt"
	"slices"
)

// ArmourNone is the armour entry meaning no body armour at all (ADR 0040
// step 1.6): a class whose list is only this wears none.
const ArmourNone = "none"

// WeaponRef is a catalogue weapon a class lists, by code and the weapon type
// the catalogue gives it.
type WeaponRef struct{ Code, WeaponType string }

// Equipment is what a class may wear and wield (ADR 0040 step 1.6): the one
// source of the class warnings (ADR 0033 step 20) and of the weapon types a
// proficiency may be picked in (ADR 0037 step 7). A warning never refuses
// anything.
type Equipment struct {
	// Armour are the Table 29 columns of the body armour the class may wear;
	// ArmourNone means no armour worn. Empty: any armour.
	Armour []string
	// Shields: the class may use a shield.
	Shields bool
	// WeaponTypes and Weapons list the weapons the class may wield, by
	// weapon type or by catalogue code; both empty (and not BluntOnly): any.
	WeaponTypes []string
	Weapons     []WeaponRef
	// BluntOnly: only a weapon with a bludgeoning damage type.
	BluntOnly bool
	// PickTypes are the weapon types a proficiency may be picked in (ADR 0037
	// step 7), derived once at import from this list and the weapon
	// catalogue (Registry.WithPickTypes); nil: any type.
	PickTypes []string
}

// AllowsArmour reports whether a worn body armour of Table 29 column column
// is allowed: any when the list is empty, none when it holds only
// ArmourNone, else when the column is listed.
func (e Equipment) AllowsArmour(column string) bool {
	if len(e.Armour) == 0 {
		return true
	}
	if e.NoArmour() {
		return false
	}
	return slices.Contains(e.Armour, column)
}

// NoArmour reports whether the class wears no body armour at all.
func (e Equipment) NoArmour() bool {
	return len(e.Armour) > 0 && !slices.ContainsFunc(e.Armour, func(a string) bool { return a != ArmourNone })
}

// Limited reports whether the class's weapons are restricted at all.
func (e Equipment) Limited() bool {
	return e.BluntOnly || len(e.WeaponTypes) > 0 || len(e.Weapons) > 0
}

func (e Equipment) clone() Equipment {
	e.PickTypes = slices.Clone(e.PickTypes)
	e.Armour = slices.Clone(e.Armour)
	e.WeaponTypes = slices.Clone(e.WeaponTypes)
	e.Weapons = slices.Clone(e.Weapons)
	return e
}

func checkEquipment(e Equipment) error {
	if err := unique(e.Armour, func(a string) string { return a }); err != nil {
		return fmt.Errorf("equipment armour: %w", err)
	}
	if err := unique(e.WeaponTypes, func(a string) string { return a }); err != nil {
		return fmt.Errorf("equipment weapon types: %w", err)
	}
	if err := unique(e.Weapons, func(w WeaponRef) string { return w.Code }); err != nil {
		return fmt.Errorf("equipment weapons: %w", err)
	}
	for _, w := range e.Weapons {
		if w.WeaponType == "" {
			return errors.New("equipment weapons: a weapon without a type")
		}
	}
	if e.BluntOnly && (len(e.WeaponTypes) > 0 || len(e.Weapons) > 0) {
		return errors.New("equipment: blunt_only with a weapon list")
	}
	return nil
}

// The weapon types whose weapons are judged by homebrew rules when they carry
// no damage type (ADR 0033 step 20): a bare homebrew weapon of one of them is
// not settled as blunt.
var unsettledTypes = []string{"", "other_melee", "other_thrown", "other_missile"}

// Weapon is a wielded weapon as the class limits judge it (ADR 0033 step 20).
type Weapon struct {
	// CatalogueCode is the catalogue weapon it copies, "" for a homebrew one.
	CatalogueCode string
	// WeaponType is its weapon type, "" when none is set.
	WeaponType string
	// DamageTypes are the damage types it deals; a launcher's are its
	// ammunition's (PHB p.33: a sling is blunt by its bullets, a bow is not).
	DamageTypes []string
}

// AllowsWeapon reports whether the class may wield w. A homebrew weapon
// without a catalogue code is judged by its weapon type and damage types;
// when those do not settle it (a type some listed weapon shares), it is
// allowed.
func (e Equipment) AllowsWeapon(w Weapon) bool {
	if e.BluntOnly {
		if len(w.DamageTypes) > 0 {
			return slices.Contains(w.DamageTypes, "b")
		}
		// No damage type: a catalogue row or a specific type (a bow) says it
		// does no bludgeoning; a bare homebrew weapon is not settled.
		return w.CatalogueCode == "" && slices.Contains(unsettledTypes, w.WeaponType)
	}
	if len(e.WeaponTypes) == 0 && len(e.Weapons) == 0 {
		return true
	}
	if slices.Contains(e.WeaponTypes, w.WeaponType) {
		return true
	}
	if w.CatalogueCode != "" {
		return slices.ContainsFunc(e.Weapons, func(l WeaponRef) bool { return l.Code == w.CatalogueCode })
	}
	return w.WeaponType == "" || slices.ContainsFunc(e.Weapons, func(l WeaponRef) bool { return l.WeaponType == w.WeaponType })
}

// WeaponTypeAllowed is ADR 0037 step 7: whether a weapon proficiency may be
// picked in weaponType. Wielding is never refused (ADR 0033 step 20).
func (e Equipment) WeaponTypeAllowed(weaponType string) bool {
	return e.PickTypes == nil || slices.Contains(e.PickTypes, weaponType)
}

// AllowedTogether is ADR 0042 step 16.1 on the equipment of several class
// entries judging one weapon (allows asks one class): a BluntOnly class
// decides alone, whatever the other classes (a multi-class priest keeps his
// weapon limits), unless lifted; otherwise the weapon is allowed when any
// class allows it. lifted is the ruleset's weapon lift (ADR 0041 step 2.5):
// on, the BluntOnly class is one class among the others, so any class
// allowing the weapon allows it. With one class it changes nothing. No limit
// at all (none given) allows everything.
func AllowedTogether(limits []Equipment, lifted bool, allows func(Equipment) bool) bool {
	if !lifted {
		for _, e := range limits {
			if e.BluntOnly {
				return allows(e)
			}
		}
	}
	return len(limits) == 0 || slices.ContainsFunc(limits, allows)
}

// AllowedWithOld is ADR 0042 steps 13 and 16.1 for a dual-class: the active
// classes' limits judged together (AllowedTogether), then the old (frozen)
// classes' allowance added. byOld reports that only an old class allows it,
// which warns before the crossover. Each use follows the limits of the class
// using it (step 13): an active BluntOnly class decides alone, so an old
// class adds no weapon to an active priest's; nor does a BluntOnly old
// class restrict the active ones. lifted is AllowedTogether's, on the active
// classes only: a dual-class has one, so the lift changes nothing for it, and
// an old class still adds nothing to an active BluntOnly one.
func AllowedWithOld(active, old []Equipment, lifted bool, allows func(Equipment) bool) (allowed, byOld bool) {
	if AllowedTogether(active, lifted, allows) {
		return true, false
	}
	if slices.ContainsFunc(active, func(e Equipment) bool { return e.BluntOnly }) {
		return false, false
	}
	if slices.ContainsFunc(old, allows) {
		return true, true
	}
	return false, false
}
