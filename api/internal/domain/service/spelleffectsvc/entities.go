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

// Damage describes hit-point damage. Exactly one of Dice, ByLevel, Parts and
// Steps is set.
type Damage struct {
	Dice    *formula.Formula
	ByLevel []LevelDice // ascending MinLevel; the last row with MinLevel <= level applies
	// Parts split one instance into parts rolled separately, each with its
	// own save effect, so the GM can drop the part a save negates (ADR 0008
	// step 13). The instance is the parts added up, in the data's order.
	Parts []DamagePart
	// Steps are dice that change from one round, or one instance, to the
	// next (ADR 0008 step 15). With Steps, Instances and Rounds are nil: the
	// steps are the rounds or the instances.
	Steps *DamageSteps
	// Instances is how many separate damage rolls one casting makes (missiles,
	// bolts). Nil means one.
	Instances *formula.Formula
	// Rounds is how many rounds the damage repeats, rolled every round
	// (Melf's acid). Nil means once.
	Rounds *formula.Formula
	Type   string
}

// StepPer says what one dice step of a damage is.
type StepPer string

// Dice step units.
const (
	StepPerRound    StepPer = "round"    // one round of a damage that lasts (Vitriolic Sphere)
	StepPerInstance StepPer = "instance" // one separate roll of one casting (a Chain Lightning strike)
)

// DamageSteps are the dice of a damage whose rounds, or instances, differ
// (ADR 0008 step 15). Exactly one of Dice and List is set: Dice is the formula
// of every step, its step variable the step's number (1-based), for Count
// steps at most; List is the formula of each step, in order. A step may have
// no damage (a round before a cloud flames); the steps after the last one
// with damage are dropped (the damage has run out).
type DamageSteps struct {
	Per   StepPer
	Dice  *formula.Formula
	Count *formula.Formula
	List  []*formula.Formula
}

// DamagePart is one part of a damage instance: its dice (exactly one of Dice
// and ByLevel, as for Damage) and what a successful save does to it (none or
// negates; ADR 0008 step 13).
type DamagePart struct {
	Dice    *formula.Formula
	ByLevel []LevelDice
	Save    SaveEffect
}

// Mode is one cast-time option of a spell: a key unique in the spell
// (snake_case, worded by the UI) and the damage a cast in this mode rolls
// (nil: none). The damage uses the entry's save (ADR 0008 step 14).
type Mode struct {
	Key    string
	Damage *Damage
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

// GrantSlots is a cast mechanic: casting the spell adds blank extra spell
// slots worth Levels spell levels in all, split by the caster at cast time
// (ADR 0008 step 9). An extra slot is blank until cast and goes at the long
// rest.
type GrantSlots struct {
	Levels int
}

// ConsumeItem is a cast mechanic: casting the spell spends one unit of an
// inventory item of type Type, chosen by the caster when several are left
// (ADR 0008 step 10). Daily items are never spent.
type ConsumeItem struct {
	Type string
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
	// Modes are the options the caster picks from when casting, each with its
	// own damage (or none); the first is the default (ADR 0008 step 14). A
	// spell with modes has no Damage of its own.
	Modes []Mode
	Buff  *Buff
	// Cast mechanics, applied when the Bard casts the spell (not when it is
	// received from someone else): GrantSlots adds extra slots, ConsumeItem
	// spends an inventory item and AttackRoll rolls a spell to-hit (one d20,
	// never a crit, ADR 0002 Consequences) against a target AC given at cast.
	GrantSlots  *GrantSlots
	ConsumeItem *ConsumeItem
	AttackRoll  bool
}

// SlotSplits lists every way the caster may split g.Levels spell levels into
// extra slots: each split is ascending, and the first is all level 1 slots
// (the default). 3 levels: [1 1 1], [1 2], [3].
func (g GrantSlots) SlotSplits() [][]int {
	var out [][]int
	var walk func(left, lowest int, acc []int)
	walk = func(left, lowest int, acc []int) {
		if left == 0 {
			out = append(out, append([]int(nil), acc...))
			return
		}
		for l := lowest; l <= left; l++ {
			walk(left-l, l, append(acc, l))
		}
	}
	walk(g.Levels, 1, nil)
	return out
}

// String describes the duration for error messages and logs.
func (d Duration) String() string {
	if d.Kind == DurationRounds && d.Rounds != nil {
		return d.Rounds.String() + " rounds"
	}
	return string(d.Kind)
}
