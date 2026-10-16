package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// List returns the newest entries first with their corrections.
func (s *service) List(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error) {
	entries, err := s.dal.ListEntries(ctx, characterID, s.cfg.clampLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	return s.views.Entries(entries), nil
}

// ActiveEntries returns the newest non-voided entries of kind.
func (s *service) ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error) {
	entries, err := s.dal.ListActiveEntries(ctx, characterID, kind, s.cfg.clampLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list active entries: %w", err)
	}
	return s.views.Entries(entries), nil
}

// HPTotal is the SUM of effective HP deltas (computed in storage).
func (s *service) HPTotal(ctx context.Context, characterID int64) (int, error) {
	total, err := s.dal.HPTotal(ctx, characterID)
	if err != nil {
		return 0, fmt.Errorf("hp total: %w", err)
	}
	return total, nil
}
