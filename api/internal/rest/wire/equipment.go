package wire

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// EquipmentCatalogueDTO is GET /api/equipment/catalogue: the catalogue's
// equipment rows a piece is picked from (ADR 0033 steps 21 and 24), in the
// artifact's order; numbers and codes, names as language maps (ADR 0023).
// Every list is empty before catalogue format 7. Damage is as printed
// ("1d4+1", "1"), empty for a launcher.
type EquipmentCatalogueDTO struct {
	Weapons    []CatalogueWeaponDTO     `json:"weapons"`
	Ammunition []CatalogueAmmunitionDTO `json:"ammunition"`
	Armour     []CatalogueArmourDTO     `json:"armour"`
	Shields    []CatalogueShieldDTO     `json:"shields"`
}

// CatalogueWeaponDTO is one catalogue weapon (PHB Table 44).
type CatalogueWeaponDTO struct {
	Code        string         `json:"code"`
	Name        localized.Text `json:"name"`
	WeaponType  string         `json:"weaponType"`
	WeaponKind  string         `json:"weaponKind"`
	Size        string         `json:"size"`
	DamageTypes []string       `json:"damageTypes"`
	Speed       int            `json:"speed"`
	DamageSM    string         `json:"damageSm"`
	DamageL     string         `json:"damageL"`
	TwoHanded   bool           `json:"twoHanded"`
	Fires       []string       `json:"fires"`
}

// CatalogueAmmunitionDTO is one catalogue ammunition row.
type CatalogueAmmunitionDTO struct {
	Code        string         `json:"code"`
	Name        localized.Text `json:"name"`
	AmmoKind    string         `json:"ammoKind"`
	DamageTypes []string       `json:"damageTypes"`
	DamageSM    string         `json:"damageSm"`
	DamageL     string         `json:"damageL"`
}

// CatalogueArmourDTO is one catalogue body armour.
type CatalogueArmourDTO struct {
	Code        string         `json:"code"`
	Name        localized.Text `json:"name"`
	BaseAC      int            `json:"baseAc"`
	SkillColumn string         `json:"skillColumn"`
	ArmorRow    string         `json:"armorRow"`
}

// CatalogueShieldDTO is one catalogue shield.
type CatalogueShieldDTO struct {
	Code       string         `json:"code"`
	Name       localized.Text `json:"name"`
	ShieldKind string         `json:"shieldKind"`
	ACMelee    int            `json:"acMelee"`
	ACMissile  int            `json:"acMissile"`
}

func list(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// EquipmentCatalogueToDTO maps the equipment reference data to its wire
// shape.
func EquipmentCatalogueToDTO(e ruletablesvc.Equipment) EquipmentCatalogueDTO {
	out := EquipmentCatalogueDTO{
		Weapons: make([]CatalogueWeaponDTO, 0, len(e.Weapons)), Ammunition: make([]CatalogueAmmunitionDTO, 0, len(e.Ammunition)),
		Armour: make([]CatalogueArmourDTO, 0, len(e.Armour)), Shields: make([]CatalogueShieldDTO, 0, len(e.Shields)),
	}
	for _, w := range e.Weapons {
		out.Weapons = append(out.Weapons, CatalogueWeaponDTO{
			Code: w.Code, Name: w.Name, WeaponType: w.WeaponType, WeaponKind: w.Kind, Size: w.Size,
			DamageTypes: list(w.Types), Speed: w.Speed, DamageSM: w.DamageSM, DamageL: w.DamageL,
			TwoHanded: w.TwoHanded, Fires: list(w.Fires),
		})
	}
	for _, a := range e.Ammunition {
		out.Ammunition = append(out.Ammunition, CatalogueAmmunitionDTO{
			Code: a.Code, Name: a.Name, AmmoKind: a.AmmoKind, DamageTypes: list(a.Types), DamageSM: a.DamageSM, DamageL: a.DamageL,
		})
	}
	for _, a := range e.Armour {
		out.Armour = append(out.Armour, CatalogueArmourDTO{
			Code: a.Code, Name: a.Name, BaseAC: a.BaseAC, SkillColumn: a.SkillColumn, ArmorRow: a.ArmorRow,
		})
	}
	for _, s := range e.Shields {
		out.Shields = append(out.Shields, CatalogueShieldDTO{
			Code: s.Code, Name: s.Name, ShieldKind: s.Kind, ACMelee: s.ACMelee, ACMissile: s.ACMissile,
		})
	}
	return out
}
