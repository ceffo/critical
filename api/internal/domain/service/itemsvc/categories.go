package itemsvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Category is a piece's category (ADR 0033 step 2), a language-neutral code
// the web names (ADR 0023).
type Category string

// The categories, as stored and sent on the wire.
const (
	CategoryWeapon     Category = "weapon"
	CategoryAmmunition Category = "ammunition"
	CategoryArmour     Category = "armour"
	CategoryShield     Category = "shield"
	CategoryMagic      Category = "magic"
	CategoryGear       Category = "gear"
)

// Categories is every category, in display order (the one owner of the list:
// the column has no CHECK).
var Categories = []Category{
	CategoryWeapon, CategoryAmmunition, CategoryArmour, CategoryShield, CategoryMagic, CategoryGear,
}

// Valid reports whether c is a known category.
func (c Category) Valid() bool { return slices.Contains(Categories, c) }

// The magic kinds (step 2), the catalogue's own list (Table 56's rows).
const (
	MagicPotion = "potion"
	MagicScroll = "scroll"
	MagicRing   = "ring"
	MagicRod    = "rod"
	MagicStaff  = "staff"
	MagicWand   = "wand"
	MagicMisc   = "misc"
)

// MagicKinds is every magic kind, in display order.
var MagicKinds = ruletablesvc.MagicKinds

// The spell components (ADR 0033 step 1.1): what a spell's consume_item names
// (ADR 0008), a language-neutral code the web names (ADR 0023).
const (
	ComponentGem = "gem"
)

// Components is every component code, in display order (the one owner of
// the list: the column has no CHECK, and the spells' consume_item is checked
// against it).
var Components = []string{ComponentGem}

// State is where a piece is (ADR 0033 step 6).
type State string

// The states, as stored and sent on the wire. The web says Equipped for worn
// and wielded alike (ADR 0035 item 3).
const (
	StateCarried State = "carried"
	StateWorn    State = "worn"
	StateWielded State = "wielded"
)

// Valid reports whether s is a known state.
func (s State) Valid() bool { return s == StateCarried || s == StateWorn || s == StateWielded }

// Allows reports whether a piece of category c (magic kind kind) may be in
// state s (step 6): every piece may be carried; body armour, a shield and a
// ring or miscellaneous magic piece may be worn; a weapon may be wielded.
// Ammunition, gear, potions, scrolls, wands, rods and staves are carried
// only.
func Allows(c Category, kind string, s State) bool {
	switch s {
	case StateCarried:
		return true
	case StateWorn:
		return c == CategoryArmour || c == CategoryShield ||
			(c == CategoryMagic && (kind == MagicRing || kind == MagicMisc))
	case StateWielded:
		return c == CategoryWeapon
	}
	return false
}

// EquipState is the state the piece is equipped in (ADR 0033 step 6): wielded
// for a weapon, worn for what may be worn, empty for a piece carried only.
// The web's Equip moves the piece there (ADR 0035 item 3).
func (i Item) EquipState() State {
	switch {
	case Allows(i.Category, i.MagicKind, StateWielded):
		return StateWielded
	case Allows(i.Category, i.MagicKind, StateWorn):
		return StateWorn
	}
	return ""
}

// slotted reports whether at most one piece of the category may be worn (one
// body armour, one shield: step 6).
func slotted(c Category) bool { return c == CategoryArmour || c == CategoryShield }

// Weapon kinds (ADR 0016 step 8): a hurled weapon is a missile weapon.
const (
	WeaponKindMelee   = "melee"
	WeaponKindMissile = "missile"
)

// Weapon types: the fixed list of language-neutral codes the class rules read (ADR 0030 step 23).
// The empty string is a weapon with no type. Display names come from the frontend's i18n
// (ADR 0023), never from here.
const (
	TypeLongBow       = "long_bow"
	TypeShortBow      = "short_bow"
	TypeLightCrossbow = "light_crossbow"
	TypeHeavyCrossbow = "heavy_crossbow"
	TypeHandCrossbow  = "hand_crossbow"
	TypeDagger        = "dagger"
	TypeDart          = "dart"
	TypeSling         = "sling"
	TypeShortSword    = "short_sword"
	TypeLongSword     = "long_sword"
	TypeOtherMelee    = "other_melee"
	TypeOtherThrown   = "other_thrown"
	TypeOtherMissile  = "other_missile"
)

// WeaponTypes is every weapon type code, in display order: ruletablesvc.WeaponTypes, the one
// owner of the list (the column has no CHECK; the weapon proficiency picks read it too, ADR 0037
// step 7). The web's WEAPON_TYPES repeats it; seedcheck's tests keep it and the seeds in line.
var WeaponTypes = ruletablesvc.WeaponTypes

// ValidWeaponType reports whether t is a weapon type code or empty (no type).
func ValidWeaponType(t string) bool { return t == "" || slices.Contains(WeaponTypes, t) }

// AmmoKinds are the ammunition kinds (ADR 0033 step 4); the hand, light and
// heavy quarrels share quarrel.
var AmmoKinds = []string{"arrow", "quarrel", "bullet", "needle", "blowgun_dart"}

// Sizes are a weapon's sizes (step 3), informative.
var Sizes = []string{"s", "m", "l"}

// DamageTypes are the damage types (step 3): bludgeoning, piercing, slashing.
var DamageTypes = []string{"b", "p", "s"}

// SkillColumns are the PHB Table 29 columns an armour reads (step 5).
var SkillColumns = ruletablesvc.SkillColumns

// ArmorRows are the PHB Table 52 rows (step 5), the catalogue's list.
var ArmorRows = ruletablesvc.ArmorRows

// ShieldKinds are the shield kinds (step 5).
var ShieldKinds = []string{"buckler", "small", "medium", "body"}

// Review reasons (ADR 0033 step 27), codes the web words.
const (
	// ReasonWeaponUnmatched: a weapon whose type matches no catalogue row
	// (other_melee, other_thrown, other_missile, none).
	ReasonWeaponUnmatched = "weapon_unmatched"
	// ReasonUncategorised: an item whose type label says nothing the
	// conversion knows; it became gear (or a miscellaneous magic piece when
	// it has powers, so that they stay on).
	ReasonUncategorised = "uncategorised"
	// ReasonAmmoKindUnknown: an ammunition, or a launcher, whose ammo kind is
	// none of AmmoKinds; it is kept as stored until the player picks one.
	ReasonAmmoKindUnknown = "ammo_kind_unknown"
	// ReasonPowersOff: a piece with powers whose category cannot be worn
	// (a wand, a potion, ammunition): it stays carried, so its powers are
	// off until the player decides.
	ReasonPowersOff = "powers_off"
)

// Field error codes of a state change (ADR 0033 step 6): the message of the
// *domain.FieldError on "state", worded by the web.
const (
	// CodeNotAllowed: a state the piece's category does not allow; also a
	// specialized weapon for a class that does not specialize.
	CodeNotAllowed = "not_allowed"
	// CodeSlotTaken: a second body armour or shield worn without the swap.
	CodeSlotTaken = "slot_taken"
)
