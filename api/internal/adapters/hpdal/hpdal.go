// Package hpdal is the storage adapter for hpsvc. It satisfies hpsvc.DAL by
// delegating to bob-generated typed queries in the queries/ subpackage and
// translating storage errors into domain sentinels.
package hpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/ceffo/critical/api/internal/adapters/hpdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of hpsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

// New constructs a DAL bound to the given Querier (pool or transaction).
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec() sqlite.BobExec { return sqlite.BobExec{Querier: d.q} }

// AppendEvent inserts a row into hp_events.
func (d *DAL) AppendEvent(ctx context.Context, characterID int64, delta int, source string) error {
	_, err := queries.AppendHpEvent([]queries.AppendHpEvent_HPEvents{
		{CharacterID: characterID, Delta: int64(delta), Source: source},
	}).Exec(ctx, d.exec())
	if err != nil {
		return fmt.Errorf("insert hp_event: %w", err)
	}
	return nil
}

// GetCurrentHPSum returns SUM(delta) over the ledger; 0 when empty.
// Uses raw SQL because bob's static parser does not support aggregate functions.
func (d *DAL) GetCurrentHPSum(ctx context.Context, characterID int64) (int, error) {
	var sum sql.NullInt64
	if err := d.q.QueryRowContext(ctx,
		`SELECT SUM(delta) FROM hp_events WHERE character_id = ?`, characterID,
	).Scan(&sum); err != nil {
		return 0, fmt.Errorf("sum hp_events: %w", err)
	}
	return int(sum.Int64), nil
}

// GetHistory returns the most recent limit rows in DESC id order.
func (d *DAL) GetHistory(ctx context.Context, characterID int64, limit int) ([]hpsvc.HpEvent, error) {
	rows, err := queries.GetHpHistory(characterID, strconv.Itoa(limit)).All(ctx, d.exec())
	if err != nil {
		return nil, fmt.Errorf("query hp_events: %w", err)
	}
	events := make([]hpsvc.HpEvent, len(rows))
	for i, r := range rows {
		events[i] = hpsvc.HpEvent{
			ID: r.ID, CharacterID: r.CharacterID,
			Delta: int(r.Delta), Source: r.Source, CreatedAt: r.CreatedAt,
		}
	}
	return events, nil
}

// GetMaxHP fetches hp_max from the character row. Maps sql.ErrNoRows to
// domain.ErrCharacterNotFound.
func (d *DAL) GetMaxHP(ctx context.Context, characterID int64) (int, error) {
	maxHP, err := queries.GetCharacterMaxHp(characterID).One(ctx, d.exec())
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get hp_max: %w", err)
	}
	return int(maxHP), nil
}

// LastEvent returns (delta, source, ok=true) when an event exists and
// (0, "", false) when the ledger is empty. ok=false is not an error.
func (d *DAL) LastEvent(ctx context.Context, characterID int64) (int, string, bool, error) {
	row, err := queries.GetLastHpEvent(characterID).One(ctx, d.exec())
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("query last hp_event: %w", err)
	}
	return int(row.Delta), row.Source, true, nil
}
