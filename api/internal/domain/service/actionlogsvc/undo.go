package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Undo pops the newest non-voided entry, of any kind, when it is the entry
// the client saw on top (ADR 0010 steps 2 to 4). The expected-top check, the
// restores and the void correction share one transaction, so an action that
// lands between the client's read and its undo makes the undo fail (ErrTopChanged)
// instead of popping something the player never saw. ErrNothingToUndo when the
// log has nothing to pop. Returns the voided entry.
func (s *service) Undo(ctx context.Context, characterID, expectedTopID int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		top, err := s.top(ctx, characterID)
		if err != nil {
			return err
		}
		if top == nil {
			return fmt.Errorf("%w: expected #%d on top", ErrNothingToUndo, expectedTopID)
		}
		if top.ID != expectedTopID {
			return fmt.Errorf("%w: the newest entry is #%d (%s, round %d), not #%d",
				ErrTopChanged, top.ID, top.Kind, top.Round, expectedTopID)
		}
		if err := s.pop(ctx, characterID, top); err != nil {
			return err
		}
		out, err = s.dal.GetEntry(ctx, characterID, top.ID)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", top.ID, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TopID is the id of the newest non-voided entry, the one Undo pops (ADR 0010
// step 2); ok is false when the log has nothing to pop.
func (s *service) TopID(ctx context.Context, characterID int64) (int64, bool, error) {
	id, ok, err := s.dal.LatestActiveID(ctx, characterID)
	if err != nil {
		return 0, false, fmt.Errorf("latest entry: %w", err)
	}
	return id, ok, nil
}
