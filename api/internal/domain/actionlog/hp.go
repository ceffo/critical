package actionlog

import (
	"encoding/json"
	"fmt"
)

// HP cap (critical-qki.38; ADR 0007 once qki.37 writes it, until then
// docs/backend.md "HP cap"). User, 2026-10-13: "a player cannot go above its
// max hp. period. max hp grows when leveling up only". No temporary HP.
//
// Current HP = hp_max + SUM(effective hp_delta), so HP is at or below max
// exactly when that SUM (the "total") is <= 0. The rule is enforced when an
// entry is written, never by rewriting the ledger:
//   - HP-1 a heal is logged capped at the gap to max: min(heal, -total);
//   - HP-2 at full HP the heal is logged as 0 (the event is kept, its payload
//     says what was requested);
//   - HP-3 an amend is capped the same way against the total without the
//     amended entry; an amend that would leave HP above max otherwise is refused;
//   - HP-4 voiding damage that a later capped heal relies on is refused;
//   - HP-5 the displayed current HP is min(hp_max, hp_max + total), a guard
//     for rows logged before the cap existed.

// CapHeal returns the HP delta to log for a requested delta when the ledger
// total is total (HP-1, HP-2). Damage (delta <= 0) is unchanged; a heal is
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
