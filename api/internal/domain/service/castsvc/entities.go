package castsvc

import (
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// ClassGroup is the Bard's class group for by-group effects (PHB: a bard is
// a rogue), e.g. Strength gives a rogue 1d6.
const ClassGroup = "rogue"

// MaxCasterLevel bounds a received spell's caster level (formulas are
// checked up to this level when the seed loads).
const MaxCasterLevel = spelleffectsvc.MaxCasterLevel

// CastRequest is what the player asks for when casting from a slot.
type CastRequest struct {
	// OnSelf says whether a buff spell affects the Bard. Nil picks the
	// default from the spell's target (see appliesToSelf). Ignored for
	// self-only spells, which always affect the caster.
	OnSelf *bool
	// SpellID names the spell to cast from a blank extra slot (the player
	// picks it at cast time). Ignored for a prepared slot.
	SpellID string
	// ExtraSlots is the split chosen when casting a spell that grants extra
	// slots: the levels of the slots granted, adding up to the spell's
	// levels (3 levels: [1 1 1], [1 2] or [3]). Empty: all level 1 slots.
	ExtraSlots []int
	// ItemID is the inventory item a spell that consumes an item spends.
	// Empty: the only one of the item type left, if there is exactly one.
	ItemID string
	// TargetAC is the target's AC, required for a spell with an attack roll.
	TargetAC *int
	// Mode is the key of the cast-time mode picked for a spell with modes.
	// Empty: the first (ADR 0008 step 14).
	Mode string
}

// ReceiveRequest is a spell a party member cast on the Bard.
type ReceiveRequest struct {
	SpellID     string
	CasterLevel int
	// Mode is the cast-time mode the caster picked (empty: the first).
	Mode string
}

// AddEffectRequest is a spell the player adds as an effect on the Bard from
// the spell lookup, at the Bard's level (ADR 0004 step 19).
type AddEffectRequest struct {
	SpellID string
}

// Source says how the spell reached the Bard.
type Source string

// Sources.
const (
	SourceCast     Source = "cast"
	SourceReceived Source = "received"
	// SourceEffect is a spell added as an effect from the lookup: no slot,
	// no cast mechanics, no damage (ADR 0004 step 19).
	SourceEffect Source = "effect"
)

// TextOnly says why a result carries no tracked effect (empty when it does).
type TextOnly string

// Reasons a spell shows only its text.
const (
	TextOnlyNone          TextOnly = ""
	TextOnlyNoEffect      TextOnly = "no_effect"      // not in the spell-effects catalog (e.g. a priest spell)
	TextOnlyLowConfidence TextOnly = "low_confidence" // the extraction is not trusted: add a manual buff
	TextOnlyUtility       TextOnly = "utility"        // nothing the engine tracks numerically
	TextOnlyNotOnSelf     TextOnly = "not_on_self"    // a buff on somebody else
	TextOnlyByHand        TextOnly = "by_hand"        // sets an ability for one body part only (Fist of Stone)
)

// DieRoll is one die thrown.
type DieRoll struct {
	Sides int
	Value int
}

// Roll is one evaluated formula: the formula, every die it threw, and the
// result.
type Roll struct {
	Formula string
	Dice    []DieRoll
	Total   int
	// Parts are the rolls of a damage instance's parts, in the order of
	// DamageResult.Parts; the instance's Dice and Total add them up (nil: the
	// damage has no parts).
	Parts []Roll
	// Step is the dice step (1-based round or instance) this roll is of
	// (0: the damage has no steps; ADR 0008 step 15).
	Step int
}

// Untracked is an effect the engine does not apply (movement, attacks per
// round, initiative, set AC, attackers' penalties, conditional bonuses,
// other abilities...), shown so the player can apply it by hand.
type Untracked struct {
	// Key is the seed's modifier key ("attacks_pct") or "<ability>_gain" /
	// "<ability>_set" ("dex_gain").
	Key   string
	Value int
	// Condition is when it applies, if not always (from the seed).
	Condition string
}

// AppliedBuff is the buff put on the Bard and the rolls behind it.
type AppliedBuff struct {
	Buff buffsvc.BuffEffect
	// Applied is false when the buff went on somebody else (the rolls are
	// still shown, e.g. Stoneskin layers to tell the ally).
	Applied bool
	// OnOther: a row marked as on someone else or on an area went on the
	// list for it (ADR 0004 step 20.5); Buff.ID is the row's.
	OnOther bool
	// Duration is the rounds roll (nil: not timed).
	Duration *Roll
	// Charges is the charges roll (nil: no charges) and ChargeUnit what one
	// charge is (attacks, images).
	Charges    *Roll
	ChargeUnit string
	// StrGain is the STR gain roll of a Strength-type spell (nil: none).
	StrGain *Roll
}

// DamageResult is a damage spell rolled at the caster level; saves are the
// DM's side.
type DamageResult struct {
	// Dice is the formula rolled for one instance at this level (empty for a
	// damage made of steps: each roll has its own).
	Dice string
	Type string
	// Instances is the number of separate rolls per round (missiles).
	Instances int
	// Rounds is how many rounds the damage repeats (1: once).
	Rounds int
	// Rolls holds every instance roll, round by round.
	Rolls []Roll
	Total int
	// Parts are the parts of one instance with their save effect (nil: the
	// damage has no parts; ADR 0008 step 13).
	Parts []DamagePart
	// TotalOnSave is Total without the parts a save negates, for a target
	// that saves against every instance (nil: the damage has no parts).
	TotalOnSave *int
	// StepsPer is set for a damage made of dice steps: each roll is one
	// round, or one instance, of its own dice, numbered by Roll.Step; Dice
	// is then empty (ADR 0008 step 15).
	StepsPer spelleffectsvc.StepPer
}

// DamagePart is one part of a damage instance: the formula it rolls and what
// a successful save does to it (none or negates).
type DamagePart struct {
	Formula string
	Save    spelleffectsvc.SaveEffect
}

// CastResult is everything a cast or receive did.
type CastResult struct {
	EntryID     int64
	Source      Source
	SpellID     string
	Name        string
	Level       int
	CasterLevel int
	// SlotIndex is the slot spent (cast only).
	SlotIndex *int
	// Mode is the key of the cast-time mode the spell was cast in (empty:
	// the spell has no modes; ADR 0008 step 14).
	Mode string
	// Effect describes the catalog entry (nil: not in the catalog).
	Kind       spelleffectsvc.Kind
	Confidence spelleffectsvc.Confidence
	Target     spelleffectsvc.Target
	// Spell is the spell database row (zero when not found).
	Spell     spellsvc.Spell
	Save      *spelleffectsvc.Save
	TextOnly  TextOnly
	Buff      *AppliedBuff
	Damage    *DamageResult
	Untracked []Untracked
	// Slots is the slot list after a cast.
	Slots []spellsvc.SpellSlot
	// ExtraSlots are the levels of the extra slots this cast added (a spell
	// that grants slots).
	ExtraSlots []int
	// Consumed is the inventory item the cast spent (nil: none).
	Consumed *ConsumedItem
	// Attack is the spell's to-hit roll (nil: the spell has no attack roll).
	// The app shows hit or miss; what a hit does is the DM's call.
	Attack *combatsvc.ToHitRoll
	// Exposure is the spell's exposure damage at the caster level, for the
	// player to roll each round of exposure from (nil: none; ADR 0008 step
	// 16).
	Exposure *spelleffectsvc.ExposureAt
	// Failure is the cast's spell failure roll (nil: no die, ADR 0030 step
	// 14). A failed spell spends its slot and applies nothing: the result
	// then carries only the spell, the slot and the roll.
	Failure *SpellFailure
}

// SpellFailure is one d100 against the caster's chance of spell failure
// (ADR 0030 step 14): Failed when Roll is at or under Chance.
type SpellFailure struct {
	Roll, Chance int
	Failed       bool
}

// ExposureRequest is one round of exposure to a spell's area (ADR 0008
// step 16).
type ExposureRequest struct {
	SpellID     string
	CasterLevel int
	// Mode is the key of the exposure mode (empty: the first).
	Mode string
	// Round is the round of exposure, from 1 (0: the first).
	Round int
	// OnSelf: the Bard is the one exposed; the damage comes off his HP.
	OnSelf bool
}

// ExposureResult is one round of exposure rolled.
type ExposureResult struct {
	EntryID     int64
	SpellID     string
	Name        string
	CasterLevel int
	// Mode is the key of the exposure mode rolled (empty: none).
	Mode  string
	Round int
	// OnSelf: the Bard was the one exposed and the entry took Damage.Total
	// off his HP (down to the floor of ADR 0011 step 1).
	OnSelf bool
	// HPDelta is the HP change the ledger applied: -Damage.Total, or less
	// when the floor cut it; 0 when the Bard was not exposed.
	HPDelta int
	Damage  *DamageResult
	// Exposure is the spell's exposure damage at the caster level, for the
	// next round of exposure.
	Exposure *spelleffectsvc.ExposureAt
}

// ConsumedItem is one unit of an inventory item a cast spent.
type ConsumedItem struct {
	ItemID string
	Name   string
	// Remaining is the count of that item left.
	Remaining int
}
