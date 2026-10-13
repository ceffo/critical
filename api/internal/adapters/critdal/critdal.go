// Package critdal is the storage adapter for the house crit table consumed by
// combatsvc (combatsvc.CritTable). It delegates to bob-generated typed queries.
package critdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/critdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of combatsvc.CritTable.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// List returns the stored rows by ascending range start; none means the
// table was never edited.
func (d *DAL) List(ctx context.Context) ([]combatsvc.CritRow, error) {
	rows, err := queries.ListCritRows().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list crit table: %w", err)
	}
	out := make([]combatsvc.CritRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, combatsvc.CritRow{From: int(r.RollFrom), To: int(r.RollTo), Multiplier: int(r.Multiplier)})
	}
	return out, nil
}

// Replace swaps the whole table. The caller runs it in a transaction so a
// failed insert leaves the previous table in place.
func (d *DAL) Replace(ctx context.Context, rows []combatsvc.CritRow) error {
	if _, err := queries.DeleteCritRows().Exec(ctx, d.exec(ctx)); err != nil {
		return fmt.Errorf("clear crit table: %w", err)
	}
	for _, r := range rows {
		_, err := queries.InsertCritRow([]queries.InsertCritRow_CritTableRows{{
			RollFrom: int64(r.From), RollTo: int64(r.To), Multiplier: int64(r.Multiplier),
		}}).Exec(ctx, d.exec(ctx))
		if err != nil {
			return fmt.Errorf("insert crit row %d-%d: %w", r.From, r.To, err)
		}
	}
	return nil
}
