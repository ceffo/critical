package hpsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
)

// AppendEvent records a delta against the ledger and returns the new state.
// The business rule "source is required" lives here, not in the handler.
func (s *service) AppendEvent(ctx context.Context, characterID int64, delta int, source string) (*HPState, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("%w: source is required", domain.ErrInvalidInput)
	}
	if err := s.dal.AppendEvent(ctx, characterID, delta, source); err != nil {
		return nil, fmt.Errorf("append event: %w", err)
	}
	return s.GetState(ctx, characterID)
}
