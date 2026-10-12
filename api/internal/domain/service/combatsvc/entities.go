package combatsvc

// Modifier is a labeled bonus or penalty applied to a roll.
type Modifier struct {
	Source string
	Value  int
}

// RollResult is the full outcome of a combat roll.
type RollResult struct {
	D20Roll        int
	D100CritRoll   int
	IsCrit         bool
	CritMultiplier int
	BaseThac0      int
	Modifiers      []Modifier
	EffectiveThac0 int
	TargetACNeeded int
	Hit            bool
	HitMargin      int

	BaseDamage      int
	SpellsBuff      float64
	WeaponBuff      int
	Multipliers     float64
	StrengthBonus   int
	TotalDamage     int
	DamageBreakdown string
}

// WeaponKind separates melee weapons from missile weapons.
type WeaponKind string

// Weapon kinds as stored and sent on the wire.
const (
	WeaponMelee   WeaponKind = "melee"
	WeaponMissile WeaponKind = "missile"
)

// Weapon is the catalog entry consumed by ResolveAttack. ResolveAttack only
// reads BaseDamage today; the magic and size fields are carried for the
// real-weapon attack resolution that follows.
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
