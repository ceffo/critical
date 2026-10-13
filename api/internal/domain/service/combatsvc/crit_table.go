package combatsvc

import (
	"context"
	"fmt"
	"sort"

	"github.com/ceffo/critical/api/internal/domain"
)

// CritRow is one inclusive D100 range of the house crit table and the
// multiplier a second D100 inside it gives.
type CritRow struct {
	From       int
	To         int
	Multiplier int
}

// Contains reports whether the D100 roll falls in the range.
func (r CritRow) Contains(roll int) bool { return roll >= r.From && roll <= r.To }

// CritTable is the whole table. Placeholder is true until the player has
// saved one: the default is a single row 1-100 -> x2.
type CritTable struct {
	Rows        []CritRow
	Placeholder bool
}

// defaultCritRows is the table before anyone edits it.
func defaultCritRows() []CritRow { return []CritRow{{From: 1, To: 100, Multiplier: 2}} }

// ValidateCritRows checks that rows (in any order) cover D100 1-100 exactly:
// every range inside 1-100 and not backwards, no overlap, no gap, every
// multiplier at least 1. The error names the offending row (1-based, as the
// editor lists them). It returns the rows sorted by range start.
func ValidateCritRows(rows []CritRow) ([]CritRow, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: the crit table needs at least one row covering 1-100", domain.ErrInvalidInput)
	}
	type indexed struct {
		CritRow
		n int
	}
	list := make([]indexed, len(rows))
	for i, r := range rows {
		n := i + 1
		switch {
		case r.From < 1 || r.From > 100 || r.To < 1 || r.To > 100:
			return nil, fmt.Errorf("%w: row %d (%d-%d) is outside 1-100", domain.ErrInvalidInput, n, r.From, r.To)
		case r.To < r.From:
			return nil, fmt.Errorf("%w: row %d (%d-%d) ends before it starts", domain.ErrInvalidInput, n, r.From, r.To)
		case r.Multiplier < 1:
			return nil, fmt.Errorf("%w: row %d (%d-%d) has multiplier %d, it must be at least 1", domain.ErrInvalidInput, n, r.From, r.To, r.Multiplier)
		}
		list[i] = indexed{r, n}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].From < list[j].From })

	next := 1 // the first D100 value not yet covered
	out := make([]CritRow, len(list))
	for i, r := range list {
		switch {
		case r.From < next:
			return nil, fmt.Errorf("%w: row %d (%d-%d) overlaps the row before it, which reaches %d", domain.ErrInvalidInput, r.n, r.From, r.To, next-1)
		case r.From > next:
			return nil, fmt.Errorf("%w: gap before row %d (%d-%d): %d-%d is not covered", domain.ErrInvalidInput, r.n, r.From, r.To, next, r.From-1)
		}
		next = r.To + 1
		out[i] = r.CritRow
	}
	if next != 101 {
		last := list[len(list)-1]
		return nil, fmt.Errorf("%w: gap after row %d (%d-%d): %d-100 is not covered", domain.ErrInvalidInput, last.n, last.From, last.To, next)
	}
	return out, nil
}

// lookupCrit finds the row holding the D100 roll. The stored table is always
// valid, so a miss means corrupt data.
func lookupCrit(rows []CritRow, roll int) (CritRow, error) {
	for _, r := range rows {
		if r.Contains(roll) {
			return r, nil
		}
	}
	return CritRow{}, fmt.Errorf("crit table has no row for D100 roll %d", roll)
}

// loadCritTable reads the stored table, or the default placeholder.
func (s *service) loadCritTable(ctx context.Context) (CritTable, error) {
	rows, err := s.crit.List(ctx)
	if err != nil {
		return CritTable{}, fmt.Errorf("load crit table: %w", err)
	}
	if len(rows) == 0 {
		return CritTable{Rows: defaultCritRows(), Placeholder: true}, nil
	}
	return CritTable{Rows: rows}, nil
}

// GetCritTable returns the table crits are looked up in.
func (s *service) GetCritTable(ctx context.Context) (CritTable, error) {
	return s.loadCritTable(ctx)
}

// SetCritTable validates and stores the whole table. An invalid table is
// refused and the stored one stays. This is not a logged game action: past
// attack entries keep the multiplier they were resolved with.
func (s *service) SetCritTable(ctx context.Context, rows []CritRow) (CritTable, error) {
	sorted, err := ValidateCritRows(rows)
	if err != nil {
		return CritTable{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.crit.Replace(ctx, sorted); err != nil {
			return fmt.Errorf("save crit table: %w", err)
		}
		return nil
	})
	if err != nil {
		return CritTable{}, err
	}
	return CritTable{Rows: sorted}, nil
}
