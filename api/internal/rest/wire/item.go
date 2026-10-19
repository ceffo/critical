package wire

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ItemDTO is the wire shape of a piece of equipment (ADR 0033 step 1): the
// shared fields, then the fields of each category, zero (or empty) for the
// others. Codes go on the wire, the web names them (ADR 0023): category,
// magicKind, state (carried, worn, wielded), behaviour (stock, charges,
// daily), component (the spell component it is, empty for none; ADR 0033
// step 1.1). type is the free-text label of before the model, read only.
type ItemDTO struct {
	ID string `json:"id"`
	// Name is a language map (ADR 0023 step 9).
	Name          localized.Text `json:"name"`
	Category      string         `json:"category"`
	MagicKind     string         `json:"magicKind"`
	CatalogueCode string         `json:"catalogueCode"`
	Component     string         `json:"component"`
	State         string         `json:"state"`
	// EquipState is the state Equip moves the piece to (worn or wielded),
	// empty for a piece that is carried only (ADR 0033 step 6).
	EquipState   string `json:"equipState"`
	Type         string `json:"type"`
	Behaviour    string `json:"behaviour"`
	MagicalBonus int    `json:"magicalBonus"`
	AmmoKind     string `json:"ammoKind"`
	Count        int    `json:"count"`
	MaxCount     int    `json:"maxCount"`
	EffectDice   string `json:"effectDice"`
	EffectSave   string `json:"effectSave"`
	Notes        string `json:"notes"`
	// Powers are the item's own powers (ADR 0016 steps 9 to 16), set by the
	// editor (ADR 0033 step 17): whether one is on is served with the effects
	// list (GET /api/buffs).
	Powers []PowerDTO `json:"powers"`
	// Weapon and ammunition.
	DamageTypes []string `json:"damageTypes"`
	DamageSM    string   `json:"damageSm"`
	DamageL     string   `json:"damageL"`
	// Weapon.
	WeaponType  string `json:"weaponType"`
	WeaponKind  string `json:"weaponKind"`
	Size        string `json:"size"`
	Speed       int    `json:"speed"`
	MagicToHit  int    `json:"magicToHit"`
	MagicDamage int    `json:"magicDamage"`
	TwoHanded   bool   `json:"twoHanded"`
	Fires       string `json:"fires"`
	Specialized bool   `json:"specialized"`
	// Armour.
	BaseAC      int    `json:"baseAc"`
	SkillColumn string `json:"skillColumn"`
	ArmorRow    string `json:"armorRow"`
	// Shield.
	ShieldKind string `json:"shieldKind"`
	ACMelee    int    `json:"acMelee"`
	ACMissile  int    `json:"acMissile"`
	// Review is the piece's review reasons (ADR 0033 step 27), empty when it
	// is not on the review list. Served by the list only.
	Review []string `json:"review"`
	// Warnings are the piece's class warnings (ADR 0033 step 20): codes the
	// web words, empty for none. Served by the list only.
	Warnings []string `json:"warnings"`
}

// PowerDTO is one of an item's powers: its id, its trigger (minStr, omitted:
// always on) and its effects, in the buffs' effect keys.
type PowerDTO struct {
	ID      string         `json:"id"`
	MinStr  *int           `json:"minStr,omitempty"`
	Effects BuffEffectsDTO `json:"effects"`
}

// PowersFromDTO is the domain list of powers sent by the editor (ADR 0033
// step 17), in the order sent; none is nil.
func PowersFromDTO(dtos []PowerDTO) []itemsvc.Power {
	if len(dtos) == 0 {
		return nil
	}
	out := make([]itemsvc.Power, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, itemsvc.Power{ID: d.ID, MinStr: d.MinStr, Effects: buffsvc.BuffEffects(d.Effects)})
	}
	return out
}

// ItemsToDTO maps items to their wire shape.
func ItemsToDTO(items []itemsvc.Item) []ItemDTO {
	out := make([]ItemDTO, 0, len(items))
	for _, i := range items {
		powers := make([]PowerDTO, 0, len(i.Powers))
		for _, p := range i.Powers {
			powers = append(powers, PowerDTO{ID: p.ID, MinStr: p.MinStr, Effects: BuffEffectsDTO(p.Effects)})
		}
		damageTypes := i.DamageTypes
		if damageTypes == nil {
			damageTypes = []string{}
		}
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Category: string(i.Category), MagicKind: i.MagicKind,
			CatalogueCode: i.CatalogueCode, Component: i.Component, State: string(i.State), EquipState: string(i.EquipState()),
			Type: i.Type, Behaviour: string(i.Behaviour),
			MagicalBonus: i.MagicBonus, AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: powers,
			DamageTypes: damageTypes, DamageSM: i.DamageSM, DamageL: i.DamageL,
			WeaponType: i.WeaponType, WeaponKind: i.WeaponKind, Size: i.Size, Speed: i.Speed,
			MagicToHit: i.MagicToHit, MagicDamage: i.MagicDamage, TwoHanded: i.TwoHanded, Fires: i.Fires,
			Specialized: i.Specialized, BaseAC: i.BaseAC, SkillColumn: i.SkillColumn, ArmorRow: i.ArmorRow,
			ShieldKind: i.ShieldKind, ACMelee: i.ACMelee, ACMissile: i.ACMissile, Review: []string{}, Warnings: []string{},
		})
	}
	return out
}
