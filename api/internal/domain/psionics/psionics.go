// Package psionics holds the rules of the powers a psionicist knows (ADR 0043
// step 9): the picks, Table 4's allowance and the checks that keep a list of
// picks within them. It is pure: the powers catalogue and Table 4 come in
// through the Book and the Allowance.
package psionics

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
)

// Pick kinds: what one entry of the list is.
const (
	// PickDiscipline is a discipline the character has access to; the first
	// one picked is the primary discipline (step 9.2).
	PickDiscipline = "discipline"
	// PickPower is a power the character knows.
	PickPower = "power"
	// PickRelearn is a relearn of a known science or devotion (+1 to its
	// score), which takes a slot of that power's kind (step 9.1).
	PickRelearn = "relearn"
)

// Pick is one entry of a character's list of known powers: a discipline id
// (PickDiscipline) or a power id (PickPower, PickRelearn). The list is
// edited as a whole and its order is kept: the first discipline is the
// primary one.
type Pick struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Allowance is Table 4 at a psionics entry's realized level (step 9.1): the
// most disciplines, sciences, devotions and defence modes known, cumulative.
type Allowance struct {
	Disciplines  int
	Sciences     int
	Devotions    int
	DefenceModes int
}

// Used counts the slots a list of picks takes, by the same kinds as
// Allowance.
type Used = Allowance

// Book is the powers catalogue the picks are checked against.
type Book interface {
	// Power is the power with id.
	Power(id string) (catalogue.Power, bool)
	// HasDiscipline reports whether id is a discipline of the catalogue.
	HasDiscipline(id string) bool
}

// Refusal codes (step 9): a list over the rules is refused with one of them.
const (
	CodeAllowanceExceeded      = "psionic_allowance_exceeded"
	CodePrimaryExceeded        = "primary_discipline_exceeded"
	CodeDevotionsBelowSciences = "devotions_below_twice_sciences"
	CodeDisciplineNotKnown     = "discipline_not_known"
	CodePrerequisiteNotKnown   = "prerequisite_not_known"
)

// Slot names a Refusal's Slot field takes (the kinds of Allowance).
const (
	SlotDiscipline  = "discipline"
	SlotScience     = "science"
	SlotDevotion    = "devotion"
	SlotDefenceMode = "defence_mode"
)

// ErrRefused matches every Refusal.
var ErrRefused = errors.New("known powers refused")

// Refusal is a list of picks that breaks the rules of step 9. It is
// structured data the web words: the code, the power or discipline that
// broke the rule and, for the counts, the slot kind with what the list takes
// and what the rules allow.
type Refusal struct {
	Code string
	// Power and Discipline name what the rule is about (either may be empty).
	Power      string
	Discipline string
	// Slot, Count and Limit: the slot kind, what the list takes and the most
	// the rule allows. For devotions_below_twice_sciences, Count is the
	// devotions and Limit the twice the sciences it needs.
	Slot  string
	Count int
	Limit int
}

func (r *Refusal) Error() string {
	return fmt.Errorf("known powers refused: %s (power %q, discipline %q, %s %d against %d)",
		r.Code, r.Power, r.Discipline, r.Slot, r.Count, r.Limit).Error()
}

// Is makes errors.Is(err, ErrRefused) true for a Refusal.
func (r *Refusal) Is(target error) bool { return target == ErrRefused }

// SlotOf is the slot kind a power takes (step 9.1): a science, a devotion or
// a defence mode. An attack mode takes a devotion's place; ADR 0043 step 9's
// book has one exception, Psionic Blast, which takes a science's slot. The
// catalogue cannot mark it yet, so every attack mode counts as a devotion
// here until a catalogue field says otherwise.
func SlotOf(p catalogue.Power) string {
	switch p.Kind {
	case catalogue.KindScience:
		return SlotScience
	case catalogue.KindDefenceMode:
		return SlotDefenceMode
	default:
		return SlotDevotion
	}
}

// Validate is ADR 0043 step 9 on a whole list of picks, against Table 4's
// allowance: it returns the slots the list takes, or the first rule it
// breaks. A malformed list (an unknown kind or id, a repeat, a relearn of a
// power not known or not a science or devotion) is a *FieldError-like plain
// error wrapping ErrInvalid; a list over the rules is a *Refusal.
func Validate(book Book, a Allowance, picks []Pick) (Used, error) {
	var used Used
	disciplines := map[string]bool{}
	known := map[string]catalogue.Power{}
	var primary string
	// Pass 1: the shape of the list, the disciplines and the known powers.
	for i, p := range picks {
		switch p.Kind {
		case PickDiscipline:
			if !book.HasDiscipline(p.ID) {
				return Used{}, invalid(i, "unknown discipline %q", p.ID)
			}
			if disciplines[p.ID] {
				return Used{}, invalid(i, "discipline %q picked twice", p.ID)
			}
			if primary == "" {
				primary = p.ID
			}
			disciplines[p.ID] = true
			used.Disciplines++
		case PickPower:
			pw, ok := book.Power(p.ID)
			if !ok {
				return Used{}, invalid(i, "unknown power %q", p.ID)
			}
			if _, dup := known[p.ID]; dup {
				return Used{}, invalid(i, "power %q picked twice", p.ID)
			}
			known[p.ID] = pw
		case PickRelearn:
		default:
			return Used{}, invalid(i, "unknown pick kind %q", p.Kind)
		}
	}
	// Pass 2: the powers' disciplines and prerequisites, the slots taken.
	type perDiscipline struct{ sciences, devotions int }
	by := map[string]*perDiscipline{}
	take := func(pw catalogue.Power) {
		slot := SlotOf(pw)
		switch slot {
		case SlotScience:
			used.Sciences++
		case SlotDevotion:
			used.Devotions++
		case SlotDefenceMode:
			used.DefenceModes++
			return // not counted in a discipline (the book, defence modes)
		}
		d := by[pw.Discipline]
		if d == nil {
			d = &perDiscipline{}
			by[pw.Discipline] = d
		}
		if slot == SlotScience {
			d.sciences++
		} else {
			d.devotions++
		}
	}
	for i, p := range picks {
		switch p.Kind {
		case PickPower:
			pw := known[p.ID]
			// A defence mode is learned whether or not the character has
			// access to its discipline.
			if SlotOf(pw) != SlotDefenceMode && !disciplines[pw.Discipline] {
				return Used{}, &Refusal{Code: CodeDisciplineNotKnown, Power: pw.ID, Discipline: pw.Discipline}
			}
			for _, pre := range pw.Prerequisites {
				if _, ok := known[pre]; !ok {
					return Used{}, &Refusal{Code: CodePrerequisiteNotKnown, Power: pw.ID, Discipline: pw.Discipline}
				}
			}
			take(pw)
		case PickRelearn:
			pw, ok := known[p.ID]
			if !ok {
				return Used{}, invalid(i, "relearn of %q, a power not known", p.ID)
			}
			if s := SlotOf(pw); s != SlotScience && s != SlotDevotion {
				return Used{}, invalid(i, "relearn of %q, which is neither a science nor a devotion", p.ID)
			}
			take(pw)
		}
	}
	if err := within(a, used); err != nil {
		return Used{}, err
	}
	// Step 9.2 and 9.3 by discipline, in the catalogue-independent sorted order
	// so the refusal is deterministic.
	ids := make([]string, 0, len(by))
	for id := range by {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		d := by[id]
		if d.devotions < 2*d.sciences {
			return Used{}, &Refusal{Code: CodeDevotionsBelowSciences, Discipline: id, Slot: SlotDevotion, Count: d.devotions, Limit: 2 * d.sciences}
		}
		if primary == "" || id == primary {
			continue
		}
		p := by[primary]
		if p == nil {
			p = &perDiscipline{}
		}
		// Never as many as the primary's, unless there are none to compare.
		if d.sciences > 0 && d.sciences >= p.sciences {
			return Used{}, &Refusal{Code: CodePrimaryExceeded, Discipline: id, Slot: SlotScience, Count: d.sciences, Limit: p.sciences - 1}
		}
		if d.devotions > 0 && d.devotions >= p.devotions {
			return Used{}, &Refusal{Code: CodePrimaryExceeded, Discipline: id, Slot: SlotDevotion, Count: d.devotions, Limit: p.devotions - 1}
		}
	}
	return used, nil
}

// within is step 9.1: every slot kind at most Table 4's count.
func within(a Allowance, u Used) error {
	for _, c := range []struct {
		slot        string
		used, limit int
	}{
		{SlotDiscipline, u.Disciplines, a.Disciplines},
		{SlotScience, u.Sciences, a.Sciences},
		{SlotDevotion, u.Devotions, a.Devotions},
		{SlotDefenceMode, u.DefenceModes, a.DefenceModes},
	} {
		if c.used > c.limit {
			return &Refusal{Code: CodeAllowanceExceeded, Slot: c.slot, Count: c.used, Limit: c.limit}
		}
	}
	return nil
}

// invalid is a malformed list: matches domain.ErrInvalidInput through the
// caller's wrapping (the service turns it into a field error on "picks").
func invalid(i int, format string, args ...any) error {
	return fmt.Errorf("%w: pick %d: %w", ErrInvalid, i, fmt.Errorf(format, args...))
}

// ErrInvalid wraps every malformed-list error of Validate.
var ErrInvalid = errors.New("invalid known powers")

// Primary is the primary discipline of a list: its first discipline pick, ""
// when it has none.
func Primary(picks []Pick) string {
	for _, p := range picks {
		if p.Kind == PickDiscipline {
			return p.ID
		}
	}
	return ""
}

// Count is the slots a list of picks takes, without checking any rule: the
// figure a read shows beside Table 4's allowance. A pick the book does not
// know takes nothing.
func Count(book Book, picks []Pick) Used {
	var used Used
	for _, p := range picks {
		switch p.Kind {
		case PickDiscipline:
			used.Disciplines++
		case PickPower, PickRelearn:
			pw, ok := book.Power(p.ID)
			if !ok {
				continue
			}
			switch SlotOf(pw) {
			case SlotScience:
				used.Sciences++
			case SlotDefenceMode:
				used.DefenceModes++
			default:
				used.Devotions++
			}
		}
	}
	return used
}

// catalogueBook is a Book over the powers catalogue read at start.
type catalogueBook struct {
	disciplines map[string]bool
	powers      map[string]catalogue.Power
}

// NewBook is the Book of a powers catalogue.
func NewBook(disciplines []catalogue.Discipline, powers []catalogue.Power) Book {
	b := catalogueBook{disciplines: make(map[string]bool, len(disciplines)), powers: make(map[string]catalogue.Power, len(powers))}
	for _, d := range disciplines {
		b.disciplines[d.ID] = true
	}
	for _, p := range powers {
		b.powers[p.ID] = p
	}
	return b
}

func (b catalogueBook) Power(id string) (catalogue.Power, bool) { p, ok := b.powers[id]; return p, ok }
func (b catalogueBook) HasDiscipline(id string) bool            { return b.disciplines[id] }
