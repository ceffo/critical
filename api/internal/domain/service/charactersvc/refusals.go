package charactersvc

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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

// cleanName is the trimmed name, or the field error naming what is wrong.
func cleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	switch {
	case name == "":
		return "", &domain.FieldError{Field: "name", Message: CodeEmpty}
	case utf8.RuneCountInString(name) > MaxNameLen:
		return "", &domain.FieldError{Field: "name", Message: CodeTooLong}
	}
	return name, nil
}

// validSkills is ADR 0030 steps 1, 17 and 18: a class with skill sets enters
// its points in its first set (skills.ValidSkillPoints, skills.FirstSet); any
// other class enters none. The armour is no entry (ADR 0033 step 14).
func validSkills(n NewCharacter, def classes.Class) (skills.SetPoints, error) {
	set := skills.FirstSet(def)
	points, err := skills.ValidSkillPoints(def, set, n.SkillPoints)
	if err != nil {
		return nil, err
	}
	return skills.SetPoints(nil).With(set, points), nil
}

// validExceptional is ADR 0030 step 9: an exceptional strength is optional,
// 1 to 100, and only with Strength 18 for a character one of whose active
// classes has exceptional_strength (ADR 0040 step 3, ADR 0042 step 12) of a
// race that rolls it.
func validExceptional(n NewCharacter, active []classes.Class, race classes.Race) error {
	switch {
	case n.StrExceptional == 0:
		return nil
	case n.StrExceptional < MinStrExceptional || n.StrExceptional > MaxStrExceptional:
		return &domain.FieldError{Field: "strExceptional", Message: CodeOutOfRange}
	}
	for _, def := range active {
		if ruletablesvc.ExceptionalStrengthAllowed(def, race, n.Str) {
			return nil
		}
	}
	return &domain.FieldError{Field: "strExceptional", Message: CodeNotAllowed}
}

// refusal is the field error of code on field.
func refusal(field, code string) error { return &domain.FieldError{Field: field, Message: code} }

// creatableClass is the class's definition and whether a character may be created with it (ADR 0040
// step 8): a class listing a kind the code does not compute yet is not creatable.
func creatableClass(defs *classes.Registry, id string) (classes.Class, bool) {
	class, ok := defs.Class(id)
	return class, ok && class.Implemented()
}

// raceKnown reports whether the catalogue defines the race.
func raceKnown(defs *classes.Registry, id string) bool { return slices.Contains(defs.RaceIDs(), id) }

// abilityRefusal is ADR 0030 step 1's range of an ability score as it stands on the sheet, nil when in range.
func abilityRefusal(field string, v int) error {
	if v < MinAbility || v > MaxAbility {
		return refusal(field, CodeOutOfRange)
	}
	return nil
}

// hpMaxRefusal is the range of the max HP entered, nil when in range.
func hpMaxRefusal(v int) error {
	if v < MinHPMax || v > MaxHPMax {
		return refusal("hpMax", CodeOutOfRange)
	}
	return nil
}

// pspMaxRefusal is the range of the PSP maximum entered (ADR 0043 step 4.4); nil enters none.
func pspMaxRefusal(v *int) error {
	if v != nil && (*v < 1 || *v > MaxPSPMax) {
		return refusal(FieldPSPMax, CodeOutOfRange)
	}
	return nil
}

// xpRefusal is the range of the starting XP entered in field, nil when in range.
func xpRefusal(field string, v int) error {
	if v < MinStartingXP || v > MaxStartingXP {
		return refusal(field, CodeOutOfRange)
	}
	return nil
}
