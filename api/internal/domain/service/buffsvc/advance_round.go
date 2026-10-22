package buffsvc

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// AdvanceRound increments the round counter and counts timed buffs down,
// atomically, with a round_advance entry snapshotting both the buff list and
// the round so voiding it steps back (expired buffs come back). In the same
// transaction it rolls the new round's initiative, logged as its own entry
// after the advance (ADR 0013 steps 1 and 2). Both entries are one action-log
// transaction (ADR 0018 step 1), so one undo pops the advance and its
// initiative together (step 2).
func (s *service) AdvanceRound(ctx context.Context, characterID int64) (active, expired []BuffEffect, round int, initiative Initiative, end EndResult, err error) {
	var ch change
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		ch, err = s.applyIn(ctx, characterID, func(buffs []BuffEffect, r int) (change, error) {
			next := r + 1
			active, expired = advanceRoundBuffs(buffs)
			// ADR 0043 step 14: the maintained powers due pay, in the
			// list's order; one the PSPs left cannot pay ends.
			m, err := s.chargeMaintenance(ctx, characterID, active, r, 1)
			if err != nil {
				return change{}, err
			}
			active, expired = m.kept, append(expired, m.unpaid...)
			names, spellIDs, profIDs := expiredNames(expired)
			return change{
				kind: actionlog.KindRoundAdvance,
				payload: logPayload{
					Action: "next_round", FromRound: &r, ToRound: &next, Expired: names, ExpiredSpellIDs: spellIDs, ExpiredProficiencyIDs: profIDs,
					Maintenance: m.charges, MaintenanceUnpaid: m.unpaidIDs(),
				},
				pspDelta: -m.spent,
				buffs:    active,
				round:    &next,
				// The previous round's initiative ends here; the new one is
				// its own entry (ADR 0013 step 2).
				clearInitiative: true,
				// ADR 0004 step 21: the rows that run out apply their
				// "expired" end effects in this entry.
				ended:    expired,
				endCause: spelleffectsvc.EndExpired,
			}, nil
		})
		if err != nil {
			return err
		}
		// ADR 0035 item 6: the weapon in hand's speed and the effects of the
		// buffs still on after the advance.
		mods, err := s.initiativeModifiers(ctx, characterID, active)
		if err != nil {
			return err
		}
		initiative, err = s.rollInitiative(ctx, characterID, *ch.round, mods)
		return err
	})
	if err != nil {
		return nil, nil, 0, Initiative{}, EndResult{}, err
	}
	return active, expired, *ch.round, initiative, ch.end, nil
}

// maintenance is what Next Round's (or passing time's) maintenance did (ADR
// 0043 steps 14 and 15).
type maintenance struct {
	kept    []BuffEffect
	unpaid  []BuffEffect
	charges []maintenanceCharge
	spent   int
}

func (m maintenance) unpaidIDs() []string {
	var ids []string
	for _, b := range m.unpaid {
		ids = append(ids, b.PowerID)
	}
	return ids
}

// chargeMaintenance charges the rows of active whose maintenance falls due
// in the rounds Next Rounds ending rounds from to from+rounds-1, from the
// current PSPs: round by round, each round's due rows in the list's order. A
// row whose cost the PSPs left cannot pay ends there, charging nothing more;
// a cheaper row after it may still pay. charges has one line per power that
// paid, in the list's order, its Cost the total it paid. The PSPs are read
// only when a row is due. It writes nothing (the preview runs it too).
func (s *service) chargeMaintenance(ctx context.Context, characterID int64, active []BuffEffect, from, rounds int) (maintenance, error) {
	m := maintenance{kept: active}
	if !slices.ContainsFunc(active, func(b BuffEffect) bool { return b.firstDue(from, rounds) >= 0 }) {
		return m, nil
	}
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return maintenance{}, fmt.Errorf("get character: %w", err)
	}
	left := 0
	if c.Derived.PSP != nil {
		left = c.Derived.PSP.Current
	}
	paid := make([]int, len(active))
	ended := make([]bool, len(active))
	for r := from; r < from+rounds; r++ {
		for i, b := range active {
			if ended[i] || !b.MaintenanceDue(r) {
				continue
			}
			if b.MaintenanceCost > left {
				ended[i] = true
				continue
			}
			left -= b.MaintenanceCost
			paid[i] += b.MaintenanceCost
		}
	}
	m.kept = make([]BuffEffect, 0, len(active))
	for i, b := range active {
		if paid[i] > 0 {
			m.spent += paid[i]
			m.charges = append(m.charges, maintenanceCharge{PowerID: b.PowerID, Cost: paid[i]})
		}
		if ended[i] {
			m.unpaid = append(m.unpaid, b)
		} else {
			m.kept = append(m.kept, b)
		}
	}
	return m, nil
}
