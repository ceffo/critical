// Package hpdal is the storage adapter for hpsvc. HP changes live in the
// action log (actionlogdal); this adapter only reads the character's maximum.
package hpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/hpdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of hpsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

// New constructs a DAL bound to the given Querier (pool or transaction).
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// GetMaxHP fetches hp_max from the character row. Maps sql.ErrNoRows to
// domain.ErrCharacterNotFound.
func (d *DAL) GetMaxHP(ctx context.Context, characterID int64) (int, error) {
	maxHP, err := queries.GetCharacterMaxHp(characterID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get hp_max: %w", err)
	}
	return int(maxHP), nil
}
