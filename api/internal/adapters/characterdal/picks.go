package characterdal

import (
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/skills"
)

// picksJSON is the stored form of characters.proficiency_picks (migrations 025, 026 and 042): an
// object by pool, {"nonweapon": [{"id", "slots"}], "weapon": [{"code", "slots"}]}, and the type
// picks left to redo, "weapon_redo": [{"type", "slots"}] (ADR 0037 step 21.8.3); an absent pool
// is none, '{}' no pick at all. Before migration 042 a weapon pick was {"type", "slots"}: the
// migration's data step turns those into codes; one read after it (written by the release before,
// in a rolling deploy) is read as a type pick to redo, never as a weapon. Add pools, never rename.
type picksJSON struct {
	Nonweapon  []pickJSON       `json:"nonweapon,omitempty"`
	Weapon     []weaponPickJSON `json:"weapon,omitempty"`
	WeaponRedo []weaponRedoJSON `json:"weapon_redo,omitempty"`
}

type weaponPickJSON struct {
	Code  string `json:"code,omitempty"`
	Type  string `json:"type,omitempty"`
	Slots int    `json:"slots"`
}

type weaponRedoJSON struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

type pickJSON struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// EncodePicks writes characters.proficiency_picks; no pick is '{}'. The seed and the migration's
// data step write the column with it too.
func EncodePicks(p skills.Picks) (string, error) {
	var out picksJSON
	for _, x := range p.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, pickJSON(x))
	}
	for _, x := range p.Weapon {
		out.Weapon = append(out.Weapon, weaponPickJSON{Code: x.Code, Slots: x.Slots})
	}
	for _, x := range p.WeaponRedo {
		out.WeaponRedo = append(out.WeaponRedo, weaponRedoJSON(x))
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode proficiency picks: %w", err)
	}
	return string(raw), nil
}

// DecodePicks reads characters.proficiency_picks; the lists are never nil.
func DecodePicks(raw string) (skills.Picks, error) {
	var in picksJSON
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return skills.Picks{}, fmt.Errorf("decode proficiency picks: %w", err)
	}
	out := skills.Picks{
		Nonweapon: make([]skills.Pick, 0, len(in.Nonweapon)), Weapon: make([]skills.WeaponPick, 0, len(in.Weapon)),
	}
	for _, x := range in.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, skills.Pick(x))
	}
	for _, x := range in.Weapon {
		if x.Code == "" && x.Type != "" {
			// A type pick of the older form (ADR 0037 step 21.8.3): to redo.
			out.WeaponRedo = append(out.WeaponRedo, skills.WeaponRedo{Type: x.Type, Slots: x.Slots})
			continue
		}
		out.Weapon = append(out.Weapon, skills.WeaponPick{Code: x.Code, Slots: x.Slots})
	}
	for _, x := range in.WeaponRedo {
		out.WeaponRedo = append(out.WeaponRedo, skills.WeaponRedo(x))
	}
	return out, nil
}
