package skills

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Proficiency picks against their slots (ADR 0037 steps 3 to 5, 7, 10, 11 and 12: the
// non-weapon and the weapon pool).

// Pick is one stored non-weapon proficiency pick (ADR 0037 step 12): the catalogue id and the
// slots spent on it. Its ability and modifier are read from the catalogue, never stored.
type Pick struct {
	ID    string
	Slots int
}

// WeaponPick is one stored weapon proficiency pick (ADR 0037 step 7): a weapon type of ADR 0030
// step 23 and the slots spent on it: WeaponPickSlots, or the type's specialization cost for a
// fighter's specialization (step 9, ruletablesvc.SpecializationSlots).
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
	Ability  string
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
func (s *ProficiencySlots) SetTargets(score func(ability string) (int, bool)) {
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
	// Penalty is Table 34's non-proficiency penalty for the class group (ADR 0037 step 8), 0 or
	// less: what an attack with a weapon type without a pick takes to hit.
	Penalty int
}

// NonproficientPenalty is ADR 0037 step 8 for an attack with a weapon of weaponType: Table 34's
// penalty for the class group when the type has no weapon pick, else 0. Any pick of the type
// counts, a kept pick the class may not make included (Allowed false): the step asks for a pick,
// and wielding is never refused. A weapon with no type takes none (the piece carries the
// weapon_type_missing warning instead), and so do no figures (nil: no catalogue, an unknown
// class).
func (s *ProficiencySlots) NonproficientPenalty(weaponType string) int {
	if s == nil {
		return 0
	}
	return nonproficientPenalty(s.Penalty, weaponType, func(t string) bool {
		return slices.ContainsFunc(s.WeaponPicks, func(p WeaponPickFigures) bool { return p.Type == t })
	})
}

// nonproficientPenalty is step 8's rule once the class's penalty and the picks are known: the
// penalty when weaponType is set and has no pick, else 0.
func nonproficientPenalty(penalty int, weaponType string, picked func(string) bool) int {
	if weaponType == "" || picked(weaponType) {
		return 0
	}
	return penalty
}

// WeaponProficiency is what a character's weapon picks make of its weapons, read from the
// character row alone (no derived figure): Specialized the type it is specialized in (step 9,
// SpecializedType; "" for none), Picked the picked types, Penalty Table 34's non-proficiency
// penalty for the class group (step 8; 0 without a Table 34 row).
type WeaponProficiency struct {
	Specialized string
	Picked      []string
	Penalty     int
}

// NonproficientPenalty is step 8 for a weapon of weaponType, as ProficiencySlots.NonproficientPenalty
// judges it.
func (w WeaponProficiency) NonproficientPenalty(weaponType string) int {
	return nonproficientPenalty(w.Penalty, weaponType, func(t string) bool { return slices.Contains(w.Picked, t) })
}

// WeaponProficiencyOf reads a character of the active class entries cls with picks: its
// specialization (step 9, the first class's) and its weapon picks against Table 34's penalty
// from book (step 8), the smallest of the classes' (ADR 0042 step 15, Q12). The levels of cls
// are not read.
func WeaponProficiencyOf(book Book, cls Classes, picks Picks) WeaponProficiency {
	out := WeaponProficiency{Specialized: SpecializedType(book, cls.first(), picks), Picked: make([]string, 0, len(picks.Weapon))}
	if b, ok := cls.basis(book); ok {
		out.Penalty = b.row.Penalty
	}
	for _, p := range picks.Weapon {
		out.Picked = append(out.Picked, p.Type)
	}
	return out
}

// specializes reports whether class lists weapon_specialization (ADR 0040 step 3,
// ruletablesvc.Specializes); false for a class the book does not define.
func specializes(book Book, class string) bool {
	def, ok := book.ClassDef(class)
	return ok && ruletablesvc.Specializes(def)
}

// weaponTypeAllowed is ADR 0037 step 7 for the active class entries cls, judged together by ADR
// 0042 step 16.1 (classes.AllowedTogether): a type any class may pick, except that a blunt_only
// class restricts the picks to its own. A class the catalogue does not define limits nothing, so
// nothing is judged (as itemsvc's class warnings).
func (cls Classes) weaponTypeAllowed(book Book, weaponType string) bool {
	limits := make([]classes.Equipment, 0, len(cls))
	for _, c := range cls {
		def, ok := book.ClassDef(c.Class)
		if !ok {
			return true
		}
		limits = append(limits, def.Equipment)
	}
	return classes.AllowedTogether(limits, func(e classes.Equipment) bool { return e.WeaponTypeAllowed(weaponType) })
}

// IsSpecialization reports whether p is a specialization pick (ADR 0037 step 9): a weapon type
// with the slots its specialization costs. A plain pick spends WeaponPickSlots.
func (p WeaponPick) IsSpecialization() bool {
	return p.Slots > WeaponPickSlots && slices.Contains(ruletablesvc.WeaponTypes, p.Type) &&
		p.Slots == ruletablesvc.SpecializationSlots(p.Type)
}

// SpecializedType is ADR 0037 step 9 amending ADR 0030 step 12: the weapon type a character of
// class with picks is specialized in, every weapon of it specialized: the type of its specialization pick when the class specializes
// (ruletablesvc.Specializes), else "". It needs no derived figure, so a read of the character
// row alone gives it.
func SpecializedType(book Book, class string, picks Picks) string {
	if !specializes(book, class) {
		return ""
	}
	for _, p := range picks.Weapon {
		if p.IsSpecialization() {
			return p.Type
		}
	}
	return ""
}

// Proficiency pick limits.
const (
	// MaxPickSlots bounds the slots one pick may spend (more could never be
	// available: Table 34 and Table 4 give at most a few dozen).
	MaxPickSlots = 99
	// WeaponPickSlots is what a plain weapon pick spends (ADR 0037 step 7); a
	// specialization spends ruletablesvc.SpecializationSlots (step 9).
	WeaponPickSlots = 1
	// LanguagesModernID is "Languages, Modern": a language is a pick of the
	// language list (ADR 0037 step 6), never picked on its own.
	LanguagesModernID = "languages_modern"
)

// pickFigures reads one pick for a class learning groups at Table 37's cost.
func pickFigures(book Book, groups []string, p Pick) PickFigures {
	row, ok := book.Get(p.ID)
	if !ok {
		// Unknown to the catalogue: it costs what it spends and has no check.
		return PickFigures{ID: p.ID, Slots: p.Slots, Cost: p.Slots}
	}
	cost := ruletablesvc.NonweaponCost(row, groups)
	return PickFigures{
		ID: p.ID, Slots: p.Slots, Cost: cost, Extra: ruletablesvc.ExtraSlots(p.Slots, cost),
		Ability: row.Ability, Modifier: row.Modifier, Known: true,
	}
}

// Spending is what a character spends from its proficiency pools: the picks and the languages
// that cost a non-weapon slot each (ADR 0037 step 6, LanguageSlots).
type Spending struct {
	Picks     Picks
	Languages int
}

// ProficiencyFigures is ADR 0037 steps 3 to 7 on what a character of class spends at the
// realized level with the sheet's intelligence: both pools (the languages in the non-weapon
// one) and each pick's figures. ok is false when the catalogue has no Table 34 row for the class
// (no catalogue, an unknown class).
func ProficiencyFigures(book Book, class string, level, intelligence int, spend Spending) (ProficiencySlots, bool) {
	return FiguresOf(book, One(class, level), intelligence, spend)
}

// FiguresOf is ProficiencyFigures for the active class entries cls (ADR 0042 step 15): the pools
// read on the basis Classes.basis combines. ok is false when no class has a Table 34 row.
func FiguresOf(book Book, cls Classes, intelligence int, spend Spending) (ProficiencySlots, bool) {
	picks := spend.Picks
	b, ok := cls.basis(book)
	if !ok {
		return ProficiencySlots{}, false
	}
	out := ProficiencySlots{Picks: make([]PickFigures, 0, len(picks.Nonweapon)), Penalty: b.row.Penalty}
	spent := 0
	for _, p := range picks.Nonweapon {
		f := pickFigures(book, b.groups, p)
		out.Picks = append(out.Picks, f)
		spent += p.Slots
	}
	out.Nonweapon = ruletablesvc.NonweaponSlots(b.row, b.nonweaponLevel, intelligence, spent+spend.Languages)
	out.Nonweapon.Languages = spend.Languages
	out.WeaponPicks = make([]WeaponPickFigures, 0, len(picks.Weapon))
	spent = 0
	for _, p := range picks.Weapon {
		out.WeaponPicks = append(out.WeaponPicks, WeaponPickFigures{
			Type: p.Type, Slots: p.Slots, Allowed: cls.weaponTypeAllowed(book, p.Type),
		})
		spent += p.Slots
	}
	out.Weapon = ruletablesvc.WeaponSlots(b.row, b.weaponLevel, spent)
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
func ValidPicks(book Book, class string, picks, before Picks) (Picks, error) {
	return validPicksOf(book, One(class, 0), picks, before)
}

// validPicksOf is ValidPicks for the active class entries cls: a non-weapon pick costs Table
// 37's slots when one of its groups is any class's (ADR 0042 step 15, Q13); a weapon pick's type
// is judged by every class together (ADR 0042 step 16.1), the specialization by the first class.
func validPicksOf(book Book, cls Classes, picks, before Picks) (Picks, error) {
	weapon := before.Clone().Weapon
	if picks.Weapon != nil {
		var err error
		if weapon, err = validWeaponPicks(book, cls, picks.Weapon, before.Weapon); err != nil {
			return Picks{}, err
		}
	}
	if picks.Nonweapon == nil {
		return Picks{Nonweapon: before.Clone().Nonweapon, Weapon: weapon}, nil
	}
	groups := cls.groups(book)
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

// validWeaponPicks is ADR 0037 steps 7 and 9 on the weapon picks of a save: each a weapon type
// (CodeUnknown) once (CodeDuplicate), spending WeaponPickSlots or, as the specialization (step
// 9), its type's ruletablesvc.SpecializationSlots (any other count CodeOutOfRange), of a type the
// classes may pick (CodeWeaponNotAllowed, Classes.weaponTypeAllowed). A specialization is a fighter's (Specializes; another
// class CodeNotAllowed), in one type at most (a second CodeNotAllowed); a type outside the class's list that before already
// has is kept (step 16.2: the migration grants every owned type), never added. A type that is no
// weapon type but that before already has is kept too, as the non-weapon pool keeps a stored id
// the catalogue does not hold (step 16: older data is never dropped). A refusal is a
// *domain.FieldError on field weapon.<type>. Never nil.
func validWeaponPicks(book Book, cls Classes, picks, before []WeaponPick) ([]WeaponPick, error) {
	class := cls.first()
	specialized := false
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
		case p.Slots != WeaponPickSlots && !p.IsSpecialization():
			return nil, &domain.FieldError{Field: field, Message: CodeOutOfRange}
		case !had[p.Type] && !cls.weaponTypeAllowed(book, p.Type):
			return nil, &domain.FieldError{Field: field, Message: CodeWeaponNotAllowed}
		case p.IsSpecialization() && (!specializes(book, class) || specialized):
			return nil, &domain.FieldError{Field: field, Message: CodeNotAllowed}
		}
		specialized = specialized || p.IsSpecialization()
		seen[p.Type] = true
		out = append(out, p)
	}
	return out, nil
}

// PicksWithinBudget is ADR 0037 step 10 on each pool (non-weapon, languages included, then
// weapon): a save from before to after is refused when it leaves a pool's slots spent over the
// available ones and above the slots that pool spent before. Any other save passes: an existing
// overspend that shrinks or stays keeps the warning, and slots left to spend are never refused.
// At creation before is empty (0 spent). The refusal is a *domain.BudgetError on field
// languages when the picks alone, with the languages as before, would pass (a languages edit, or
// a creation whose languages overspend, step 6), else nonweapon or weapon. No Table 34 row for the
// class: no budget, nil.
func PicksWithinBudget(book Book, class string, level, intelligence int, before, after Spending) error {
	return picksWithinBudgetOf(book, One(class, level), intelligence, before, after)
}

// picksWithinBudgetOf is PicksWithinBudget for the active class entries cls (ADR 0042 step 15).
func picksWithinBudgetOf(book Book, cls Classes, intelligence int, before, after Spending) error {
	b, ok := FiguresOf(book, cls, intelligence, after)
	if !ok {
		return nil
	}
	was, _ := FiguresOf(book, cls, intelligence, before)
	over := func(f ProficiencySlots) bool {
		return f.Nonweapon.Spent > f.Nonweapon.Available && f.Nonweapon.Spent > was.Nonweapon.Spent
	}
	if over(b) {
		// The languages are to blame when the picks alone, with the languages as before, pass.
		field := FieldNonweapon
		if picksOnly, _ := FiguresOf(book, cls, intelligence,
			Spending{Picks: after.Picks, Languages: before.Languages}); !over(picksOnly) {
			field = FieldLanguages
		}
		return &domain.BudgetError{Field: field, Spent: b.Nonweapon.Spent, Limit: b.Nonweapon.Available}
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
