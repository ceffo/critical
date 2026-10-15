package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ResetRound is End combat: the round counter goes to 0.
func (s *service) ResetRound(ctx context.Context, characterID int64) ([]BuffEffect, int, error) {
	return s.moveRound(ctx, characterID, actionlog.KindRoundReset, "end_combat", func(int) (int, error) { return 0, nil })
}

// SetRound sets the round counter to an arbitrary value >= 0.
func (s *service) SetRound(ctx context.Context, characterID int64, to int) ([]BuffEffect, int, error) {
	if to < 0 {
		return nil, 0, fmt.Errorf("%w: round must be >= 0, got %d", domain.ErrInvalidInput, to)
	}
	return s.moveRound(ctx, characterID, actionlog.KindRoundSet, "set_round", func(int) (int, error) { return to, nil })
}

// moveRound logs and applies a round change that leaves buffs alone, with a
// snapshot of the round key only.
func (s *service) moveRound(ctx context.Context, characterID int64, kind actionlog.Kind, action string, target func(from int) (int, error)) ([]BuffEffect, int, error) {
	var active []BuffEffect
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, from int) (change, error) {
		to, err := target(from)
		if err != nil {
			return change{}, err
		}
		active = buffs
		return change{
			kind:      kind,
			payload:   logPayload{Action: action, FromRound: &from, ToRound: &to},
			round:     &to,
			roundOnly: true,
			// Round 0 is out of combat: no initiative (ADR 0013 step 5). A
			// set to another round keeps the round's initiative.
			clearInitiative: to == 0,
		}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return active, *ch.round, nil
}
