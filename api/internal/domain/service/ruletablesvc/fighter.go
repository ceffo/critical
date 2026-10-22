package ruletablesvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The fighter's rules (ADR 0030 steps 9, 11 and 12): who has an exceptional
// strength, who specializes, and the attacks per round a weapon gives. The
// numbers are the class tables' (PHB Tables 15, 35 and 45); this file only
// says which row and column a weapon reads.

// ExceptionalStrengthAllowed reports whether a character of class def and
// race with Strength str may enter an exceptional strength (ADR 0030 step 9,
// ADR 0040 steps 3 and 4): its class lists exceptional_strength, its Strength
// is 18 and its race does not list no_exceptional_strength. The entry is
// checked before a level is known, so the class's listing at any level
// counts.
func ExceptionalStrengthAllowed(def classes.Class, race classes.Race, str int) bool {
	_, ok := def.Lists(classes.KindExceptionalStrength)
	_, barred := race.Trait(classes.TraitNoExceptionalStrength)
	return ok && str == 18 && !barred
}

// Specializes reports whether class def may specialize in a weapon (ADR 0030
// step 12, ADR 0040 step 3: it lists weapon_specialization). A pick is
// checked whatever the level, so the listing at any level counts.
func Specializes(def classes.Class) bool {
	_, ok := def.Lists(classes.KindWeaponSpecialization)
	return ok
}

// Weapon kinds as the weapons carry them.
const (
	weaponMelee   = "melee"
	weaponMissile = "missile"
)

// bowTypes are the weapon types whose specialist gains no attacks (PHB "bow
// specialists do not gain any additional attacks", ADR 0030 step 11.2).
var bowTypes = []string{"long_bow", "short_bow"}

// Specialization's cost in weapon slots, in all (ADR 0037 step 9, PHB p.53, "Cost of
// Specialization"): a bow's specialist spends 3, any other type's 2 (melee weapons and
// crossbows by the book; darts, slings and the other thrown and missile types by the step's
// proposed reading).
const (
	specializationSlots    = 2
	bowSpecializationSlots = 3
)

// SpecializationSlots is what a specialization pick of weaponType spends in all (ADR 0037 step
// 9): 3 slots for a bow, 2 for any other weapon type.
func SpecializationSlots(weaponType string) int {
	if slices.Contains(bowTypes, weaponType) {
		return bowSpecializationSlots
	}
	return specializationSlots
}

// specialistColumns maps a missile weapon type to its PHB Table 35 column;
// every other non-bow missile type reads "other_missile" (ADR 0030 step 11.2).
var specialistColumns = map[string]string{
	"light_crossbow": "light_crossbow",
	"heavy_crossbow": "heavy_crossbow",
	"dagger":         "thrown_dagger",
	"dart":           "thrown_dart",
}

// AttackRate is a number of attacks per number of rounds (3/2: three
// attacks every two rounds).
type AttackRate struct {
	Attacks, Rounds int
}

// InRound is the count of attacks in round (ADR 0030 step 11.4, proposed):
// with a rate over two rounds, the lower half in odd rounds and the higher
// in even rounds (3/2: 1 then 2; 1/2: none then 1). Rounds start at 1; a
// round below 1 counts as the first.
func (r AttackRate) InRound(round int) int {
	if r.Rounds != 2 {
		return r.Attacks / max(r.Rounds, 1)
	}
	if round%2 == 0 && round > 0 {
		return (r.Attacks + 1) / 2
	}
	return r.Attacks / 2
}

// AttackRateFor is ADR 0030 step 11: the attacks per round of a weapon of
// kind ("melee" or "missile") and weaponType in the hands of a character of
// class at level (its definition's features), specialized or not. ok is false when the weapon shows
// no rate (step 11.3: no type, or a missile type without a rate of fire), and
// for a class without melee_attacks_per_round at level (ADR 0040 step 3: the
// rate is a fighter's, Table 15).
//
//  1. Melee: Table 15 by level; specialized, Table 35's melee column.
//  2. Missile: the type's rate of fire (Table 45) whatever the level;
//     specialized in a non-bow missile, Table 35's column for it by level; a
//     bow specialist keeps the rate of fire.
func (t *Tables) AttackRateFor(class string, level int, kind, weaponType string, specialized bool) (AttackRate, bool, error) {
	def, err := t.Class(class)
	if err != nil {
		return AttackRate{}, false, err
	}
	if !def.Has(classes.KindMeleeAttacksPerRound, level) || weaponType == "" {
		return AttackRate{}, false, nil
	}
	c, err := t.classTables()
	if err != nil {
		return AttackRate{}, false, err
	}
	switch kind {
	case weaponMelee:
		if specialized {
			return specialistRate(c, "melee", level)
		}
		for _, r := range c.WarriorAttacks {
			if level >= r.LevelMin && level <= r.LevelMax {
				return AttackRate{Attacks: r.Attacks, Rounds: r.Rounds}, true, nil
			}
		}
		return AttackRate{}, false, fmt.Errorf("warrior attacks: level %d outside the table", level)
	case weaponMissile:
		if specialized && !slices.Contains(bowTypes, weaponType) {
			column, ok := specialistColumns[weaponType]
			if !ok {
				column = "other_missile"
			}
			return specialistRate(c, column, level)
		}
		for _, r := range c.RateOfFire {
			if r.WeaponType == weaponType {
				return AttackRate{Attacks: r.Attacks, Rounds: r.Rounds}, true, nil
			}
		}
		return AttackRate{}, false, nil
	}
	return AttackRate{}, false, fmt.Errorf("attack rate: unknown weapon kind %q", kind)
}

func specialistRate(c *ClassTables, column string, level int) (AttackRate, bool, error) {
	for _, r := range c.SpecialistAttacks {
		if r.Weapon == column && level >= r.LevelMin && level <= r.LevelMax {
			return AttackRate{Attacks: r.Attacks, Rounds: r.Rounds}, true, nil
		}
	}
	return AttackRate{}, false, fmt.Errorf("specialist attacks: %s level %d outside the table", column, level)
}
