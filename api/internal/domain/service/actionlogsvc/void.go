package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Void records a void correction. When the entry carries snapshots, each
// state is restored to its "before" value through the Restorer registered for
// its key, in the same transaction. The void is refused with ErrVoidBlocked
// when a later non-voided entry changed one of the same states: restoring
// would silently undo that later change, so it must be voided first.
func (s *service) Void(ctx context.Context, characterID, id int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.void(ctx, characterID, id); err != nil {
			return err
		}
		var err error
		out, err = s.dal.GetEntry(ctx, characterID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VoidLatest voids the newest non-voided entry of kind (the HP panel's undo).
func (s *service) VoidLatest(ctx context.Context, characterID int64, kind actionlog.Kind) (bool, error) {
	var ok bool
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		id, found, err := s.dal.LatestActiveID(ctx, characterID, kind)
		if err != nil {
			return fmt.Errorf("latest %s entry: %w", kind, err)
		}
		if !found {
			return nil
		}
		ok = true
		return s.void(ctx, characterID, id)
	})
	return ok, err
}

// void must run inside a transaction.
func (s *service) void(ctx context.Context, characterID, id int64) error {
	e, err := s.dal.GetEntry(ctx, characterID, id)
	if err != nil {
		return fmt.Errorf("get entry %d: %w", id, err)
	}
	if e.Status() == actionlog.StatusVoided {
		return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
	}
	if len(e.Snapshots) > 0 {
		laterID, blocked, err := s.dal.LaterEntryTouchingState(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("check later entries: %w", err)
		}
		if blocked {
			return fmt.Errorf("%w: entry #%d changed the same state later, void it first", ErrVoidBlocked, laterID)
		}
		for _, snap := range e.Snapshots {
			r := s.restorer(snap.Key)
			if r == nil {
				return fmt.Errorf("no restorer registered for state %q", snap.Key)
			}
			if err := r.Restore(ctx, characterID, snap.Before); err != nil {
				return fmt.Errorf("restore %s: %w", snap.Key, err)
			}
		}
	}
	if err := s.dal.InsertCorrection(ctx, actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid}); err != nil {
		return fmt.Errorf("insert void: %w", err)
	}
	return nil
}
