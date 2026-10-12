package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Dismiss removes the buff with id and logs it. Unknown id: ErrNotFound.
func (s *service) Dismiss(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		name := buffs[i].Name
		return change{
			kind:    actionlog.KindBuffChange,
			summary: fmt.Sprintf("Dismissed %s", name),
			payload: logPayload{Action: "dismiss", Buff: name},
			buffs:   without(buffs, i),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return ch.buffs, nil
}
