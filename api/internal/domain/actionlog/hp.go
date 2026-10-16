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

// HPTotal is the ledger total of ADR 0007 step 2: the sum of the effective
// HP deltas of every entry of the character, each with its corrections
// (Entry.EffectiveHPDelta: a popped entry counts 0, an amended one its latest
// amend in effect). Current HP is hp_max plus it. Every kind counts (ADR 0018
// step 4). This is the rule's one implementation; storage only reads the
// rows.
func HPTotal(entries []Entry) int {
	total := 0
	for _, e := range entries {
		total += e.EffectiveHPDelta()
	}
	return total
}

// CapHeal returns the HP delta to log for a requested delta when the ledger
// total is total (ADR 0007 steps 4 and 5). Damage (delta <= 0) is unchanged; a heal is
// clipped to the gap to max, and to 0 when HP is at or above max.
func CapHeal(total, delta int) int {
	if delta <= 0 {
		return delta
	}
	return min(delta, max(0, -total))
}

// HPFloor is the lowest current HP a character can have (ADR 0011 step 1):
// at -10 the character is dead, and damage beyond it is not recorded.
const HPFloor = -10

// FloorDamage returns the HP delta to log for a requested delta when the
// character's max HP is maxHP and the ledger total is total (current HP is
// maxHP+total), ADR 0011 step 1. A heal (delta >= 0) is unchanged; damage is
// clipped so current HP stops at HPFloor, and to 0 when HP is already at or
// below it. It mirrors CapHeal at the other end of the scale.
func FloorDamage(maxHP, total, delta int) int {
	if delta >= 0 {
		return delta
	}
	return max(delta, min(0, HPFloor-(maxHP+total)))
}

// HPBreakdown is the requested vs applied HP change, merged into the payload of
// every heal entry and of a damage entry clamped at HPFloor as
// hpRequested/hpApplied.
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
