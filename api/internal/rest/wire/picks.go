package wire

import "github.com/ceffo/critical/api/internal/domain/skills"

// PicksDTO is a character's proficiency picks as the editor and the creation send them (ADR 0037
// step 12): `nonweapon` the non-weapon picks, each a catalogue id and the slots spent on it,
// `weapon` the weapon picks, each a weapon type (ADR 0030 step 23) and its slots (1), in the
// player's order. A pool absent or null is left as stored (none at creation); an empty list
// removes its picks.
type PicksDTO struct {
	Nonweapon []PickInputDTO       `json:"nonweapon"`
	Weapon    []WeaponPickInputDTO `json:"weapon"`
}

// WeaponPickInputDTO is one weapon pick: the weapon type and the slots spent.
type WeaponPickInputDTO struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

// PickInputDTO is one pick: the catalogue id and the slots spent.
type PickInputDTO struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// Picks maps the body to the domain picks: a pool absent from the body stays nil (left as
// stored, skills.ValidPicks); nil is no pick in either pool.
func (p *PicksDTO) Picks() skills.Picks {
	if p == nil {
		return skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{}}
	}
	var out skills.Picks
	if p.Nonweapon != nil {
		out.Nonweapon = make([]skills.Pick, 0, len(p.Nonweapon))
		for _, x := range p.Nonweapon {
			out.Nonweapon = append(out.Nonweapon, skills.Pick{ID: x.ID, Slots: x.Slots})
		}
	}
	if p.Weapon != nil {
		out.Weapon = make([]skills.WeaponPick, 0, len(p.Weapon))
		for _, x := range p.Weapon {
			out.Weapon = append(out.Weapon, skills.WeaponPick{Type: x.Type, Slots: x.Slots})
		}
	}
	return out
}

// ProficiencySlotsDTO is the slot pools (ADR 0037 step 3), the non-weapon
// picks with their figures (`picks`) and the weapon picks (`weaponPicks`).
type ProficiencySlotsDTO struct {
	NonWeapon   NonweaponPoolDTO `json:"nonWeapon"`
	Weapon      WeaponPoolDTO    `json:"weapon"`
	Picks       []PickDTO        `json:"picks"`
	WeaponPicks []WeaponPickDTO  `json:"weaponPicks"`
}

// WeaponPoolDTO is ADR 0037 step 3's weapon pool: `available` = `initial`
// (Table 34) + `levelGained` (one slot at every realized level divisible by
// `perLevels`), `spent` by the weapon picks, `left` = available - spent
// (negative over budget).
type WeaponPoolDTO struct {
	Initial     int `json:"initial"`
	PerLevels   int `json:"perLevels"`
	LevelGained int `json:"levelGained"`
	Available   int `json:"available"`
	Spent       int `json:"spent"`
	Left        int `json:"left"`
}

// WeaponPickDTO is one weapon pick: the weapon type, the slots spent and
// `allowed` false for a type the class may not pick (ADR 0037 step 7), kept
// from older data but never added.
type WeaponPickDTO struct {
	Type    string `json:"type"`
	Slots   int    `json:"slots"`
	Allowed bool   `json:"allowed"`
}

// NonweaponPoolDTO is ADR 0037 step 3's non-weapon pool: `available` =
// `initial` (Table 34) + `levelGained` (one slot at every realized level
// divisible by `perLevels`) + `intelligence` (Table 4's languages), `spent`
// by the picks and the languages, `left` = available - spent (negative over
// budget). `languages` is the part of `spent` the languages beyond the native
// one and Common spend (step 6). Additive.
type NonweaponPoolDTO struct {
	Initial      int `json:"initial"`
	PerLevels    int `json:"perLevels"`
	LevelGained  int `json:"levelGained"`
	Intelligence int `json:"intelligence"`
	Available    int `json:"available"`
	Spent        int `json:"spent"`
	Left         int `json:"left"`
	Languages    int `json:"languages"`
}

// PickDTO is one non-weapon pick: the slots spent, its cost for the class
// (ADR 0037 step 4), the `extraSlots` beyond it (+1 each to the check, step
// 5), the check's `ability` and `modifier` from the catalogue (`ability` ""
// for a proficiency with no check) and `known` false for an id the
// catalogue does not hold (kept from older data, no check).
type PickDTO struct {
	ID         string `json:"id"`
	Slots      int    `json:"slots"`
	Cost       int    `json:"cost"`
	ExtraSlots int    `json:"extraSlots"`
	Ability    string `json:"ability"`
	Modifier   int    `json:"modifier"`
	Known      bool   `json:"known"`
	// Target is the check's target (ability score + modifier + extraSlots,
	// ADR 0037 step 5); null for a pick with no check.
	Target *int `json:"target"`
}

// ProficiencySlotsToDTO maps the derived slots; nil stays nil.
func ProficiencySlotsToDTO(p *skills.ProficiencySlots) *ProficiencySlotsDTO {
	if p == nil {
		return nil
	}
	out := &ProficiencySlotsDTO{
		NonWeapon: NonweaponPoolDTO(p.Nonweapon), Weapon: WeaponPoolDTO(p.Weapon),
		Picks: make([]PickDTO, 0, len(p.Picks)), WeaponPicks: make([]WeaponPickDTO, 0, len(p.WeaponPicks)),
	}
	for _, w := range p.WeaponPicks {
		out.WeaponPicks = append(out.WeaponPicks, WeaponPickDTO(w))
	}
	for _, f := range p.Picks {
		d := PickDTO{
			ID: f.ID, Slots: f.Slots, Cost: f.Cost, ExtraSlots: f.Extra, Ability: f.Ability, Modifier: f.Modifier, Known: f.Known,
		}
		if f.HasCheck {
			target := f.Target
			d.Target = &target
		}
		out.Picks = append(out.Picks, d)
	}
	return out
}
