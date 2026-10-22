package characterdal

import (
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// EncodePsionicPicks writes characters.psionic_picks (migration 038): a JSON
// list of {"kind", "id"} in the player's order; no pick is '[]'. Add fields,
// never rename.
func EncodePsionicPicks(picks []psionics.Pick) (string, error) {
	if picks == nil {
		picks = []psionics.Pick{}
	}
	raw, err := json.Marshal(picks)
	if err != nil {
		return "", fmt.Errorf("encode psionic picks: %w", err)
	}
	return string(raw), nil
}

// DecodePsionicPicks reads characters.psionic_picks; the list is never nil.
func DecodePsionicPicks(raw string) ([]psionics.Pick, error) {
	out := []psionics.Pick{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode psionic picks: %w", err)
	}
	return out, nil
}
