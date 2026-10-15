package spelleffectsvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/pkg/formula"
)

// ErrNoExposure is an exposure roll for a spell that has no exposure damage
// a roll can use: none in its data, a low-confidence entry, or no entry at
// all (ADR 0008 step 16).
var ErrNoExposure = errors.New("spell has no exposure damage")

// MaxExposureRound is the highest round of exposure a roll may name.
const MaxExposureRound = formula.MaxSteps

// ExposureAt is the exposure damage of a spell at a caster level, evaluated
// down to the dice left to roll (ADR 0008 step 16): Damage, or the Modes to
// pick from at each exposure, each with its damage.
type ExposureAt struct {
	Damage *DamageAt
	Modes  []ModeAt
}

// ExposureAt is the exposure damage of e at level; nil when a roll uses none
// (no exposure, not a damage spell, or a low-confidence entry).
func (e Entry) ExposureAt(level int) (*ExposureAt, error) {
	x := e.Exposure
	if x == nil || e.Kind != KindDamage || e.Confidence == ConfidenceLow {
		return nil, nil
	}
	out := &ExposureAt{}
	var err error
	if x.Damage != nil {
		if out.Damage, err = e.damageAt(x.Damage, level); err != nil {
			return nil, fmt.Errorf("exposure: %w", err)
		}
		return out, nil
	}
	for i := range x.Modes {
		d, err := e.damageAt(x.Modes[i].Damage, level)
		if err != nil {
			return nil, fmt.Errorf("exposure mode %s: %w", x.Modes[i].Key, err)
		}
		out.Modes = append(out.Modes, ModeAt{Key: x.Modes[i].Key, Damage: d})
	}
	return out, nil
}

// ExposureFor is the damage data one exposure in mode key rolls, and the
// key of the mode used (empty: the exposure has no modes). An exposure with
// modes uses the first when key is empty; a key it does not list, or any key
// for an exposure without modes, is ErrUnknownMode. A spell a roll can use
// no exposure of (see ExposureAt) is ErrNoExposure.
func (e Entry) ExposureFor(key string) (*Damage, string, error) {
	x := e.Exposure
	if x == nil || e.Kind != KindDamage || e.Confidence == ConfidenceLow {
		return nil, "", fmt.Errorf("%w: %s", ErrNoExposure, e.ID)
	}
	if len(x.Modes) == 0 {
		if key != "" {
			return nil, "", fmt.Errorf("%w: %s exposure has no modes, not %q", ErrUnknownMode, e.Name, key)
		}
		return x.Damage, "", nil
	}
	if key == "" {
		return x.Modes[0].Damage, x.Modes[0].Key, nil
	}
	for _, m := range x.Modes {
		if m.Key == key {
			return m.Damage, m.Key, nil
		}
	}
	return nil, "", fmt.Errorf("%w: %s exposure has no mode %q", ErrUnknownMode, e.Name, key)
}

// ExposureStep is the dice step a round of exposure (1-based) rolls for a
// damage made of steps (ADR 0008 step 16): the step of that round, or the
// last one for a later round. ok is false for a damage without steps (every
// round rolls the same dice).
func (d *Damage) ExposureStep(level, round int) (step int, ok bool, err error) {
	if round < 1 {
		return 0, false, fmt.Errorf("round of exposure %d: rounds start at 1", round)
	}
	if d.Steps == nil {
		return 0, false, nil
	}
	steps, err := d.Steps.At(level)
	if err != nil {
		return 0, false, err
	}
	return steps[min(round, len(steps))-1].Step, true, nil
}
