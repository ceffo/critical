package combatsvc

// ModifierKind says where a modifier comes from, so the UI can label it in
// the player's language. Source is the English label.
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
	ModCrit        ModifierKind = "crit"         // house crit multiplier; Detail = matched range or "placeholder"
	ModAdjustment  ModifierKind = "adjustment"   // save adjustment set after the roll, Detail = reason
)

// Modifier is a labeled bonus or penalty applied to a roll.
type Modifier struct {
	Kind   ModifierKind
	Source string
	Detail string
	Value  int
}

// DamageTermKind is the place of a contribution in the damage formula
// ((Base × Spells) + Weapon) × Multiplier + Strength + Bonus.
type DamageTermKind string

// Damage formula terms.
const (
	TermSpells     DamageTermKind = "spells"     // × on the dice
	TermWeapon     DamageTermKind = "weapon"     // + before the multipliers
	TermMultiplier DamageTermKind = "multiplier" // × on dice and weapon
	TermStrength   DamageTermKind = "strength"   // + after the multipliers
	TermBonus      DamageTermKind = "bonus"      // + after the multipliers (buff damage)
)

// DamageTerm is one contribution to the damage formula.
type DamageTerm struct {
	Term   DamageTermKind
	Kind   ModifierKind
	Source string
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
	AmmoItemID  string
	TargetAC    int
	TargetSize  TargetSize
	Situational *Situational
}

// AmmoUse is the ammunition an attack spent.
type AmmoUse struct {
	ItemID    string
	Name      string
	Magic     int
	Remaining int
}

// RollResult is the full outcome of a combat roll.
type RollResult struct {
	D20Roll        int
	D100CritRoll   int
	IsCrit         bool
	CritMultiplier float64
	// CritFrom..CritTo is the crit-table range the crit d100 fell in (zero
	// when the attack is not a crit).
	CritFrom, CritTo int
	// CritPlaceholder marks the multiplier as coming from the default
	// table, which nobody has edited yet.
	CritPlaceholder bool
	BaseThac0       int
	Modifiers       []Modifier
	EffectiveThac0  int
	TargetACNeeded  int
	Hit             bool
	HitMargin       int

	BaseDamage    int
	DamageDice    string
	SpellsBuff    float64
	WeaponBuff    int
	Multipliers   float64
	StrengthBonus int
	BonusDamage   int
	// DamageTerms lists every contribution to the damage formula.
	DamageTerms []DamageTerm
	// UnroundedDamage is the formula result before the final floor and the
	// zero clamp; TotalDamage is its floor.
	UnroundedDamage float64
	TotalDamage     int
	DamageBreakdown string

	// Attack context (empty for saving throws).
	WeaponID   string
	WeaponName string
	TargetAC   int
	TargetSize TargetSize
	Ammo       *AmmoUse
	// Rolled is what the damage is computed from, rolled with every attack
	// (even a miss) so a target AC correction never rolls again. Nil on
	// saves and on attacks logged before corrections existed.
	Rolled *RolledDamage
	// RollCritRow is the crit-table row the crit d100 falls in, looked up
	// when rolling whatever the outcome (nil like Rolled).
	RollCritRow *CritRow
	// RollCritPlaceholder: RollCritRow came from the unedited default table.
	RollCritPlaceholder bool
	// PreviousTargetAC, PreviousHit and PreviousCrit are the target AC and
	// outcome before the latest target AC correction (nil when never
	// corrected).
	PreviousTargetAC *int
	PreviousHit      *bool
	PreviousCrit     *bool
	// Saving-throw context (empty for attacks): the category and the
	// adjustment set after the roll (0: none), with its optional reason.
	SaveCategory     SaveCategory
	Adjustment       int
	AdjustmentReason string

	// LogEntryID is the action-log entry recording the attack or save.
	LogEntryID int64
}

// RolledDamage is the damage side of an attack as rolled, before the crit:
// the weapon dice and every buff, weapon and strength term. The damage of a
// hit is ((Dice × SpellsBuff) + WeaponBuff) × (crit × BuffMultipliers) +
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
