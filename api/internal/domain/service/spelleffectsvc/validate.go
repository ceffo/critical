package spelleffectsvc

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/pkg/formula"
)

var (
	validKinds       = []Kind{KindBuff, KindDamage, KindUtility}
	validConfidences = []Confidence{ConfidenceHigh, ConfidenceLow}
	// validSaveTypes mirror the AD&D 2e saving throw categories, plus "none".
	validSaveTypes   = []SaveType{"none", "spell", "poison", "paralyzation", "death", "petrification", "breath", "rod_staff_wand", "polymorph"}
	validSaveEffects = []SaveEffect{SaveHalf, SaveNegates, SaveNone, SaveSpecial}
	validTargets     = []Target{TargetSelf, TargetOther, TargetSelfOrOther, TargetArea, TargetNone}
	validDurations   = []DurationKind{DurationRounds, DurationInstant, DurationPermanent, DurationUntilDischarged, DurationSpecial}
	validAbilities   = []string{"str", "dex", "con", "int", "wis", "cha"}
	validGroups      = []string{"warrior", "priest", "rogue", "wizard"}
	// validModifiers are the scalar buff keys, each a formula.
	validModifiers = []string{
		"to_hit", "damage", "ac", "ac_set", "ac_set_thrown", "ac_set_missile", "ac_set_other",
		"save", "attackers_to_hit", "attackers_to_hit_first", "initiative",
		"move_pct", "attacks_pct", "damage_multiplier_pct", "weapon_buff",
		"regen_hp_per_round",
	}
)

// Slug is the interim spell id: the snake_case of the English spell name.
// Lowercase, apostrophes dropped, every other run of non-alphanumerics becomes
// one underscore ("Melf's Acid Arrow" -> "melfs_acid_arrow").
func Slug(name string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r == '\'' || r == '’' || r == '*':
			// dropped, does not split words
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
		default:
			pendingSep = true
		}
	}
	return b.String()
}

func oneOf[T comparable](v T, set []T, what string) error {
	if !slices.Contains(set, v) {
		return fmt.Errorf("%s %v is not one of %v", what, v, set)
	}
	return nil
}

// Validate checks the invariants of an entry.
func (e Entry) Validate() error {
	if e.ID == "" || e.Name == "" {
		return fmt.Errorf("id and name are required")
	}
	if want := Slug(e.Name); e.ID != want {
		return fmt.Errorf("id must be the slug of the name %q (%q)", e.Name, want)
	}
	if e.Level < 1 || e.Level > 9 {
		return fmt.Errorf("level %d out of range 1..9", e.Level)
	}
	if err := oneOf(e.Kind, validKinds, "kind"); err != nil {
		return err
	}
	if err := oneOf(e.Confidence, validConfidences, "confidence"); err != nil {
		return err
	}
	if e.Confidence == ConfidenceLow && strings.TrimSpace(e.Note) == "" {
		return fmt.Errorf("a low-confidence entry needs a note")
	}
	if err := oneOf(e.Target, validTargets, "target"); err != nil {
		return err
	}
	if err := e.validateDuration(); err != nil {
		return err
	}
	if e.Save != nil {
		if err := oneOf(e.Save.Type, validSaveTypes, "save.type"); err != nil {
			return err
		}
		if err := oneOf(e.Save.Effect, validSaveEffects, "save.effect"); err != nil {
			return err
		}
	}
	if e.Charges != nil && strings.TrimSpace(e.Charges.Unit) == "" {
		return fmt.Errorf("charges.unit is required")
	}
	if e.GrantSlots != nil && (e.GrantSlots.Levels < 1 || e.GrantSlots.Levels > 9) {
		return fmt.Errorf("grant_slots.levels %d out of range 1..9", e.GrantSlots.Levels)
	}
	if e.ConsumeItem != nil && strings.TrimSpace(e.ConsumeItem.Type) == "" {
		return fmt.Errorf("consume_item.type is required")
	}
	hasBuff := e.Buff != nil && (len(e.Buff.Modifiers) > 0 || len(e.Buff.AbilityGain) > 0 || len(e.Buff.AbilitySet) > 0)
	switch e.Kind {
	case KindBuff:
		if !hasBuff && e.Charges == nil {
			return fmt.Errorf("a buff needs modifiers, ability_gain, ability_set or charges")
		}
		if e.Damage != nil {
			return fmt.Errorf("a buff cannot have damage")
		}
		return e.Buff.validate()
	case KindDamage:
		if e.Damage == nil {
			return fmt.Errorf("a damage spell needs damage")
		}
		if hasBuff || e.Charges != nil {
			return fmt.Errorf("a damage spell cannot have buff fields")
		}
		return e.Damage.validate()
	default: // utility
		if hasBuff || e.Charges != nil || e.Damage != nil {
			return fmt.Errorf("a utility spell cannot have buff, charges or damage fields")
		}
	}
	return nil
}

func (e Entry) validateDuration() error {
	d := e.Duration
	if err := oneOf(d.Kind, validDurations, "duration.kind"); err != nil {
		return err
	}
	switch {
	case d.Kind == DurationRounds && d.Rounds == nil:
		return fmt.Errorf("duration.rounds is required with kind %q", DurationRounds)
	case d.Kind != DurationRounds && d.Rounds != nil:
		return fmt.Errorf("duration.rounds is only allowed with kind %q", DurationRounds)
	}
	return nil
}

func (b *Buff) validate() error {
	if b == nil {
		return nil
	}
	for k := range b.Modifiers {
		if err := oneOf(k, validModifiers, "modifiers key"); err != nil {
			return err
		}
	}
	for ab, g := range b.AbilityGain {
		if err := oneOf(ab, validAbilities, "ability_gain key"); err != nil {
			return err
		}
		if err := g.validate("ability_gain." + ab); err != nil {
			return err
		}
	}
	for ab, v := range b.AbilitySet {
		if err := oneOf(ab, validAbilities, "ability_set key"); err != nil {
			return err
		}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("ability_set.%s is empty", ab)
		}
	}
	return nil
}

func (g AbilityGain) validate(field string) error {
	if (g.Gain == nil) == (len(g.ByGroup) == 0) {
		return fmt.Errorf("%s: set exactly one of gain and by_group", field)
	}
	for grp := range g.ByGroup {
		if err := oneOf(grp, validGroups, field+".by_group key"); err != nil {
			return err
		}
	}
	for grp, m := range g.MaxByGroup {
		if err := oneOf(grp, validGroups, field+".max_by_group key"); err != nil {
			return err
		}
		if m < 1 || m > 30 {
			return fmt.Errorf("%s.max_by_group.%s: %d out of range", field, grp, m)
		}
	}
	if g.Max < 0 || g.Max > 30 {
		return fmt.Errorf("%s.max: %d out of range", field, g.Max)
	}
	return nil
}

func (d *Damage) validate() error {
	if (d.Dice == nil) == (len(d.ByLevel) == 0) {
		return fmt.Errorf("damage: set exactly one of dice and by_level")
	}
	prev := 0
	for i, row := range d.ByLevel {
		if row.MinLevel <= prev {
			return fmt.Errorf("damage.by_level[%d]: min_level %d must be above the previous row", i, row.MinLevel)
		}
		prev = row.MinLevel
	}
	if len(d.ByLevel) > 0 && d.ByLevel[0].MinLevel != 1 {
		return fmt.Errorf("damage.by_level must start at min_level 1")
	}
	// The dice are served evaluated at the caster level (DamageAt): each
	// formula must reduce to dice plus a constant at every level.
	for level := 1; level <= MaxCasterLevel; level++ {
		for _, f := range []*formula.Formula{d.DiceAt(level), d.Instances, d.Rounds} {
			if f == nil {
				continue
			}
			if _, err := f.Expand(level); err != nil {
				return fmt.Errorf("damage: %w", err)
			}
		}
	}
	return nil
}
