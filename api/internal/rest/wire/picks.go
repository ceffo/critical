package wire

import "github.com/ceffo/critical/api/internal/domain/skills"

// PicksDTO is a character's proficiency picks as the editor and the creation send them (ADR 0037
// step 12): `nonweapon` the non-weapon picks, each a catalogue id and the slots spent on it,
// `weapon` the weapon picks, each a catalogue weapon code (ADR 0037 step 21.6) and its slots (1,
// or the specialization's), in the player's order. A pool absent or null is left as stored (none
// at creation); an empty list removes its picks, and a sent weapon pool clears the type picks
// left to redo (step 21.8.3). A pick without a `code` (an older client's `type`) is refused.
type PicksDTO struct {
	Nonweapon []PickInputDTO       `json:"nonweapon"`
	Weapon    []WeaponPickInputDTO `json:"weapon"`
}

// WeaponPickInputDTO is one weapon pick: the catalogue weapon code and the slots spent. Type is
// an older client's weapon type pick (ADR 0037 step 21.6): read so the body decodes, never used, so
// the pick reaches the rules without a code and is refused empty on field weapon.
type WeaponPickInputDTO struct {
	Code  string `json:"code"`
	Slots int    `json:"slots"`
	Type  string `json:"type,omitempty"`
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
			out.Weapon = append(out.Weapon, skills.WeaponPick{Code: x.Code, Slots: x.Slots})
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
	// Weapons are the catalogue weapons the character may pick, with the slots a specialization
	// of each spends (0 for none), ADR 0042 step 16.1 as amended by ADR 0037 step 21.6, in offer
	// order. It replaces weaponTypes.
	Weapons []WeaponDTO `json:"weapons"`
	// WeaponRedo are the weapon type picks left to redo (step 21.8.3), each also warned
	// weapon_pick_to_redo.
	WeaponRedo []WeaponRedoDTO `json:"weaponRedo"`
	// Groups are the character's Table 38 groups (every active class entry's, ADR 0042 step 15)
	// and Catalogue each pickable catalogue proficiency's cost for the character, what a preview
	// and a save charge: the editor lists and prices the catalogue from these, never from the
	// first class's creation options. Additive.
	Groups    []string       `json:"groups"`
	Catalogue []CatalogueDTO `json:"catalogue"`
	// WeaponClasses are the class entries the weapon pool is counted on (several on a tie) and
	// NonWeaponClasses the entries the non-weapon pool reads, every active one (ADR 0037 step
	// 23.1); NonProficientPenalty is Table 34's penalty to hit with a weapon the character has
	// no pick of, 0 or less (step 23.2); LanguageCosts the held languages with their slots
	// (step 23.3). Additive.
	WeaponClasses        []ClassRefDTO     `json:"weaponClasses"`
	NonWeaponClasses     []ClassRefDTO     `json:"nonWeaponClasses"`
	NonProficientPenalty int               `json:"nonProficientPenalty"`
	LanguageCosts        []LanguageCostDTO `json:"languageCosts"`
}

// CatalogueDTO is one catalogue proficiency's cost for the character, 0 for an id a class entry
// grants free (ADR 0037 step 22.2).
type CatalogueDTO struct {
	ID   string `json:"id"`
	Cost int    `json:"cost"`
	// Ability and Modifier are the check's (Ability "" for none); Target is the check's target
	// for a new pick, at no extra slot (ADR 0037 step 23.4), null without a check.
	Ability  string `json:"ability"`
	Modifier int    `json:"modifier"`
	Target   *int   `json:"target"`
}

// ClassRefDTO is a class entry as the source of a pool or a skill set (ADR 0037 step 23.1): the
// class code and its level; the client words the name.
type ClassRefDTO struct {
	Class string `json:"class"`
	Level int    `json:"level"`
}

// ClassRefsToDTO maps class refs; never nil.
func ClassRefsToDTO(in []skills.ClassRef) []ClassRefDTO {
	out := make([]ClassRefDTO, 0, len(in))
	for _, r := range in {
		out = append(out, ClassRefDTO(r))
	}
	return out
}

// LanguageCostDTO is one held language and the non-weapon slots it spends (ADR 0037 step 23.3):
// 1, or 0 for the race's native language and Common.
type LanguageCostDTO struct {
	Code  string `json:"code"`
	Slots int    `json:"slots"`
}

// WeaponDTO is one pickable catalogue weapon and its specialization cost.
type WeaponDTO struct {
	Code                string `json:"code"`
	SpecializationSlots int    `json:"specializationSlots"`
}

// WeaponRedoDTO is one weapon type pick left to redo: the type and the slots it spent.
type WeaponRedoDTO struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
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

// WeaponPickDTO is one weapon pick: the catalogue weapon code, the slots
// spent and `allowed` false for a weapon the class may not pick (ADR 0037
// step 21.2), kept from older data but never added.
type WeaponPickDTO struct {
	Code    string `json:"code"`
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
	// Free is true when a class entry grants the id at no slot cost (ADR 0037
	// step 22.2): cost 0, every slot an extra slot.
	Free bool `json:"free"`
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
	out.Weapons = make([]WeaponDTO, 0, len(p.Weapons))
	for _, w := range p.Weapons {
		out.Weapons = append(out.Weapons, WeaponDTO(w))
	}
	out.Groups = append([]string{}, p.Groups...)
	out.Catalogue = make([]CatalogueDTO, 0, len(p.Catalogue))
	for _, c := range p.Catalogue {
		d := CatalogueDTO{ID: c.ID, Cost: c.Cost, Ability: c.Ability, Modifier: c.Modifier}
		if c.HasCheck {
			target := c.Target
			d.Target = &target
		}
		out.Catalogue = append(out.Catalogue, d)
	}
	out.WeaponClasses, out.NonWeaponClasses = ClassRefsToDTO(p.WeaponClasses), ClassRefsToDTO(p.NonweaponClasses)
	out.NonProficientPenalty = p.Penalty
	out.LanguageCosts = make([]LanguageCostDTO, 0, len(p.LanguageCosts))
	for _, l := range p.LanguageCosts {
		out.LanguageCosts = append(out.LanguageCosts, LanguageCostDTO(l))
	}
	out.WeaponRedo = make([]WeaponRedoDTO, 0, len(p.WeaponRedo))
	for _, r := range p.WeaponRedo {
		out.WeaponRedo = append(out.WeaponRedo, WeaponRedoDTO(r))
	}
	for _, w := range p.WeaponPicks {
		out.WeaponPicks = append(out.WeaponPicks, WeaponPickDTO(w))
	}
	for _, f := range p.Picks {
		d := PickDTO{
			ID: f.ID, Slots: f.Slots, Cost: f.Cost, ExtraSlots: f.Extra, Ability: f.Ability, Modifier: f.Modifier, Known: f.Known,
			Free: f.Free,
		}
		if f.HasCheck {
			target := f.Target
			d.Target = &target
		}
		out.Picks = append(out.Picks, d)
	}
	return out
}
