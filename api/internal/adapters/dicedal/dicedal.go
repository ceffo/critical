// Package dicedal is the storage adapter for the dice composer's favourites
// (dicesvc.DAL). It delegates to bob-generated typed queries.
package dicedal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/dicedal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of dicesvc.DAL.
type DAL struct {
	q sqlite.Querier
}

var _ dicesvc.DAL = (*DAL)(nil)

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// List returns the character's favourites, in the user's order.
func (d *DAL) List(ctx context.Context, characterID int64) ([]dicesvc.Favourite, error) {
	rows, err := queries.ListDiceFavourites(characterID).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list dice_favourites: %w", err)
	}
	out := make([]dicesvc.Favourite, 0, len(rows))
	for _, r := range rows {
		out = append(out, dicesvc.Favourite{ID: r.ID, Formula: r.Formula})
	}
	return out, nil
}

// Insert saves a formula after the last favourite and returns its id.
func (d *DAL) Insert(ctx context.Context, characterID int64, formula string) (int64, error) {
	last, err := queries.LastDiceFavouritePosition(characterID).All(ctx, d.exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("last dice_favourites position: %w", err)
	}
	var position int64 = 1
	if len(last) > 0 {
		position = last[0] + 1
	}
	id, err := queries.InsertDiceFavourite([]queries.InsertDiceFavourite_DiceFavourites{{
		CharacterID: characterID, Formula: formula, Position: position,
	}}).One(ctx, d.exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("insert dice_favourites: %w", err)
	}
	return id, nil
}

// Delete removes a favourite and reports whether it existed.
func (d *DAL) Delete(ctx context.Context, characterID, id int64) (bool, error) {
	ids, err := queries.DeleteDiceFavourite(characterID, id).All(ctx, d.exec(ctx))
	if err != nil {
		return false, fmt.Errorf("delete dice_favourites %d: %w", id, err)
	}
	return len(ids) > 0, nil
}

// SetPosition moves a favourite to the given position and reports whether it
// existed.
func (d *DAL) SetPosition(ctx context.Context, characterID, id, position int64) (bool, error) {
	ids, err := queries.SetDiceFavouritePosition(position, characterID, id).All(ctx, d.exec(ctx))
	if err != nil {
		return false, fmt.Errorf("set dice_favourites %d position: %w", id, err)
	}
	return len(ids) > 0, nil
}
