// Package actionlogdal is the storage adapter for actionlogsvc. Every method
// runs on the transaction carried by ctx when there is one (sqlite.Conn), so
// entries and corrections commit together with the caller's state write.
package actionlogdal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/aarondl/opt/null"
	"github.com/ceffo/critical/api/internal/adapters/actionlogdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of actionlogsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL on the pool; calls join the ctx transaction if any.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) conn(ctx context.Context) sqlite.Querier { return sqlite.Conn(ctx, d.q) }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: d.conn(ctx)}
}

// InsertEntry inserts the entry and its snapshots. Callers run it inside a
// transaction (actionlogsvc.Record does).
func (d *DAL) InsertEntry(ctx context.Context, e actionlog.NewEntry) (int64, error) {
	payload := string(e.Payload)
	if payload == "" {
		payload = "{}"
	}
	id, err := queries.InsertAction([]queries.InsertAction_ActionLog{{
		CharacterID: e.CharacterID,
		Round:       int64(e.Round),
		Kind:        string(e.Kind),
		Summary:     e.Summary,
		HPDelta:     int64(e.HPDelta),
		Payload:     payload,
	}}).One(ctx, d.exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("insert action_log: %w", err)
	}
	for _, s := range e.Snapshots {
		_, err := queries.InsertSnapshot([]queries.InsertSnapshot_ActionSnapshots{{
			ActionID: id, StateKey: s.Key, BeforeState: string(s.Before), AfterState: string(s.After),
		}}).Exec(ctx, d.exec(ctx))
		if err != nil {
			return 0, fmt.Errorf("insert action_snapshots %s: %w", s.Key, err)
		}
	}
	return id, nil
}

// InsertCorrection appends an amend or void row. An empty payload is stored
// as NULL (the entry's payload is unchanged).
func (d *DAL) InsertCorrection(ctx context.Context, c actionlog.Correction) error {
	row := queries.InsertCorrection_ActionCorrections{ActionID: c.ActionID, Kind: string(c.Kind)}
	if c.HPDelta != nil {
		row.HPDelta = null.From(int64(*c.HPDelta))
	}
	if c.Summary != "" {
		row.Summary = null.From(c.Summary)
	}
	if len(c.Payload) > 0 {
		row.Payload = null.From(string(c.Payload))
	}
	if c.Note != nil {
		row.Note = null.From(*c.Note)
	}
	if _, err := queries.InsertCorrection([]queries.InsertCorrection_ActionCorrections{row}).Exec(ctx, d.exec(ctx)); err != nil {
		return fmt.Errorf("insert action_corrections: %w", err)
	}
	return nil
}

// GetEntry returns one entry with snapshots and corrections, or
// domain.ErrNotFound when it does not exist for this character.
func (d *DAL) GetEntry(ctx context.Context, characterID, id int64) (*actionlog.Entry, error) {
	row, err := queries.GetAction(characterID, id).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: log entry %d", domain.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("get action_log: %w", err)
	}
	entries := []actionlog.Entry{toEntry(row)}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return &entries[0], nil
}

// ListEntries returns the newest limit entries, newest first.
func (d *DAL) ListEntries(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error) {
	rows, err := queries.ListActions(characterID, strconv.Itoa(limit)).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list action_log: %w", err)
	}
	entries := make([]actionlog.Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(r)
	}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListActiveEntries returns the newest limit non-voided entries of kind.
func (d *DAL) ListActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error) {
	rows, err := queries.ListActiveActionsByKind(characterID, string(kind), strconv.Itoa(limit)).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list active action_log: %w", err)
	}
	entries := make([]actionlog.Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(r)
	}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// LatestActiveID returns the newest non-voided entry, any kind (the top of
// the stack).
func (d *DAL) LatestActiveID(ctx context.Context, characterID int64) (int64, bool, error) {
	id, err := queries.LatestActiveActionID(characterID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("latest action_log: %w", err)
	}
	return id, true, nil
}

// LatestID returns the newest entry of kind, voided or not.
func (d *DAL) LatestID(ctx context.Context, characterID int64, kind actionlog.Kind) (int64, bool, error) {
	id, err := queries.LatestActionID(characterID, string(kind)).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("latest action_log: %w", err)
	}
	return id, true, nil
}

// hpTotalSQL sums effective HP deltas: a voided entry counts 0, an amended one
// counts its latest amend's delta. Raw SQL because bob's parser does not
// support aggregates.
const hpTotalSQL = `
SELECT COALESCE(SUM(
  CASE
    WHEN EXISTS (SELECT 1 FROM action_corrections v WHERE v.action_id = a.id AND v.kind = 'void') THEN 0
    ELSE COALESCE(
      (SELECT m.hp_delta FROM action_corrections m
        WHERE m.action_id = a.id AND m.kind = 'amend' AND m.hp_delta IS NOT NULL
        ORDER BY m.id DESC LIMIT 1),
      a.hp_delta)
  END), 0)
FROM action_log a
WHERE a.character_id = ?`

// HPTotal returns the SUM of effective HP deltas; 0 when the log is empty.
func (d *DAL) HPTotal(ctx context.Context, characterID int64) (int, error) {
	var total int64
	if err := d.conn(ctx).QueryRowContext(ctx, hpTotalSQL, characterID).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum action_log hp: %w", err)
	}
	return int(total), nil
}

// attach loads snapshots and corrections for entries (any order) in two
// queries bounded below by the smallest id.
func (d *DAL) attach(ctx context.Context, characterID int64, entries []actionlog.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	byID := make(map[int64]*actionlog.Entry, len(entries))
	minID := entries[0].ID
	for i := range entries {
		byID[entries[i].ID] = &entries[i]
		minID = min(minID, entries[i].ID)
	}

	snaps, err := queries.ListSnapshotsFrom(characterID, minID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("list action_snapshots: %w", err)
	}
	for _, s := range snaps {
		if e, ok := byID[s.ActionID]; ok {
			e.Snapshots = append(e.Snapshots, actionlog.Snapshot{
				Key: s.StateKey, Before: json.RawMessage(s.BeforeState), After: json.RawMessage(s.AfterState),
			})
		}
	}

	corrs, err := queries.ListCorrectionsFrom(characterID, minID).All(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("list action_corrections: %w", err)
	}
	for _, c := range corrs {
		e, ok := byID[c.ActionID]
		if !ok {
			continue
		}
		corr := actionlog.Correction{
			ID: c.ID, ActionID: c.ActionID, Kind: actionlog.CorrectionKind(c.Kind),
			Summary: c.Summary.GetOr(""), CreatedAt: c.CreatedAt,
		}
		if v, ok := c.HPDelta.Get(); ok {
			delta := int(v)
			corr.HPDelta = &delta
		}
		if v, ok := c.Payload.Get(); ok {
			corr.Payload = json.RawMessage(v)
		}
		if v, ok := c.Note.Get(); ok {
			corr.Note = &v
		}
		e.Corrections = append(e.Corrections, corr)
	}
	return nil
}

// toEntry maps an action_log row. GetActionRow, ListActionsRow and
// ListActiveActionsByKindRow are aliases of the same struct type.
func toEntry(f queries.GetActionRow) actionlog.Entry {
	return actionlog.Entry{
		ID: f.ID, CharacterID: f.CharacterID, Round: int(f.Round), Kind: actionlog.Kind(f.Kind),
		Summary: f.Summary, HPDelta: int(f.HPDelta), Payload: json.RawMessage(f.Payload), CreatedAt: f.CreatedAt,
	}
}
