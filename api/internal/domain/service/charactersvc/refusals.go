package charactersvc

import (
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// The creation's refusal rules (ADR 0030 step 1, ADR 0040 step 8, ADR 0042 steps 1 and 2): pure
// functions of the entered character, the class and race definitions and the ruleset. They read
// no store and derive nothing, so each family is tested on its own; check (create.go) calls them
// in the form's order, for Create and for Draft alike.

// entries are the class entries n enters: Classes, else the one active entry Class and XP state.
func (n NewCharacter) entries() []NewClass {
	if len(n.Classes) > 0 {
		return n.Classes
	}
	return []NewClass{{Class: n.Class, XP: n.XP}}
}

// field is the name of an entry's field in a refusal: the one-class case keeps the flat names.
func (n NewCharacter) field(i int, name string) string {
	if len(n.Classes) == 0 {
		return name
	}
	return FieldClasses + "." + strconv.Itoa(i) + "." + name
}

// frozen reports whether the entry is a class left (a dual-class's old one).
func (e NewClass) frozen() bool { return e.LeftAtLevel != 0 }

// classShape is ADR 0042 step 1's refusals of the entries as a whole (the book's limits on the
// classes of a character), in the order of the entries.
func classShape(n NewCharacter) []error {
	if len(n.Classes) == 0 {
		return nil
	}
	var out []error
	refuse := func(field, code string) { out = append(out, &domain.FieldError{Field: field, Message: code}) }
	active, frozen := 0, 0
	for i, e := range n.Classes {
		if e.frozen() {
			frozen++
		} else {
			active++
		}
		for j := range i {
			if n.Classes[j].Class != e.Class {
				continue
			}
			if n.Classes[j].frozen() != e.frozen() {
				// The class listed twice is one left and one advanced in: the new class is one already left.
				if !e.frozen() {
					refuse(n.field(i, "class"), CodeClassAlreadyLeft)
				} else {
					refuse(n.field(j, "class"), CodeClassAlreadyLeft)
				}
			} else {
				refuse(n.field(i, "class"), CodeDuplicateClass)
			}
			break
		}
		switch {
		case e.LeftAtLevel < 0 || e.LeftAtLevel > classes.MaxLevel:
			refuse(n.field(i, "leftAtLevel"), CodeOutOfRange)
		case e.frozen() && e.LeftAtLevel < classes.MinLeftAtLevel:
			refuse(n.field(i, "leftAtLevel"), CodeLevelTooLow)
		}
		if e.Level < 0 || e.Level > classes.MaxLevel || (e.frozen() && e.Level != 0 && e.Level != e.LeftAtLevel) {
			refuse(n.field(i, "level"), CodeOutOfRange)
		}
	}
	switch {
	case active == 0:
		refuse(FieldClasses, CodeNoActiveClass)
	case active > classes.MaxMultiClasses:
		refuse(FieldClasses, CodeTooManyClasses)
	case active > 1 && frozen > 0:
		refuse(FieldClasses, CodeMultiAndDualClass)
	}
	return out
}

// multiClassClassRefusals are ADR 0042 step 2.1's refusal of a class the catalogue marks as never
// one of a multi-class character's classes (`multiclass: false`): each such active entry of a
// character with more than one active entry. A class left (dual-class) is not concerned.
func multiClassClassRefusals(n NewCharacter, defs *classes.Registry) []error {
	active := 0
	for _, e := range n.Classes {
		if !e.frozen() {
			active++
		}
	}
	if active < 2 {
		return nil
	}
	var out []error
	for i, e := range n.Classes {
		if e.frozen() {
			continue
		}
		if def, ok := defs.Class(e.Class); ok && def.NoMultiClass {
			out = append(out, &domain.FieldError{Field: n.field(i, "class"), Message: CodeMultiClassClassNotAllowed})
		}
	}
	return out
}

// raceRefusals are ADR 0042 step 2's refusals of the race for the entries: the book's limit on a
// multi-class or a dual-class race (waived by the ruleset's racial lift, ADR 0041 step 2.2, which
// also waives DMG Table 7's dash for these characters) and the dash otherwise. The book's
// combinations per race are not enforced: the catalogue has no rows for them, so with the lift off
// a multi-class character needs a restricted race and no more.
//
// allows is DMG Table 7's verdict for a race and a class (the Tables' RaceAllows).
func raceRefusals(lift ruleset.RacialLift, defs *classes.Registry, n NewCharacter, allows func(race, class string) (bool, error)) ([]error, error) {
	race := raceOf(defs, n.Race)
	active, frozen := 0, 0
	for _, e := range n.entries() {
		if e.frozen() {
			frozen++
		} else {
			active++
		}
	}
	multi, dual := active > 1, frozen > 0
	switch {
	case dual && !lift.DualclassAnyRace && race.Restricted():
		return []error{&domain.FieldError{Field: FieldClasses, Message: CodeDualClassNotAllowed}}, nil
	case multi && !lift.MulticlassAnyRace && !race.Restricted():
		return []error{&domain.FieldError{Field: FieldClasses, Message: CodeMultiClassNotAllowed}}, nil
	case (dual && lift.DualclassAnyRace) || (multi && lift.MulticlassAnyRace):
		return nil, nil // the lift waives the dash
	}
	for _, e := range n.entries() {
		allowed, err := allows(n.Race, e.Class)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return []error{&domain.FieldError{Field: "race", Message: CodeRaceNotAllowed}}, nil
		}
	}
	return nil, nil
}

// primeRefusals are ADR 0042 step 2.2's prime requisites of a dual-class character: 15 in every one
// of a class it left, 17 in every one of its new class.
func primeRefusals(n NewCharacter, defs *classes.Registry, scores map[string]int) []error {
	var out []error
	if len(n.Classes) == 0 {
		return nil
	}
	dual := false
	for _, e := range n.Classes {
		dual = dual || e.frozen()
	}
	if !dual {
		return nil
	}
	for i, e := range n.Classes {
		def, ok := defs.Class(e.Class)
		if !ok {
			continue
		}
		min := DualClassNewPrime
		if e.frozen() {
			min = DualClassOldPrime
		}
		for _, a := range def.PrimeRequisites {
			if v, ok := scores[a]; ok && v < min {
				out = append(out, &domain.FieldError{Field: n.field(i, "class"), Message: CodePrimeRequisiteTooLow})
				break
			}
		}
	}
	return out
}
