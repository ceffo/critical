package buffsvc

import (
	"context"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// AdvanceRound increments the round counter and counts timed buffs down,
// atomically, with a round_advance entry snapshotting both the buff list and
// the round so voiding it steps back (expired buffs come back). In the same
// transaction it rolls the new round's initiative, logged as its own entry
// after the advance (ADR 0013 steps 1 and 2): undo pops the initiative first,
// then the advance.
func (s *service) AdvanceRound(ctx context.Context, characterID int64) (active []BuffEffect, expired []BuffEffect, round int, initiative Initiative, err error) {
	var ch change
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		ch, err = s.applyIn(ctx, characterID, func(buffs []BuffEffect, r int) (change, error) {
			next := r + 1
			active, expired = advanceRoundBuffs(buffs)
			names := make([]string, len(expired))
			for i, b := range expired {
				names[i] = b.Name
			}
			return change{
				kind:    actionlog.KindRoundAdvance,
				payload: logPayload{Action: "next_round", FromRound: &r, ToRound: &next, Expired: names},
				buffs:   active,
				round:   &next,
				// The previous round's initiative ends here; the new one is
				// its own entry (ADR 0013 step 2).
				clearInitiative: true,
			}, nil
		})
		if err != nil {
			return err
		}
		initiative, err = s.rollInitiative(ctx, characterID, *ch.round)
		return err
	})
	if err != nil {
		return nil, nil, 0, Initiative{}, err
	}
	return active, expired, *ch.round, initiative, nil
}
