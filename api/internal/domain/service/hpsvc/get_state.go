package hpsvc

import (
	"context"
	"fmt"
)

// GetState returns the current hit-point materialized view: maxHP from the
// character row, plus the running sum of every ledger event, plus the last N
// events for the UI.
func (s *service) GetState(ctx context.Context, characterID int64) (*HPState, error) {
	maxHP, err := s.dal.GetMaxHP(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get max hp: %w", err)
	}
	sum, err := s.dal.GetCurrentHPSum(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get hp sum: %w", err)
	}
	events, err := s.dal.GetHistory(ctx, characterID, s.cfg.HistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	return &HPState{
		CurrentHp: maxHP + sum,
		MaxHp:     maxHP,
		Events:    events,
	}, nil
}
