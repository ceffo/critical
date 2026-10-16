// Package dicedal is the storage adapter for the dice composer's favourites
// (dicesvc.DAL). It delegates to sqlc-generated queries.
package dicedal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/dicedal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
)

// DAL is the implementation of dicesvc.DAL over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

var _ dicesvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// List returns the character's favourites, in the user's order.
func (d *DAL) List(ctx context.Context, characterID int64) ([]dicesvc.Favourite, error) {
	rows, err := d.q.ListDiceFavourites(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list dice_favourites: %w", err)
	}
	out := make([]dicesvc.Favourite, 0, len(rows))
	for _, r := range rows {
		out = append(out, dicesvc.Favourite{ID: r.ID, Formula: r.Formula})
	}
	return out, nil
}

// Insert saves a formula after the last favourite (position 1 for the first
// one) and returns its id.
func (d *DAL) Insert(ctx context.Context, characterID int64, formula string) (int64, error) {
	var position int64 = 1
	last, err := d.q.LastDiceFavouritePosition(ctx, characterID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return 0, fmt.Errorf("last dice_favourites position: %w", err)
	default:
		position = last + 1
	}
	id, err := d.q.InsertDiceFavourite(ctx, queries.InsertDiceFavouriteParams{
		CharacterID: characterID, Formula: formula, Position: position,
	})
	if err != nil {
		return 0, fmt.Errorf("insert dice_favourites: %w", err)
	}
	return id, nil
}

// Delete removes a favourite and reports whether it existed.
func (d *DAL) Delete(ctx context.Context, characterID, id int64) (bool, error) {
	n, err := d.q.DeleteDiceFavourite(ctx, queries.DeleteDiceFavouriteParams{CharacterID: characterID, ID: id})
	if err != nil {
		return false, fmt.Errorf("delete dice_favourites %d: %w", id, err)
	}
	return n > 0, nil
}

// SetPosition moves a favourite to the given position and reports whether it
// existed.
func (d *DAL) SetPosition(ctx context.Context, characterID, id, position int64) (bool, error) {
	n, err := d.q.SetDiceFavouritePosition(ctx, queries.SetDiceFavouritePositionParams{
		Position: position, CharacterID: characterID, ID: id,
	})
	if err != nil {
		return false, fmt.Errorf("set dice_favourites %d position: %w", id, err)
	}
	return n > 0, nil
}
