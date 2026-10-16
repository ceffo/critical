// Package xpdal is the storage adapter for the XP ledger (xpsvc.DAL). It
// delegates to sqlc-generated queries.
package xpdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/xpdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// DAL is the implementation of xpsvc.DAL over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

var _ xpsvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// bonus is stored as 0 or 1 (the column's CHECK).
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func entry(id int64, date, session, reason string, base, bonus int64) xpsvc.Entry {
	return xpsvc.Entry{
		ID: id, Date: date, Session: session, Reason: reason, Base: int(base), Bonus: bonus != 0,
	}
}

// List returns the character's entries, newest first.
func (d *DAL) List(ctx context.Context, characterID int64) ([]xpsvc.Entry, error) {
	rows, err := d.q.ListXPEntries(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list xp_entries: %w", err)
	}
	out := make([]xpsvc.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, entry(r.ID, r.EntryDate, r.Session, r.Reason, r.Base, r.Bonus))
	}
	return out, nil
}

// Get returns one entry, or xpsvc.ErrEntryNotFound.
func (d *DAL) Get(ctx context.Context, characterID, entryID int64) (xpsvc.Entry, error) {
	r, err := d.q.GetXPEntry(ctx, queries.GetXPEntryParams{CharacterID: characterID, ID: entryID})
	if errors.Is(err, sql.ErrNoRows) {
		return xpsvc.Entry{}, xpsvc.ErrEntryNotFound
	}
	if err != nil {
		return xpsvc.Entry{}, fmt.Errorf("get xp_entries %d: %w", entryID, err)
	}
	return entry(r.ID, r.EntryDate, r.Session, r.Reason, r.Base, r.Bonus), nil
}

// Insert writes a new entry and returns its id.
func (d *DAL) Insert(ctx context.Context, characterID int64, e xpsvc.Entry) (int64, error) {
	id, err := d.q.InsertXPEntry(ctx, queries.InsertXPEntryParams{
		CharacterID: characterID, EntryDate: e.Date, Session: e.Session, Reason: e.Reason,
		Base: int64(e.Base), Bonus: boolInt(e.Bonus),
	})
	if err != nil {
		return 0, fmt.Errorf("insert xp_entries: %w", err)
	}
	return id, nil
}

// Reinsert writes an entry back under its own id and date (an undone
// delete).
func (d *DAL) Reinsert(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	err := d.q.RestoreXPEntry(ctx, queries.RestoreXPEntryParams{
		ID: e.ID, CharacterID: characterID, EntryDate: e.Date, Session: e.Session, Reason: e.Reason,
		Base: int64(e.Base), Bonus: boolInt(e.Bonus),
	})
	if err != nil {
		return fmt.Errorf("reinsert xp_entries %d: %w", e.ID, err)
	}
	return nil
}

// Update overwrites the session, reason, base and bonus of an entry.
func (d *DAL) Update(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	n, err := d.q.UpdateXPEntry(ctx, queries.UpdateXPEntryParams{
		Session: e.Session, Reason: e.Reason, Base: int64(e.Base), Bonus: boolInt(e.Bonus),
		CharacterID: characterID, ID: e.ID,
	})
	if err != nil {
		return fmt.Errorf("update xp_entries %d: %w", e.ID, err)
	}
	if n == 0 {
		return xpsvc.ErrEntryNotFound
	}
	return nil
}

// Delete removes an entry.
func (d *DAL) Delete(ctx context.Context, characterID, entryID int64) error {
	n, err := d.q.DeleteXPEntry(ctx, queries.DeleteXPEntryParams{CharacterID: characterID, ID: entryID})
	if err != nil {
		return fmt.Errorf("delete xp_entries %d: %w", entryID, err)
	}
	if n == 0 {
		return xpsvc.ErrEntryNotFound
	}
	return nil
}
