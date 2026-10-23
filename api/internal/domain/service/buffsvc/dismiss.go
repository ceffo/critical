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
//
// ADR 0044 step 10: dismissing a contact row ends the powers kept up through
// it, in the same entry.
func (s *service) Dismiss(ctx context.Context, characterID int64, buffID string, cause spelleffectsvc.EndCause) ([]BuffEffect, EndResult, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		next, powers := withoutContact(buffs, i)
		c := change{
			kind:    actionlog.KindBuffChange,
			payload: logPayload{Action: "dismiss", Buff: buffs[i].Name, SpellID: buffs[i].SpellID, ProficiencyID: buffs[i].ProficiencyID, PowerID: buffs[i].PowerID, Target: buffs[i].Target, PowerNames: PowerNames(append(powers, buffs[i]))},
			buffs:   next,
		}
		for _, p := range powers {
			c.payload.ContactPowersEnded = append(c.payload.ContactPowersEnded, p.PowerID)
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

// withoutContact is buffs without row i and, when it is a contact row, the
// powers kept up through it (ADR 0044 step 10), which it returns.
func withoutContact(buffs []BuffEffect, i int) (next, powers []BuffEffect) {
	c := buffs[i]
	next = make([]BuffEffect, 0, len(buffs))
	for j, b := range buffs {
		switch {
		case j == i:
		case c.ContactRow() && usesContact(b, c):
			powers = append(powers, b)
		default:
			next = append(next, b)
		}
	}
	return next, powers
}
