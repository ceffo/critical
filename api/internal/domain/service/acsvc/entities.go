package acsvc

import "github.com/ceffo/critical/api/internal/domain/service/itemsvc"

// Term is one term of the AC (ADR 0033 step 10), a code the web names.
type Term string

// The terms, in the order step 10 sums them.
const (
	// TermBase is step 10.1: the worn body armour's base AC, else the best
	// base_ac effect, else 10.
	TermBase Term = "base"
	// TermArmourMagic is step 10.2: the worn armour's ac powers.
	TermArmourMagic Term = "armour_magic"
	// TermShield is step 10.3: minus the worn shield's AC bonus.
	TermShield Term = "shield"
	// TermShieldMagic is step 10.3's second half: the worn shield's ac
	// powers.
	TermShieldMagic Term = "shield_magic"
	// TermDexterity is step 10.4: PHB Table 2's defensive adjustment.
	TermDexterity Term = "dexterity"
	// TermProtection is step 10.5: the other equipped pieces' ac powers.
	TermProtection Term = "protection"
	// TermBuffs is step 10.6: the ac effects of the buffs that apply.
	TermBuffs Term = "buffs"
)

// Off is why a row is shown but does not add (ADR 0033 step 12), a code the
// web words. Empty: the row adds.
type Off string

// The reasons a row does not add.
const (
	// OffMagicArmour: a without_magic_armour effect while the worn body
	// armour has an ac power (a ring of protection with magic armour).
	OffMagicArmour Off = "magic_armour"
	// OffDuplicate: a second worn piece of the same catalogue code; only
	// the best one counts (DMG p.149's two rings).
	OffDuplicate Off = "duplicate"
	// OffArmourWorn: a base_ac effect while a body armour is worn.
	OffArmourWorn Off = "armour_worn"
	// OffNotBest: a base_ac effect beaten by a better one.
	OffNotBest Off = "not_best"
)

// Row is one row of the breakdown: its term, its source and its value. The
// source is a piece (ItemID, as ADR 0016 step 14: the web names it from the
// piece), a buff (Buff, its stored name, and SpellID when it comes from a
// spell: the web names it from the catalogue), or none (an empty term: no
// armour, no shield, no magic). Value is the base AC for TermBase and a
// change of the AC number for every other term (negative betters it).
type Row struct {
	Term    Term
	ItemID  string
	Buff    string
	SpellID string
	// ProficiencyID names a buff a successful proficiency check put on
	// (ADR 0037 step 20.1): the web names it from the proficiency.
	ProficiencyID string
	Value         int
	// MissileValue is the shield row's value against missiles when it
	// differs from Value (a body shield, step 10); nil otherwise.
	MissileValue *int
	// Dexterity is the sheet's score the dexterity row read; 0 elsewhere.
	Dexterity int
	// Off is set on a row that is shown and does not add.
	Off Off
}

// AC is the character's armour class: derived on every read, never stored
// (ADR 0033 step 10), and the override the player set (step 13).
type AC struct {
	// Value is the derived AC, against melee (and every attack when Missile
	// is nil).
	Value int
	// Missile is the derived AC against missiles when a worn body shield
	// makes it differ from Value; nil otherwise.
	Missile *int
	// Floored is true when the sum was better than -10 and the floor of
	// step 10.7 raised it.
	Floored bool
	// Rows is the breakdown, in step 10's order.
	Rows []Row
	// Override is the AC the player set (step 13); nil: none.
	Override *Override
	// Warnings are the class warnings (ADR 0033 step 20) of the worn body
	// armour and shield, with the piece's id; nothing is refused or changed.
	Warnings []itemsvc.Warning
}

// Override is an AC the player sets for what the model cannot hold (ADR
// 0033 step 13: the Shield spell): a number -10 to 10 and an optional label
// the player typed.
type Override struct {
	Value int
	Label string
}

// MaxLabelLen is the longest override label, in characters.
const MaxLabelLen = 60

// Field error codes of an override, the messages of its *domain.FieldError
// (the web words them).
const (
	CodeOutOfRange = "out_of_range"
	CodeTooLong    = "too_long"
)
