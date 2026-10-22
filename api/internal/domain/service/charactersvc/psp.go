package charactersvc

import (
	"context"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// psionicsEntry is c's first class entry whose class lists psionics (ADR
// 0043 step 2), with the feature; ok is false for a character with none.
func psionicsEntry(c *Character, rules Rules) (entry classes.Entry, f classes.Feature, ok bool, err error) {
	for _, e := range c.Entries() {
		def, err := rules.Class(e.Class)
		if err != nil {
			return classes.Entry{}, classes.Feature{}, false, err
		}
		if f, ok := def.Lists(classes.KindPsionics); ok {
			return e, f, true, nil
		}
	}
	return classes.Entry{}, classes.Feature{}, false, nil
}

// PSPMaxOf is ADR 0043 step 4 for c: the PSP maximum of its psionics entry at
// that entry's realized level, from the sheet's scores. A multi-class
// character's maximum is the psionics entry's alone (a thief 15/psionicist 14
// reads level 14) and its gain is not divided by its classes (Q2). ok is false
// for a character with no psionics entry. It computes; the maximum a
// character has is the stored one (step 4.4, Character.PSPMax).
func PSPMaxOf(c *Character, rules Rules) (m ruletablesvc.PSPMax, ok bool, err error) {
	e, f, ok, err := psionicsEntry(c, rules)
	if err != nil || !ok {
		return ruletablesvc.PSPMax{}, false, err
	}
	score := func(a string) int {
		s, _ := c.Score(Ability(a))
		return s
	}
	m, err = rules.PSPMaxFor(f.Psionics.PSP, score, e.RealizedLevel)
	if err != nil {
		return ruletablesvc.PSPMax{}, false, err
	}
	return m, true, nil
}

// setPSP is ADR 0043 step 5 on a read of c: a character with a psionics entry
// gets its PSP pool, the stored maximum (step 4.4) and the current PSPs (the
// maximum plus the ledger total, actionlog.CurrentPSP). Only such a character
// pays the ledger read; any other has no pool (nil).
func (s *service) setPSP(ctx context.Context, c *Character) error {
	_, _, ok, err := psionicsEntry(c, s.rules)
	if err != nil || !ok {
		return err
	}
	total, err := s.hp.PSPTotal(ctx, c.ID)
	if err != nil {
		return err
	}
	c.Derived.PSP = &PSPPool{Current: actionlog.CurrentPSP(c.PSPMax, total), Max: c.PSPMax}
	return nil
}
