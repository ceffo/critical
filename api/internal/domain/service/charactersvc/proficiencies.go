package charactersvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Proficiency picks against their slots (ADR 0037 steps 3 to 5, 7, 10, 11 and 12: the
// non-weapon and the weapon pool). The rules live here, as the skill points' do; skillsvc writes
// the picks.

// Pick is one stored non-weapon proficiency pick (ADR 0037 step 12): the catalogue id and the
// slots spent on it. Its ability and modifier are read from the catalogue, never stored.
type Pick struct {
	ID    string
	Slots int
}

// WeaponPick is one stored weapon proficiency pick (ADR 0037 step 7): a weapon type of ADR 0030
// step 23 and the slots spent on it (1; specialization, step 9, is not built).
type WeaponPick struct {
	Type  string
	Slots int
}

// Picks are a character's proficiency picks, by pool. As an edit (ValidPicks), a nil pool is
// one the edit leaves as stored.
type Picks struct {
	Nonweapon []Pick
	Weapon    []WeaponPick
}

// Equal reports the same picks, in any order (ADR 0037 step 13: a save that only reorders the
// picks changes nothing, so it writes and logs nothing). The order is the sheet's display order,
// not a rule.
func (p Picks) Equal(o Picks) bool {
	return sameItems(p.Nonweapon, o.Nonweapon) && sameItems(p.Weapon, o.Weapon)
}

// sameItems reports a and b hold the same items, in any order.
func sameItems[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[T]int, len(a))
	for _, k := range a {
		count[k]++
	}
	for _, k := range b {
		if count[k] == 0 {
			return false
		}
		count[k]--
	}
	return true
}

// Clone is a copy of p that shares nothing with it; never nil lists.
func (p Picks) Clone() Picks {
	return Picks{Nonweapon: append([]Pick{}, p.Nonweapon...), Weapon: append([]WeaponPick{}, p.Weapon...)}
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

// WeaponPickFigures is a weapon pick as the rules read it: Allowed is false for a type the
// class may not pick (step 7), kept from older data (the migration grants every owned type,
// step 16.2) but never added.
type WeaponPickFigures struct {
	Type    string
	Slots   int
	Allowed bool
}

// ProficiencySlots are the pools (step 3) and each pick's figures: Picks the non-weapon picks,
// WeaponPicks the weapon picks.
type ProficiencySlots struct {
	Nonweapon   ruletablesvc.NonweaponPool
	Weapon      ruletablesvc.WeaponPool
	Picks       []PickFigures
	WeaponPicks []WeaponPickFigures
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
	// FieldWeapon is the wire name of the weapon picks: the budget refusal's
	// field, a pick's field is FieldWeapon + "." + type.
	FieldWeapon = "weapon"
	// PoolWeapon names the weapon pool in the slot warnings.
	PoolWeapon = "weapon"
	// WeaponPickSlots is what a weapon pick spends (ADR 0037 step 7); step 9's
	// specialization, which spends more, is not built.
	WeaponPickSlots = 1
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
	// CodeWeaponNotAllowed refuses a weapon pick of a type the class may not
	// pick (ADR 0037 step 7).
	CodeWeaponNotAllowed = "weapon_not_allowed"
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

// ProficiencyFigures is ADR 0037 steps 3 to 5 and 7 on picks for a character of class at the
// realized level with the sheet's intelligence: both pools and each pick's figures. ok is false
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
	out.WeaponPicks = make([]WeaponPickFigures, 0, len(picks.Weapon))
	spent = 0
	for _, p := range picks.Weapon {
		out.WeaponPicks = append(out.WeaponPicks, WeaponPickFigures{
			Type: p.Type, Slots: p.Slots, Allowed: ruletablesvc.WeaponTypeAllowed(class, p.Type),
		})
		spent += p.Slots
	}
	out.Weapon = ruletablesvc.WeaponSlots(row, level, spent)
	return out, true
}

// ValidPicks is ADR 0037 step 12's entry check on picks, at creation and on every save. A nil
// pool of picks is the stored one (before), unchanged: an editor of one pool leaves the other.
// Non-weapon picks: each id once (CodeDuplicate), a catalogue id (CodeUnknown) other than
// LanguagesModernID (CodeNotAllowed), spending from its cost for the class (step 5) to
// MaxPickSlots (CodeOutOfRange). An id the catalogue does not hold that before already has is
// kept (step 16: older data is never dropped) when it spends 1 to MaxPickSlots. A refusal is a
// *domain.FieldError on field nonweapon.<id>. Weapon picks: validWeaponPicks. The picks come
// back in their order, never nil.
func ValidPicks(book ProficiencyBook, class string, picks, before Picks) (Picks, error) {
	weapon := before.Clone().Weapon
	if picks.Weapon != nil {
		var err error
		if weapon, err = validWeaponPicks(class, picks.Weapon, before.Weapon); err != nil {
			return Picks{}, err
		}
	}
	if picks.Nonweapon == nil {
		return Picks{Nonweapon: before.Clone().Nonweapon, Weapon: weapon}, nil
	}
	groups := book.GroupsOf(class)
	had := map[string]bool{}
	for _, p := range before.Nonweapon {
		had[p.ID] = true
	}
	seen := map[string]bool{}
	out := Picks{Nonweapon: make([]Pick, 0, len(picks.Nonweapon)), Weapon: weapon}
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

// validWeaponPicks is ADR 0037 step 7 on the weapon picks of a save: each a weapon type
// (CodeUnknown) once (CodeDuplicate), spending WeaponPickSlots (CodeOutOfRange), of a type the
// class may pick (CodeWeaponNotAllowed); a type outside the class's list that before already
// has is kept (step 16.2: the migration grants every owned type), never added. A type that is no
// weapon type but that before already has is kept too, as the non-weapon pool keeps a stored id
// the catalogue does not hold (step 16: older data is never dropped). A refusal is a
// *domain.FieldError on field weapon.<type>. Never nil.
func validWeaponPicks(class string, picks, before []WeaponPick) ([]WeaponPick, error) {
	had := map[string]bool{}
	for _, p := range before {
		had[p.Type] = true
	}
	seen := map[string]bool{}
	out := make([]WeaponPick, 0, len(picks))
	for _, p := range picks {
		field := FieldWeapon + "." + p.Type
		switch {
		case p.Type == "":
			return nil, &domain.FieldError{Field: FieldWeapon, Message: CodeEmpty}
		case !had[p.Type] && !slices.Contains(ruletablesvc.WeaponTypes, p.Type):
			return nil, &domain.FieldError{Field: field, Message: CodeUnknown}
		case seen[p.Type]:
			return nil, &domain.FieldError{Field: field, Message: CodeDuplicate}
		case p.Slots != WeaponPickSlots:
			return nil, &domain.FieldError{Field: field, Message: CodeOutOfRange}
		case !had[p.Type] && !ruletablesvc.WeaponTypeAllowed(class, p.Type):
			return nil, &domain.FieldError{Field: field, Message: CodeWeaponNotAllowed}
		}
		seen[p.Type] = true
		out = append(out, p)
	}
	return out, nil
}

// PicksWithinBudget is ADR 0037 step 10 on each pool (non-weapon, then weapon): a save from
// before to after is refused when it leaves a pool's slots spent over the available ones and
// above the slots that pool spent before. Any other save passes: an existing overspend that
// shrinks or stays keeps the warning, and slots left to spend are never refused. At creation
// before is empty (0 spent). The refusal is a *domain.BudgetError on field nonweapon or weapon.
// No Table 34 row for the class: no budget, nil.
func PicksWithinBudget(book ProficiencyBook, class string, level, intelligence int, before, after Picks) error {
	b, ok := ProficiencyFigures(book, class, level, intelligence, after)
	if !ok {
		return nil
	}
	was, _ := ProficiencyFigures(book, class, level, intelligence, before)
	if b.Nonweapon.Spent > b.Nonweapon.Available && b.Nonweapon.Spent > was.Nonweapon.Spent {
		return &domain.BudgetError{Field: FieldNonweapon, Spent: b.Nonweapon.Spent, Limit: b.Nonweapon.Available}
	}
	if b.Weapon.Spent > b.Weapon.Available && b.Weapon.Spent > was.Weapon.Spent {
		return &domain.BudgetError{Field: FieldWeapon, Spent: b.Weapon.Spent, Limit: b.Weapon.Available}
	}
	return nil
}

// SlotWarnings are ADR 0037 step 11 on each pool (non-weapon, then weapon), never refusals:
// slots_over_budget (spent over available), slots_to_spend (slots left, the reminder), none when
// the pool is spent exactly; then proficiency_unknown for each non-weapon pick the catalogue does
// not hold.
func SlotWarnings(s ProficiencySlots) []Warning {
	out := poolWarnings(FieldNonweapon, PoolNonweapon, s.Nonweapon.Spent, s.Nonweapon.Available)
	out = append(out, poolWarnings(FieldWeapon, PoolWeapon, s.Weapon.Spent, s.Weapon.Available)...)
	for _, p := range s.Picks {
		if !p.Known {
			out = append(out, Warning{Code: CodeProficiencyUnknown, Field: FieldNonweapon + "." + p.ID})
		}
	}
	return out
}

// poolWarnings is step 11 on one pool.
func poolWarnings(field, pool string, spent, available int) []Warning {
	switch {
	case spent > available:
		return []Warning{{Code: CodeSlotsOverBudget, Field: field, Pool: pool, Spent: &spent, Available: &available}}
	case available > spent:
		left := available - spent
		return []Warning{{Code: CodeSlotsToSpend, Field: field, Pool: pool, Left: &left}}
	}
	return nil
}
