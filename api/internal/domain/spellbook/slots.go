package spellbook

import "errors"

// SchoolSlotsPerLevel is ADR 0050 step 10's school slot: a specialist has one
// more slot of each spell level he has slots of, for a spell of his school
// (PHB p.32), the same for every specialist (ADR 0040 step 3.3: a constant of
// the kind's code).
const SchoolSlotsPerLevel = 1

// ErrSchoolOpposed is a spell of a school in the specialist's opposition
// (ADR 0050 steps 6 and 11, PHB p.32): never learned, added, prepared or cast
// from a blank extra slot. One sentinel, so every service refusing it renders
// the one code spell_school_opposed.
var ErrSchoolOpposed = errors.New("spell of an opposition school")

// SchoolSlots is ADR 0050 step 10: the specialist s's school slots per spell
// level, one for each spell level whose allotment (ADR 0030 step 8, the
// entry's slots per day) is 1 or more. Nil for one who is no specialist and
// for an allotment with no slot.
func SchoolSlots(s Specialization, allotment map[int]int) map[int]int {
	if !s.Specialist() {
		return nil
	}
	var out map[int]int
	for level, n := range allotment {
		if n > 0 {
			if out == nil {
				out = map[int]int{}
			}
			out[level] = SchoolSlotsPerLevel
		}
	}
	return out
}
