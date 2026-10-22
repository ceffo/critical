package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// Dismiss removes the buff with id and logs it. Unknown id: ErrNotFound.
//
// ADR 0004 step 21: a row whose spell has end effects needs the cause the
// player names (voluntary or dispelled; without one:
// spelleffectsvc.ErrEndCauseRequired, nothing written), and the effects that
// cause triggers apply in the dismiss's entry. Any other row ignores the cause.
func (s *service) Dismiss(ctx context.Context, characterID int64, buffID string, cause spelleffectsvc.EndCause) ([]BuffEffect, EndResult, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		c := change{
			kind:    actionlog.KindBuffChange,
			payload: logPayload{Action: "dismiss", Buff: buffs[i].Name, SpellID: buffs[i].SpellID, ProficiencyID: buffs[i].ProficiencyID, PowerID: buffs[i].PowerID, PowerNames: PowerNames(buffs[i : i+1])},
			buffs:   without(buffs, i),
		}
		if s.asksEndCause(buffs[i]) {
			if cause == "" {
				return change{}, fmt.Errorf("%w: buff %q", spelleffectsvc.ErrEndCauseRequired, buffID)
			}
			c.payload.Cause = string(cause)
			c.ended, c.endCause = []BuffEffect{buffs[i]}, cause
		}
		return c, nil
	})
	if err != nil {
		return nil, EndResult{}, err
	}
	return ch.buffs, ch.end, nil
}
