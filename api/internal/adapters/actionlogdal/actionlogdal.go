// Package actionlogdal is the storage adapter for actionlogsvc, over the
// sqlc-generated queries in the queries/ subpackage. The composition root
// routes them to the transaction the context carries (sqlite.Routed), so
// entries and corrections commit together with the caller's state write.
package actionlogdal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// DAL is the implementation of actionlogsvc.DAL over the sqlc-generated
// queries. It only ever inserts: an entry, its snapshots, a correction (ADR
// 0007, 0010); the one update gives a new entry its own transaction id.
type DAL struct {
	q queries.Querier
}

var _ actionlogsvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

func nullInt(v int64, ok bool) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: ok} }

func nullString(v string, ok bool) sql.NullString { return sql.NullString{String: v, Valid: ok} }

// InsertEntry inserts the entry and its snapshots into action-log
// transaction transactionID; 0 starts a new transaction, whose id is the new
// entry's (ADR 0018 step 1). Callers run it inside a DB transaction
// (actionlogsvc.Record does).
func (d *DAL) InsertEntry(ctx context.Context, e actionlog.NewEntry, transactionID int64) (int64, error) {
	payload := string(e.Payload)
	if payload == "" {
		payload = "{}"
	}
	id, err := d.q.InsertAction(ctx, queries.InsertActionParams{
		CharacterID:   e.CharacterID,
		Round:         int64(e.Round),
		Kind:          string(e.Kind),
		Summary:       e.Summary,
		HpDelta:       int64(e.HPDelta),
		Payload:       payload,
		TransactionID: nullInt(transactionID, transactionID != 0),
	})
	if err != nil {
		return 0, fmt.Errorf("insert action_log: %w", err)
	}
	if transactionID == 0 {
		if err := d.q.StartActionTransaction(ctx, id); err != nil {
			return 0, fmt.Errorf("start action_log transaction %d: %w", id, err)
		}
	}
	for _, s := range e.Snapshots {
		err := d.q.InsertSnapshot(ctx, queries.InsertSnapshotParams{
			ActionID: id, StateKey: s.Key, BeforeState: string(s.Before), AfterState: string(s.After),
		})
		if err != nil {
			return 0, fmt.Errorf("insert action_snapshots %s: %w", s.Key, err)
		}
	}
	return id, nil
}

// InsertCorrection appends an amend or void row. An empty payload is stored
// as NULL (the entry's payload is unchanged).
func (d *DAL) InsertCorrection(ctx context.Context, c actionlog.Correction) error {
	row := queries.InsertCorrectionParams{
		ActionID: c.ActionID, Kind: string(c.Kind),
		Summary: nullString(c.Summary, c.Summary != ""),
		Payload: nullString(string(c.Payload), len(c.Payload) > 0),
	}
	if c.HPDelta != nil {
		row.HpDelta = nullInt(int64(*c.HPDelta), true)
	}
	if c.Note != nil {
		row.Note = nullString(*c.Note, true)
	}
	if c.StepID != nil {
		row.StepID = nullInt(*c.StepID, true)
	}
	if err := d.q.InsertCorrection(ctx, row); err != nil {
		return fmt.Errorf("insert action_corrections: %w", err)
	}
	return nil
}

// GetEntry returns one entry with snapshots and corrections, or
// domain.ErrNotFound when it does not exist for this character.
func (d *DAL) GetEntry(ctx context.Context, characterID, id int64) (*actionlog.Entry, error) {
	row, err := d.q.GetAction(ctx, queries.GetActionParams{CharacterID: characterID, ID: id})
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
	rows, err := d.q.ListActions(ctx, queries.ListActionsParams{CharacterID: characterID, Limit: int64(limit)})
	if err != nil {
		return nil, fmt.Errorf("list action_log: %w", err)
	}
	entries := make([]actionlog.Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(queries.GetActionRow(r))
	}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListTransaction returns every entry of transaction transactionID, voided
// ones included, newest first, with snapshots and corrections.
func (d *DAL) ListTransaction(ctx context.Context, characterID, transactionID int64) ([]actionlog.Entry, error) {
	rows, err := d.q.ListTransactionActions(ctx, queries.ListTransactionActionsParams{
		CharacterID: characterID, TransactionID: nullInt(transactionID, true),
	})
	if err != nil {
		return nil, fmt.Errorf("list action_log transaction %d: %w", transactionID, err)
	}
	entries := make([]actionlog.Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(queries.GetActionRow(r))
	}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListActiveEntries returns the newest limit non-voided entries of kind.
func (d *DAL) ListActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error) {
	rows, err := d.q.ListActiveActionsByKind(ctx, queries.ListActiveActionsByKindParams{
		CharacterID: characterID, Kind: string(kind), Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list active action_log: %w", err)
	}
	entries := make([]actionlog.Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(queries.GetActionRow(r))
	}
	if err := d.attach(ctx, characterID, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// LatestActiveID returns the newest non-voided entry, any kind (the top of
// the stack).
func (d *DAL) LatestActiveID(ctx context.Context, characterID int64) (int64, bool, error) {
	id, err := d.q.LatestActiveActionID(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("latest action_log: %w", err)
	}
	return id, true, nil
}

// LatestPopped returns the entry whose standing void is the newest (popped
// last and not redone since) and its transaction; ok is false when no entry
// is popped (ADR 0019 step 2).
func (d *DAL) LatestPopped(ctx context.Context, characterID int64) (id, transactionID int64, ok bool, err error) {
	row, err := d.q.LatestPoppedAction(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("latest popped action_log: %w", err)
	}
	return row.ID, ownTransaction(row.ID, row.TransactionID), true, nil
}

// LatestID returns the newest entry of kind, voided or not.
func (d *DAL) LatestID(ctx context.Context, characterID int64, kind actionlog.Kind) (int64, bool, error) {
	id, err := d.q.LatestActionID(ctx, queries.LatestActionIDParams{CharacterID: characterID, Kind: string(kind)})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("latest action_log: %w", err)
	}
	return id, true, nil
}

// HPTotal returns the SUM of effective HP deltas (the HPTotal query); 0 when
// the log is empty.
func (d *DAL) HPTotal(ctx context.Context, characterID int64) (int, error) {
	total, err := d.q.HPTotal(ctx, characterID)
	if err != nil {
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

	snaps, err := d.q.ListSnapshotsFrom(ctx, queries.ListSnapshotsFromParams{CharacterID: characterID, ActionID: minID})
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

	corrs, err := d.q.ListCorrectionsFrom(ctx, queries.ListCorrectionsFromParams{CharacterID: characterID, ActionID: minID})
	if err != nil {
		return fmt.Errorf("list action_corrections: %w", err)
	}
	// A correction step is newer than the entry it corrects, so its voids
	// and redos are among the corrections listed from minID, in id order: the
	// newest one says whether the step is popped (ADR 0019 step 1).
	voided := map[int64]bool{}
	for _, c := range corrs {
		switch actionlog.CorrectionKind(c.Kind) {
		case actionlog.CorrectionVoid:
			voided[c.ActionID] = true
		case actionlog.CorrectionRedo:
			voided[c.ActionID] = false
		}
	}
	for _, c := range corrs {
		e, ok := byID[c.ActionID]
		if !ok {
			continue
		}
		corr := actionlog.Correction{
			ID: c.ID, ActionID: c.ActionID, Kind: actionlog.CorrectionKind(c.Kind),
			Summary: c.Summary.String, CreatedAt: c.CreatedAt,
		}
		if c.HpDelta.Valid {
			delta := int(c.HpDelta.Int64)
			corr.HPDelta = &delta
		}
		if c.Payload.Valid {
			corr.Payload = json.RawMessage(c.Payload.String)
		}
		if c.Note.Valid {
			note := c.Note.String
			corr.Note = &note
		}
		if c.StepID.Valid {
			step := c.StepID.Int64
			corr.StepID, corr.StepUndone = &step, voided[step]
		}
		e.Corrections = append(e.Corrections, corr)
	}
	return nil
}

// toEntry maps an action_log row. ListActionsRow, ListActiveActionsByKindRow
// and ListTransactionActionsRow have the fields of GetActionRow and convert
// to it.
func toEntry(f queries.GetActionRow) actionlog.Entry {
	return actionlog.Entry{
		ID: f.ID, TransactionID: ownTransaction(f.ID, f.TransactionID), CharacterID: f.CharacterID, Round: int(f.Round), Kind: actionlog.Kind(f.Kind),
		Summary: f.Summary, HPDelta: int(f.HpDelta), Payload: json.RawMessage(f.Payload), CreatedAt: f.CreatedAt,
	}
}

// ownTransaction is the entry's transaction id; a row without one (none is
// left after the migration that added it) is its own transaction.
func ownTransaction(id int64, transactionID sql.NullInt64) int64 {
	if transactionID.Valid {
		return transactionID.Int64
	}
	return id
}
