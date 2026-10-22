package ruletablesvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The racial level limits of DMG Table 7 (ADR 0030 step 21), carried by
// catalogue format 5 (ADR 0021 step 11). The levels are shown on the race
// card, never enforced; a dash (a class the race does not admit) refuses a
// new character of that race and class (RaceAllows). Humans have no row: they
// have no limit in any class.

// LevelLimitRow is the DMG Table 7 cell for a race in a class: the highest
// level the race reaches (MaxLevel 1 to 20), no limit (Unlimited, the
// book's U; MaxLevel is then 0 and means nothing), or the race cannot belong
// to the class (MaxLevel 0, not Unlimited, the book's dash).
type LevelLimitRow struct {
	Race, Class string
	MaxLevel    int
	Unlimited   bool
}

// LevelLimits holds the validated racial level limits, one row per race and
// class. Built by NewLevelLimits only.
type LevelLimits struct {
	rows map[string]LevelLimitRow
}

// NewLevelLimits validates the DMG Table 7 rows (the rules the catalogue
// schema cannot express, ADR 0021 step 11.3): every race x class
// combination at most once, a race with a row having one per defined class
// (a race with none, a human, has no limit), and a level within 1..MaxLevel unless the cell
// is unlimited or a dash. A violation names the combination.
func NewLevelLimits(rows []LevelLimitRow, defs *classes.Registry) (*LevelLimits, error) {
	if defs == nil {
		return nil, errors.New("no class definitions")
	}
	if err := perRace(rows, func(r LevelLimitRow) (string, string) { return r.Race, r.Class }, defs.RaceIDs(), defs.ClassIDs()); err != nil {
		return nil, err
	}
	out := &LevelLimits{rows: make(map[string]LevelLimitRow, len(rows))}
	for _, r := range rows {
		switch {
		case r.Unlimited && r.MaxLevel != 0:
			return nil, fmt.Errorf("%s %s: unlimited with a level %d", r.Race, r.Class, r.MaxLevel)
		case r.MaxLevel < 0 || r.MaxLevel > MaxLevel:
			return nil, fmt.Errorf("%s %s: level %d outside 0..%d", r.Race, r.Class, r.MaxLevel, MaxLevel)
		}
		out.rows[r.Race+" "+r.Class] = r
	}
	return out, nil
}

// Limit returns the DMG Table 7 cell for race in class, false when the
// tables hold no row for the pair (a human, who has no limit in any class,
// or a code the table does not know).
func (l *LevelLimits) Limit(race, class string) (LevelLimitRow, bool) {
	r, ok := l.rows[race+" "+class]
	return r, ok
}

// RaceAllows reports whether a character of race may belong to class (DMG
// Table 7: a dash is a race the class does not admit), the check a new
// character's race and class pass (ADR 0030 step 21, as built for character
// creation). Humans may belong to any class. An error is a race or class the
// tables do not know, or a catalogue without the level limits.
func (t *Tables) RaceAllows(race, class string) (bool, error) {
	if race == "human" {
		return true, nil
	}
	l, ok := t.LevelLimits()
	if !ok {
		return false, fmt.Errorf("race %s class %s: the catalogue carries no racial level limits", race, class)
	}
	r, ok := l.Limit(race, class)
	if !ok {
		return false, fmt.Errorf("race %s class %s: no racial level limit", race, class)
	}
	return r.Unlimited || r.MaxLevel > 0, nil
}
