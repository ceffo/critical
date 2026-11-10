package spelleffectsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/pkg/formula"
)

// Heal is the hit points a heal spell gives back at once, when it is cast or
// received (ADR 0008 step 17): exactly one of Dice (a sum of dice plus a
// constant, which may read the caster level, never below 0) and Full (every
// lost hit point, nothing rolled).
type Heal struct {
	Dice *formula.Formula
	Full bool
}

// Heals reports whether a cast or a receive of the spell heals (ADR 0008
// step 17): a heal entry the extraction trusts.
func (e Entry) Heals() bool {
	return e.Kind == KindHeal && e.Heal != nil && e.Confidence != ConfidenceLow
}

// healDurations are the durations of a heal entry: none puts a row (ADR 0004
// step 20.2), the heal is instant.
var healDurations = []DurationKind{DurationInstant, DurationPermanent}

// validateHeal checks a heal entry (ADR 0008 step 17.1): a heal block with
// exactly one of dice and full, dice that reduce to dice plus a constant and
// never go below 0 at any caster level from 1 to MaxCasterLevel, a duration
// that puts no row, and no other block (damage, modes and exposure are
// refused for every kind but damage, before this).
func (e Entry) validateHeal(hasBuff bool) error {
	h := e.Heal
	switch {
	case h == nil:
		return fmt.Errorf("a heal spell needs a heal block")
	case (h.Dice != nil) == h.Full:
		return fmt.Errorf("heal: set exactly one of dice and full")
	case hasBuff || e.Charges != nil || e.Damage != nil:
		return fmt.Errorf("a heal spell cannot have buff, charges or damage fields")
	case e.GrantSlots != nil || e.ConsumeItem != nil || e.AttackRoll:
		return fmt.Errorf("a heal spell cannot have cast mechanics")
	case e.EndEffects != nil:
		return fmt.Errorf("a heal spell cannot have end effects")
	}
	if err := oneOf(e.Duration.Kind, healDurations, "a heal spell's duration.kind"); err != nil {
		return err
	}
	if h.Dice == nil {
		return nil
	}
	for level := 1; level <= MaxCasterLevel; level++ {
		if _, err := h.Dice.Expand(level); err != nil {
			return fmt.Errorf("heal.dice: %w", err)
		}
		lo, err := h.Dice.Min(level)
		if err != nil {
			return fmt.Errorf("heal.dice: %w", err)
		}
		if lo < 0 {
			return fmt.Errorf("heal.dice %s: goes below 0 at level %d", h.Dice, level)
		}
	}
	return nil
}
