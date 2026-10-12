package spelleffectsvc

import "github.com/ceffo/critical/api/pkg/formula"

// Kind classifies what a spell does to the combat/buff engine.
type Kind string

// Spell effect kinds.
const (
	KindBuff    Kind = "buff"    // modifies a creature for a while (includes debuffs)
	KindDamage  Kind = "damage"  // deals hit-point damage
	KindUtility Kind = "utility" // nothing the engine tracks numerically
)

// Confidence says how far the extraction can be trusted.
type Confidence string

// Confidence levels. A low entry always carries a note.
const (
	ConfidenceHigh Confidence = "high"
	ConfidenceLow  Confidence = "low"
)

// DurationKind says how a spell's duration is expressed.
type DurationKind string

// Duration kinds.
const (
	DurationRounds          DurationKind = "rounds"           // Rounds formula; 1 turn = 10 rounds, 1 hour = 60
	DurationInstant         DurationKind = "instant"          // effect happens once
	DurationPermanent       DurationKind = "permanent"        // until dispelled or undone
	DurationUntilDischarged DurationKind = "until_discharged" // ends when its charges are used up
	DurationSpecial         DurationKind = "special"          // see the spell text
)

// Target says who a spell affects.
type Target string

// Targets.
const (
	TargetSelf        Target = "self"
	TargetOther       Target = "other"
	TargetSelfOrOther Target = "self_or_other"
	TargetArea        Target = "area"
	TargetNone        Target = "none"
)

// SaveType is the saving throw category.
type SaveType string

// SaveEffect is what a successful save does.
type SaveEffect string

// Save effects.
const (
	SaveHalf    SaveEffect = "half"
	SaveNegates SaveEffect = "negates"
	SaveNone    SaveEffect = "none"
	SaveSpecial SaveEffect = "special"
)

// Duration is a spell duration. Rounds is set only when Kind is DurationRounds.
type Duration struct {
	Kind   DurationKind
	Rounds *formula.Formula
}

// Save is the saving throw a target gets.
type Save struct {
	Type   SaveType
	Effect SaveEffect
}

// Charges is a pool consumed instead of (or besides) time, e.g. Stoneskin attacks.
type Charges struct {
	Count *formula.Formula
	Unit  string
}

// LevelDice is one row of a by-caster-level damage table.
type LevelDice struct {
	MinLevel int
	Dice     *formula.Formula
}

// Damage describes hit-point damage. Exactly one of Dice and ByLevel is set.
type Damage struct {
	Dice    *formula.Formula
	ByLevel []LevelDice // ascending MinLevel; the last row with MinLevel <= level applies
	// Instances is how many separate damage rolls one casting makes (missiles,
	// bolts). Nil means one.
	Instances *formula.Formula
	// Rounds is how many rounds the damage repeats, rolled every round
	// (Melf's acid). Nil means once.
	Rounds *formula.Formula
	Type   string
}

// AbilityGain raises an ability score. Gain applies to everybody; ByGroup picks
// the die by class group (warrior, priest, rogue, wizard; a bard is a rogue).
type AbilityGain struct {
	Gain       *formula.Formula
	ByGroup    map[string]*formula.Formula
	Max        int
	MaxByGroup map[string]int
}

// Buff is the modifier payload of a buff or debuff. Sign conventions are in
// seeds/spell_effects.schema.json.
type Buff struct {
	Modifiers   map[string]*formula.Formula
	AbilityGain map[string]AbilityGain
	AbilitySet  map[string]string
	Condition   string
}

// Entry is one spell's effect.
type Entry struct {
	ID         string
	Name       string
	Level      int
	Kind       Kind
	Confidence Confidence
	Note       string
	Target     Target
	Duration   Duration
	Charges    *Charges
	Save       *Save
	Damage     *Damage
	Buff       *Buff
}

// String describes the duration for error messages and logs.
func (d Duration) String() string {
	if d.Kind == DurationRounds && d.Rounds != nil {
		return d.Rounds.String() + " rounds"
	}
	return string(d.Kind)
}
