// Package critdal is the storage adapter for the house crit table consumed by
// combatsvc (combatsvc.CritTable). It delegates to sqlc-generated queries.
package critdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/critdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// DAL is the implementation of combatsvc.CritTable over the sqlc-generated
// queries.
type DAL struct {
	q queries.Querier
}

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// List returns the stored rows by ascending range start; none means the
// table was never edited.
func (d *DAL) List(ctx context.Context) ([]combatsvc.CritRow, error) {
	rows, err := d.q.ListCritRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list crit table: %w", err)
	}
	out := make([]combatsvc.CritRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, combatsvc.CritRow{From: int(r.RollFrom), To: int(r.RollTo), Multiplier: r.Multiplier})
	}
	return out, nil
}

// Replace swaps the whole table: it clears it, then inserts the rows in order.
// The caller runs it in a transaction so a failed insert leaves the previous
// table in place.
func (d *DAL) Replace(ctx context.Context, rows []combatsvc.CritRow) error {
	if err := d.q.DeleteCritRows(ctx); err != nil {
		return fmt.Errorf("clear crit table: %w", err)
	}
	for _, r := range rows {
		err := d.q.InsertCritRow(ctx, queries.InsertCritRowParams{
			RollFrom: int64(r.From), RollTo: int64(r.To), Multiplier: r.Multiplier,
		})
		if err != nil {
			return fmt.Errorf("insert crit row %d-%d: %w", r.From, r.To, err)
		}
	}
	return nil
}
