package actionlog

import (
	"encoding/json"
	"fmt"
)

// HP cap, ADR 0007 steps 4 to 6. User, 2026-10-13: "a player cannot go above
// its max hp. period. max hp grows when leveling up only". No temporary HP.
//
// Current HP = hp_max + SUM(effective hp_delta), so HP is at or below max
// exactly when that SUM (the "total") is <= 0. The rule is enforced when an
// entry is written, never by rewriting the ledger:
//   - a heal is logged capped at the gap to max: min(heal, -total) (step 4);
//   - at full HP the heal is logged as 0 (the event is kept, its payload says
//     what was requested) (step 5);
//   - the displayed current HP is min(hp_max, hp_max + total), a guard for
//     rows logged before the cap existed (step 6).
// An amend sets only the note and a void pops the newest entry (step 8), so
// neither can push HP over max.

// CapHeal returns the HP delta to log for a requested delta when the ledger
// total is total (ADR 0007 steps 4 and 5). Damage (delta <= 0) is unchanged; a heal is
// clipped to the gap to max, and to 0 when HP is at or above max.
func CapHeal(total, delta int) int {
	if delta <= 0 {
		return delta
	}
	return min(delta, max(0, -total))
}

// HPBreakdown is the requested vs applied heal, merged into the payload of a
// heal entry (and of a heal amend) as hpRequested/hpApplied.
type HPBreakdown struct {
	Requested int `json:"hpRequested"`
	Applied   int `json:"hpApplied"`
}

// WithHPBreakdown merges b into payload, a JSON object (empty means {}).
func WithHPBreakdown(payload json.RawMessage, b HPBreakdown) (json.RawMessage, error) {
	obj := map[string]json.RawMessage{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
			return nil, fmt.Errorf("hp breakdown: payload is not a JSON object")
		}
	}
	obj["hpRequested"], _ = json.Marshal(b.Requested)
	obj["hpApplied"], _ = json.Marshal(b.Applied)
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("hp breakdown: %w", err)
	}
	return out, nil
}
