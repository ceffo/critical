package weaponsvc

import "github.com/ceffo/critical/api/internal/domain/localized"

// Weapon kinds as stored and sent on the wire.
const (
	KindMelee   = "melee"
	KindMissile = "missile"
)

// Weapon is a catalog entry. The to-hit and damage magic are separate: some
// weapons differ. AmmoKind is the ammo item kind a missile weapon fires
// (arrow, bolt); empty for a melee or hurled weapon.
type Weapon struct {
	ID       string
	Position int // list order, kept when a removed weapon is restored
	// Name is a localized text (ADR 0023 step 9): a body per language.
	Name        localized.Text
	DamageSM    string // dice vs small / medium targets, e.g. "1d8"
	DamageL     string // dice vs large targets
	MagicToHit  int
	MagicDamage int
	Kind        string
	TwoHanded   bool
	AmmoKind    string
	Type        string // weapon type code (Types, ADR 0030 step 23); empty: none
	// Specialized is the fighter's weapon specialization (ADR 0030 step 12),
	// refused for a class that does not specialize.
	Specialized bool
}

// WeaponInput is a new weapon.
type WeaponInput struct {
	// Name holds the languages the player typed (ADR 0023 step 11).
	Name        localized.Text
	DamageSM    string
	DamageL     string
	MagicToHit  int
	MagicDamage int
	Kind        string
	TwoHanded   bool
	AmmoKind    string
	Type        string
	Specialized bool
}

// WeaponPatch is an edit: only the set (non-nil) fields change.
type WeaponPatch struct {
	// Name replaces the bodies of the languages it holds and keeps the
	// others; a blank body removes its language; nil: unchanged (ADR 0023
	// step 11).
	Name        localized.Text
	DamageSM    *string
	DamageL     *string
	MagicToHit  *int
	MagicDamage *int
	Kind        *string
	TwoHanded   *bool
	AmmoKind    *string
	Type        *string
	Specialized *bool
}

// ChangeResult is one add, edit or remove: the weapon as it is now (as it
// was, for a remove) and the log entry written.
type ChangeResult struct {
	Weapon     Weapon
	LogEntryID int64
}
