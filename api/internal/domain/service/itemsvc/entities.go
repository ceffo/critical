package itemsvc

// Kind classifies an item.
type Kind string

// Item kinds as stored and sent on the wire.
const (
	KindAmmo       Kind = "ammo"
	KindCharged    Kind = "charged"
	KindConsumable Kind = "consumable"
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	return k == KindAmmo || k == KindCharged || k == KindConsumable
}

// Item is a counted possession: ammo, a charged item or a consumable.
type Item struct {
	ID         string
	Name       string
	Kind       Kind
	MagicBonus int    // ammo to-hit/damage bonus
	AmmoKind   string // which launcher it feeds, e.g. "arrow"
	Count      int    // remaining uses / charges / pieces
	MaxCount   int    // value restored by a long rest when PerDay
	PerDay     bool   // refilled to MaxCount by ResetDaily
	EffectDice string // damage dice, e.g. "6d6"
	EffectSave string // save category for the effect, e.g. "rod_staff_wand"
	Notes      string
}
