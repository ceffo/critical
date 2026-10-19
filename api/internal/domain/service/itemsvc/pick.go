package itemsvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// CodeUnknown is the field error code of a catalogue code no row has (ADR
// 0033 step 25).
const CodeUnknown = "unknown"

// Catalogue is the equipment reference data a piece is picked from (ADR
// 0033 steps 21 and 24), empty before catalogue format 7.
func (s *service) Catalogue() ruletablesvc.Equipment {
	if s.equipment == nil {
		return ruletablesvc.Equipment{}
	}
	return *s.equipment
}

// picked is the piece a pick makes (ADR 0033 steps 23 and 24): the row's
// category, numbers and codes copied, its code kept; from the player only the
// name (the row's when none is typed), the count (at least 1, 1 when not
// given), the note, the powers (an armour's or a shield's "+N", ADR 0033 step
// 17), the spell (kept so Validate refuses it: no row is a scroll) and the magic of a weapon (to hit, damage) or ammunition.
// Everything else of in is ignored: the piece is edited afterwards like any other. A code no row
// has is refused (field catalogueCode, CodeUnknown).
func (s *service) picked(in ItemInput) (Item, error) {
	// 0 is a count not given (1); a negative one is refused, not raised to 1.
	if in.Count < 0 {
		return Item{}, fieldErr("count", "Count must be at least 1")
	}
	e := s.Catalogue()
	item := Item{
		CatalogueCode: in.CatalogueCode, Name: in.Name.Trimmed(), State: StateCarried,
		Behaviour: BehaviourStock, Count: max(in.Count, 1), Notes: in.Notes, Powers: withPowerIDs(in.Powers),
		// No catalogue row is a scroll, so a spell here is refused by Validate
		// (spellId not_allowed), not silently dropped.
		SpellID: in.SpellID,
	}
	found := false
	if i := slices.IndexFunc(e.Weapons, func(r ruletablesvc.Weapon) bool { return r.Code == in.CatalogueCode }); i >= 0 {
		r := e.Weapons[i]
		item.Category, item.WeaponType, item.WeaponKind, item.Size = CategoryWeapon, r.WeaponType, r.Kind, r.Size
		item.DamageTypes, item.Speed, item.DamageSM, item.DamageL = slices.Clone(r.Types), r.Speed, r.DamageSM, r.DamageL
		item.TwoHanded, item.MagicToHit, item.MagicDamage = r.TwoHanded, in.MagicToHit, in.MagicDamage
		// The piece fires one kind (step 3); a launcher of several (the
		// blowgun) gets its first, changed by an edit.
		if len(r.Fires) > 0 {
			item.Fires = r.Fires[0]
		}
		item.Name, found = r.Name.With(item.Name), true
	}
	if i := slices.IndexFunc(e.Ammunition, func(r ruletablesvc.Ammunition) bool { return r.Code == in.CatalogueCode }); i >= 0 {
		r := e.Ammunition[i]
		item.Category, item.AmmoKind, item.DamageTypes = CategoryAmmunition, r.AmmoKind, slices.Clone(r.Types)
		item.DamageSM, item.DamageL, item.MagicBonus = r.DamageSM, r.DamageL, in.MagicBonus
		item.Name, found = r.Name.With(item.Name), true
	}
	if i := slices.IndexFunc(e.Armour, func(r ruletablesvc.Armour) bool { return r.Code == in.CatalogueCode }); i >= 0 {
		r := e.Armour[i]
		item.Category, item.BaseAC, item.SkillColumn, item.ArmorRow = CategoryArmour, r.BaseAC, r.SkillColumn, r.ArmorRow
		item.Name, found = r.Name.With(item.Name), true
	}
	if i := slices.IndexFunc(e.Shields, func(r ruletablesvc.Shield) bool { return r.Code == in.CatalogueCode }); i >= 0 {
		r := e.Shields[i]
		item.Category, item.ShieldKind, item.ACMelee, item.ACMissile = CategoryShield, r.Kind, r.ACMelee, r.ACMissile
		item.Name, found = r.Name.With(item.Name), true
	}
	if !found {
		return Item{}, fieldErr("catalogueCode", CodeUnknown)
	}
	return clean(item), nil
}
