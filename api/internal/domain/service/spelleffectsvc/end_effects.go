package spelleffectsvc

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/pkg/formula"
)

// EndCause is how a spell's row ends (ADR 0004 step 21).
type EndCause string

// End causes.
const (
	// EndExpired: the spell runs out (Next Round or passing time takes its
	// last round).
	EndExpired EndCause = "expired"
	// EndVoluntary: the player ends it ("ended by me").
	EndVoluntary EndCause = "voluntary"
	// EndDispelled: it is dispelled, or its bearer is slain.
	EndDispelled EndCause = "dispelled"
)

// EndKind is what an end effect does.
type EndKind string

// End effect kinds.
const (
	// EndHeal: the bearer regains hit points, the dice rolled.
	EndHeal EndKind = "heal"
)

var (
	validEndCauses = []EndCause{EndExpired, EndVoluntary, EndDispelled}
	validEndKinds  = []EndKind{EndHeal}
	// PlayerEndCauses are the causes the player picks when he stops a row
	// whose spell has end effects; running out is never his choice.
	PlayerEndCauses = []EndCause{EndVoluntary, EndDispelled}
)

// ErrEndCauseRequired is a row stopped without a cause when its spell has
// end effects (ADR 0004 step 21).
var ErrEndCauseRequired = errors.New("the spell has end effects: say how it ends")

// EndEffect is an effect that applies when the spell's row ends by one of
// the causes in On (ADR 0004 step 21). Dice is a sum of dice plus a
// constant that does not read the caster level.
type EndEffect struct {
	Kind EndKind
	Dice *formula.Formula
	On   []EndCause
}

// AsksEndCause reports whether stopping the spell's row asks how it ends:
// the spell has at least one end effect.
func (e Entry) AsksEndCause() bool { return len(e.EndEffects) > 0 }

// EndEffectsOn is the spell's end effects that a cause triggers, in the
// data's order.
func (e Entry) EndEffectsOn(cause EndCause) []EndEffect {
	var out []EndEffect
	for _, x := range e.EndEffects {
		if slices.Contains(x.On, cause) {
			out = append(out, x)
		}
	}
	return out
}

// ParseEndCause maps a cause the player sent; "" is no cause. Running out
// and anything unknown are refused (domain.ErrInvalidInput by the caller).
func ParseEndCause(s string) (EndCause, error) {
	c := EndCause(s)
	if s == "" || slices.Contains(PlayerEndCauses, c) {
		return c, nil
	}
	return "", fmt.Errorf("end cause %q is not one of %v", s, PlayerEndCauses)
}

// validateEndEffects checks the end effects of an entry.
func (e Entry) validateEndEffects() error {
	if e.EndEffects != nil && len(e.EndEffects) == 0 {
		return fmt.Errorf("end_effects: empty list")
	}
	for i, x := range e.EndEffects {
		field := fmt.Sprintf("end_effects[%d]", i)
		if err := oneOf(x.Kind, validEndKinds, field+".kind"); err != nil {
			return err
		}
		if len(x.On) == 0 {
			return fmt.Errorf("%s.on: at least one cause", field)
		}
		for j, c := range x.On {
			if err := oneOf(c, validEndCauses, field+".on"); err != nil {
				return err
			}
			if slices.Contains(x.On[:j], c) {
				return fmt.Errorf("%s.on: %s twice", field, c)
			}
		}
		if x.Dice == nil {
			return fmt.Errorf("%s.dice is required", field)
		}
		if x.Dice.UsesLevel() {
			return fmt.Errorf("%s.dice %s: may not use level (the row keeps no caster level)", field, x.Dice)
		}
		if _, err := x.Dice.Expand(0); err != nil {
			return fmt.Errorf("%s.dice: %w", field, err)
		}
	}
	return nil
}
