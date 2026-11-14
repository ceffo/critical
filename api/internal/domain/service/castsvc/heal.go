package castsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// heal applies a heal entry's heal (ADR 0008 step 17) inside the cast's or
// the receive's transaction. It lands as a friendly buff does (17.3): a
// spell received on the receiver, a cast where appliesToSelf says. The dice
// roll once at the caster level (17.4); full rolls nothing and asks for the
// gap to max HP the ledger holds before the entry. On the bearer the amount
// is the HP delta the cast entry asks for (record), which the recording path
// caps at max HP (17.5); HPBefore and HPAfter are his current HP around it,
// the ledger's own cap applied. Cast on someone else it is rolled and shown,
// and changes no HP (17.6).
func (s *service) heal(ctx context.Context, characterID int64, r effectReq, entry spelleffectsvc.Entry) (*HealResult, error) {
	onSelf := true
	if r.source.casts() {
		var err error
		if onSelf, err = appliesToSelf(entry, r.onSelf, r.casterLevel); err != nil {
			return nil, err
		}
	}
	h := &HealResult{Full: entry.Heal.Full, OnOther: !onSelf}
	if d := entry.Heal.Dice; d != nil {
		roll, err := s.roll(d, r.casterLevel)
		if err != nil {
			return nil, fmt.Errorf("%s heal: %w", entry.ID, err)
		}
		h.Roll, h.Total = &roll, roll.Total
	}
	if !onSelf {
		return h, nil
	}
	total, err := s.Log.HPTotal(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("hp total: %w", err)
	}
	// ADR 0049 step 12: an item ability's earlier heals are not in the ledger
	// until the use's entry is recorded.
	total += r.hpPending
	if h.Full {
		h.Total = max(0, -total)
	}
	h.HPDelta = h.Total
	before := actionlog.CurrentHP(r.hpMax, total)
	after := actionlog.CurrentHP(r.hpMax, total+actionlog.CapHeal(total, h.HPDelta))
	h.HPBefore, h.HPAfter = &before, &after
	return h, nil
}
