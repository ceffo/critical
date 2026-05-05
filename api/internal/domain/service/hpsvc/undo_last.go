package hpsvc

import (
	"context"
	"fmt"
)

// UndoLast appends a correction row that negates the most recent event,
// preserving the append-only ledger invariant. Returns the new state. No-op
// when there are no events.
func (s *service) UndoLast(ctx context.Context, characterID int64) (*HPState, error) {
	delta, source, ok, err := s.dal.LastEvent(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("last event: %w", err)
	}
	if ok {
		if err := s.dal.AppendEvent(ctx, characterID, -delta, "undo:"+source); err != nil {
			return nil, fmt.Errorf("append undo: %w", err)
		}
	}
	return s.GetState(ctx, characterID)
}
