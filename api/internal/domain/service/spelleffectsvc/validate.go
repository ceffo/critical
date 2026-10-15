package spelleffectsvc

import (
	"fmt"
	"regexp"
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
	if e.Modes != nil && e.Kind != KindDamage {
		return fmt.Errorf("only a damage spell can have modes")
	}
	if e.Exposure != nil && e.Kind != KindDamage {
		return fmt.Errorf("only a damage spell can have an exposure")
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
		if hasBuff || e.Charges != nil {
			return fmt.Errorf("a damage spell cannot have buff fields")
		}
		if e.Exposure != nil {
			if err := e.validateExposure(); err != nil {
				return fmt.Errorf("exposure%w", err)
			}
		}
		if e.Modes != nil {
			return e.validateModes()
		}
		if e.Damage == nil {
			if e.Exposure != nil {
				return nil // ADR 0008 step 16: a cast rolls nothing, exposures do
			}
			return fmt.Errorf("a damage spell needs damage, modes or an exposure")
		}
		if err := e.Damage.validate(); err != nil {
			return fmt.Errorf("damage%w", err)
		}
		if err := e.validateParts(e.Damage); err != nil {
			return fmt.Errorf("damage%w", err)
		}
		return nil
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

// validPartSaves are the save effects a damage part may have: a part exists to
// say exactly what a save does to it (ADR 0008 step 13).
var validPartSaves = []SaveEffect{SaveNone, SaveNegates}

// validate checks a damage: its error starts after the field path (": ..."
// or ".parts[0]..."), which the caller prefixes ("damage", "modes[1].damage").
func (d *Damage) validate() error {
	set := 0
	for _, ok := range []bool{d.Dice != nil, len(d.ByLevel) > 0, d.Parts != nil, d.Steps != nil} {
		if ok {
			set++
		}
	}
	switch {
	case set != 1:
		return fmt.Errorf(": set exactly one of dice, by_level and parts (or steps)")
	case d.Parts != nil && len(d.Parts) == 0:
		return fmt.Errorf(".parts: the list is empty")
	case d.Steps != nil:
		return d.validateSteps()
	}
	if err := validateByLevel(d.ByLevel); err != nil {
		return fmt.Errorf(".%w", err)
	}
	for i, p := range d.Parts {
		if (p.Dice == nil) == (len(p.ByLevel) == 0) {
			return fmt.Errorf(".parts[%d]: set exactly one of dice and by_level", i)
		}
		if err := validateByLevel(p.ByLevel); err != nil {
			return fmt.Errorf(".parts[%d].%w", i, err)
		}
	}
	// The dice are served evaluated at the caster level (DamageAt): each
	// formula must reduce to dice plus a constant at every level.
	for level := 1; level <= MaxCasterLevel; level++ {
		for i, p := range d.Parts {
			if _, err := p.DiceAt(level).Expand(level); err != nil {
				return fmt.Errorf(".parts[%d]: %w", i, err)
			}
		}
		for _, f := range []*formula.Formula{d.DiceAt(level), d.Instances, d.Rounds} {
			if f == nil {
				continue
			}
			if _, err := f.Expand(level); err != nil {
				return fmt.Errorf(": %w", err)
			}
		}
	}
	return nil
}

var validStepPers = []StepPer{StepPerRound, StepPerInstance}

// validateSteps checks a damage made of dice steps (ADR 0008 step 15): steps
// per round or per instance, which replace instances and rounds; exactly one
// of dice (with count) and a non-empty list; at every caster level each step
// reduces to dice plus a constant, count is a number from 1 to
// formula.MaxSteps and at least one step has damage.
func (d *Damage) validateSteps() error {
	st := d.Steps
	switch {
	case d.Instances != nil || d.Rounds != nil:
		return fmt.Errorf(": steps replace instances and rounds, set neither")
	case (st.Dice == nil) == (st.List == nil):
		return fmt.Errorf(".steps: set exactly one of dice and list")
	case st.Dice != nil && st.Count == nil:
		return fmt.Errorf(".steps: dice needs a count")
	case st.List != nil && st.Count != nil:
		return fmt.Errorf(".steps: a list has no count (it is its length)")
	case st.List != nil && len(st.List) == 0:
		return fmt.Errorf(".steps.list: the list is empty")
	case len(st.List) > formula.MaxSteps:
		return fmt.Errorf(".steps.list: more than %d steps", formula.MaxSteps)
	}
	if err := oneOf(st.Per, validStepPers, "per"); err != nil {
		return fmt.Errorf(".steps: %w", err)
	}
	for level := 1; level <= MaxCasterLevel; level++ {
		if _, err := st.At(level); err != nil {
			return fmt.Errorf(".steps: %w", err)
		}
	}
	return nil
}

// validateByLevel checks a by-level table: ascending rows from min_level 1.
// The error names the field from by_level on; the caller prefixes its path.
func validateByLevel(byLevel []LevelDice) error {
	prev := 0
	for i, row := range byLevel {
		if row.MinLevel <= prev {
			return fmt.Errorf("by_level[%d]: min_level %d must be above the previous row", i, row.MinLevel)
		}
		prev = row.MinLevel
	}
	if len(byLevel) > 0 && byLevel[0].MinLevel != 1 {
		return fmt.Errorf("by_level must start at min_level 1")
	}
	return nil
}

// validateParts checks each damage part's save effect, its own or the
// entry's (ADR 0008 step 13). Its error starts after the damage's path, as
// Damage.validate's.
func (e Entry) validateParts(d *Damage) error {
	for i, p := range d.Parts {
		if p.Save != "" {
			if err := oneOf(p.Save, validSaveEffects, "save"); err != nil {
				return fmt.Errorf(".parts[%d]: %w", i, err)
			}
		}
		if err := oneOf(e.PartSave(p), validPartSaves, "save effect"); err != nil {
			return fmt.Errorf(".parts[%d]: %w", i, err)
		}
	}
	return nil
}

// modeKey is the shape of a mode key: snake_case, as spell ids.
var modeKey = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// validateModes checks a spell's cast-time modes (ADR 0008 step 14): at
// least two, keys snake_case and unique, no damage of the spell's own, each
// mode's damage valid (step 8, parts step 13) and at least one mode with
// damage.
func (e Entry) validateModes() error {
	if e.Damage != nil {
		return fmt.Errorf("set damage or modes, not both")
	}
	withDamage, err := e.validateModeList("modes", e.Modes, nil)
	if err != nil {
		return err
	}
	if !withDamage {
		return fmt.Errorf("modes: no mode has damage (a utility spell)")
	}
	return nil
}

// validateModeList checks a list of modes at path ("modes",
// "exposure.modes"): at least two, keys snake_case and unique, each damage
// valid (and passing extra, when set). It reports whether a mode has damage.
func (e Entry) validateModeList(path string, modes []Mode, extra func(*Damage) error) (bool, error) {
	if len(modes) < 2 {
		return false, fmt.Errorf("%s: at least two are needed, got %d", path, len(modes))
	}
	seen := map[string]bool{}
	withDamage := false
	for i, m := range modes {
		if !modeKey.MatchString(m.Key) {
			return false, fmt.Errorf("%s[%d]: key %q is not snake_case", path, i, m.Key)
		}
		if seen[m.Key] {
			return false, fmt.Errorf("%s[%d]: duplicate key %q", path, i, m.Key)
		}
		seen[m.Key] = true
		if m.Damage == nil {
			continue
		}
		withDamage = true
		if err := e.validateDamage(m.Damage, extra); err != nil {
			return false, fmt.Errorf("%s[%d].damage%w", path, i, err)
		}
	}
	return withDamage, nil
}

// validateDamage checks a damage (step 8, parts step 13, steps step 15) and,
// when set, extra; the error starts after the damage's path.
func (e Entry) validateDamage(d *Damage, extra func(*Damage) error) error {
	if err := d.validate(); err != nil {
		return err
	}
	if err := e.validateParts(d); err != nil {
		return err
	}
	if extra != nil {
		return extra(d)
	}
	return nil
}

// validateExposure checks a spell's exposure (ADR 0008 step 16): exactly one
// of damage and modes; the damage, or every mode's, valid as a cast's and of
// one roll per round of exposure (no instances or rounds, steps per round
// only). Its error starts after "exposure".
func (e Entry) validateExposure() error {
	x := e.Exposure
	if (x.Damage == nil) == (x.Modes == nil) {
		return fmt.Errorf(": set exactly one of damage and modes")
	}
	if x.Damage != nil {
		if err := e.validateDamage(x.Damage, oneRoundOfExposure); err != nil {
			return fmt.Errorf(".damage%w", err)
		}
		return nil
	}
	if _, err := e.validateModeList(".modes", x.Modes, oneRoundOfExposure); err != nil {
		return err
	}
	for i, m := range x.Modes {
		if m.Damage == nil {
			return fmt.Errorf(".modes[%d]: an exposure mode needs damage", i)
		}
	}
	return nil
}

// oneRoundOfExposure refuses a damage that is not one roll per round of
// exposure: instances, rounds, or steps per instance.
func oneRoundOfExposure(d *Damage) error {
	switch {
	case d.Instances != nil || d.Rounds != nil:
		return fmt.Errorf(": one roll is one round of exposure, set neither instances nor rounds")
	case d.Steps != nil && d.Steps.Per != StepPerRound:
		return fmt.Errorf(".steps: the steps of an exposure are its rounds, per must be %q", StepPerRound)
	}
	return nil
}
