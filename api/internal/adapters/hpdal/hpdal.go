// Package hpdal is the storage adapter for hpsvc. It satisfies hpsvc.DAL by
// translating SQL rows into hpsvc domain types and storage errors into domain
// sentinels.
package hpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

// AppendEvent inserts a row into hp_events.
func (d *DAL) AppendEvent(ctx context.Context, characterID int64, delta int, source string) error {
	if _, err := d.q.ExecContext(ctx,
		`INSERT INTO hp_events (character_id, delta, source) VALUES (?, ?, ?)`,
		characterID, delta, source,
	); err != nil {
		return fmt.Errorf("insert hp_event: %w", err)
	}
	return nil
}

// GetCurrentHPSum returns SUM(delta) over the ledger; 0 when empty.
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
	rows, err := d.q.QueryContext(ctx,
		`SELECT id, character_id, delta, source, created_at
		   FROM hp_events
		  WHERE character_id = ?
		  ORDER BY id DESC
		  LIMIT ?`,
		characterID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query hp_events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []hpsvc.HpEvent
	for rows.Next() {
		var e hpsvc.HpEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.CharacterID, &e.Delta, &e.Source, &createdAt); err != nil {
			return nil, fmt.Errorf("scan hp_event: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetMaxHP fetches hp_max from the character row. Maps sql.ErrNoRows to
// domain.ErrCharacterNotFound.
func (d *DAL) GetMaxHP(ctx context.Context, characterID int64) (int, error) {
	var maxHP int
	err := d.q.QueryRowContext(ctx, `SELECT hp_max FROM characters WHERE id = ?`, characterID).Scan(&maxHP)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get hp_max: %w", err)
	}
	return maxHP, nil
}

// LastEvent returns (delta, source, ok=true) when an event exists and
// (0, "", false) when the ledger is empty. ok=false is not an error.
func (d *DAL) LastEvent(ctx context.Context, characterID int64) (int, string, bool, error) {
	var delta int
	var source string
	err := d.q.QueryRowContext(ctx,
		`SELECT delta, source FROM hp_events WHERE character_id = ? ORDER BY id DESC LIMIT 1`,
		characterID,
	).Scan(&delta, &source)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("query last hp_event: %w", err)
	}
	return delta, source, true, nil
}
