package itemsvc

import (
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// Behaviour is how an item's count behaves at the table (ADR 0016 steps 1 to
// 3), whatever the piece's category.
type Behaviour string

// Item behaviours as stored and sent on the wire.
const (
	// BehaviourStock is used and restocked at the table (arrows, fire
	// flasks); it stays listed at 0.
	BehaviourStock Behaviour = "stock"
	// BehaviourCharges is used only, never recharged (a wand); the use that
	// spends the last charge removes the item.
	BehaviourCharges Behaviour = "charges"
	// BehaviourDaily is used only and stays listed at 0; a long rest (not a
	// time fast-forward) refills it to MaxCount (Power Gem dispel 1/day).
	BehaviourDaily Behaviour = "daily"
)

// Valid reports whether b is a known behaviour.
func (b Behaviour) Valid() bool {
	return b == BehaviourStock || b == BehaviourCharges || b == BehaviourDaily
}

// Restockable reports whether the count may go up at the table (stock only).
func (b Behaviour) Restockable() bool { return b == BehaviourStock }

// RemovedAtZero reports whether the item is gone once its count reaches 0.
func (b Behaviour) RemovedAtZero() bool { return b == BehaviourCharges }

// Item is one piece of a character's equipment (ADR 0033 step 1): weapons,
// ammunition, armour, shields, magic items and gear are one model, one list
// and one editor. Every piece has the shared fields; the fields of the other
// categories are zero (normalized on every write).
type Item struct {
	ID       string
	Position int // list order, kept when a removed item is restored
	// Name is a localized text (ADR 0023 step 9): a body per language.
	Name localized.Text
	// Category is the piece's category code (step 2); MagicKind its magic
	// kind when it is a magic piece, else empty.
	Category  Category
	MagicKind string
	// CatalogueCode is the catalogue row the piece was picked from or
	// matched to (step 21), empty for homebrew.
	CatalogueCode string
	// State is carried, worn or wielded (step 6).
	State State
	// Type is the free-text label of before the equipment model, kept as
	// stored (step 26.1) and no longer edited. The spells that spend an item
	// still match it (Spendable).
	Type       string
	Behaviour  Behaviour
	Count      int
	MaxCount   int    // a daily item's refill value (informational otherwise)
	EffectDice string // damage dice of a use, e.g. "6d6"
	EffectSave string // save category for the effect, e.g. "rod_staff_wand"
	Notes      string
	// Powers are the piece's own powers (ADR 0016 steps 9 to 16), never set
	// by the editor; on only while the piece is worn or wielded (step 9).
	Powers []Power

	// Ammunition (step 4): its ammo kind and its magic, to hit and damage.
	AmmoKind   string
	MagicBonus int

	// Weapon and ammunition: the damage types (one or two of b, p, s) and
	// the damage against small-medium and large targets (NdM).
	DamageTypes []string
	DamageSM    string
	DamageL     string

	// Weapon (step 3).
	WeaponType  string // ADR 0030 step 23's code; empty: none
	WeaponKind  string // melee or missile
	Size        string // s, m, l; empty: unknown
	Speed       int    // speed factor, 0 to 20
	MagicToHit  int
	MagicDamage int
	TwoHanded   bool
	Fires       string // the ammo kind a launcher fires; empty: none
	// Specialized is the fighter's weapon specialization (ADR 0030 step 12).
	Specialized bool

	// Armour (step 5): base AC, Table 29 column, Table 52 row.
	BaseAC      int
	SkillColumn string
	ArmorRow    string

	// Shield (step 5): its kind and its AC bonus against melee and missiles.
	ShieldKind string
	ACMelee    int
	ACMissile  int
}

// Spendable tells whether a spell that consumes an item of itemType can spend
// it: of that type (the stored label, case-insensitive), not a daily item (it
// recharges and is never spent by a spell), some left. The one rule behind
// both the cast and the list of items offered for a spell.
func (i Item) Spendable(itemType string) bool {
	return strings.EqualFold(i.Type, itemType) && i.Behaviour != BehaviourDaily && i.Count > 0
}

// Removed reports whether this item, as returned by a use, is gone: a
// charges item at 0.
func (i Item) Removed() bool { return i.Behaviour.RemovedAtZero() && i.Count == 0 }

// Equipped reports whether the piece is worn or wielded: its powers are on
// (step 9).
func (i Item) Equipped() bool { return i.State == StateWorn || i.State == StateWielded }

// FiresAmmo reports whether the piece is a launcher that spends ammunition
// (a bow); a missile weapon without Fires is hurled (darts).
func (i Item) FiresAmmo() bool {
	return i.Category == CategoryWeapon && i.WeaponKind == WeaponKindMissile && i.Fires != ""
}

// Review is one row of the review list (ADR 0033 step 27): a piece the
// conversion could not settle, with the reason as a code.
type Review struct {
	ItemID string
	Reason string
}
