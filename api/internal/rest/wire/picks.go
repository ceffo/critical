package wire

import "github.com/ceffo/critical/api/internal/domain/service/charactersvc"

// PicksDTO is a character's proficiency picks as the editor and the creation send them (ADR 0037
// step 12): `nonweapon` the non-weapon picks, each a catalogue id and the slots spent on it, in
// the player's order. Absent or null is none.
type PicksDTO struct {
	Nonweapon []PickInputDTO `json:"nonweapon"`
}

// PickInputDTO is one pick: the catalogue id and the slots spent.
type PickInputDTO struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// Picks maps the body to the domain picks; nil is none.
func (p *PicksDTO) Picks() charactersvc.Picks {
	out := charactersvc.Picks{Nonweapon: []charactersvc.Pick{}}
	if p == nil {
		return out
	}
	for _, x := range p.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, charactersvc.Pick{ID: x.ID, Slots: x.Slots})
	}
	return out
}

// ProficiencySlotsDTO is the slot pools (ADR 0037 step 3; only the
// non-weapon one is built) and the non-weapon picks with their figures.
type ProficiencySlotsDTO struct {
	NonWeapon NonweaponPoolDTO `json:"nonWeapon"`
	Picks     []PickDTO        `json:"picks"`
}

// NonweaponPoolDTO is ADR 0037 step 3's non-weapon pool: `available` =
// `initial` (Table 34) + `levelGained` (one slot at every realized level
// divisible by `perLevels`) + `intelligence` (Table 4's languages), `spent`
// by the picks, `left` = available - spent (negative over budget).
type NonweaponPoolDTO struct {
	Initial      int `json:"initial"`
	PerLevels    int `json:"perLevels"`
	LevelGained  int `json:"levelGained"`
	Intelligence int `json:"intelligence"`
	Available    int `json:"available"`
	Spent        int `json:"spent"`
	Left         int `json:"left"`
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
func ProficiencySlotsToDTO(p *charactersvc.ProficiencySlots) *ProficiencySlotsDTO {
	if p == nil {
		return nil
	}
	out := &ProficiencySlotsDTO{NonWeapon: NonweaponPoolDTO(p.Nonweapon), Picks: make([]PickDTO, 0, len(p.Picks))}
	for _, f := range p.Picks {
		d := PickDTO{
			ID: f.ID, Slots: f.Slots, Cost: f.Cost, ExtraSlots: f.Extra, Ability: string(f.Ability), Modifier: f.Modifier, Known: f.Known,
		}
		if f.HasCheck {
			target := f.Target
			d.Target = &target
		}
		out.Picks = append(out.Picks, d)
	}
	return out
}
