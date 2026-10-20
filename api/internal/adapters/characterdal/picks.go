package characterdal

import (
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// picksJSON is the stored form of characters.proficiency_picks (migrations 025 and 026): an
// object by pool, {"nonweapon": [{"id", "slots"}], "weapon": [{"type", "slots"}]}; an absent pool
// is none, '{}' no pick at all. Add pools, never rename.
type picksJSON struct {
	Nonweapon []pickJSON       `json:"nonweapon,omitempty"`
	Weapon    []weaponPickJSON `json:"weapon,omitempty"`
}

type weaponPickJSON struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

type pickJSON struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// EncodePicks writes characters.proficiency_picks; no pick is '{}'. The seed and the migration's
// data step write the column with it too.
func EncodePicks(p charactersvc.Picks) (string, error) {
	var out picksJSON
	for _, x := range p.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, pickJSON(x))
	}
	for _, x := range p.Weapon {
		out.Weapon = append(out.Weapon, weaponPickJSON(x))
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode proficiency picks: %w", err)
	}
	return string(raw), nil
}

// DecodePicks reads characters.proficiency_picks; the lists are never nil.
func DecodePicks(raw string) (charactersvc.Picks, error) {
	var in picksJSON
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return charactersvc.Picks{}, fmt.Errorf("decode proficiency picks: %w", err)
	}
	out := charactersvc.Picks{
		Nonweapon: make([]charactersvc.Pick, 0, len(in.Nonweapon)), Weapon: make([]charactersvc.WeaponPick, 0, len(in.Weapon)),
	}
	for _, x := range in.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, charactersvc.Pick(x))
	}
	for _, x := range in.Weapon {
		out.Weapon = append(out.Weapon, charactersvc.WeaponPick(x))
	}
	return out, nil
}
