// Package strength owns the Strength figure: the hit-probability and damage
// adjustments of PHB Table 1 (ADR 0046 step 1, applied to Strength by the
// follow-up of its question 6; ADR 0002 step 4). It holds the table's row
// type, the validation of a table and the lookup, and nothing else reads
// those rows. It is pure (ADR 0046 step 1.2): the rows are values handed in
// by the caller that reads the catalogue, no port, no dice, no store. No type
// here carries a transport tag (ADR 0046 step 6.1).
package strength

import (
	"errors"
	"fmt"
)

// ExceptionalMax is the exceptional percentile of 18/00. Exceptional strength
// is the percentile of "18/xx": 0 for a plain 18, 1-99 for 18/01-18/99,
// ExceptionalMax for 18/00.
const ExceptionalMax = 100

// MaxScore is the highest Strength score of Table 1.
const MaxScore = 25

// Row is one row of PHB Table 1. ExcMin/ExcMax are the 18/xx range (0/0 for
// rows with no exceptional part).
type Row struct {
	StrMin, StrMax int
	ExcMin, ExcMax int
	Hit, Dmg       int
}

// Mods are the adjustments a Strength score gives: the hit-probability
// adjustment and the damage adjustment.
type Mods struct {
	Hit int
	Dmg int
}

// Table is a validated PHB Table 1: scores 1..MaxScore, with 18 split into a
// contiguous 0..ExceptionalMax exceptional range. The zero Table has no rows
// and refuses every score.
type Table struct {
	rows []Row
}

// NewTable validates the rows and builds the table: they must cover the
// scores 1..MaxScore in order, 18 as contiguous exceptional ranges from 0 to
// ExceptionalMax.
func NewTable(rows []Row) (Table, error) {
	if err := validate(rows); err != nil {
		return Table{}, err
	}
	return Table{rows: append([]Row(nil), rows...)}, nil
}

// Of is the adjustments of a score. exceptional only matters when score is
// 18 (it is ignored below and above); a score outside 1..MaxScore or an
// exceptional outside 0..ExceptionalMax is an error.
func (t Table) Of(score, exceptional int) (Mods, error) {
	if exceptional < 0 || exceptional > ExceptionalMax {
		return Mods{}, fmt.Errorf("exceptional strength %d out of range 0..%d", exceptional, ExceptionalMax)
	}
	if score != 18 {
		exceptional = 0
	}
	for _, r := range t.rows {
		if score >= r.StrMin && score <= r.StrMax && exceptional >= r.ExcMin && exceptional <= r.ExcMax {
			return Mods{Hit: r.Hit, Dmg: r.Dmg}, nil
		}
	}
	return Mods{}, fmt.Errorf("strength %d out of range 1..%d", score, MaxScore)
}

func validate(rows []Row) error {
	next := 1 // next expected str score
	exc := 0  // next expected exceptional percentile within score 18
	for i, r := range rows {
		if r.StrMin > r.StrMax || r.ExcMin > r.ExcMax {
			return fmt.Errorf("row %d: min above max", i)
		}
		if r.StrMin != r.StrMax && (r.ExcMin != 0 || r.ExcMax != 0) {
			return fmt.Errorf("row %d: exceptional range on a multi-score row", i)
		}
		if r.StrMin != 18 && (r.ExcMin != 0 || r.ExcMax != 0) {
			return fmt.Errorf("row %d: exceptional range outside STR 18", i)
		}
		if r.StrMin == 18 {
			if next != 18 || r.StrMax != 18 || r.ExcMin != exc || r.ExcMax > ExceptionalMax {
				return fmt.Errorf("row %d: STR 18 exceptional ranges must be contiguous from 0 to %d", i, ExceptionalMax)
			}
			exc = r.ExcMax + 1
			if r.ExcMax == ExceptionalMax {
				next = 19
			}
			continue
		}
		if r.StrMin != next {
			return fmt.Errorf("row %d: score %d expected, got %d", i, next, r.StrMin)
		}
		next = r.StrMax + 1
	}
	if next != MaxScore+1 {
		return errors.New("must cover scores 1..25 (with 18/00 as the last 18 row)")
	}
	return nil
}
