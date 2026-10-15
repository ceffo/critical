package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// The log is a stack (ADR 0007 step 8). User, 2026-10-14: "no amending values.
// just the note.. can only pop elements", for every entry kind. The unit
// popped is a transaction (ADR 0018 step 2): the one of the newest non-voided
// entry, whole; its snapshots are restored, so popping in order walks the
// state back gesture by gesture.

// Void pops the transaction of entry id: for each of its entries, newest
// first, it restores each snapshotted state to its "before" value through the
// Restorer registered for its key and records a void correction, in one DB
// transaction. Refused with ErrNotLatest unless id is the newest non-voided
// entry. Returns the transaction's first entry.
func (s *service) Void(ctx context.Context, characterID, id int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.GetEntry(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", id, err)
		}
		if e.Status() == actionlog.StatusVoided {
			return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
		}
		top, err := s.top(ctx, characterID)
		if err != nil {
			return err
		}
		if top.ID != id {
			return fmt.Errorf("%w: only the latest entry can be undone, undo #%d (%s, round %d) first",
				ErrNotLatest, top.ID, top.Kind, top.Round)
		}
		out, err = s.popTransaction(ctx, characterID, e.TransactionID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// top returns the newest non-voided entry, nil when there is none.
func (s *service) top(ctx context.Context, characterID int64) (*actionlog.Entry, error) {
	id, found, err := s.dal.LatestActiveID(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("latest entry: %w", err)
	}
	if !found {
		return nil, nil
	}
	e, err := s.dal.GetEntry(ctx, characterID, id)
	if err != nil {
		return nil, fmt.Errorf("get entry %d: %w", id, err)
	}
	return e, nil
}

// popTransaction pops every non-voided entry of transaction transactionID,
// newest first, and returns its first entry as it reads after the pop. It
// must run inside a DB transaction, on the transaction of the newest
// non-voided entry: an error anywhere rolls every restore and void back.
func (s *service) popTransaction(ctx context.Context, characterID, transactionID int64) (*actionlog.Entry, error) {
	entries, err := s.dal.ListTransaction(ctx, characterID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list transaction %d: %w", transactionID, err)
	}
	for i := range entries {
		if entries[i].Status() == actionlog.StatusVoided {
			continue
		}
		if err := s.pop(ctx, characterID, &entries[i]); err != nil {
			return nil, err
		}
	}
	first, err := s.dal.GetEntry(ctx, characterID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get entry %d: %w", transactionID, err)
	}
	return first, nil
}

// pop restores e's snapshots and records the void; it must run inside a
// transaction, as part of popTransaction.
func (s *service) pop(ctx context.Context, characterID int64, e *actionlog.Entry) error {
	for _, snap := range e.Snapshots {
		r := s.restorer(snap.Key)
		if r == nil {
			return fmt.Errorf("no restorer registered for state %q", snap.Key)
		}
		if err := r.Restore(ctx, characterID, snap.Before); err != nil {
			return fmt.Errorf("restore %s: %w", snap.Key, err)
		}
	}
	if err := s.dal.InsertCorrection(ctx, actionlog.Correction{ActionID: e.ID, Kind: actionlog.CorrectionVoid}); err != nil {
		return fmt.Errorf("insert void: %w", err)
	}
	return nil
}
