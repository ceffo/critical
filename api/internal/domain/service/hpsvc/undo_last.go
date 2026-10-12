package hpsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// UndoLast voids the newest non-voided HP change in the action log (an
// audited correction, never a delete) and returns the new state. No-op when
// there is none.
func (s *service) UndoLast(ctx context.Context, characterID int64) (*HPState, error) {
	if _, err := s.log.VoidLatest(ctx, characterID, actionlog.KindHPChange); err != nil {
		return nil, fmt.Errorf("void last hp change: %w", err)
	}
	return s.GetState(ctx, characterID)
}
