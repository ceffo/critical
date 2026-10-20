package characterdal

import (
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// picksJSON is the stored form of characters.proficiency_picks (migration 025): an object by
// pool, {"nonweapon": [{"id", "slots"}]}; '{}' is none. Add pools, never rename.
type picksJSON struct {
	Nonweapon []pickJSON `json:"nonweapon,omitempty"`
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
	out := charactersvc.Picks{Nonweapon: make([]charactersvc.Pick, 0, len(in.Nonweapon))}
	for _, x := range in.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, charactersvc.Pick(x))
	}
	return out, nil
}
