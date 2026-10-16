package actionlog

// StandingVoids maps every entry the corrections cs pop to the id of its
// standing void (ADR 0019 step 1: an entry is popped when its newest void or
// redo is a void). cs may be in any order and mix entries; amends are
// ignored. The storage adapter applies it to choose the undo and redo
// targets and the active entries, so the rule has this one implementation.
func StandingVoids(cs []Correction) map[int64]int64 {
	newest := map[int64]Correction{}
	for _, c := range cs {
		if c.Kind != CorrectionVoid && c.Kind != CorrectionRedo {
			continue
		}
		if n, ok := newest[c.ActionID]; !ok || c.ID > n.ID {
			newest[c.ActionID] = c
		}
	}
	voids := map[int64]int64{}
	for id, c := range newest {
		if c.Kind == CorrectionVoid {
			voids[id] = c.ID
		}
	}
	return voids
}

// LatestPopped returns the entry whose standing void is the newest (popped
// last and not redone since, ADR 0019 step 2) among voids, a map from
// StandingVoids; ok is false when no entry is popped.
func LatestPopped(voids map[int64]int64) (id int64, ok bool) {
	var newest int64
	for entry, void := range voids {
		if !ok || void > newest {
			id, newest, ok = entry, void, true
		}
	}
	return id, ok
}
