package spelleffectsvc

import (
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/pkg/formula"
)

// Row is what a spell puts on the buff list when the character casts it,
// receives it or adds it as an effect (ADR 0004 step 20): its effects when
// it adds a buff (step 19's rule), and its rounds when it lasts a number of
// rounds. A spell with neither puts nothing on.
type Row struct {
	// Effects: the spell adds a buff (AddsBuff), with its effects.
	Effects bool
	// Rounds is how many rounds it lasts at the caster level (nil: it does
	// not last a number of rounds).
	Rounds *formula.Formula
}

// Any reports whether the spell puts a row on the list: what the lookup
// offers as an effect and what the add-as-effect action takes.
func (r Row) Any() bool { return r.Effects || r.Rounds != nil }

// RowOf is the one rule of ADR 0004 step 20: what a spell puts on the list.
// entry is its effect entry (nil: none) and catalogue its duration as the
// catalogue parsed it. The entry, when there is one, says how long the
// spell lasts (its rounds formula); without one (the Priest spells), the
// catalogue's duration converted to rounds does (step 20.1). Any other
// duration (instantaneous, permanent, concentration, special, "until …")
// gets no rounds (step 20.2); an "until discharged" buff keeps its charges.
func RowOf(entry *Entry, catalogue spellstats.Duration) Row {
	if entry != nil {
		r := Row{Effects: entry.AddsBuff()}
		if entry.Duration.Kind == DurationRounds && entry.Duration.Rounds != nil {
			r.Rounds = entry.Duration.Rounds
		}
		return r
	}
	src, ok := catalogue.RoundsFormula()
	if !ok {
		return Row{}
	}
	f, err := formula.Parse(src)
	if err != nil {
		return Row{}
	}
	return Row{Rounds: f}
}

// RowOnSelf says whether a row with no effects that the character casts
// lands on him (ADR 0004 step 20.5): a self-only spell always; otherwise
// his explicit choice; else a spell on someone else or on an area does not,
// and every other one does (self or other, no target: Polymorph Self, no
// effect entry). A spell that adds a buff picks its target as a buff does
// (DefaultOnSelf).
func RowOnSelf(entry *Entry, onSelf *bool) bool {
	if entry != nil && entry.Target == TargetSelf {
		return true
	}
	if onSelf != nil {
		return *onSelf
	}
	return entry == nil || (entry.Target != TargetOther && entry.Target != TargetArea)
}
