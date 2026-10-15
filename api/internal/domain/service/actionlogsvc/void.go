package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// The log is a stack (ADR 0007 step 8). User, 2026-10-14: "no amending values.
// just the note.. can only pop elements", for every entry kind. Only the
// newest non-voided entry can be voided; its snapshots are restored, so popping
// in order walks the state back step by step.

// Void pops entry id: it records a void correction and restores each
// snapshotted state to its "before" value through the Restorer registered for
// its key, in one transaction. Refused with ErrNotLatest unless id is the
// newest non-voided entry.
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
			return fmt.Errorf("%w: only the latest entry can be voided, void %q (round %d) first",
				ErrNotLatest, top.EffectiveSummary(), top.Round)
		}
		if err := s.pop(ctx, characterID, e); err != nil {
			return err
		}
		out, err = s.dal.GetEntry(ctx, characterID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VoidLatest pops the newest non-voided entry when it is of kind (the HP
// panel's undo). ok is false when the log has nothing to pop; ErrNotLatest
// when the newest entry is another kind (it must be popped first).
func (s *service) VoidLatest(ctx context.Context, characterID int64, kind actionlog.Kind) (bool, error) {
	var ok bool
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		top, err := s.top(ctx, characterID)
		if err != nil || top == nil {
			return err
		}
		if top.Kind != kind {
			return fmt.Errorf("%w: the latest entry is not a %s, void %q (round %d) first",
				ErrNotLatest, kind, top.EffectiveSummary(), top.Round)
		}
		ok = true
		return s.pop(ctx, characterID, top)
	})
	return ok, err
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

// pop restores e's snapshots and records the void; it must run inside a
// transaction, on the newest non-voided entry.
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
