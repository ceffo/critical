// Package xpdal is the storage adapter for the XP ledger (xpsvc.DAL). It
// delegates to bob-generated typed queries.
package xpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/xpdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of xpsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

var _ xpsvc.DAL = (*DAL)(nil)

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// List returns the character's entries, newest first.
func (d *DAL) List(ctx context.Context, characterID int64) ([]xpsvc.Entry, error) {
	rows, err := queries.ListXPEntries(characterID).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list xp_entries: %w", err)
	}
	out := make([]xpsvc.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, xpsvc.Entry{
			ID: r.ID, Date: r.EntryDate, Session: r.Session, Reason: r.Reason,
			Base: int(r.Base), Bonus: r.Bonus != 0,
		})
	}
	return out, nil
}

// Get returns one entry, or xpsvc.ErrEntryNotFound.
func (d *DAL) Get(ctx context.Context, characterID, entryID int64) (xpsvc.Entry, error) {
	r, err := queries.GetXPEntry(characterID, entryID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return xpsvc.Entry{}, xpsvc.ErrEntryNotFound
	}
	if err != nil {
		return xpsvc.Entry{}, fmt.Errorf("get xp_entries %d: %w", entryID, err)
	}
	return xpsvc.Entry{
		ID: r.ID, Date: r.EntryDate, Session: r.Session, Reason: r.Reason,
		Base: int(r.Base), Bonus: r.Bonus != 0,
	}, nil
}

// Insert writes a new entry and returns its id.
func (d *DAL) Insert(ctx context.Context, characterID int64, e xpsvc.Entry) (int64, error) {
	id, err := queries.InsertXPEntry([]queries.InsertXPEntry_XPEntries{{
		CharacterID: characterID, EntryDate: e.Date, Session: e.Session, Reason: e.Reason,
		Base: int64(e.Base), Bonus: boolInt(e.Bonus),
	}}).One(ctx, d.exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("insert xp_entries: %w", err)
	}
	return id, nil
}

// Reinsert writes an entry back under its own id and date (an undone
// delete).
func (d *DAL) Reinsert(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	_, err := queries.RestoreXPEntry([]queries.RestoreXPEntry_XPEntries{{
		ID: e.ID, CharacterID: characterID, EntryDate: e.Date, Session: e.Session, Reason: e.Reason,
		Base: int64(e.Base), Bonus: boolInt(e.Bonus),
	}}).One(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("reinsert xp_entries %d: %w", e.ID, err)
	}
	return nil
}

// Update overwrites the session, reason, base and bonus of an entry.
func (d *DAL) Update(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	ids, err := queries.UpdateXPEntry(e.Session, e.Reason, int64(e.Base), boolInt(e.Bonus), characterID, e.ID).
		All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("update xp_entries %d: %w", e.ID, err)
	}
	if len(ids) == 0 {
		return xpsvc.ErrEntryNotFound
	}
	return nil
}

// Delete removes an entry.
func (d *DAL) Delete(ctx context.Context, characterID, entryID int64) error {
	ids, err := queries.DeleteXPEntry(characterID, entryID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("delete xp_entries %d: %w", entryID, err)
	}
	if len(ids) == 0 {
		return xpsvc.ErrEntryNotFound
	}
	return nil
}
