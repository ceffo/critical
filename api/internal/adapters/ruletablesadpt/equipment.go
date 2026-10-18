package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The format-7 equipment object and its two rule tables (ADR 0021 step 13,
// ADR 0033 steps 21 to 23): private JSON DTOs and the mapping; the entities
// and their validation live in ruletablesvc.

type equipmentDTO struct {
	Weapons    []weaponDTO     `json:"weapons"`
	Ammunition []ammunitionDTO `json:"ammunition"`
	Armour     []armourDTO     `json:"armour"`
	Shields    []shieldDTO     `json:"shields"`
}

type weaponDTO struct {
	Code       string         `json:"code"`
	Name       localized.Text `json:"name"`
	WeaponType string         `json:"weapon_type"`
	Kind       string         `json:"kind"`
	Size       string         `json:"size"`
	Types      []string       `json:"damage_types"`
	Speed      int            `json:"speed"`
	DamageSM   string         `json:"damage_sm"`
	DamageL    string         `json:"damage_l"`
	TwoHanded  bool           `json:"two_handed"`
	Fires      []string       `json:"fires"`
}

type ammunitionDTO struct {
	Code     string         `json:"code"`
	Name     localized.Text `json:"name"`
	AmmoKind string         `json:"ammo_kind"`
	Types    []string       `json:"damage_types"`
	DamageSM string         `json:"damage_sm"`
	DamageL  string         `json:"damage_l"`
}

type armourDTO struct {
	Code        string         `json:"code"`
	Name        localized.Text `json:"name"`
	BaseAC      int            `json:"base_ac"`
	SkillColumn string         `json:"skill_column"`
	ArmorRow    string         `json:"armor_row"`
}

type shieldDTO struct {
	Code      string         `json:"code"`
	Name      localized.Text `json:"name"`
	Kind      string         `json:"kind"`
	ACMelee   int            `json:"ac_melee"`
	ACMissile int            `json:"ac_missile"`
}

type weaponVsArmorDTO struct {
	ArmorRow string `json:"armor_row"`
	Slash    int    `json:"s"`
	Pierce   int    `json:"p"`
	Bludgeon int    `json:"b"`
}

type initiativeItemDTO struct {
	Kind     string `json:"kind"`
	Modifier *int   `json:"modifier"`
}

// parseEquipment maps the equipment object to the validated catalogue; format
// 7 requires it.
func parseEquipment(raw []byte) (*ruletablesvc.Equipment, error) {
	if len(raw) == 0 {
		return nil, errors.New("format 7 needs the equipment object")
	}
	var d equipmentDTO
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("equipment: parse: %w", err)
	}
	var e ruletablesvc.Equipment
	for _, w := range d.Weapons {
		e.Weapons = append(e.Weapons, ruletablesvc.Weapon{
			Code: w.Code, Name: w.Name.Clean(), WeaponType: w.WeaponType, Kind: w.Kind, Size: w.Size, Types: w.Types,
			Speed: w.Speed, DamageSM: w.DamageSM, DamageL: w.DamageL, TwoHanded: w.TwoHanded, Fires: w.Fires,
		})
	}
	for _, a := range d.Ammunition {
		e.Ammunition = append(e.Ammunition, ruletablesvc.Ammunition{
			Code: a.Code, Name: a.Name.Clean(), AmmoKind: a.AmmoKind, Types: a.Types, DamageSM: a.DamageSM, DamageL: a.DamageL,
		})
	}
	for _, a := range d.Armour {
		e.Armour = append(e.Armour, ruletablesvc.Armour{
			Code: a.Code, Name: a.Name.Clean(), BaseAC: a.BaseAC, SkillColumn: a.SkillColumn, ArmorRow: a.ArmorRow,
		})
	}
	for _, s := range d.Shields {
		e.Shields = append(e.Shields, ruletablesvc.Shield{
			Code: s.Code, Name: s.Name.Clean(), Kind: s.Kind, ACMelee: s.ACMelee, ACMissile: s.ACMissile,
		})
	}
	out, err := ruletablesvc.NewEquipment(e)
	if err != nil {
		return nil, fmt.Errorf("equipment: %w", err)
	}
	return out, nil
}

// weaponVsArmor maps weapon_vs_armor (Table 52) to the validated table; format
// 7 requires it.
func weaponVsArmor(rows []weaponVsArmorDTO) (*ruletablesvc.WeaponVsArmor, error) {
	if len(rows) == 0 {
		return nil, errors.New("format 7 needs the weapon_vs_armor table")
	}
	out := make([]ruletablesvc.WeaponVsArmorRow, len(rows))
	for i, r := range rows {
		out[i] = ruletablesvc.WeaponVsArmorRow(r)
	}
	t, err := ruletablesvc.NewWeaponVsArmor(out)
	if err != nil {
		return nil, fmt.Errorf("weapon_vs_armor table: %w", err)
	}
	return t, nil
}

// initiativeItems maps initiative_item (Table 56) to the validated table;
// format 7 requires it. An absent modifier is the scroll's (the casting time).
func initiativeItems(rows []initiativeItemDTO) (*ruletablesvc.InitiativeItems, error) {
	if len(rows) == 0 {
		return nil, errors.New("format 7 needs the initiative_item table")
	}
	out := make([]ruletablesvc.InitiativeItemRow, len(rows))
	for i, r := range rows {
		out[i] = ruletablesvc.InitiativeItemRow{Kind: r.Kind, HasModifier: r.Modifier != nil}
		if r.Modifier != nil {
			out[i].Modifier = *r.Modifier
		}
	}
	t, err := ruletablesvc.NewInitiativeItems(out)
	if err != nil {
		return nil, fmt.Errorf("initiative_item table: %w", err)
	}
	return t, nil
}
