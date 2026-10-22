package classes

import (
	"errors"
	"fmt"
	"slices"
)

// The closed list of race trait kinds (ADR 0040 step 4). The code implements
// each kind once and switches on the kind, never on a race id.
const (
	// TraitConSaveBonus: PHB Table 9's Constitution bonus on the saves of its
	// categories (ADR 0030 step 22.1). Params Categories and Poison.
	TraitConSaveBonus = "con_save_bonus"
	// TraitNoExceptionalStrength: the race never rolls an exceptional
	// Strength (ADR 0030 step 9). No params.
	TraitNoExceptionalStrength = "no_exceptional_strength"
	// TraitWeaponToHit: a to-hit bonus with weapon types (ADR 0030 step 22.3,
	// not computed yet). Params WeaponTypes and Bonus.
	TraitWeaponToHit = "weapon_to_hit"
)

// TraitKinds is the closed list of step 4.
var TraitKinds = []string{TraitConSaveBonus, TraitNoExceptionalStrength, TraitWeaponToHit}

// SaveCategories are the saving throw category codes a con_save_bonus names.
var SaveCategories = []string{"para_poison", "rod_staff_wand", "petri_poly", "breath", "spell"}

// Trait is one entry of a race's trait list (ADR 0040 step 4). Only the
// fields of its kind are set.
type Trait struct {
	Kind string
	// Categories and Poison are con_save_bonus's params. Poison is carried but
	// not read yet (ADR 0030 step 22.1).
	Categories []string
	Poison     bool
	// WeaponTypes and Bonus are weapon_to_hit's params.
	WeaponTypes []string
	Bonus       int
}

// Dice is count dice of sides sides ("2d6").
type Dice struct{ Count, Sides int }

// Min is the lowest total of the dice (every die 1), Max the highest.
func (d Dice) Min() int { return d.Count }

// Max is the highest total of the dice (every die at its maximum).
func (d Dice) Max() int { return d.Count * d.Sides }

// Identity is a race's rows of PHB Tables 10 and 11 (ADR 0040 step 2, ADR
// 0032 step 5). Base heights are in inches and base weights in pounds, male
// then female; ages in years.
type Identity struct {
	HeightMale, HeightFemale int
	HeightDice               Dice
	WeightMale, WeightFemale int
	WeightDice               Dice
	StartAge                 int
	StartAgeDice             Dice
	MaxAge                   int
	MaxAgeDice               Dice
}

// Trait is the race's trait of kind, false when it has none.
func (x Race) Trait(kind string) (Trait, bool) {
	for _, t := range x.Traits {
		if t.Kind == kind {
			return t.clone(), true
		}
	}
	return Trait{}, false
}

func (t Trait) clone() Trait {
	t.Categories = slices.Clone(t.Categories)
	t.WeaponTypes = slices.Clone(t.WeaponTypes)
	return t
}

func checkTraits(ts []Trait) ([]Trait, error) {
	if err := unique(ts, func(t Trait) string { return t.Kind }); err != nil {
		return nil, fmt.Errorf("traits: %w", err)
	}
	out := make([]Trait, 0, len(ts))
	for _, t := range ts {
		if err := checkTrait(t); err != nil {
			return nil, fmt.Errorf("trait %s: %w", t.Kind, err)
		}
		out = append(out, t.clone())
	}
	return out, nil
}

func checkTrait(t Trait) error {
	con, weapon := len(t.Categories) > 0 || t.Poison, len(t.WeaponTypes) > 0 || t.Bonus != 0
	switch t.Kind {
	case TraitNoExceptionalStrength:
		if con || weapon {
			return errors.New("unexpected parameter")
		}
	case TraitConSaveBonus:
		if weapon || len(t.Categories) == 0 {
			return errors.New("a missing or unexpected parameter")
		}
		if err := unique(t.Categories, func(c string) string { return c }); err != nil {
			return fmt.Errorf("categories: %w", err)
		}
		for _, c := range t.Categories {
			if !slices.Contains(SaveCategories, c) {
				return fmt.Errorf("unknown save category %q", c)
			}
		}
	case TraitWeaponToHit:
		if con || len(t.WeaponTypes) == 0 {
			return errors.New("a missing or unexpected parameter")
		}
	default:
		return fmt.Errorf("unknown trait kind %q", t.Kind)
	}
	return nil
}

func checkIdentity(i Identity) error {
	for name, d := range map[string]Dice{"height": i.HeightDice, "weight": i.WeightDice, "start age": i.StartAgeDice, "max age": i.MaxAgeDice} {
		if d.Count < 1 || d.Sides < 1 {
			return fmt.Errorf("identity %s: dice need a count and sides of 1 or more", name)
		}
	}
	if i.HeightMale < 1 || i.HeightFemale < 1 || i.WeightMale < 1 || i.WeightFemale < 1 || i.StartAge < 1 || i.MaxAge < 1 {
		return errors.New("identity: a base under 1")
	}
	return nil
}
