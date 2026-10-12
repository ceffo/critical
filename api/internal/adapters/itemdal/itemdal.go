// Package itemdal is the storage adapter for itemsvc. It delegates to
// bob-generated typed queries in the queries/ subpackage.
package itemdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/itemdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of itemsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

// exec joins the transaction carried by ctx (sqlite.InTx), if any.
func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

type row = queries.ListItemsRow

func toItem(r row) itemsvc.Item {
	return itemsvc.Item{
		ID: r.ID, Name: r.Name, Kind: itemsvc.Kind(r.Kind),
		MagicBonus: int(r.MagicBonus), AmmoKind: r.AmmoKind,
		Count: int(r.Count), MaxCount: int(r.MaxCount), PerDay: r.PerDay != 0,
		EffectDice: r.EffectDice, EffectSave: r.EffectSave, Notes: r.Notes,
	}
}

// List returns every item in sheet order.
func (d *DAL) List(ctx context.Context) ([]itemsvc.Item, error) {
	rows, err := queries.ListItems().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := make([]itemsvc.Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, toItem(r))
	}
	return out, nil
}

// Decrement spends one count with a single conditional UPDATE, so two
// concurrent uses of the last charge cannot both succeed and the count never
// goes below 0. When no row was updated, a follow-up read tells an unknown id
// from an empty item.
func (d *DAL) Decrement(ctx context.Context, itemID string) (itemsvc.Item, error) {
	rows, err := queries.DecrementItem(itemID).All(ctx, d.exec(ctx))
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("decrement item %s: %w", itemID, err)
	}
	if len(rows) == 1 {
		return toItem(rows[0]), nil
	}
	_, err = queries.GetItem(itemID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("get item %s: %w", itemID, err)
	}
	return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemEmpty, itemID)
}

// ResetPerDay refills every per-day item to its max count.
func (d *DAL) ResetPerDay(ctx context.Context) error {
	if _, err := queries.ResetPerDayItems().Exec(ctx, d.exec(ctx)); err != nil {
		return fmt.Errorf("reset per-day items: %w", err)
	}
	return nil
}
