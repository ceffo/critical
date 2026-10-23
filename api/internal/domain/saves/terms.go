package saves

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// Inputs are what a character's save is built from, as the view states it
// (ADR 0046 step 3.3): the five targets and what the roll's terms come from.
// The caller (charactersvc.View.SaveInputs) fills it from the derived block.
type Inputs struct {
	// Saves are the five targets (PHB Table 60, the best of the entries
	// that count).
	Saves Saves
	// Race is the race's bonus per category (ADR 0030 step 22.1) and RaceID
	// the race code, the race term's source.
	Race   Saves
	RaceID string
	// MagicDefence is PHB Table 5's adjustment at Wis (ADR 0006 step 9).
	MagicDefence int
	Wis          int
	// Class are the class save_bonus grants, the best per tag (ADR 0043
	// step 3).
	Class []ClassBonus
}

// Flags are the request's flags the player sets before the roll.
type Flags struct {
	// MindAffecting marks a save against magic that attacks the mind (ADR
	// 0006 step 9.3, the DM's call).
	MindAffecting bool
}

// carries says whether a request with these flags carries the tag a class
// save_bonus is against; the one tag is the mind-affecting flag (ADR 0043
// Q12).
func (fl Flags) carries(tag string) bool {
	return tag == classes.SaveAgainstMindAffecting && fl.MindAffecting
}

// Term is one named modifier of a save roll (ADR 0046 step 1.3): its code,
// its source (a race code, the Wisdom score, a class code) and its value on
// the roll. Off marks a term shown that does not add; no save term is Off
// today. The owner never words a term: the web does.
type Term struct {
	Code   string
	Source string
	Value  int
	Off    bool
}

// The term codes, the modifier kinds of the save breakdown.
const (
	TermRace   = "race"
	TermWisdom = "wisdom"
	TermClass  = "class"
)

// Check refuses a code that is none of Categories (invalid input).
func Check(cat string) error {
	if !slices.Contains(Categories, cat) {
		return fmt.Errorf("%w: unknown save category %q", domain.ErrInvalidInput, cat)
	}
	return nil
}

// Target is the save's target in category cat (ADR 0030 step 6, amending ADR
// 0006 step 1): the figure's Table 60 value, never moved by a term. An
// unknown category is refused.
func Target(f Inputs, cat string) (int, error) {
	if err := Check(cat); err != nil {
		return 0, err
	}
	v, _ := f.Saves.Of(cat)
	return v, nil
}

// Terms are the save's named modifiers on the roll in category cat, in order
// (ADR 0046 step 3.3): the race's bonus (ADR 0030 step 22.1), the Wisdom
// adjustment when the request is mind-affecting (ADR 0006 step 9.4, in any
// category), then each class save_bonus whose tag the request carries (ADR
// 0043 step 3). A zero race or Wisdom value adds no term. Nil for none; the
// caller appends the buffs' terms.
func Terms(f Inputs, cat string, fl Flags) []Term {
	var out []Term
	if b, _ := f.Race.Of(cat); b != 0 {
		out = append(out, Term{Code: TermRace, Source: f.RaceID, Value: b})
	}
	if fl.MindAffecting && f.MagicDefence != 0 {
		out = append(out, Term{Code: TermWisdom, Source: strconv.Itoa(f.Wis), Value: f.MagicDefence})
	}
	for _, b := range f.Class {
		if fl.carries(b.Against) {
			out = append(out, Term{Code: TermClass, Source: b.Class, Value: b.Bonus})
		}
	}
	return out
}
