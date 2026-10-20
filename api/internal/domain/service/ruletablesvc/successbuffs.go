package ruletablesvc

import (
	"maps"
	"slices"
)

// The buffs a successful proficiency check puts on (ADR 0037 step 20.6):
// committed rules data keyed by proficiency id, a house rule each, never a
// case in the check's code. A campaign adds a row; the check reads it.

// SuccessBuff is the buff a proficiency's successful check puts on the
// character (ADR 0037 step 20.1): its AC change (descending AC, negative
// betters it, ADR 0004 step 11) and the rounds it lasts (ADR 0004 step 2: 1
// ends at the next Next Round). The buff is named from the proficiency's id
// by the client, as a spell's buff from its spell id.
type SuccessBuff struct {
	AC     int
	Rounds int
}

// successBuffs is the table. User, triage of 2026-10-20 (house rule):
// "Acrobatics ... reduces the AC by 4 for the round".
var successBuffs = map[string]SuccessBuff{
	"tumbling": {AC: -4, Rounds: 1},
}

// SuccessBuffFor is the buff a successful check of proficiency id puts on;
// false when it puts none (ADR 0037 step 20.6).
func SuccessBuffFor(id string) (SuccessBuff, bool) {
	b, ok := successBuffs[id]
	return b, ok
}

// SuccessBuffIDs are the proficiencies whose successful check puts a buff
// on, sorted.
func SuccessBuffIDs() []string {
	return slices.Sorted(maps.Keys(successBuffs))
}
