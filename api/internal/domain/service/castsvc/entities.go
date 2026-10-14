package castsvc

import (
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// ClassGroup is the Bard's class group for by-group effects (PHB: a bard is
// a rogue), e.g. Strength gives a rogue 1d6.
const ClassGroup = "rogue"

// MaxCasterLevel bounds a received spell's caster level (formulas are
// checked up to level 40 when the seed loads).
const MaxCasterLevel = 40

// CastRequest is what the player asks for when casting from a slot.
type CastRequest struct {
	// OnSelf says whether a buff spell affects the Bard. Nil picks the
	// default from the spell's target (see appliesToSelf). Ignored for
	// self-only spells, which always affect the caster.
	OnSelf *bool
	// SpellID names the spell to cast from a blank Rary's extra slot (the
	// player picks it at cast time). Ignored for a prepared slot.
	SpellID string
	// ExtraSlots is the split chosen when casting Rary's Mnemonic Enhancer:
	// the levels of the slots granted (3 levels in all: [1 1 1], [1 2] or
	// [3]). Empty: three level 1 slots.
	ExtraSlots []int
}

// ReceiveRequest is a spell a party member cast on the Bard.
type ReceiveRequest struct {
	SpellID     string
	CasterLevel int
}

// Source says how the spell reached the Bard.
type Source string

// Sources.
const (
	SourceCast     Source = "cast"
	SourceReceived Source = "received"
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
	// Dice is the formula rolled for one instance at this level.
	Dice string
	Type string
	// Instances is the number of separate rolls per round (missiles).
	Instances int
	// Rounds is how many rounds the damage repeats (1: once).
	Rounds int
	// Rolls holds every instance roll, round by round.
	Rolls []Roll
	Total int
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
	// Effect describes the catalog entry (nil: not in the catalog).
	Kind       spelleffectsvc.Kind
	Confidence spelleffectsvc.Confidence
	Target     spelleffectsvc.Target
	Note       string
	// Spell is the spell database row (zero when not found).
	Spell     spellsvc.Spell
	Save      *spelleffectsvc.Save
	TextOnly  TextOnly
	Buff      *AppliedBuff
	Damage    *DamageResult
	Untracked []Untracked
	// Slots is the slot list after a cast.
	Slots []spellsvc.SpellSlot
	// ExtraSlots are the levels of the Rary's extra slots this cast added.
	ExtraSlots []int
}
