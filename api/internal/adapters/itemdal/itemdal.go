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

func toItem(r row) (itemsvc.Item, error) {
	powers, err := itemsvc.DecodePowers(r.Powers)
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("item %s: %w", r.ID, err)
	}
	return itemsvc.Item{
		ID: r.ID, Position: int(r.Position), Name: r.Name, Type: r.Type,
		Behaviour:  itemsvc.Behaviour(r.Behaviour),
		MagicBonus: int(r.MagicBonus), AmmoKind: r.AmmoKind,
		Count: int(r.Count), MaxCount: int(r.MaxCount),
		EffectDice: r.EffectDice, EffectSave: r.EffectSave, Notes: r.Notes,
		Powers: powers,
	}, nil
}

// List returns every item in sheet order.
func (d *DAL) List(ctx context.Context) ([]itemsvc.Item, error) {
	rows, err := queries.ListItems().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := make([]itemsvc.Item, 0, len(rows))
	for _, r := range rows {
		it, err := toItem(r)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
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
		return toItem(rows[0])
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

// Increment adds n with a single UPDATE and returns the updated item;
// ErrItemNotFound when the id is unknown.
func (d *DAL) Increment(ctx context.Context, itemID string, n int) (itemsvc.Item, error) {
	rows, err := queries.IncrementItem(int64(n), itemID).All(ctx, d.exec(ctx))
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("increment item %s: %w", itemID, err)
	}
	if len(rows) == 0 {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	return toItem(rows[0])
}

// ResetDaily refills every daily item to its max count.
func (d *DAL) ResetDaily(ctx context.Context) error {
	if _, err := queries.ResetDailyItems().Exec(ctx, d.exec(ctx)); err != nil {
		return fmt.Errorf("reset daily items: %w", err)
	}
	return nil
}

// Delete removes an item (a charges item spent to 0). ErrItemNotFound when
// the id is unknown.
func (d *DAL) Delete(ctx context.Context, itemID string) error {
	ids, err := queries.DeleteItem(itemID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("delete item %s: %w", itemID, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	return nil
}

// Insert writes a whole item (a void restoring a removed item), powers
// included.
func (d *DAL) Insert(ctx context.Context, it itemsvc.Item) error {
	powers, err := itemsvc.EncodePowers(it.Powers)
	if err != nil {
		return err
	}
	_, err = queries.InsertItem([]queries.InsertItem_Items{{
		ID: it.ID, Position: int64(it.Position), Name: it.Name, Type: it.Type,
		Behaviour: string(it.Behaviour), MagicBonus: int64(it.MagicBonus), AmmoKind: it.AmmoKind,
		Count: int64(it.Count), MaxCount: int64(it.MaxCount),
		EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes, Powers: powers,
	}}).Exec(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("insert item %s: %w", it.ID, err)
	}
	return nil
}

// SetCount writes an item's count back (an action-log void restoring a
// snapshot). ErrItemNotFound when the id is unknown.
func (d *DAL) SetCount(ctx context.Context, itemID string, count int) error {
	ids, err := queries.SetItemCount(int64(count), itemID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("set item %s count: %w", itemID, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	return nil
}

// Get returns one item; ErrItemNotFound when the id is unknown.
func (d *DAL) Get(ctx context.Context, itemID string) (itemsvc.Item, error) {
	r, err := queries.GetItem(itemID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("get item %s: %w", itemID, err)
	}
	return toItem(r)
}

// Update overwrites every field of an existing item, powers included.
// ErrItemNotFound when the id is unknown.
func (d *DAL) Update(ctx context.Context, it itemsvc.Item) error {
	powers, err := itemsvc.EncodePowers(it.Powers)
	if err != nil {
		return err
	}
	ids, err := queries.UpdateItem(int64(it.Position), it.Name, it.Type, string(it.Behaviour),
		int64(it.MagicBonus), it.AmmoKind, int64(it.Count), int64(it.MaxCount),
		it.EffectDice, it.EffectSave, it.Notes, powers, it.ID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("update item %s: %w", it.ID, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, it.ID)
	}
	return nil
}
