package combatsvc

// ModifierKind says where a modifier comes from, so the UI can label it in
// the player's language from the kind and the detail.
type ModifierKind string

// Modifier kinds.
const (
	ModStrength    ModifierKind = "strength"     // PHB Table 1, Detail = effective STR
	ModDexterity   ModifierKind = "dexterity"    // PHB Table 2 missile adj., Detail = DEX
	ModWeaponMagic ModifierKind = "weapon_magic" // the weapon's own +N, Detail = weapon name
	ModWeaponBuff  ModifierKind = "weapon_buff"  // a buff making the weapon +N, Detail = buff name
	ModAmmoMagic   ModifierKind = "ammo_magic"   // the arrow's +N, Detail = ammo name
	ModBuff        ModifierKind = "buff"         // a buff's to-hit/damage/save bonus, Detail = buff name
	ModSituational ModifierKind = "situational"  // entered at the table, Detail = its label
	ModCrit        ModifierKind = "crit"         // house crit multiplier; the range is the roll's CritFrom..CritTo
	ModAdjustment  ModifierKind = "adjustment"   // save adjustment set after the roll, Detail = reason
)

// Modifier is a labeled bonus or penalty applied to a roll.
type Modifier struct {
	Kind   ModifierKind
	Detail string
	Value  int
}

// DamageTermKind is the place of a contribution in the damage formula
// ((Base × Spells × DiceMultiplier) + Weapon) × Crit + Strength + Bonus
// (ADR 0003 step 2).
type DamageTermKind string

// Damage formula terms.
const (
	TermSpells     DamageTermKind = "spells"          // × on the dice
	TermDiceMult   DamageTermKind = "dice_multiplier" // × on the raw dice only (Power Gem ×2)
	TermWeapon     DamageTermKind = "weapon"          // + before the crit multiplier
	TermMultiplier DamageTermKind = "multiplier"      // crit × on dice and weapon
	TermStrength   DamageTermKind = "strength"        // + after the multipliers
	TermBonus      DamageTermKind = "bonus"           // + after the multipliers (buff damage)
)

// DamageTerm is one contribution to the damage formula.
type DamageTerm struct {
	Term   DamageTermKind
	Kind   ModifierKind
	Detail string
	Value  float64
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

// AmmoUse is the ammunition an attack spent.
type AmmoUse struct {
	ItemID    string
	Name      string
	Magic     int
	Remaining int
}

// RollResult is the full outcome of a combat roll.
type RollResult struct {
	D20Roll int
	// D100CritRoll is the crit-check d100 (ADR 0001 step 1): crit when
	// d20 - D100CritRoll >= the roll needed.
	D100CritRoll int
	// D100MultiplierRoll is the second, independent d100 (step 3) whose
	// crit-table row gives the multiplier of a crit. Rolled on every attack;
	// 0 on saves and on attacks logged before it existed.
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

	BaseDamage  int
	DamageDice  string
	SpellsBuff  float64
	WeaponBuff  int
	Multipliers float64 // crit multiplier (1 without a crit)
	// DiceMultiplier is the buffs' multiplier on the raw dice only.
	DiceMultiplier float64
	StrengthBonus  int
	BonusDamage    int
	// DamageTerms lists every contribution to the damage formula.
	DamageTerms []DamageTerm
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
	// saves and on attacks logged before corrections existed.
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
	// Saving-throw context (empty for attacks): the category and the
	// adjustment set after the roll (0: none), with its optional reason.
	SaveCategory     SaveCategory
	Adjustment       int
	AdjustmentReason string
	// SaveBonus is the sum of a save's modifiers (buffs, not the
	// adjustment) and SaveTotal is d20 + SaveBonus + Adjustment, compared
	// with the target (ADR 0006).
	SaveBonus int
	SaveTotal int

	// LogEntryID is the action-log entry recording the attack or save.
	LogEntryID int64
}

// RolledDamage is the damage side of an attack as rolled, before the crit:
// the weapon dice and every buff, weapon and strength term. The damage of a
// hit is ((Dice × SpellsBuff × BuffMultipliers) + WeaponBuff) × crit +
// StrengthBonus + BonusDamage.
type RolledDamage struct {
	Dice            string
	Total           int // the dice total
	SpellsBuff      float64
	WeaponBuff      int
	BuffMultipliers float64
	StrengthBonus   int
	BonusDamage     int
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

// Weapon is the catalog entry consumed by ResolveAttack.
type Weapon struct {
	ID          string
	Name        string
	BaseDamage  string // damage dice vs small/medium targets, e.g. "1d8"
	DamageLarge string // damage dice vs large targets, e.g. "1d12"
	MagicToHit  int
	MagicDamage int
	Kind        WeaponKind
	TwoHanded   bool
	AmmoKind    string // ammo item kind a missile weapon fires, e.g. "arrow"
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
