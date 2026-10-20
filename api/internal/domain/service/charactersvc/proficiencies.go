package charactersvc

import (
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Non-weapon proficiency picks against their slots (ADR 0037 steps 3 to 5, 10, 11 and 12, the
// non-weapon pool). The rules live here, as the skill points' do; skillsvc writes the picks.

// Pick is one stored non-weapon proficiency pick (ADR 0037 step 12): the catalogue id and the
// slots spent on it. Its ability and modifier are read from the catalogue, never stored.
type Pick struct {
	ID    string
	Slots int
}

// Picks are a character's proficiency picks, by pool. Only the non-weapon pool is built.
type Picks struct {
	Nonweapon []Pick
}

// Equal reports the same picks in the same order.
func (p Picks) Equal(o Picks) bool {
	if len(p.Nonweapon) != len(o.Nonweapon) {
		return false
	}
	for i := range p.Nonweapon {
		if p.Nonweapon[i] != o.Nonweapon[i] {
			return false
		}
	}
	return true
}

// Clone is a copy of p that shares nothing with it; never nil lists.
func (p Picks) Clone() Picks {
	return Picks{Nonweapon: append([]Pick{}, p.Nonweapon...)}
}

// PickFigures is a pick as the rules read it: its cost for the class (step 4), the slots beyond
// it (step 5), the ability and modifier of its check from the catalogue. Known is false for an
// id the catalogue does not hold (kept from older data at its stored slots, no check);
// Ability is "" for a proficiency with no check.
type PickFigures struct {
	ID       string
	Slots    int
	Cost     int
	Extra    int
	Ability  Ability
	Modifier int
	Known    bool
	// Target is the check's target (ADR 0014 step 1 as amended by ADR 0037
	// step 5): the sheet's score in Ability + Modifier + Extra; 0 when the
	// pick has no check (HasCheck false: no ability, or unknown) or the
	// score is not known yet (a creation draft).
	Target   int
	HasCheck bool
}

// ProficiencyTarget is ADR 0014 step 1 as amended by ADR 0037 step 5: the
// ability score plus the catalogue's modifier plus the slots spent beyond
// the cost.
func ProficiencyTarget(score, modifier, extra int) int {
	return score + modifier + extra
}

// SetTargets fills each pick's check target from score (the sheet's score in
// an ability, ok false when not known): ADR 0037 step 5.
func (s *ProficiencySlots) SetTargets(score func(Ability) (int, bool)) {
	for i := range s.Picks {
		p := &s.Picks[i]
		if !p.Known || p.Ability == "" {
			continue
		}
		if v, ok := score(p.Ability); ok {
			p.Target, p.HasCheck = ProficiencyTarget(v, p.Modifier, p.Extra), true
		}
	}
}

// ProficiencySlots are the non-weapon pool (step 3) and each pick's figures.
type ProficiencySlots struct {
	Nonweapon ruletablesvc.NonweaponPool
	Picks     []PickFigures
}

// Proficiency pick limits and wire names.
const (
	// MaxPickSlots bounds the slots one pick may spend (more could never be
	// available: Table 34 and Table 4 give at most a few dozen).
	MaxPickSlots = 99
	// FieldNonweapon is the wire name of the non-weapon picks: the budget
	// refusal's field, a pick's field is FieldNonweapon + "." + id.
	FieldNonweapon = "nonweapon"
	// PoolNonweapon names the non-weapon pool in the slot warnings.
	PoolNonweapon = "nonweapon"
	// LanguagesModernID is "Languages, Modern": a language is a pick of the
	// language list (ADR 0037 step 6), never picked on its own.
	LanguagesModernID = "languages_modern"
)

// Warning codes of the slots (ADR 0037 step 11) and of a stored pick the catalogue does not
// hold (step 16, kept as a pick of its stored slots). The web words them.
const (
	CodeSlotsOverBudget    = "slots_over_budget"
	CodeSlotsToSpend       = "slots_to_spend"
	CodeProficiencyUnknown = "proficiency_unknown"
)

// pickFigures reads one pick for a class learning groups at Table 37's cost.
func pickFigures(book ProficiencyBook, groups []string, p Pick) PickFigures {
	row, ok := book.Get(p.ID)
	if !ok {
		// Unknown to the catalogue: it costs what it spends and has no check.
		return PickFigures{ID: p.ID, Slots: p.Slots, Cost: p.Slots}
	}
	cost := ruletablesvc.NonweaponCost(row, groups)
	return PickFigures{
		ID: p.ID, Slots: p.Slots, Cost: cost, Extra: ruletablesvc.ExtraSlots(p.Slots, cost),
		Ability: Ability(row.Ability), Modifier: row.Modifier, Known: true,
	}
}

// ProficiencyFigures is ADR 0037 steps 3 to 5 on picks for a character of class at the realized
// level with the sheet's intelligence: the non-weapon pool and each pick's figures. ok is false
// when the catalogue has no Table 34 row for the class (no catalogue, an unknown class).
func ProficiencyFigures(book ProficiencyBook, class string, level, intelligence int, picks Picks) (ProficiencySlots, bool) {
	row, ok := book.SlotRowOf(class)
	if !ok {
		return ProficiencySlots{}, false
	}
	groups := book.GroupsOf(class)
	out := ProficiencySlots{Picks: make([]PickFigures, 0, len(picks.Nonweapon))}
	spent := 0
	for _, p := range picks.Nonweapon {
		f := pickFigures(book, groups, p)
		out.Picks = append(out.Picks, f)
		spent += p.Slots
	}
	out.Nonweapon = ruletablesvc.NonweaponSlots(row, level, intelligence, spent)
	return out, true
}

// ValidPicks is ADR 0037 step 12's entry check on non-weapon picks, at creation and on every
// save: each id once (CodeDuplicate), a catalogue id (CodeUnknown) other than LanguagesModernID
// (CodeNotAllowed), spending from its cost for the class (step 5) to MaxPickSlots
// (CodeOutOfRange). An id the catalogue does not hold that before already has is kept (step 16:
// older data is never dropped) when it spends 1 to MaxPickSlots. A refusal is a
// *domain.FieldError on field nonweapon.<id>. The picks come back in their order, never nil.
func ValidPicks(book ProficiencyBook, class string, picks, before Picks) (Picks, error) {
	groups := book.GroupsOf(class)
	had := map[string]bool{}
	for _, p := range before.Nonweapon {
		had[p.ID] = true
	}
	seen := map[string]bool{}
	out := Picks{Nonweapon: make([]Pick, 0, len(picks.Nonweapon))}
	for _, p := range picks.Nonweapon {
		field := FieldNonweapon + "." + p.ID
		switch {
		case p.ID == "":
			return Picks{}, &domain.FieldError{Field: FieldNonweapon, Message: CodeEmpty}
		case seen[p.ID]:
			return Picks{}, &domain.FieldError{Field: field, Message: CodeDuplicate}
		case p.ID == LanguagesModernID:
			return Picks{}, &domain.FieldError{Field: field, Message: CodeNotAllowed}
		}
		seen[p.ID] = true
		row, known := book.Get(p.ID)
		switch {
		case !known && !had[p.ID]:
			return Picks{}, &domain.FieldError{Field: field, Message: CodeUnknown}
		case !known && (p.Slots < 1 || p.Slots > MaxPickSlots):
			return Picks{}, &domain.FieldError{Field: field, Message: CodeOutOfRange}
		case known && (p.Slots < ruletablesvc.NonweaponCost(row, groups) || p.Slots > MaxPickSlots):
			return Picks{}, &domain.FieldError{Field: field, Message: CodeOutOfRange}
		}
		out.Nonweapon = append(out.Nonweapon, p)
	}
	return out, nil
}

// PicksWithinBudget is ADR 0037 step 10 on the non-weapon pool: a save from before to after is
// refused when it leaves the slots spent over the available ones and above the slots spent
// before. Any other save passes: an existing overspend that shrinks or stays keeps the warning,
// and slots left to spend are never refused. At creation before is empty (0 spent). The refusal
// is a *domain.BudgetError on field nonweapon. No Table 34 row for the class: no budget, nil.
func PicksWithinBudget(book ProficiencyBook, class string, level, intelligence int, before, after Picks) error {
	b, ok := ProficiencyFigures(book, class, level, intelligence, after)
	if !ok {
		return nil
	}
	was, _ := ProficiencyFigures(book, class, level, intelligence, before)
	if b.Nonweapon.Spent > b.Nonweapon.Available && b.Nonweapon.Spent > was.Nonweapon.Spent {
		return &domain.BudgetError{Field: FieldNonweapon, Spent: b.Nonweapon.Spent, Limit: b.Nonweapon.Available}
	}
	return nil
}

// SlotWarnings are ADR 0037 step 11 on the non-weapon pool, never refusals: slots_over_budget
// (spent over available), slots_to_spend (slots left, the reminder), none when the pool is spent
// exactly; then proficiency_unknown for each pick the catalogue does not hold.
func SlotWarnings(s ProficiencySlots) []Warning {
	var out []Warning
	pool := s.Nonweapon
	switch {
	case pool.Spent > pool.Available:
		spent, available := pool.Spent, pool.Available
		out = append(out, Warning{Code: CodeSlotsOverBudget, Field: FieldNonweapon, Pool: PoolNonweapon, Spent: &spent, Available: &available})
	case pool.Left > 0:
		left := pool.Left
		out = append(out, Warning{Code: CodeSlotsToSpend, Field: FieldNonweapon, Pool: PoolNonweapon, Left: &left})
	}
	for _, p := range s.Picks {
		if !p.Known {
			out = append(out, Warning{Code: CodeProficiencyUnknown, Field: FieldNonweapon + "." + p.ID})
		}
	}
	return out
}
