package buffsvc

import "time"

// State keys under which buffsvc snapshots its state in the action log. Each
// has a Restorer (see Restorers) that the composition root registers.
const (
	StateKeyBuffs = "buffs"
	StateKeyRound = "round"
	// StateKeySavedBuffs is the whole saved-buff template list.
	StateKeySavedBuffs = "saved_buffs"
	// StateKeyInitiative is the round's initiative (ADR 0013), JSON null when
	// there is none.
	StateKeyInitiative = "initiative"
)

// InitiativeDie is the die initiative is rolled with (ADR 0013 step 1).
const InitiativeDie = 10

// Initiative is the round's initiative roll (ADR 0013): the plain d10,
// lowest acts first, and the action-log entry that rolled it. There is
// none out of combat (round 0).
type Initiative struct {
	D10        int
	LogEntryID int64
}

// BuffEffects is the modifier payload of one buff. Pointers are used so a
// missing field means "no contribution" rather than zero.
//
// STR effects (StrBonus, StrMax) feed the effective STR score; the
// other effects are gated by MinStr against that effective score.
type BuffEffects struct {
	// MinStr gates the buff: it contributes only while effective STR >= MinStr
	// (the Power Gem needs STR 18 or more).
	MinStr     *int
	SpellsBuff *float64
	// WeaponBuff makes the wielded weapon count as +N (Power Gem: "the
	// weapon becomes +5"), to hit and damage. It replaces the weapon's own
	// magic when higher; it does not add to it.
	WeaponBuff *int
	Multiplier *float64
	SaveBonus  *int

	// StrBonus adds to the base STR score (Strength: +1d6 for a bard;
	// Improved Strength: 1d6+4, max 23). StrMax above 18 marks Improved
	// Strength; every other bonus stacks, total capped at 18 (see
	// effectiveStr).
	StrBonus *int
	StrMax   *int
	// AC is added to armour class (descending AC: negative is better).
	AC *int
	// ToHit is added to attack rolls (Bless +1, Prayer +1, Chant +1).
	ToHit *int
	// Damage is added to damage rolls after the multipliers, like the
	// Strength bonus.
	Damage *int

	// WeaponKind is a condition on the effects above, not an effect (ADR
	// 0004 step 18): with one (melee or missile) they count only for an
	// attack, or its preview, made with a weapon of that kind, never for a
	// save or a spell's to-hit. Empty: every roll. The Strength effects
	// (StrBonus, StrMax) and MinStr are not conditioned by it: they make the
	// one effective STR of the character.
	WeaponKind WeaponKind
}

// WeaponKind is the kind of weapon a roll is made with (ADR 0016 step 8),
// the condition of ADR 0004 step 18. A hurled weapon is missile.
type WeaponKind string

// Weapon kinds as stored and sent on the wire. WeaponNone is a roll with no
// weapon (a save, a spell's to-hit) or an effect without the condition.
const (
	WeaponNone    WeaponKind = ""
	WeaponMelee   WeaponKind = "melee"
	WeaponMissile WeaponKind = "missile"
)

// Valid reports whether k is a known kind or none.
func (k WeaponKind) Valid() bool { return k == WeaponNone || k == WeaponMelee || k == WeaponMissile }

// ItemPower is one power of a carried item as the aggregate reads it (ADR
// 0016 steps 9 and 10): the item's id and stored name (the breakdowns'
// fallback label), the power's id, its trigger (MinStr, nil: always on) and
// its effects. It has no name of its own, no counters and is never on the
// buff list.
type ItemPower struct {
	ItemID   string
	ItemName string
	PowerID  string
	MinStr   *int
	Effects  BuffEffects
}

// On reports whether the power's trigger is met at the effective STR (ADR
// 0016 step 10): the same comparison as a buff's MinStr gate.
func (p ItemPower) On(effectiveStr int) bool {
	return p.MinStr == nil || effectiveStr >= *p.MinStr
}

// BuffEffect is a single active buff or debuff on a character.
//
// Duration is the number of rounds left (0: not timed); Next Round
// decrements it and the buff expires when it reaches 0. Charges is what is
// left to absorb (Stoneskin layers, Mirror Image images; 0: no charges); each
// absorbed hit decrements it and the buff ends at 0. A buff can have both
// (Mirror Image) or neither (lasts until dismissed).
type BuffEffect struct {
	ID       string
	Name     string
	Charges  int
	Duration int
	Effects  BuffEffects
	// Note is free text shown on the buff (e.g. "+4 saves vs fire"). No
	// rules meaning.
	Note string
	// StartRound is the round counter when the buff was put on the list
	// (activated, cast or received); 0 for a buff added out of combat or
	// stored before the field existed. Set by Add and Activate only, never
	// by the caller; nothing changes it afterwards (ADR 0004 step 15). It
	// has no rules meaning: the turn header draws the elapsed part from it.
	StartRound int
	// ChargesTotal is the charge count the buff started with (the "M" of
	// "N / M"), stamped from Charges by Add and Activate, never by the
	// caller (ADR 0004 step 16). 0: unknown (a buff stored before the field
	// existed) or no charges.
	ChargesTotal int
	// SpellID is the catalogue id of the spell the buff comes from (cast or
	// received), stamped by the spell cast; empty for a buff entered by hand
	// and for one stored before the field existed whose spell could not be
	// identified (ADR 0004 step 17). It only names the buff: the client shows
	// the catalogue's name in the player's language, Name otherwise. No rules
	// meaning.
	SpellID string
}

// Timed reports whether Next Round counts this buff down.
func (b BuffEffect) Timed() bool { return b.Duration > 0 }

// Charged reports whether the buff absorbs hits.
func (b BuffEffect) Charged() bool { return b.Charges > 0 }

// BuffStatus is an active buff plus whether its effects currently apply
// (its MinStr requirement is met by the effective STR).
type BuffStatus struct {
	BuffEffect
	Applies bool
}

// BuffEdit sets a buff's remaining rounds, charges and/or note. Nil leaves
// the value unchanged (a non-nil empty note clears it). Setting a counter the
// buff uses to 0 ends the buff.
type BuffEdit struct {
	Duration *int
	Charges  *int
	Note     *string
}

// SavedBuff is a buff template the player re-adds with one click: the
// starting rounds, charges, effects and note of a buff. Activating one is the
// normal Activate with these values (editable first). ID is 0 until stored.
// CreatedAt is when it was first stored (zero until then); a restore from a
// snapshot writes it back unchanged.
type SavedBuff struct {
	ID       int64
	Name     string
	Duration int
	Charges  int
	Effects  BuffEffects
	Note     string

	CreatedAt time.Time
}

// Strength is the base STR score from the character sheet and the effective
// score with active buffs.
type Strength struct {
	Base      int
	Effective int
}

// AbilityScore is the sheet's score in one ability and the score in effect
// with the effects the app applies to it (ADR 0028 step 2).
type AbilityScore struct {
	Base      int
	Effective int
}

// Contribution is one applying buff's, or item power's, share of an
// additive effect, so a roll breakdown can name every source. Buff is the
// buff's name, or the item's stored name for a power, whose ItemID is set
// (ADR 0016 step 14); the client names a power from its item.
type Contribution struct {
	Buff   string
	ItemID string
	Value  int
}

// Factor is one applying buff's, or item power's, share of a multiplicative
// effect (named as in Contribution).
type Factor struct {
	Buff   string
	ItemID string
	Value  float64
}

// BuffModifiers is the aggregated effect set across all currently active buffs
// (after requirement gating like MinStr) and the carried items' powers that
// are on (ADR 0016 step 11), for the weapon kind of one roll (ADR 0004 step
// 18). The *From fields name the sources behind each total: the buffs in
// list order, then the powers.
type BuffModifiers struct {
	SpellsBuff  float64
	Multipliers float64
	SaveBonus   int
	AC          int
	ToHit       int
	Damage      int
	// WeaponBuff is the best "weapon becomes +N" effect (0: none) and
	// WeaponBuffFrom the buff that gives it, or the stored name of the item
	// whose power gives it (WeaponBuffItemID set).
	WeaponBuff       int
	WeaponBuffFrom   string
	WeaponBuffItemID string
	// EffectiveStr is the STR score the gating used (base plus STR buffs).
	EffectiveStr int

	SpellsBuffFrom  []Factor
	MultipliersFrom []Factor
	ToHitFrom       []Contribution
	DamageFrom      []Contribution
	SaveBonusFrom   []Contribution
}
