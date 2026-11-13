package wire

import (
	"encoding/json"

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
	// SpellID is the catalogue id of the spell a scroll holds (ADR 0033
	// step 2), empty for none and on every other piece.
	SpellID string `json:"spellId"`
	State   string `json:"state"`
	// EquipState is the state Equip moves the piece to (worn or wielded),
	// empty for a piece that is carried only (ADR 0033 step 6).
	EquipState string `json:"equipState"`
	// Usable is true for a piece with a stock (ammunition, gear, a potion, a
	// scroll): its quantity moves by spend one and restock; false for a
	// one-object piece (ADR 0049 step 2). The server owns it; whether an
	// ability can be used is on the ability.
	Usable       bool   `json:"usable"`
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
	// Abilities are what the item does when used, each under a usage limit
	// (ADR 0049 step 4); absent for a piece not converted yet (step 18).
	Abilities *[]AbilityDTO `json:"abilities,omitempty"`
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
	// StrengthRating is a bow's Strength rating (ADR 0033 step 3): the
	// Strength a strength bow is built for; null for an ordinary bow and
	// every other piece.
	StrengthRating *StrengthRatingDTO `json:"strengthRating"`
	Specialized    bool               `json:"specialized"`
	// NonproficientPenalty is ADR 0037 step 8: the to-hit penalty an attack
	// with the weapon takes for its type having no weapon pick (0: none).
	NonproficientPenalty int `json:"nonproficientPenalty"`
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
	// web words, empty for none. Served with every piece.
	Warnings []string `json:"warnings"`
}

// StrengthRatingDTO is a Strength rating: a score, and at 18 its exceptional
// percentile (1 to 100, 100 for 18/00; 0 for none).
type StrengthRatingDTO struct {
	Score       int `json:"score"`
	Exceptional int `json:"exceptional"`
}

// StrengthRatingToDTO is a rating's wire shape: nil (null) for none.
func StrengthRatingToDTO(r itemsvc.StrengthRating) *StrengthRatingDTO {
	if !r.Rated() {
		return nil
	}
	return &StrengthRatingDTO{Score: r.Score, Exceptional: r.Exceptional}
}

// PowerDTO is one of an item's powers: its id, its trigger (minStr, omitted:
// always on) and its effects, in the buffs' effect keys.
type PowerDTO struct {
	ID      string         `json:"id"`
	MinStr  *int           `json:"minStr,omitempty"`
	Effects BuffEffectsDTO `json:"effects"`
}

// AbilityDTO is one item ability (ADR 0049 step 4): its id, its optional name
// (a language map; none means the item's) and note, its limit and its effects.
type AbilityDTO struct {
	ID      string         `json:"id"`
	Name    localized.Text `json:"name,omitempty"`
	Note    string         `json:"note,omitempty"`
	Limit   LimitDTO       `json:"limit"`
	Effects []EffectDTO    `json:"effects"`
	// Usable and Reason are served, never read: whether a use is paid now, and
	// the reason code when not ("exhausted"; ADR 0049 steps 5 and 14), so the
	// web re-derives nothing. Both are ignored in a request body.
	Usable bool   `json:"usable"`
	Reason string `json:"reason,omitempty"`
}

// LimitDTO is an ability's usage limit (ADR 0049 step 5): kind is unlimited,
// per_day, charges or with_item; max, left and cost are sent only for the
// kinds that have them, and always for those (an exhausted left is 0, not
// absent).
type LimitDTO struct {
	Kind string `json:"kind"`
	Max  int    `json:"max,omitempty"`
	Left int    `json:"left,omitempty"`
	Cost int    `json:"cost,omitempty"`
}

// MarshalJSON writes max and left for per_day and charges, cost for charges,
// zeros included.
func (l LimitDTO) MarshalJSON() ([]byte, error) {
	type out struct {
		Kind string `json:"kind"`
		Max  *int   `json:"max,omitempty"`
		Left *int   `json:"left,omitempty"`
		Cost *int   `json:"cost,omitempty"`
	}
	o := out{Kind: l.Kind}
	switch l.Kind {
	case string(itemsvc.LimitPerDay):
		o.Max, o.Left = &l.Max, &l.Left
	case string(itemsvc.LimitCharges):
		o.Max, o.Left, o.Cost = &l.Max, &l.Left, &l.Cost
	}
	return json.Marshal(o)
}

// EffectDTO is one effect of an ability (ADR 0049 step 10); "dice" and "heal"
// exist so far: dice (a heal has dice or full), an optional save category (not
// for a heal) and an optional label.
type EffectDTO struct {
	Kind  string `json:"kind"`
	Dice  string `json:"dice,omitempty"`
	Full  bool   `json:"full,omitempty"`
	Save  string `json:"save,omitempty"`
	Label string `json:"label,omitempty"`
}

// AbilitiesFromDTO is the domain list sent by the editor, in the order sent;
// an empty list is empty, not nil.
func AbilitiesFromDTO(dtos []AbilityDTO) []itemsvc.Ability {
	out := make([]itemsvc.Ability, 0, len(dtos))
	for _, d := range dtos {
		a := itemsvc.Ability{ID: d.ID, Name: d.Name, Note: d.Note, Limit: itemsvc.Limit{
			Kind: itemsvc.LimitKind(d.Limit.Kind), Max: d.Limit.Max, Left: d.Limit.Left, Cost: d.Limit.Cost,
		}}
		for _, e := range d.Effects {
			a.Effects = append(a.Effects, itemsvc.Effect{Kind: itemsvc.EffectKind(e.Kind), Dice: e.Dice, Full: e.Full, Save: e.Save, Label: e.Label})
		}
		out = append(out, a)
	}
	return out
}

// AbilitiesToDTO is the wire list of an item's abilities, each with whether it
// can be used now: nil (omitted) for a piece not converted yet.
func AbilitiesToDTO(item itemsvc.Item) *[]AbilityDTO {
	if item.Abilities == nil {
		return nil
	}
	out := make([]AbilityDTO, 0, len(item.Abilities))
	for _, a := range item.Abilities {
		reason := item.UseRefusal(a)
		effects := make([]EffectDTO, 0, len(a.Effects))
		for _, e := range a.Effects {
			effects = append(effects, EffectDTO{Kind: string(e.Kind), Dice: e.Dice, Full: e.Full, Save: e.Save, Label: e.Label})
		}
		out = append(out, AbilityDTO{
			ID: a.ID, Name: a.Name, Note: a.Note, Effects: effects, Usable: reason == "", Reason: reason,
			Limit: LimitDTO{Kind: string(a.Limit.Kind), Max: a.Limit.Max, Left: a.Limit.Left, Cost: a.Limit.Cost},
		})
	}
	return &out
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

// ItemsToDTO maps items to their wire shape, each with its class warnings
// (ADR 0033 step 20) from warnings, the character's, computed once.
func ItemsToDTO(items []itemsvc.Item, warnings []itemsvc.Warning) []ItemDTO {
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
		codes := []string{}
		for _, w := range warnings {
			if w.ItemID == i.ID {
				codes = append(codes, w.Code)
			}
		}
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Category: string(i.Category), MagicKind: i.MagicKind,
			CatalogueCode: i.CatalogueCode, Component: i.Component, SpellID: i.SpellID, State: string(i.State), EquipState: string(i.EquipState()), Usable: i.Usable(),
			Type: i.Type, Behaviour: string(i.Behaviour),
			MagicalBonus: i.MagicBonus, AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: powers, Abilities: AbilitiesToDTO(i),
			DamageTypes: damageTypes, DamageSM: i.DamageSM, DamageL: i.DamageL,
			WeaponType: i.WeaponType, WeaponKind: i.WeaponKind, Size: i.Size, Speed: i.Speed,
			MagicToHit: i.MagicToHit, MagicDamage: i.MagicDamage, TwoHanded: i.TwoHanded, Fires: i.Fires,
			StrengthRating: StrengthRatingToDTO(i.StrengthRating),
			Specialized:    i.Specialized, NonproficientPenalty: i.NonproficientPenalty, BaseAC: i.BaseAC, SkillColumn: i.SkillColumn, ArmorRow: i.ArmorRow,
			ShieldKind: i.ShieldKind, ACMelee: i.ACMelee, ACMissile: i.ACMissile, Review: []string{}, Warnings: codes,
		})
	}
	return out
}
