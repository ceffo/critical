package weaponsvc

// Weapon kinds as stored and sent on the wire.
const (
	KindMelee   = "melee"
	KindMissile = "missile"
)

// Weapon is a catalog entry. The to-hit and damage magic are separate: some
// weapons differ. AmmoKind is the ammo item kind a missile weapon fires
// (arrow, bolt); empty for a melee or hurled weapon.
type Weapon struct {
	ID          string
	Position    int // list order, kept when a removed weapon is restored
	Name        string
	DamageSM    string // dice vs small / medium targets, e.g. "1d8"
	DamageL     string // dice vs large targets
	MagicToHit  int
	MagicDamage int
	Kind        string
	TwoHanded   bool
	AmmoKind    string
}

// WeaponInput is a new weapon.
type WeaponInput struct {
	Name        string
	DamageSM    string
	DamageL     string
	MagicToHit  int
	MagicDamage int
	Kind        string
	TwoHanded   bool
	AmmoKind    string
}

// WeaponPatch is an edit: only the set (non-nil) fields change.
type WeaponPatch struct {
	Name        *string
	DamageSM    *string
	DamageL     *string
	MagicToHit  *int
	MagicDamage *int
	Kind        *string
	TwoHanded   *bool
	AmmoKind    *string
}

// ChangeResult is one add, edit or remove: the weapon as it is now (as it
// was, for a remove) and the log entry written.
type ChangeResult struct {
	Weapon     Weapon
	LogEntryID int64
}
