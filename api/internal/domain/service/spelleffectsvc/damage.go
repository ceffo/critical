package spelleffectsvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/pkg/formula"
)

// MaxCasterLevel is the highest caster level a damage entry is checked at
// (the formulas are probed up to the same level).
const MaxCasterLevel = 40

// DiceAt is the formula of one damage instance at a caster level: Dice, or
// the last ByLevel row whose MinLevel is at most level. Nil when no row
// applies.
// A damage made of Parts has no single formula: DiceAt is nil (see
// PartDiceAt).
func (d *Damage) DiceAt(level int) *formula.Formula { return diceAt(d.Dice, d.ByLevel, level) }

// DiceAt is the formula of the part at a caster level, as Damage.DiceAt.
func (p DamagePart) DiceAt(level int) *formula.Formula { return diceAt(p.Dice, p.ByLevel, level) }

func diceAt(dice *formula.Formula, byLevel []LevelDice, level int) *formula.Formula {
	if dice != nil {
		return dice
	}
	var out *formula.Formula
	for _, row := range byLevel {
		if row.MinLevel <= level {
			out = row.Dice
		}
	}
	return out
}

// PartSave is what a successful save does to a damage part: the part's own
// effect, else the entry's, else none (ADR 0008 step 13).
func (e Entry) PartSave(p DamagePart) SaveEffect {
	switch {
	case p.Save != "":
		return p.Save
	case e.Save != nil && e.Save.Effect != "":
		return e.Save.Effect
	default:
		return SaveNone
	}
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
	// Dice is one whole instance; with Parts, the parts added up (their dice
	// groups in the data's order, their constants summed).
	Dice      formula.Sum
	Instances formula.Sum
	Rounds    formula.Sum
	Type      string
	// Parts are the parts of one instance, each rolled on its own (nil: the
	// damage has no parts).
	Parts []DamagePartAt
}

// DamagePartAt is one damage part at a caster level: its dice and what a
// successful save does to it.
type DamagePartAt struct {
	Dice formula.Sum
	Save SaveEffect
}

// ErrUnknownMode is a cast naming a mode its spell does not list (ADR 0008
// step 14).
var ErrUnknownMode = errors.New("unknown spell mode")

// ModeFor is the mode a cast naming key uses: the mode with that key, or the
// first (the default) when key is empty; nil for a spell without modes cast
// without one. A key the spell does not list, or any key for a spell without
// modes, is ErrUnknownMode (ADR 0008 step 14).
func (e Entry) ModeFor(key string) (*Mode, error) {
	if len(e.Modes) == 0 {
		if key != "" {
			return nil, fmt.Errorf("%w: %s has no modes, not %q", ErrUnknownMode, e.Name, key)
		}
		return nil, nil
	}
	if key == "" {
		return &e.Modes[0], nil
	}
	for i := range e.Modes {
		if e.Modes[i].Key == key {
			return &e.Modes[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s has no mode %q", ErrUnknownMode, e.Name, key)
}

// CastDamage is the damage data a cast in mode m rolls: the mode's (nil: the
// mode rolls none), or the entry's for a spell without modes (m nil).
func (e Entry) CastDamage(m *Mode) *Damage {
	if m != nil {
		return m.Damage
	}
	return e.Damage
}

// ModeAt is one cast-time mode at a caster level: its key and the damage a
// cast in this mode rolls (nil: none).
type ModeAt struct {
	Key    string
	Damage *DamageAt
}

// DamageAt returns what a cast of e without a mode rolls for damage at level
// (for a spell with modes, the default mode's), or nil when a cast rolls
// none: not a damage spell, no damage data, or a low-confidence entry (a cast
// shows its text only).
func (e Entry) DamageAt(level int) (*DamageAt, error) {
	m, err := e.ModeFor("")
	if err != nil {
		return nil, err
	}
	return e.ModeDamageAt(m, level)
}

// ModesAt is every cast-time mode of e with the damage a cast in it rolls at
// level, in the data's order (nil: the spell has no modes; ADR 0008 step 14).
func (e Entry) ModesAt(level int) ([]ModeAt, error) {
	var out []ModeAt
	for i := range e.Modes {
		d, err := e.ModeDamageAt(&e.Modes[i], level)
		if err != nil {
			return nil, err
		}
		out = append(out, ModeAt{Key: e.Modes[i].Key, Damage: d})
	}
	return out, nil
}

// ModeDamageAt is the damage a cast of e in mode m (nil: a spell without
// modes) rolls at level; nil when it rolls none.
func (e Entry) ModeDamageAt(m *Mode, level int) (*DamageAt, error) {
	d := e.CastDamage(m)
	if e.Kind != KindDamage || d == nil || e.Confidence == ConfidenceLow {
		return nil, nil
	}
	out := &DamageAt{
		CasterLevel: level, Type: d.Type,
		Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1},
	}
	var err error
	if len(d.Parts) > 0 {
		for i, p := range d.Parts {
			dice := p.DiceAt(level)
			if dice == nil {
				return nil, fmt.Errorf("%s: no damage dice for part %d at caster level %d", e.ID, i, level)
			}
			sum, err := dice.Expand(level)
			if err != nil {
				return nil, fmt.Errorf("%s damage part %d: %w", e.ID, i, err)
			}
			out.Parts = append(out.Parts, DamagePartAt{Dice: sum, Save: e.PartSave(p)})
			out.Dice.Dice = append(out.Dice.Dice, sum.Dice...)
			out.Dice.Constant += sum.Constant
		}
	} else {
		dice := d.DiceAt(level)
		if dice == nil {
			return nil, fmt.Errorf("%s: no damage dice at caster level %d", e.ID, level)
		}
		if out.Dice, err = dice.Expand(level); err != nil {
			return nil, fmt.Errorf("%s damage: %w", e.ID, err)
		}
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
