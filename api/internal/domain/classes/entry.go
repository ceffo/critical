package classes

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
)

// EntryState is whether a class entry earns XP and levels (ADR 0042 step 1.4).
type EntryState string

const (
	// EntryActive is an entry that earns XP and levels.
	EntryActive EntryState = "active"
	// EntryFrozen is a dual-class character's old class: no XP, no level.
	EntryFrozen EntryState = "frozen"
)

// MaxMultiClasses is the most active classes of a multi-class character (ADR
// 0042 step 1: the book's combinations have at most three).
const MaxMultiClasses = 3

// MinLeftAtLevel is the lowest level a class can be left at (ADR 0042 step 1:
// the book requires at least 2nd level before switching).
const MinLeftAtLevel = 2

// Entry is one class of a character (ADR 0042 step 1): the class, the level
// taken in it, the XP entered for it and, for a frozen one, the level it was
// left at.
type Entry struct {
	// Class is a class id of the catalogue.
	Class string
	// RealizedLevel is the level taken in the class (ADR 0030 step 7).
	RealizedLevel int
	// StartingXP is the entry's XP as entered; its XP is this plus its share
	// of the ledger (ADR 0042 step 5).
	StartingXP int
	// State is active or frozen.
	State EntryState
	// LeftAtLevel is a frozen entry's level when it was left (equal to its
	// RealizedLevel); 0 for an active one.
	LeftAtLevel int
}

// SingleEntry is the one active entry of a single-class character.
func SingleEntry(class string, realizedLevel, startingXP int) Entry {
	return Entry{Class: class, RealizedLevel: realizedLevel, StartingXP: startingXP, State: EntryActive}
}

// Primary is the first active entry, ok false when there is none. A
// single-class character's only entry; the single-class rules read it until
// the multi-class math lands.
func Primary(entries []Entry) (Entry, bool) {
	for _, e := range entries {
		if e.State == EntryActive {
			return e, true
		}
	}
	return Entry{}, false
}

// ValidateEntries refuses the shapes ADR 0042 step 1 refuses, as field errors
// on "classes": no active entry, a class listed twice, a frozen entry without
// a left-at level or with one under 2, a multi-class with more than three
// classes, a character both multi-class and dual-class. It does not read the
// catalogue (an unknown class id is the caller's).
func ValidateEntries(entries []Entry) error {
	const field = "classes"
	active, frozen := 0, 0
	var seen []string
	for _, e := range entries {
		if slices.Contains(seen, e.Class) {
			return domain.FieldErrorf(field, "class %q listed twice", e.Class)
		}
		seen = append(seen, e.Class)
		switch e.State {
		case EntryActive:
			active++
			if e.LeftAtLevel != 0 {
				return domain.FieldErrorf(field, "active class %q has a left-at level", e.Class)
			}
		case EntryFrozen:
			frozen++
			if e.LeftAtLevel < MinLeftAtLevel {
				return domain.FieldErrorf(field, "frozen class %q needs a left-at level of %d or more", e.Class, MinLeftAtLevel)
			}
			if e.LeftAtLevel != e.RealizedLevel {
				return domain.FieldErrorf(field, "frozen class %q left at a level other than its own", e.Class)
			}
		default:
			return domain.FieldErrorf(field, "class %q has an unknown state %q", e.Class, e.State)
		}
	}
	switch {
	case active == 0:
		return domain.FieldErrorf(field, "a character needs an active class")
	case active > MaxMultiClasses:
		return domain.FieldErrorf(field, "a multi-class character has at most %d classes", MaxMultiClasses)
	case active > 1 && frozen > 0:
		return domain.FieldErrorf(field, "a character cannot be both multi-class and dual-class")
	}
	return nil
}

// Active are the active entries, in order (ADR 0042 step 1.4): the classes
// that earn XP and level.
func Active(entries []Entry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.State == EntryActive {
			out = append(out, e)
		}
	}
	return out
}

// Level is the level the entry's features and figures are read at (ADR 0042
// steps 12 and 14): its realized level, a frozen entry's left-at level.
func (e Entry) Level() int {
	if e.State == EntryFrozen {
		return e.LeftAtLevel
	}
	return e.RealizedLevel
}

// Grant is a feature kind a character has through one of its class entries
// (ADR 0042 step 12): the feature with its params, the entry's class and the
// level the rule reads it at (the entry's Level).
type Grant struct {
	Feature Feature
	Class   Class
	Level   int
}

// Grants is ADR 0042 step 12 (amending ADR 0040 step 5): the grants of kind
// through entries, in entry order: one for each entry whose class lists kind
// at or under the entry's Level. Two entries granting the same kind each keep
// their own grant, read at their own level. def is the class definition of an
// id (Rules.Class); its error is returned.
func Grants(entries []Entry, def func(class string) (Class, error), kind string) ([]Grant, error) {
	var out []Grant
	for _, e := range entries {
		c, err := def(e.Class)
		if err != nil {
			return nil, err
		}
		if f, ok := c.Feature(kind, e.Level()); ok {
			out = append(out, Grant{Feature: f, Class: c, Level: e.Level()})
		}
	}
	return out, nil
}

// DualClass is a character with a frozen entry (ADR 0042 step 1): a
// dual-class's old class beside its one active class.
func DualClass(entries []Entry) bool {
	return slices.ContainsFunc(entries, func(e Entry) bool { return e.State == EntryFrozen })
}

// Crossed is ADR 0042 step 13: a dual-class character has reached the
// crossover when its active entry's realized level is above every frozen
// entry's left-at level (PHB multi/dual, Dual-Class: higher than his maximum
// level in any previous class). Before it, the old classes' abilities warn
// dual_class_old_ability when used, their THAC0 and saves do not count (steps
// 8 and 9) and a level-up gains no hit points (step 11). A character without
// a frozen entry has nothing to cross: true.
func Crossed(entries []Entry) bool {
	active, ok := Primary(entries)
	if !ok {
		return true
	}
	return active.RealizedLevel > MaxLeftAt(entries)
}

// MaxLeftAt is the highest left-at level of the frozen entries (ADR 0042 step
// 13), 0 without one: the level a dual-class's active entry must pass.
func MaxLeftAt(entries []Entry) int {
	m := 0
	for _, e := range entries {
		if e.State == EntryFrozen {
			m = max(m, e.LeftAtLevel)
		}
	}
	return m
}

// Dormant is a frozen entry before the crossover (ADR 0042 step 13): its
// features, spells and equipment allowance may be used, each use warning
// dual_class_old_ability.
func Dormant(entries []Entry, e Entry) bool {
	return e.State == EntryFrozen && !Crossed(entries)
}
