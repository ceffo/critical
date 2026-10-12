package buffsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// AdvanceRound increments the round counter and counts timed buffs down,
// atomically, with a round_advance entry snapshotting both the buff list and
// the round so voiding it steps back (expired buffs come back).
func (s *service) AdvanceRound(ctx context.Context, characterID int64) (active []BuffEffect, expired []BuffEffect, round int, err error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, r int) (change, error) {
		next := r + 1
		active, expired = advanceRoundBuffs(buffs)
		names := make([]string, len(expired))
		for i, b := range expired {
			names[i] = b.Name
		}
		summary := fmt.Sprintf("Round %d → %d", r, next)
		if len(names) > 0 {
			summary += " (expired: " + strings.Join(names, ", ") + ")"
		}
		return change{
			kind:    actionlog.KindRoundAdvance,
			summary: summary,
			payload: logPayload{Action: "next_round", FromRound: &r, ToRound: &next, Expired: names},
			buffs:   active,
			round:   &next,
		}, nil
	})
	if err != nil {
		return nil, nil, 0, err
	}
	return active, expired, *ch.round, nil
}
