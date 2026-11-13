package itemsvc

import "slices"

// Review reasons of the conversion to abilities (ADR 0049 steps 19 and 21.3),
// codes the web words on the review band.
const (
	// ReasonWeaponWithUse: a weapon, armour or shield that was charged, daily
	// or had dice: it keeps an ability, and the player decides whether it
	// should.
	ReasonWeaponWithUse = "weapon_with_use"
	// ReasonQuantityAboveOne: a ring, rod, staff, wand or miscellaneous item
	// stored with more than one object; kept as stored.
	ReasonQuantityAboveOne = "quantity_above_one"
	// ReasonChargesTotal: a charges ability whose total is the count that was
	// left (the original total did not survive); the player sets it.
	ReasonChargesTotal = "charges_total"
	// ReasonAmmoNotStock: ammunition that was charged or daily; it is stock
	// now.
	ReasonAmmoNotStock = "ammo_not_stock"
	// ReasonLimitClamped: a charges total or a daily number above what an
	// ability holds (MaxCharges, MaxPerDay), or a daily count above its max,
	// brought down to the ceiling; the player checks the number.
	ReasonLimitClamped = "limit_clamped"
)

// LegacyAbilityID is the id of the ability the conversion gives (use_2 and so
// on when several; the conversion gives at most one).
const LegacyAbilityID = "use"

// Converted is a legacy item as the equipment model of ADR 0049 reads it.
// Quantity is what the item's count means in the new model; the boot
// conversion does not write it (slice 2's ruling: the stored count keeps its
// old meaning, so the previous release still runs, until Use moves to
// abilities), while Abilities and Reasons are written and listed at once.
type Converted struct {
	Quantity  int
	Abilities []Ability // never nil: converted with none is an empty list
	Reasons   []string  // review reason codes
}

// oneObjectMagic reports whether the piece is a magic item of one object (a
// ring, rod, staff, wand or miscellaneous item): its quantity is 1 (ADR 0049
// step 2).
func (i Item) oneObjectMagic() bool {
	return i.Category == CategoryMagic && !i.HasStock()
}

// ConvertLegacy is ADR 0049 step 19: one item whose behaviour, count, max
// count and effect dice carry the old meaning, as a quantity, abilities and
// review reasons. A pure function of the item, which must be categorised (ADR
// 0033): the boot step, the snapshot restorer and the seed loader all use it.
//
// A charges item's original total cannot be known: max is the count left.
// A with_item limit needs a stock (Item.HasStock); a piece with none (a
// weapon, armour, shield, ring, rod, staff, wand, misc) that was stock with
// dice gets an unlimited ability instead, which the editor accepts.
func ConvertLegacy(it Item) Converted {
	out := Converted{Quantity: it.Count, Abilities: []Ability{}}
	var effects []Effect
	if it.EffectDice != "" {
		effects = []Effect{{Kind: EffectDice, Dice: it.EffectDice, Save: it.EffectSave}}
	}
	ability := func(l Limit) Ability {
		return Ability{ID: LegacyAbilityID, Limit: l, Effects: slices.Clone(effects)}
	}
	behaviour := it.Behaviour
	if it.Category == CategoryAmmunition && behaviour != BehaviourStock {
		behaviour = BehaviourStock
		out.Reasons = append(out.Reasons, ReasonAmmoNotStock)
	}
	stack := it.HasStock() && it.Category == CategoryMagic // a potion or a scroll
	stockLimit := Limit{Kind: LimitUnlimited}
	if it.HasStock() {
		stockLimit = Limit{Kind: LimitWithItem}
	}
	switch behaviour {
	case BehaviourCharges:
		if stack {
			// A potion or a scroll of some doses: the doses are the quantity.
			out.Abilities = append(out.Abilities, ability(Limit{Kind: LimitWithItem}))
			break
		}
		total := min(max(it.Count, 1), MaxCharges)
		out.Quantity = 1
		out.Abilities = append(out.Abilities, ability(Limit{Kind: LimitCharges, Max: total, Left: min(it.Count, total), Cost: 1}))
		out.Reasons = append(out.Reasons, ReasonChargesTotal)
		if it.Count > MaxCharges {
			out.Reasons = append(out.Reasons, ReasonLimitClamped)
		}
	case BehaviourDaily:
		out.Quantity = 1
		perDay := min(max(it.MaxCount, 1), MaxPerDay)
		out.Abilities = append(out.Abilities, ability(Limit{Kind: LimitPerDay, Max: perDay, Left: min(it.Count, perDay)}))
		if it.MaxCount > MaxPerDay || it.Count > perDay {
			out.Reasons = append(out.Reasons, ReasonLimitClamped)
		}
	default:
		switch {
		case len(effects) > 0:
			out.Abilities = append(out.Abilities, ability(stockLimit))
		case stack:
			out.Abilities = append(out.Abilities, ability(Limit{Kind: LimitWithItem}))
		}
	}
	if it.Category.OneObject() && len(out.Abilities) > 0 {
		out.Reasons = append(out.Reasons, ReasonWeaponWithUse)
	}
	if it.oneObjectMagic() && out.Quantity > 1 {
		out.Reasons = append(out.Reasons, ReasonQuantityAboveOne)
	}
	return out
}

// Check is the abilities ConvertLegacy gave it validated as the editor would
// (Item.Validate's ability rules, on the item they came from): the boot step
// and the restorer never store a list the editor then refuses.
func (c Converted) Check(it Item) error {
	it.Abilities = c.Abilities
	return it.validAbilities()
}

// LegacyFields is the old columns an item with these abilities keeps while the
// previous release still reads them (ADR 0049 step 18, until the contract):
// what a seed row written in the new shape stores besides its abilities.
type LegacyFields struct {
	Behaviour  Behaviour
	Count      int
	MaxCount   int
	EffectDice string
	EffectSave string
}

// MirrorLegacy is ConvertLegacy's inverse for the seed: the old columns of an
// item whose quantity and abilities are given. The first ability decides the
// behaviour (with_item or none: stock; charges; per_day: daily), the count is
// the ability's left or the quantity, and the first dice effect is the item's
// effect dice.
func MirrorLegacy(quantity int, abilities []Ability) LegacyFields {
	out := LegacyFields{Behaviour: BehaviourStock, Count: quantity, MaxCount: quantity}
	if len(abilities) == 0 {
		return out
	}
	first := abilities[0]
	switch first.Limit.Kind {
	case LimitCharges:
		out.Behaviour, out.Count, out.MaxCount = BehaviourCharges, first.Limit.Left, first.Limit.Left
	case LimitPerDay:
		out.Behaviour, out.Count, out.MaxCount = BehaviourDaily, first.Limit.Left, first.Limit.Max
	}
	for _, e := range first.Effects {
		if e.Kind == EffectDice {
			out.EffectDice, out.EffectSave = e.Dice, e.Save
			break
		}
	}
	return out
}

// NamedFrom gives each ability without a name the name of the ability of the
// same id in named (the abilities of the seed row an item came from, or of
// the item as it is stored now): a name is data, never a branch on an item.
// abilities is changed in place and returned.
func NamedFrom(abilities, named []Ability) []Ability {
	for n := range abilities {
		if len(abilities[n].Name) > 0 {
			continue
		}
		for _, o := range named {
			if o.ID == abilities[n].ID && len(o.Name) > 0 {
				abilities[n].Name = o.Name
				break
			}
		}
	}
	return abilities
}

// BootConversion is what one run of the boot conversion did: the items it
// converted, and how many of them it put on the review list (ADR 0049 step
// 18's log line "converted N, review M").
type BootConversion struct {
	Converted int
	Review    int
}
