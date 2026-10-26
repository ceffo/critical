package charactersvc

import (
	"context"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/psionics"
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
	rows, err := s.rules.PSPRecoveryRows()
	if err != nil {
		return err
	}
	c.Derived.PSP = &PSPPool{Current: actionlog.CurrentPSP(c.PSPMax, total), Max: c.PSPMax, Recovery: rows}
	return nil
}

// setPowers is ADR 0043 step 9 on a read of c: a character with a psionics
// entry gets its known powers, Table 4's allowance at that entry's realized
// level beside what the stored picks take. It reads no store; the picks come
// with the character.
func (s *service) setPowers(c *Character) error {
	e, _, ok, err := psionicsEntry(c, s.rules)
	if err != nil || !ok {
		return err
	}
	row, err := s.rules.PowerAllowanceFor(e.RealizedLevel)
	if err != nil {
		return err
	}
	picks := slices.Clone(c.PsionicPicks)
	if picks == nil {
		picks = []psionics.Pick{}
	}
	c.Derived.Powers = &PowerKnowledge{
		Allowance: psionics.Allowance{
			Disciplines: row.Disciplines, Sciences: row.Sciences, Devotions: row.Devotions, DefenceModes: row.DefenceModes,
		},
		Used:    psionics.Count(s.powers, picks),
		Primary: psionics.Primary(picks),
		Picks:   picks,
	}
	return nil
}

// A night's sleep (ADR 0043 step 7, Q3): 8 hours at Table 6's sleeping rate,
// the book's constants for every psionic class.
const (
	nightHours    = 8
	sleepActivity = "sleeping"
)

// NightsPSPRecovery is ADR 0043 step 7: a night's sleep, nightHours at Table
// 6's sleeping rate (96 at the book's 12 an hour), for a character with a
// psionics entry; 0 for any other. The long rest's entry carries it and the
// recording path caps it at the PSPs spent (step 5).
func (s *service) NightsPSPRecovery(ctx context.Context, id int64) (int, error) {
	c, err := s.reader.GetCharacter(ctx, id)
	if err != nil {
		return 0, err
	}
	_, _, ok, err := psionicsEntry(c, s.rules)
	if err != nil || !ok {
		return 0, err
	}
	rate, err := s.rules.PSPRecoveryRate(sleepActivity)
	if err != nil {
		return 0, err
	}
	return nightHours * rate, nil
}
