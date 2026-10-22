// Package hpdal is the storage adapter for the HP and PSP read model
// (actionlogsvc.Maxima). HP and PSP changes live in the action log
// (actionlogdal); this adapter only reads the character's maxima.
package hpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/hpdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
)

// DAL is the implementation of actionlogsvc.Maxima over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// GetMaxHP fetches hp_max from the character row. Maps sql.ErrNoRows to
// domain.ErrCharacterNotFound.
func (d *DAL) GetMaxHP(ctx context.Context, characterID int64) (int, error) {
	maxHP, err := d.q.GetCharacterMaxHP(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get hp_max: %w", err)
	}
	return int(maxHP), nil
}

// GetMaxPSP fetches psp_max from the character row (ADR 0043 step 4.4; 0 for
// a character with no PSPs). Maps sql.ErrNoRows to
// domain.ErrCharacterNotFound.
func (d *DAL) GetMaxPSP(ctx context.Context, characterID int64) (int, error) {
	maxPSP, err := d.q.GetCharacterMaxPSP(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get psp_max: %w", err)
	}
	return int(maxPSP), nil
}
