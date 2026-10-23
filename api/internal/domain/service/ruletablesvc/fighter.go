package ruletablesvc

import (
	"fmt"

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

// specialization is the weapon_specialization params of def, nil when it does not list the kind
// (ADR 0040 step 13: a class's costs and columns are its own parameters, never a constant).
func specialization(def classes.Class) *classes.WeaponSpecialization {
	f, ok := def.Lists(classes.KindWeaponSpecialization)
	if !ok {
		return nil
	}
	return f.Specialization
}

// SpecializationSlots is what a specialization pick of weaponType spends in all for a character of
// class def (ADR 0037 step 9, ADR 0040 step 13): the cost its weapon_specialization params give
// the type (PHB p.53, "Cost of Specialization"), 0 when the class does not specialize.
func SpecializationSlots(def classes.Class, weaponType string) int {
	w := specialization(def)
	if w == nil {
		return 0
	}
	return w.SlotsFor(weaponType)
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
//     specialized, the Table 35 column the class's weapon_specialization
//     params name for the type, by level; a type whose row says rate_of_fire
//     (the bows) keeps the rate of fire.
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
	spec := specialization(def)
	if !specialized || spec == nil {
		spec = nil // a class that does not specialize has no specialist rate
	}
	switch kind {
	case weaponMelee:
		if spec != nil {
			column, _ := spec.ColumnFor(true, weaponType)
			return specialistRate(c, column, level)
		}
		for _, r := range c.WarriorAttacks {
			if level >= r.LevelMin && level <= r.LevelMax {
				return AttackRate{Attacks: r.Attacks, Rounds: r.Rounds}, true, nil
			}
		}
		return AttackRate{}, false, fmt.Errorf("warrior attacks: level %d outside the table", level)
	case weaponMissile:
		if spec != nil {
			if column, keeps := spec.ColumnFor(false, weaponType); !keeps {
				return specialistRate(c, column, level)
			}
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
