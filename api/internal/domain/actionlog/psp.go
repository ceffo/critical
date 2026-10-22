package actionlog

import (
	"encoding/json"
	"fmt"
)

// The PSP ledger, ADR 0043 step 5, modelled on the HP ledger (ADR 0007 step
// 2, ADR 0018 step 4). Any entry may carry a signed PSP delta; current PSPs
// are psp_max + SUM(effective psp_delta), so the pool is full exactly when
// that SUM (the "total") is 0. The bounds are kept when an entry is written,
// never by rewriting the ledger:
//   - a gain is logged capped at the PSPs spent: min(gain, -total) (the book
//     recovers no more than was lost), 0 when the pool is full;
//   - a loss is logged floored so the pool never goes under 0;
//   - the displayed current PSPs are clamped to 0..psp_max, a guard for a
//     maximum entered lower than the ledger already spent.
// Undo pops the entry (a popped entry counts 0); no correction amends a PSP
// delta.

// PSPTotal is the ledger total of ADR 0043 step 5: the sum of the effective
// PSP deltas of every entry of the character (Entry.EffectivePSPDelta: a
// popped entry counts 0). Current PSPs are psp_max plus it. Every kind
// counts. This is the rule's one implementation; storage only reads the rows.
func PSPTotal(entries []Entry) int {
	total := 0
	for _, e := range entries {
		total += e.EffectivePSPDelta()
	}
	return total
}

// EffectivePSPDelta is 0 for a voided entry, the entry's own delta otherwise
// (ADR 0043 step 5: entries are never edited, undo pops them).
func (e Entry) EffectivePSPDelta() int {
	if e.Status() == StatusVoided {
		return 0
	}
	return e.PSPDelta
}

// CapPSP returns the PSP delta to log for a requested delta when the
// character's maximum is maxPSP and the ledger total is total (current PSPs
// are maxPSP+total), ADR 0043 step 5: a gain is clipped to the PSPs spent (0
// when the pool is full), a loss to the PSPs left (0 when none are left).
func CapPSP(maxPSP, total, delta int) int {
	if delta > 0 {
		return min(delta, max(0, -total))
	}
	return max(delta, min(0, -(maxPSP+total)))
}

// CurrentPSP is a character's current PSPs: the maximum plus the ledger
// total, shown within 0..maxPSP (ADR 0043 step 5).
func CurrentPSP(maxPSP, total int) int {
	return max(0, min(maxPSP, maxPSP+total))
}

// PSPBreakdown is the requested vs applied PSP change, merged into the
// payload of an entry whose delta CapPSP changed as pspRequested/pspApplied
// (ADR 0043 step 5).
type PSPBreakdown struct {
	Requested int `json:"pspRequested"`
	Applied   int `json:"pspApplied"`
}

// WithPSPBreakdown merges b into payload, a JSON object (empty means {}).
func WithPSPBreakdown(payload json.RawMessage, b PSPBreakdown) (json.RawMessage, error) {
	obj := map[string]json.RawMessage{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
			return nil, fmt.Errorf("psp breakdown: payload is not a JSON object")
		}
	}
	obj["pspRequested"], _ = json.Marshal(b.Requested)
	obj["pspApplied"], _ = json.Marshal(b.Applied)
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("psp breakdown: %w", err)
	}
	return out, nil
}

// PSPState is a character's PSP pool after a change (ADR 0043 step 5): the
// current PSPs (CurrentPSP) and the stored maximum.
type PSPState struct {
	Current int
	Max     int
}
