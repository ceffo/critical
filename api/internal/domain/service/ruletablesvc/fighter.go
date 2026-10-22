package ruletablesvc

import (
	"fmt"
	"slices"
)

// The fighter's rules (ADR 0030 steps 9, 11 and 12): who has an exceptional
// strength, who specializes, and the attacks per round a weapon gives. The
// numbers are the class tables' (PHB Tables 15, 35 and 45); this file only
// says which row and column a weapon reads.

// exceptionalStrengthGroups are the class groups whose Strength 18 takes an
// exceptional percentile (ADR 0030 step 9: warriors only, PHB Table 1).
var exceptionalStrengthGroups = []string{"warrior"}

// noExceptionalStrengthRaces are the races that never roll it (PHB Table 7,
// note: halfling fighters do not roll exceptional strength).
var noExceptionalStrengthRaces = []string{"halfling"}

// specializingClasses are the classes that may specialize in a weapon (ADR
// 0030 step 12: fighters only).
var specializingClasses = []string{"fighter"}

// HasExceptionalStrength reports whether a character of class group takes an
// exceptional strength percentile at Strength 18 (ADR 0030 step 9) and,
// through it, the warrior's scale of Strength spells above 18 (step 10).
func HasExceptionalStrength(group string) bool {
	return slices.Contains(exceptionalStrengthGroups, group)
}

// ExceptionalStrengthAllowed reports whether a character of class group and
// race with Strength str may enter an exceptional strength (ADR 0030 step 9):
// a warrior, at Strength 18, of a race that rolls it.
func ExceptionalStrengthAllowed(group, race string, str int) bool {
	return HasExceptionalStrength(group) && str == 18 &&
		!slices.Contains(noExceptionalStrengthRaces, race)
}

// Specializes reports whether class may specialize in a weapon (ADR 0030
// step 12).
func Specializes(class string) bool { return slices.Contains(specializingClasses, class) }

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
// class group at level, specialized or not. ok is false when the weapon shows
// no rate (step 11.3: no type, or a missile type without a rate of fire), and
// for a group other than the warriors (the rate is a fighter's: Table 15 is
// the warriors').
//
//  1. Melee: Table 15 by level; specialized, Table 35's melee column.
//  2. Missile: the type's rate of fire (Table 45) whatever the level;
//     specialized in a non-bow missile, Table 35's column for it by level; a
//     bow specialist keeps the rate of fire.
func (t *Tables) AttackRateFor(group string, level int, kind, weaponType string, specialized bool) (AttackRate, bool, error) {
	if group != "warrior" || weaponType == "" {
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
