package hpsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// GetState returns the current hit-point view: maxHP from the character row
// plus the action log's SUM of effective HP deltas, and the last N non-voided
// HP changes (amended values shown) for the UI.
func (s *service) GetState(ctx context.Context, characterID int64) (*HPState, error) {
	maxHP, err := s.dal.GetMaxHP(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get max hp: %w", err)
	}
	sum, err := s.log.HPTotal(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get hp sum: %w", err)
	}
	entries, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindHPChange, s.cfg.HistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	events := make([]HpEvent, len(entries))
	for i, e := range entries {
		events[i] = HpEvent{
			ID:          e.ID,
			CharacterID: e.CharacterID,
			Delta:       e.EffectiveHPDelta(),
			Source:      e.EffectiveSummary(),
			CreatedAt:   e.CreatedAt,
		}
	}
	return &HPState{
		CurrentHp: maxHP + sum,
		MaxHp:     maxHP,
		Events:    events,
	}, nil
}
