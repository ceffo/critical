package combatsvc

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ModifierKind says where a modifier comes from, so the UI can label it in
// the player's language from the kind and the detail.
type ModifierKind string

// Modifier kinds.
const (
	ModStrength    ModifierKind = "strength"     // PHB Table 1, Detail = effective STR
	ModDexterity   ModifierKind = "dexterity"    // PHB Table 2 missile adj., Detail = DEX
	ModWeaponMagic ModifierKind = "weapon_magic" // the weapon's own +N, Detail = weapon name
	ModWeaponBuff  ModifierKind = "weapon_buff"  // a buff or item power making the weapon +N, Detail = buff (or item) name
	ModAmmoMagic   ModifierKind = "ammo_magic"   // the arrow's +N, Detail = ammo name, ItemID = the ammo item
	ModBuff        ModifierKind = "buff"         // a buff's or item power's to-hit/damage/save bonus, Detail = buff (or item) name
	ModSituational ModifierKind = "situational"  // entered at the table, Detail = its label
	ModRace        ModifierKind = "race"         // the race's save bonus (ADR 0030 step 22.1), Detail = the race code
	ModCrit        ModifierKind = "crit"         // house crit multiplier; the range is the roll's CritFrom..CritTo
	// ModSpecialization is weapon specialization (ADR 0030 step 12): +1 to hit and +2 damage with a
	// melee or thrown weapon, not magical; Detail is empty.
	ModSpecialization ModifierKind = "specialization"
	// ModPointBlank is a specialized launcher's +2 to hit at point-blank range (ADR 0030 step 12),
	// an option of the attack request; Detail is empty.
	ModPointBlank ModifierKind = "point_blank"
	// ModBackstab is a thief's backstab (ADR 0030 step 19): +4 to hit and, as a standard_multiplier
	// damage term, PHB Table 30's multiplier at the realized level. An option of the attack
	// request; Detail is empty.
	ModBackstab ModifierKind = "backstab"
	// ModNonproficient is PHB Table 34's non-proficiency penalty for the class group (ADR 0037
	// step 8): an attack with a weapon whose type has no weapon pick. Not magical; Detail is the
	// weapon type.
	ModNonproficient ModifierKind = "nonproficient"
	// ModDiceModifier is the flat part of the damage as printed (1d4+1,
	// a flat 1; ADR 0003 step 2), a weapon damage term; Detail is the damage
	// as printed.
	ModDiceModifier ModifierKind = "dice_modifier"
)

// Modifier is a labeled bonus or penalty applied to a roll. ItemID is set
// on a contribution of an item power (ADR 0016 step 14): Detail is then the
// item's stored name, a fallback, and the client names it from the item.
type Modifier struct {
	Kind   ModifierKind
	Detail string
	ItemID string
	Value  int
	// StrExceptional is a Strength modifier's 18/xx percentile (1 to 100,
	// ADR 0030 step 9), 0 for none; the UI writes the notation.
	StrExceptional int
}

// DamageTermKind is the place of a contribution in the damage formula
// ((Base × DiceMultiplier + DiceModifier) × Spells × Backstab + Weapon) ×
// Crit + Strength + Bonus (ADR 0003 step 2).
type DamageTermKind string

// Damage formula terms.
const (
	TermSpells   DamageTermKind = "spells"          // × on the dice
	TermDiceMult DamageTermKind = "dice_multiplier" // × on the raw dice only (Power Gem ×2; backstab on attacks logged before standard_multiplier)
	// TermStandardMult multiplies the weapon's standard damage, its dice and
	// flat part, not its magic (backstab, PHB p.40).
	TermStandardMult DamageTermKind = "standard_multiplier"
	TermWeapon       DamageTermKind = "weapon"     // + : the damage's flat part (before Spells), the weapon's magic (before the crit)
	TermMultiplier   DamageTermKind = "multiplier" // crit × on dice and weapon
	TermStrength     DamageTermKind = "strength"   // + after the multipliers
	TermBonus        DamageTermKind = "bonus"      // + after the multipliers (buff damage)
)

// DamageTerm is one contribution to the damage formula; ItemID as on
// Modifier.
type DamageTerm struct {
	Term   DamageTermKind
	Kind   ModifierKind
	Detail string
	ItemID string
	Value  float64
	// StrExceptional is as on Modifier.
	StrExceptional int
}

// TargetSize picks the weapon's damage dice.
type TargetSize string

// Target sizes as sent on the wire: small/medium or large.
const (
	SizeSmallMedium TargetSize = "sm"
	SizeLarge       TargetSize = "l"
)

// Valid reports whether s is a known size.
func (s TargetSize) Valid() bool { return s == SizeSmallMedium || s == SizeLarge }

// Situational is a to-hit modifier entered at the table (rear attack, cover,
// higher ground...). Label is free text.
type Situational struct {
	Value int
	Label string
}

// AttackRequest is what the player asks for.
type AttackRequest struct {
	WeaponID string
	// AmmoItemID is required for weapons that fire ammunition (bows) and
	// must be empty otherwise.
	AmmoItemID string
	// TargetAC is nil when the target's AC is unknown (ADR 0012 step 1).
	TargetAC    *int
	TargetSize  TargetSize
	Situational *Situational
	// PointBlank is the target at point-blank range (ADR 0030 step 12): a specialized launcher's
	// option, refused for any other weapon. Not saved.
	PointBlank bool
	// Backstab is a thief's backstab (ADR 0030 step 19), refused for a class without it. Not
	// saved: first-attack-only and surprise are the player's call.
	Backstab bool
}

// Verdict is the GM's word on an attack against an unknown AC.
type Verdict string

// Verdicts as sent on the wire.
const (
	VerdictNone Verdict = ""
	VerdictHit  Verdict = "hit"
	VerdictMiss Verdict = "miss"
)

// Settled reports whether the attack's hit, crit and damage are final: the
// target AC is known or the GM gave a verdict (ADR 0012).
func (r *RollResult) Settled() bool { return r.ACKnown || r.Verdict != VerdictNone }

// MaxRealAC is the highest armor class the rules allow (AC 10, no armor).
// An attack whose CritsFromAC is above it cannot crit any real target (ADR
// 0012 step 7).
const MaxRealAC = 10

// CritPossible reports whether some real target AC (at most MaxRealAC) is
// crit by the attack: CritsFromAC <= MaxRealAC.
func (r *RollResult) CritPossible() bool { return r.CritsFromAC <= MaxRealAC }

// AmmoUse is the ammunition an attack spent.
type AmmoUse struct {
	ItemID    string
	Name      string
	Magic     int
	Remaining int
}

// RollResult is the full outcome of an attack.
type RollResult struct {
	D20Roll int
	// D100CritRoll is the crit-check d100 (ADR 0001 step 1): crit when
	// d20 - D100CritRoll >= the roll needed.
	D100CritRoll int
	// D100MultiplierRoll is the second, independent d100 (step 3) whose
	// crit-table row gives the multiplier of a crit. Rolled on every attack;
	// 0 on attacks logged before it existed.
	D100MultiplierRoll int
	IsCrit             bool
	CritMultiplier     float64
	// CritFrom..CritTo is the crit-table range the multiplier d100 fell in
	// (zero when the attack is not a crit).
	CritFrom, CritTo int
	// CritPlaceholder marks the multiplier as coming from the default
	// table, which nobody has edited yet.
	CritPlaceholder bool
	BaseThac0       int
	Modifiers       []Modifier
	EffectiveThac0  int
	// TargetACNeeded is the lowest AC the d20 hits (effective THAC0 - d20),
	// the same number as HitsAC; kept for the entries logged before HitsAC.
	TargetACNeeded int
	// HitsAC is the lowest target AC the attack hits: effective THAC0 - d20
	// (ADR 0012 step 2). CritsFromAC is the lowest target AC it crits:
	// HitsAC + crit-check d100 (step 3, ADR 0001 step 2 solved for the AC).
	HitsAC      int
	CritsFromAC int
	// Hit, HitMargin, IsCrit and the damage are final only when Settled():
	// the AC is known or the GM gave a verdict. Unsettled, Hit is false,
	// there is no crit and the damage is the one of a hit at x1 (ADR 0012).
	Hit       bool
	HitMargin int

	BaseDamage int
	DamageDice string
	// DiceModifier is the flat part K of the damage as printed (1d4+1:
	// 1), part of the standard damage: added after the Power Gem's raw-dice
	// factor, multiplied by SpellsBuff, the backstab and the crit (ADR 0003
	// step 2).
	DiceModifier int
	SpellsBuff   float64
	WeaponBuff   int
	Multipliers  float64 // crit multiplier (1 without a crit)
	// DiceMultiplier is the buffs' multiplier on the raw dice only.
	DiceMultiplier float64
	// BackstabMultiplier multiplies the standard damage, dice and
	// DiceModifier (ADR 0030 step 19); 0 without a backstab.
	BackstabMultiplier float64
	StrengthBonus      int
	BonusDamage        int
	// DamageTerms lists every contribution to the damage formula.
	DamageTerms []DamageTerm
	// DamageRolls are the damage dice as thrown (empty when the attack
	// deals no damage); BaseDamage is their sum.
	DamageRolls []DieRoll
	// UnroundedDamage is the formula result before the final floor and the
	// zero clamp; TotalDamage is its floor.
	UnroundedDamage float64
	TotalDamage     int

	// Attack context (empty for saving throws).
	WeaponID   string
	WeaponName string
	TargetAC   int
	TargetSize TargetSize
	Ammo       *AmmoUse
	// ACKnown is false while the target AC is unknown (TargetAC is then
	// meaningless); RolledACUnknown says the attack was rolled without it,
	// even once the GM gave it.
	ACKnown         bool
	RolledACUnknown bool
	// Verdict is the GM's hit or miss given instead of the AC (ADR 0012
	// step 4); VerdictNone once the AC is known.
	Verdict Verdict
	// Rolled is what the damage is computed from, rolled with every attack
	// (even a miss) so a target AC correction never rolls again. Nil on
	// attacks logged before corrections existed.
	Rolled *RolledDamage
	// RollCritRow is the crit-table row the multiplier d100 falls in,
	// looked up when rolling whatever the outcome (nil like Rolled), so a
	// correction uses the table as it was at the roll.
	RollCritRow *CritRow
	// RollCritPlaceholder: RollCritRow came from the unedited default table.
	RollCritPlaceholder bool
	// RolledTargetAC, RolledHit and RolledCrit are the target AC and
	// outcome as rolled, set while a correction differs from them.
	RolledTargetAC *int
	RolledHit      *bool
	RolledCrit     *bool
	// LogEntryID is the action-log entry recording the attack.
	LogEntryID int64
}

// DieRoll is one die thrown: its sides and the value it showed.
type DieRoll struct {
	Sides int
	Value int
}

// RolledDamage is the damage side of an attack as rolled, before the crit:
// the weapon dice and every buff, weapon and strength term. The damage of a
// hit is ((Dice × BuffMultipliers + DiceModifier) × SpellsBuff ×
// BackstabMultiplier + WeaponBuff) × crit + StrengthBonus + BonusDamage.
type RolledDamage struct {
	Dice  string
	Total int // the dice total
	// Rolls are the dice thrown for Total, in order.
	Rolls []DieRoll
	// DiceModifier is the flat part of Dice ("1d4+1": 1, a flat "1": 1).
	DiceModifier    int
	SpellsBuff      float64
	WeaponBuff      int
	BuffMultipliers float64
	// BackstabMultiplier is the backstab's PHB Table 30 factor, 0 without
	// a backstab (ADR 0030 step 19).
	BackstabMultiplier float64
	StrengthBonus      int
	BonusDamage        int
	// Terms are the damage terms in formula order, without the crit term.
	Terms []DamageTerm
}

// WeaponKind separates melee weapons from missile weapons.
type WeaponKind string

// Weapon kinds as stored and sent on the wire.
const (
	WeaponMelee   WeaponKind = "melee"
	WeaponMissile WeaponKind = "missile"
)

// Weapon is a weapon piece of the character's equipment (ADR 0033 step 3) as
// the attack reads it (weaponOf).
type Weapon struct {
	ID string
	// Name is a localized text (ADR 0023 step 9); results and payloads
	// record Name.Any().
	Name        localized.Text
	BaseDamage  string // damage dice vs small/medium targets, e.g. "1d8"
	DamageLarge string // damage dice vs large targets, e.g. "1d12"
	MagicToHit  int
	MagicDamage int
	Kind        WeaponKind
	TwoHanded   bool
	AmmoKind    string // ammo kind a missile weapon fires, e.g. "arrow" (the piece's Fires)
	Type        string // weapon type code (itemsvc.WeaponTypes); empty: none
	// Specialized is the fighter's weapon specialization (ADR 0030 step 12).
	Specialized bool
	// Speed is the speed factor (0 to 20), Wielded whether the piece is
	// wielded (ADR 0033 step 6): only a wielded weapon is in hand.
	Speed   int
	Wielded bool
}

// weaponOf is a weapon piece as the attack reads it.
func weaponOf(it itemsvc.Item) Weapon {
	return Weapon{
		ID: it.ID, Name: it.Name, BaseDamage: it.DamageSM, DamageLarge: it.DamageL,
		MagicToHit: it.MagicToHit, MagicDamage: it.MagicDamage, Kind: WeaponKind(it.WeaponKind),
		TwoHanded: it.TwoHanded, AmmoKind: it.Fires, Type: it.WeaponType, Specialized: it.Specialized,
		Speed: it.Speed, Wielded: it.State == itemsvc.StateWielded,
	}
}

// isAmmo reports whether the piece is ammunition the weapon fires.
func isAmmo(it itemsvc.Item, w Weapon) bool {
	return it.Category == itemsvc.CategoryAmmunition && w.AmmoKind != "" && it.AmmoKind == w.AmmoKind
}

// FiresAmmo reports whether the weapon is a launcher (a bow) that spends
// ammunition. A missile weapon without AmmoKind is hurled (darts).
func (w Weapon) FiresAmmo() bool { return w.Kind == WeaponMissile && w.AmmoKind != "" }

// SaveCategory enumerates AD&D saving-throw categories.
type SaveCategory string

// Saving-throw category names as they appear on the wire.
const (
	SaveParaPoison   SaveCategory = "para_poison"
	SaveRodStaffWand SaveCategory = "rod_staff_wand"
	SavePetriPoly    SaveCategory = "petri_poly"
	SaveBreath       SaveCategory = "breath"
	SaveSpell        SaveCategory = "spell"
)
