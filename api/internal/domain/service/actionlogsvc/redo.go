package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0019: redo replays the newest popped transaction. Replaying un-pops
// its entries: each gets a redo correction (the log stays append-only) and
// its snapshots' "after" is written back through the same restorers the undo
// used, in one DB transaction. Nothing is rerolled or recomputed: the
// entries, payloads, HP deltas and snapshots are the ones the undo kept (ADR
// 0018 step 6).

// redoTarget is the transaction a redo replays, nil when there is none (ADR
// 0019 step 2): the transaction of the entry popped last and not redone
// since, provided it is above the live top. A transaction pushed after a pop
// is above that pop: it is either live (then the popped one is below the live
// top) or popped later (then it is the one popped last), so pushing empties
// the redo stack. It returns the transaction's first entry.
func (s *service) redoTarget(ctx context.Context, characterID int64) (*actionlog.Entry, error) {
	_, transactionID, found, err := s.dal.LatestPopped(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("latest popped entry: %w", err)
	}
	if !found {
		return nil, nil
	}
	top, live, err := s.dal.LatestActiveID(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("latest entry: %w", err)
	}
	if live && top >= transactionID {
		return nil, nil
	}
	first, err := s.dal.GetEntry(ctx, characterID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get entry %d: %w", transactionID, err)
	}
	return first, nil
}

// RedoTarget is the first entry of the transaction Redo replays, nil when
// there is nothing to redo (ADR 0019 step 2).
func (s *service) RedoTarget(ctx context.Context, characterID int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.redoTarget(ctx, characterID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.views.EntryPtr(out), nil
}

// Redo replays the redo target when its first entry's id is expectedRedoID
// (ADR 0019 step 3): ErrRedoChanged when another transaction is the target,
// ErrNothingToRedo when there is none. The check, every restore and every redo
// correction share one DB transaction, so a failing restore leaves no redo and
// no partial state. Returns the transaction's first entry, live again.
func (s *service) Redo(ctx context.Context, characterID, expectedRedoID int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		target, err := s.redoTarget(ctx, characterID)
		if err != nil {
			return err
		}
		if target == nil {
			return fmt.Errorf("%w: expected #%d", ErrNothingToRedo, expectedRedoID)
		}
		if target.ID != expectedRedoID {
			return fmt.Errorf("%w: the transaction to redo is #%d (%s, round %d), not #%d",
				ErrRedoChanged, target.ID, target.Kind, target.Round, expectedRedoID)
		}
		out, err = s.replayTransaction(ctx, characterID, target.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.views.EntryPtr(out), nil
}

// replayTransaction un-pops every popped entry of transaction transactionID,
// oldest first (the reverse of the pop, so a state two entries both snapshot
// ends at the newest "after"), and returns its first entry as it reads after
// the replay. It must run inside a DB transaction.
func (s *service) replayTransaction(ctx context.Context, characterID, transactionID int64) (*actionlog.Entry, error) {
	entries, err := s.dal.ListTransaction(ctx, characterID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list transaction %d: %w", transactionID, err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].Voided() {
			continue
		}
		if err := s.replay(ctx, characterID, &entries[i]); err != nil {
			return nil, err
		}
	}
	first, err := s.dal.GetEntry(ctx, characterID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get entry %d: %w", transactionID, err)
	}
	return first, nil
}

// replay writes e's snapshots' "after" back (through Replay, with e's id, for
// a restorer that needs it) and records the redo; it must
// run inside a transaction, as part of replayTransaction.
func (s *service) replay(ctx context.Context, characterID int64, e *actionlog.Entry) error {
	for _, snap := range e.Snapshots {
		r := s.restorer(snap.Key)
		if r == nil {
			return fmt.Errorf("no restorer registered for state %q", snap.Key)
		}
		if snap.After == nil {
			return fmt.Errorf("%w: entry #%d, state %q", ErrSnapshotNotReencoded, e.ID, snap.Key)
		}
		var err error
		if rp, ok := r.(actionlog.Replayer); ok {
			err = rp.Replay(ctx, characterID, e.ID, snap.After)
		} else {
			err = r.Restore(ctx, characterID, snap.After)
		}
		if err != nil {
			return fmt.Errorf("replay %s: %w", snap.Key, err)
		}
	}
	if err := s.dal.InsertCorrection(ctx, actionlog.Correction{ActionID: e.ID, Kind: actionlog.CorrectionRedo}); err != nil {
		return fmt.Errorf("insert redo: %w", err)
	}
	return nil
}
