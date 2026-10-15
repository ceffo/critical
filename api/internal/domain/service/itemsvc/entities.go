package itemsvc

import "strings"

// Behaviour is how an item's count behaves at the table.
// It is the only real difference between items: arrows, flasks, wands and
// gems are all one model with a free-text Type label.
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

// Item is a counted possession. Ammo is any item with an AmmoKind: a weapon
// firing that kind spends it, and its MagicBonus adds to hit and damage.
type Item struct {
	ID        string
	Position  int // list order, kept when a removed item is restored
	Name      string
	Type      string // free-text label: arrow, flask, wand, gem...
	Behaviour Behaviour
	// MagicBonus is the item's magical bonus (+N), to hit and damage when
	// fired as ammo.
	MagicBonus int
	AmmoKind   string // which launcher it feeds, e.g. "arrow"; empty if none
	Count      int
	MaxCount   int    // a daily item's refill value (informational otherwise)
	EffectDice string // damage dice, e.g. "6d6"
	EffectSave string // save category for the effect, e.g. "rod_staff_wand"
	Notes      string
}

// Spendable tells whether a spell that consumes an item of itemType can spend
// it: of that type (case-insensitive), not a daily item (it recharges and is
// never spent by a spell), some left. The one rule behind both the cast and
// the list of items offered for a spell.
func (i Item) Spendable(itemType string) bool {
	return strings.EqualFold(i.Type, itemType) && i.Behaviour != BehaviourDaily && i.Count > 0
}

// Removed reports whether this item, as returned by a use, is gone: a
// charges item at 0.
func (i Item) Removed() bool { return i.Behaviour.RemovedAtZero() && i.Count == 0 }
