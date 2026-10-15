package buffsvc

import (
	"context"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Trigger uses one charge of a charged buff (a Stoneskin layer or a Mirror
// Image absorbed a hit) and logs it; the buff ends at 0. A buff without
// charges is ErrInvalidInput, an unknown id ErrNotFound.
func (s *service) Trigger(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		next, after, ended, err := absorb(buffs, i)
		if err != nil {
			return change{}, err
		}
		return change{
			kind:    actionlog.KindBuffChange,
			payload: logPayload{Action: "absorb", Buff: after.Name, Charges: after.Charges, Ended: ended},
			buffs:   next,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return ch.buffs, nil
}
