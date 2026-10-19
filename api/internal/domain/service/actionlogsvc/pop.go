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
		if entries[i].Voided() {
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
		if snap.Before == nil {
			return fmt.Errorf("%w: entry #%d, state %q", ErrSnapshotNotReencoded, e.ID, snap.Key)
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
