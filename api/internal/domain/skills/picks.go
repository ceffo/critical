package skills

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// Proficiency picks against their slots (ADR 0037 steps 3 to 5, 7, 10, 11 and 12: the
// non-weapon and the weapon pool).

// Pick is one stored non-weapon proficiency pick (ADR 0037 step 12): the catalogue id and the
// slots spent on it. Its ability and modifier are read from the catalogue, never stored.
type Pick struct {
	ID    string
	Slots int
}

// WeaponPick is one stored weapon proficiency pick (ADR 0037 step 21.1): a catalogue weapon code
// (ADR 0033 step 21) and the slots spent on it: WeaponPickSlots, or the specialization cost of the
// weapon's type for a fighter's specialization (step 21.5, ruletablesvc.SpecializationSlots).
type WeaponPick struct {
	Code  string
	Slots int
}

// WeaponRedo is a weapon type pick the migration to weapon codes found no owned weapon for (ADR
// 0037 step 21.8.3 and 4): the type and the slots it spent, listed for the player to redo. Its
// slots are back to spend; it is no pick.
type WeaponRedo struct {
	Type  string
	Slots int
}

// Picks are a character's proficiency picks, by pool, and the weapon type picks left to redo
// (step 21.8.3). As an edit (ValidPicks), a nil pool is one the edit leaves as stored, and
// WeaponRedo is not read: a save of the weapon pool clears it.
type Picks struct {
	Nonweapon  []Pick
	Weapon     []WeaponPick
	WeaponRedo []WeaponRedo
}

// Equal reports the same picks, in any order (ADR 0037 step 13: a save that only reorders the
// picks changes nothing, so it writes and logs nothing), and the same redo list. The order is the
// sheet's display order, not a rule.
func (p Picks) Equal(o Picks) bool {
	return sameItems(p.Nonweapon, o.Nonweapon) && sameItems(p.Weapon, o.Weapon) && sameItems(p.WeaponRedo, o.WeaponRedo)
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
	out := Picks{Nonweapon: append([]Pick{}, p.Nonweapon...), Weapon: append([]WeaponPick{}, p.Weapon...)}
	if len(p.WeaponRedo) > 0 {
		out.WeaponRedo = slices.Clone(p.WeaponRedo)
	}
	return out
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

// WeaponPickFigures is a weapon pick as the rules read it: Allowed is false for a weapon the
// class may not pick (step 21.2), kept from older data (the migration of step 21.8 keeps every
// owned weapon) but never added.
type WeaponPickFigures struct {
	Code    string
	Slots   int
	Allowed bool
}

// WeaponFigure is a catalogue weapon the character may pick now (ADR 0037 step 21.6), with the
// slots its specialization would spend (step 21.5: its type's cost), 0 when the character may not
// specialize.
type WeaponFigure struct {
	Code                string
	SpecializationSlots int
}

// ProficiencySlots are the pools (step 3) and each pick's figures: Picks the non-weapon picks,
// WeaponPicks the weapon picks.
type ProficiencySlots struct {
	Nonweapon   slots.NonweaponPool
	Weapon      slots.WeaponPool
	Picks       []PickFigures
	WeaponPicks []WeaponPickFigures
	// Weapons are the catalogue weapons the character may pick (step 21.2, judged by ADR 0042 step
	// 16.1: Classes.weaponCodeAllowed, the rule a save is judged by, the ruleset's weapon lift
	// included), in offer order (step 21.6).
	Weapons []WeaponFigure
	// WeaponRedo are the type picks left to redo (step 21.8.3), each warned weapon_pick_to_redo.
	WeaponRedo []WeaponRedo
	// Penalty is Table 34's non-proficiency penalty for the class group (ADR 0037 step 8), 0 or
	// less: what an attack with a weapon without a pick takes to hit.
	Penalty int
}

// NonproficientPenalty is ADR 0037 step 21.3 (amending step 8) for an attack with a weapon piece
// of catalogue code code: Table 34's penalty for the class group when no pick names the code,
// else 0. Any pick of the code counts, a kept pick the class may not make included (Allowed
// false): the step asks for a pick, and wielding is never refused. A piece with no code ("", step
// 21.4) is non-proficient. No figures (nil: no catalogue, an unknown class) take none.
func (s *ProficiencySlots) NonproficientPenalty(code string) int {
	if s == nil {
		return 0
	}
	return nonproficientPenalty(s.Penalty, code, func(c string) bool {
		return slices.ContainsFunc(s.WeaponPicks, func(p WeaponPickFigures) bool { return p.Code == c })
	})
}

// nonproficientPenalty is step 21.3's rule once the class's penalty and the picks are known: the
// penalty when no pick names code (a piece without a code included, step 21.4), else 0.
func nonproficientPenalty(penalty int, code string, picked func(string) bool) int {
	if code != "" && picked(code) {
		return 0
	}
	return penalty
}

// WeaponProficiency is what a character's weapon picks make of its weapons, read from the
// character row alone (no derived figure): Specialized the catalogue code it is specialized in
// (step 21.5, SpecializedCode; "" for none), Picked the picked codes, Penalty Table 34's
// non-proficiency penalty for the class group (step 8; 0 without a Table 34 row).
type WeaponProficiency struct {
	Specialized string
	Picked      []string
	Penalty     int
}

// NonproficientPenalty is step 21.3 for a weapon piece of catalogue code code, as
// ProficiencySlots.NonproficientPenalty judges it.
func (w WeaponProficiency) NonproficientPenalty(code string) int {
	return nonproficientPenalty(w.Penalty, code, func(c string) bool { return slices.Contains(w.Picked, c) })
}

// WeaponProficiencyOf reads a character of the class entries cls with picks: its
// specialization (step 21.5, a single-class character's only: Classes.specialist; a dual-class's
// kept one past the crossover, Classes.specializedCode) and its weapon picks against Table 34's
// penalty from book (step 8), the smallest of the active classes' (ADR 0042 step 15, Q12). The
// levels of cls are read only for the crossover.
func WeaponProficiencyOf(book Book, cls Classes, picks Picks) WeaponProficiency {
	out := WeaponProficiency{Specialized: cls.specializedCode(book, picks), Picked: make([]string, 0, len(picks.Weapon))}
	if b, ok := cls.basis(book); ok {
		out.Penalty = b.row.Penalty
	}
	for _, p := range picks.Weapon {
		out.Picked = append(out.Picked, p.Code)
	}
	return out
}

// specializedCode is step 21.5 for cls (ADR 0042): the single active class's specialization; for
// a dual-class whose active class does not specialize, a specialization pick kept from an old
// class that does counts past the crossover only (step 13: before it the active class's own
// figures apply, so the kept pick gives no bonus).
func (cls Classes) specializedCode(book Book, picks Picks) string {
	if c := SpecializedCode(book, cls.specialist(), picks); c != "" || !cls.crossed() {
		return c
	}
	for _, c := range cls {
		if c.Frozen {
			if code := SpecializedCode(book, c.Class, picks); code != "" {
				return code
			}
		}
	}
	return ""
}

// oldSpecialist reports whether one of a dual-class's old (frozen) classes specializes.
func (cls Classes) oldSpecialist(book Book) bool {
	return slices.ContainsFunc(cls, func(c ClassLevel) bool { return c.Frozen && specializes(book, c.Class) })
}

// specializes reports whether class lists weapon_specialization (ADR 0040 step 3,
// ruletablesvc.Specializes); false for a class the book does not define.
func specializes(book Book, class string) bool {
	def, ok := book.ClassDef(class)
	return ok && ruletablesvc.Specializes(def)
}

// weaponCodeAllowed is ADR 0037 step 21.2 for the class entries cls, judged together by ADR 0042
// step 16.1 (classes.AllowedTogether): a catalogue weapon any active class may pick
// (classes.Equipment.WeaponPickAllowed: the wielding warnings' rule), except that a blunt_only
// active class restricts the picks to its own unless lifted (the ruleset's weapon lift, ADR 0041
// step 2.5, which only several active classes feel); a dual-class's old (frozen) classes add what
// they may pick (classes.AllowedWithOld, as itemsvc's class warnings), before and after the
// crossover: a pick is not a use (step 13 warns the wielding, itemsvc). A class the catalogue does
// not define limits nothing, so nothing is judged (as itemsvc's class warnings).
func (cls Classes) weaponCodeAllowed(book Book, lifted bool, code string) bool {
	var active, old []classes.Equipment
	for _, c := range cls {
		def, ok := book.ClassDef(c.Class)
		switch {
		case !ok && !c.Frozen:
			return true
		case !ok:
		case c.Frozen:
			old = append(old, def.Equipment)
		default:
			active = append(active, def.Equipment)
		}
	}
	allowed, _ := classes.AllowedWithOld(active, old, lifted, func(e classes.Equipment) bool { return e.WeaponPickAllowed(code) })
	return allowed
}

// IsSpecializationOf reports whether p is a specialization pick of a class def (ADR 0037 step
// 21.5, ADR 0040 step 13): a catalogue weapon of book with the slots def's weapon_specialization
// params give the weapon's type. A plain pick spends WeaponPickSlots.
func (p WeaponPick) IsSpecializationOf(book Book, def classes.Class) bool {
	w, ok := book.Weapon(p.Code)
	return ok && p.Slots > WeaponPickSlots && p.Slots == ruletablesvc.SpecializationSlots(def, w.WeaponType)
}

// anySpecializes reports whether a class entry of cls, a dual-class's old (frozen) ones included,
// lists weapon_specialization.
func (cls Classes) anySpecializes(book Book) bool {
	return slices.ContainsFunc(cls, func(c ClassLevel) bool { return specializes(book, c.Class) })
}

// isSpecialization reports whether p is a specialization pick of any class entry of cls, a
// dual-class's old (frozen) ones included.
func (cls Classes) isSpecialization(book Book, p WeaponPick) bool {
	return slices.ContainsFunc(cls, func(c ClassLevel) bool {
		def, ok := book.ClassDef(c.Class)
		return ok && p.IsSpecializationOf(book, def)
	})
}

// SpecializedCode is ADR 0037 step 21.5 amending step 9: the catalogue weapon a character of class
// with picks is specialized in, only the pieces of that code specialized: the code of its
// specialization pick when the class specializes (ruletablesvc.Specializes), else "". It needs no
// derived figure, so a read of the character row alone gives it.
func SpecializedCode(book Book, class string, picks Picks) string {
	def, ok := book.ClassDef(class)
	if !ok || !ruletablesvc.Specializes(def) {
		return ""
	}
	for _, p := range picks.Weapon {
		if p.IsSpecializationOf(book, def) {
			return p.Code
		}
	}
	return ""
}

// Proficiency pick limits.
const (
	// MaxPickSlots bounds the slots one pick may spend (more could never be
	// available: Table 34 and Table 4 give at most a few dozen).
	MaxPickSlots = 99
	// WeaponPickSlots is what a plain weapon pick spends (ADR 0037 steps 7 and
	// 21.1); a specialization spends ruletablesvc.SpecializationSlots of the
	// weapon's type (step 21.5).
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
		ID: p.ID, Slots: p.Slots, Cost: cost, Extra: slots.Extra(p.Slots, cost),
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

// FiguresOf is ProficiencyFigures for the class entries cls (ADR 0042 step 15): the pools read on
// the basis Classes.basis combines from the active entries, plus a dual-class's kept slots
// (Classes.kept). ok is false when no active class has a Table 34 row.
func FiguresOf(book Book, cls Classes, intelligence int, spend Spending) (ProficiencySlots, bool) {
	return figuresOf(book, cls, false, intelligence, spend)
}

// figuresOf is FiguresOf with the ruleset's weapon lift (ADR 0041 step 2.5), which only each
// weapon pick's Allowed reads (Classes.weaponTypeAllowed).
func figuresOf(book Book, cls Classes, lifted bool, intelligence int, spend Spending) (ProficiencySlots, bool) {
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
	out.Nonweapon = slots.Nonweapon(b.row, b.nonweaponLevel, ruletablesvc.LanguageLimit(intelligence), spent+spend.Languages)
	out.Nonweapon.Languages = spend.Languages
	out.WeaponPicks = make([]WeaponPickFigures, 0, len(picks.Weapon))
	spent = 0
	for _, p := range picks.Weapon {
		out.WeaponPicks = append(out.WeaponPicks, WeaponPickFigures{
			Code: p.Code, Slots: p.Slots, Allowed: cls.weaponCodeAllowed(book, lifted, p.Code),
		})
		spent += p.Slots
	}
	// Step 21.8.3: a type pick left to redo spends nothing, its slots are back to spend.
	out.WeaponRedo = slices.Clone(picks.WeaponRedo)
	out.Weapon = slots.Weapon(b.row, b.weaponLevel, spent)
	out.Weapons = make([]WeaponFigure, 0, len(book.Weapons()))
	specialistDef, specialist := book.ClassDef(cls.specialist())
	specialist = specialist && ruletablesvc.Specializes(specialistDef)
	for _, w := range book.Weapons() {
		if !cls.weaponCodeAllowed(book, lifted, w.Code) {
			continue
		}
		f := WeaponFigure{Code: w.Code}
		if specialist {
			f.SpecializationSlots = ruletablesvc.SpecializationSlots(specialistDef, w.WeaponType)
		}
		out.Weapons = append(out.Weapons, f)
	}
	// ADR 0042 step 15 (Q14): a dual-class keeps the slots of its old class at its left-at level
	// (the first class's initial count) and adds the new class's gained slots at the new rate.
	if k, ok := cls.kept(book); ok {
		out.Weapon.Initial = k.weaponInitial
		out.Weapon.LevelGained += k.weaponGained
		out.Weapon.Available = out.Weapon.Initial + out.Weapon.LevelGained
		out.Weapon.Left = out.Weapon.Available - out.Weapon.Spent
		out.Nonweapon.Initial = k.nonweaponInitial
		out.Nonweapon.LevelGained += k.nonweaponGained
		out.Nonweapon.Available = out.Nonweapon.Initial + out.Nonweapon.LevelGained + out.Nonweapon.Intelligence
		out.Nonweapon.Left = out.Nonweapon.Available - out.Nonweapon.Spent
	}
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
	return validPicksOf(book, One(class, 0), false, picks, before)
}

// validPicksOf is ValidPicks for the active class entries cls: a non-weapon pick costs Table
// 37's slots when one of its groups is any class's (ADR 0042 step 15, Q13); a weapon pick's
// weapon is judged by every class together (ADR 0042 step 16.1 as amended by ADR 0037 step 21.2,
// lifted by the ruleset's weapon lift, ADR 0041 step 2.5), the specialization by the first class.
// A save of the weapon pool clears the type picks left to redo (step 21.8.3); one that leaves the
// pool keeps them.
func validPicksOf(book Book, cls Classes, lifted bool, picks, before Picks) (Picks, error) {
	kept := before.Clone()
	weapon, redo := kept.Weapon, kept.WeaponRedo
	if picks.Weapon != nil {
		var err error
		if weapon, err = validWeaponPicks(book, cls, lifted, picks.Weapon, before.Weapon); err != nil {
			return Picks{}, err
		}
		redo = nil
	}
	if picks.Nonweapon == nil {
		return Picks{Nonweapon: kept.Nonweapon, Weapon: weapon, WeaponRedo: redo}, nil
	}
	groups := cls.groups(book)
	had := map[string]bool{}
	for _, p := range before.Nonweapon {
		had[p.ID] = true
	}
	seen := map[string]bool{}
	out := Picks{Nonweapon: make([]Pick, 0, len(picks.Nonweapon)), Weapon: weapon, WeaponRedo: redo}
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

// validWeaponPicks is ADR 0037 step 21 (amending steps 7 and 9) on the weapon picks of a save:
// each a catalogue weapon code (CodeUnknown) once (CodeDuplicate), spending WeaponPickSlots or, as
// the specialization (step 21.5), its type's ruletablesvc.SpecializationSlots (any other count
// CodeOutOfRange), of a weapon the classes may pick (CodeWeaponNotAllowed,
// Classes.weaponCodeAllowed). A specialization is a single-class fighter's (Specializes and
// Classes.specialist; another class or a multi-class character CodeNotAllowed), in one weapon at
// most (a second CodeNotAllowed); a code outside the class's list that before already has is kept
// (step 21.8.5: the migration keeps every owned weapon), never added. A code the catalogue does not
// hold but that before already has is kept too, as the non-weapon pool keeps a stored id the
// catalogue does not hold (step 16: older data is never dropped). A pick without a code (an older
// client's type pick, step 21.6) is CodeEmpty on field weapon; another refusal is a
// *domain.FieldError on field weapon.<code>. Never nil.
func validWeaponPicks(book Book, cls Classes, lifted bool, picks, before []WeaponPick) ([]WeaponPick, error) {
	class := cls.specialist()
	specialized := false
	had, kept := map[string]bool{}, map[string]bool{}
	for _, p := range before {
		had[p.Code] = true
		// ADR 0042 step 15: a dual-class keeps its old class's picks, its
		// specialization too when that class specializes.
		kept[p.Code] = cls.isSpecialization(book, p) && cls.oldSpecialist(book)
	}
	seen := map[string]bool{}
	out := make([]WeaponPick, 0, len(picks))
	for _, p := range picks {
		field := FieldWeapon + "." + p.Code
		w, known := book.Weapon(p.Code)
		switch {
		case p.Code == "":
			return nil, &domain.FieldError{Field: FieldWeapon, Message: CodeEmpty}
		case !had[p.Code] && !known:
			return nil, &domain.FieldError{Field: field, Message: CodeUnknown}
		case seen[p.Code]:
			return nil, &domain.FieldError{Field: field, Message: CodeDuplicate}
		case p.Slots > WeaponPickSlots && known && !cls.isSpecialization(book, p) && !cls.anySpecializes(book) &&
			book.SpecializationCostedAt(w.WeaponType, p.Slots):
			// A specialization's cost, asked of a class none of whose entries specializes.
			return nil, &domain.FieldError{Field: field, Message: CodeNotAllowed}
		case p.Slots != WeaponPickSlots && !cls.isSpecialization(book, p):
			return nil, &domain.FieldError{Field: field, Message: CodeOutOfRange}
		case !had[p.Code] && !cls.weaponCodeAllowed(book, lifted, p.Code):
			return nil, &domain.FieldError{Field: field, Message: CodeWeaponNotAllowed}
		case cls.isSpecialization(book, p) && ((!specializes(book, class) && !kept[p.Code]) || specialized):
			return nil, &domain.FieldError{Field: field, Message: CodeNotAllowed}
		}
		specialized = specialized || cls.isSpecialization(book, p)
		seen[p.Code] = true
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
// not hold, and weapon_pick_to_redo {type, slots} on field weapon.<type> for each type pick the
// migration to weapon codes left to redo (step 21.8.3).
func SlotWarnings(s ProficiencySlots) []Warning {
	out := poolWarnings(FieldNonweapon, PoolNonweapon, s.Nonweapon.Spent, s.Nonweapon.Available)
	out = append(out, poolWarnings(FieldWeapon, PoolWeapon, s.Weapon.Spent, s.Weapon.Available)...)
	for _, p := range s.Picks {
		if !p.Known {
			out = append(out, Warning{Code: CodeProficiencyUnknown, Field: FieldNonweapon + "." + p.ID})
		}
	}
	for _, r := range s.WeaponRedo {
		n := r.Slots
		out = append(out, Warning{Code: CodeWeaponPickToRedo, Field: FieldWeapon + "." + r.Type, Type: r.Type, Slots: &n})
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
