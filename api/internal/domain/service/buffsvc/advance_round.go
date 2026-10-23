package buffsvc

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
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
			m, err := s.chargeMaintenance(ctx, characterID, active, 1)
			if err != nil {
				return change{}, err
			}
			// An unpaid power is listed once, as a power ended, not as expired.
			names, spellIDs, profIDs := expiredNames(expired)
			active, expired = m.kept, append(expired, m.unpaid...)
			return change{
				kind: actionlog.KindRoundAdvance,
				payload: logPayload{
					Action: "next_round", FromRound: &r, ToRound: &next, Expired: names, ExpiredSpellIDs: spellIDs, ExpiredProficiencyIDs: profIDs,
					Maintenance: m.charges, MaintenanceUnpaid: m.unpaidIDs(), UnpaidTargets: m.unpaidTargets(), PowerNames: m.powerNames(),
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

// powerNames is the localized name of every power that paid or ended, by id;
// nil when none has a name.
func (m maintenance) powerNames() map[string]localized.Text {
	return PowerNames(append(slices.Clone(m.kept), m.unpaid...))
}

// PowerNames maps the maintained rows' power ids to their names in every
// language; nil when no row has one.
func PowerNames(rows []BuffEffect) map[string]localized.Text {
	var out map[string]localized.Text
	for _, b := range rows {
		if b.PowerID == "" || len(b.PowerName) == 0 {
			continue
		}
		if out == nil {
			out = map[string]localized.Text{}
		}
		out[b.PowerID] = b.PowerName
	}
	return out
}

// unpaidIDs are the unpaid powers' ids, in the list's order.
func (m maintenance) unpaidIDs() []string {
	var ids []string
	for _, b := range m.unpaid {
		if b.PowerID != "" {
			ids = append(ids, b.PowerID)
		}
	}
	return ids
}

// unpaidTargets are the minds of the unpaid tangent and contact rows, in the
// list's order.
func (m maintenance) unpaidTargets() []string {
	var targets []string
	for _, b := range m.unpaid {
		if b.linkRow() {
			targets = append(targets, b.Target)
		}
	}
	return targets
}

// chargeMaintenance keeps the maintained rows of active for rounds rounds
// (a Next Round: 1; passing time: its rounds), charging from the current
// PSPs: round by round, each round's due rows in the list's order, the
// contact rows after the others, free while a power on their target is kept
// up (ADR 0044 step 10). A row whose cost the PSPs left cannot pay ends there, charging nothing more; a
// cheaper row after it may still pay. Every kept maintained row's RoundsKept
// grows by rounds. charges has one line per power that paid, in the list's
// order, its Cost the total it paid. The PSPs are read only when a row is
// due. It writes nothing (the preview runs it too).
func (s *service) chargeMaintenance(ctx context.Context, characterID int64, active []BuffEffect, rounds int) (maintenance, error) {
	m := maintenance{kept: make([]BuffEffect, 0, len(active))}
	due := slices.ContainsFunc(active, func(b BuffEffect) bool { return b.firstDue(rounds) >= 0 })
	left := 0
	if due {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return maintenance{}, fmt.Errorf("get character: %w", err)
		}
		if c.Derived.PSP != nil {
			left = c.Derived.PSP.Current
		}
	}
	rows := slices.Clone(active)
	paid := make([]int, len(rows))
	ended := make([]bool, len(rows))
	// ADR 0044 step 10: a contact row is free in a round where a power kept
	// up through it is still on, so each round the contact rows come after
	// the other rows.
	freeContact := func(c BuffEffect) bool {
		for j, b := range rows {
			if !ended[j] && usesContact(b, c) {
				return true
			}
		}
		return false
	}
	for range rounds {
		for _, contacts := range []bool{false, true} {
			for i := range rows {
				b := &rows[i]
				if ended[i] || !b.Maintained() || b.ContactRow() != contacts {
					continue
				}
				if b.MaintenanceDue() && (!contacts || !freeContact(*b)) {
					if b.MaintenanceCost > left {
						ended[i] = true
						continue
					}
					left -= b.MaintenanceCost
					paid[i] += b.MaintenanceCost
				}
				b.RoundsKept++
			}
		}
	}
	for i, b := range rows {
		if paid[i] > 0 {
			m.spent += paid[i]
			m.charges = append(m.charges, maintenanceCharge{PowerID: b.PowerID, Target: b.Target, Cost: paid[i]})
		}
		if ended[i] {
			m.unpaid = append(m.unpaid, active[i])
		} else {
			m.kept = append(m.kept, b)
		}
	}
	return m, nil
}

// firstDue is how many rounds, out of the next rounds kept, pass before the
// row's maintenance falls due (0: the next Next Round charges), or -1 when
// none of them does.
func (b BuffEffect) firstDue(rounds int) int {
	if !b.Maintained() || rounds < 1 {
		return -1
	}
	period := max(b.MaintenancePeriod, 1)
	k := b.RoundsKept
	next := max(k, 1)
	if r := next % period; r != 0 {
		next += period - r
	}
	if next-k >= rounds {
		return -1
	}
	return next - k
}
