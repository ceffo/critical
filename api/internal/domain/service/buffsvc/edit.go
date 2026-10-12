package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Edit corrects a buff's remaining rounds and/or charges and logs the change.
// Setting a counter the buff uses to 0 ends it.
func (s *service) Edit(ctx context.Context, characterID int64, buffID string, e BuffEdit) ([]BuffEffect, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		before := buffs[i]
		next, after, ended, err := edit(buffs, i, e)
		if err != nil {
			return change{}, err
		}
		summary := fmt.Sprintf("Edited %s: %s → %s", before.Name,
			counters(before.Duration, before.Charges), counters(after.Duration, after.Charges))
		if ended {
			summary = fmt.Sprintf("Edited %s: %s → ended", before.Name, counters(before.Duration, before.Charges))
		}
		return change{
			kind:    actionlog.KindBuffChange,
			summary: summary,
			payload: logPayload{
				Action: "edit", Buff: before.Name,
				FromRounds: before.Duration, FromCharges: before.Charges,
				Rounds: after.Duration, Charges: after.Charges, Ended: ended,
			},
			buffs: next,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return ch.buffs, nil
}
