package spelleffectsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/pkg/formula"
)

// MaxCasterLevel is the highest caster level a damage entry is checked at
// (the formulas are probed up to the same level).
const MaxCasterLevel = 40

// DiceAt is the formula of one damage instance at a caster level: Dice, or
// the last ByLevel row whose MinLevel is at most level. Nil when no row
// applies.
func (d *Damage) DiceAt(level int) *formula.Formula {
	if d.Dice != nil {
		return d.Dice
	}
	var out *formula.Formula
	for _, row := range d.ByLevel {
		if row.MinLevel <= level {
			out = row.Dice
		}
	}
	return out
}

// InstanceCount is the number of damage instances a rolled Instances
// formula gives: never below zero.
func InstanceCount(n int) int { return max(n, 0) }

// RoundCount is the number of rounds a rolled Rounds formula gives: at
// least one.
func RoundCount(n int) int { return max(n, 1) }

// DamageAt is the damage a cast rolls at one caster level, evaluated down to
// the dice left to roll: Dice for one instance, Instances separate instances
// every round, for Rounds rounds. Instances and Rounds are constants unless
// their formula rolls dice (then the cast rolls them).
type DamageAt struct {
	CasterLevel int
	Dice        formula.Sum
	Instances   formula.Sum
	Rounds      formula.Sum
	Type        string
}

// DamageAt returns what a cast of e rolls for damage at level, or nil when a
// cast rolls none: not a damage spell, no damage data, or a low-confidence
// entry (a cast shows its text only).
func (e Entry) DamageAt(level int) (*DamageAt, error) {
	if e.Kind != KindDamage || e.Damage == nil || e.Confidence == ConfidenceLow {
		return nil, nil
	}
	d := e.Damage
	dice := d.DiceAt(level)
	if dice == nil {
		return nil, fmt.Errorf("%s: no damage dice at caster level %d", e.ID, level)
	}
	out := &DamageAt{
		CasterLevel: level, Type: d.Type,
		Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1},
	}
	var err error
	if out.Dice, err = dice.Expand(level); err != nil {
		return nil, fmt.Errorf("%s damage: %w", e.ID, err)
	}
	if d.Instances != nil {
		if out.Instances, err = d.Instances.Expand(level); err != nil {
			return nil, fmt.Errorf("%s instances: %w", e.ID, err)
		}
		if len(out.Instances.Dice) == 0 {
			out.Instances.Constant = InstanceCount(out.Instances.Constant)
		}
	}
	if d.Rounds != nil {
		if out.Rounds, err = d.Rounds.Expand(level); err != nil {
			return nil, fmt.Errorf("%s rounds: %w", e.ID, err)
		}
		if len(out.Rounds.Dice) == 0 {
			out.Rounds.Constant = RoundCount(out.Rounds.Constant)
		}
	}
	return out, nil
}
