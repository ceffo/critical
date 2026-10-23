package buffsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Add puts buff on the active list without recording an entry, for a use
// case that records one entry covering several states (a spell cast: the
// slot and the buff). Call it inside the caller's transaction (ctx carries
// it) and record the returned snapshot (StateKeyBuffs) in that entry.
//
// An active buff with the same name is replaced: spell effects are not
// cumulative with themselves, so casting one again renews it. A Strength
// bonus (not Improved) is the exception: it stacks with itself, so a second
// cast is a second buff (ADR 0004 steps 6 and 7), and so is a row on someone
// else (step 20.5).
func (s *service) Add(ctx context.Context, characterID int64, buff BuffEffect) (BuffEffect, actionlog.Snapshot, error) {
	buff.Name = strings.TrimSpace(buff.Name)
	buff.Note = strings.TrimSpace(buff.Note)
	if err := validateBuff(buff); err != nil {
		return BuffEffect{}, actionlog.Snapshot{}, err
	}
	if buff.ID == "" {
		buff.ID = uuid.New().String()
	}
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return BuffEffect{}, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	// ADR 0004 step 15: a buff starts in the round it is added (a renewed
	// one starts again).
	round, err := s.dal.GetRoundNumber(ctx, characterID)
	if err != nil {
		return BuffEffect{}, actionlog.Snapshot{}, fmt.Errorf("get round: %w", err)
	}
	buff.StartRound = round
	buff.ChargesTotal = buff.Charges // ADR 0004 step 16
	buff.RoundsKept = 0              // ADR 0043 step 14: a renewal starts again
	after := make([]BuffEffect, 0, len(before)+1)
	// ADR 0004 step 20.5: a row on someone else or on an area never renews
	// and is never renewed (two Webs on two areas are two rows).
	stacks := isStrengthBonus(buff.Effects) || buff.OnOther
	for _, b := range before {
		if !renews(buff, b, stacks) {
			after = append(after, b)
		}
	}
	after = append(after, buff)
	snaps, err := snapshots(before, after, 0, nil, false)
	if err != nil {
		return BuffEffect{}, actionlog.Snapshot{}, err
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, after); err != nil {
		return BuffEffect{}, actionlog.Snapshot{}, fmt.Errorf("save active buffs: %w", err)
	}
	return buff, snaps[0], nil
}

// renews reports whether buff, put on, replaces row b.
func renews(buff, b BuffEffect, stacks bool) bool {
	// ADR 0043 step 13: a power activated again renews its row.
	samePower := buff.PowerID != "" && b.PowerID == buff.PowerID
	// ADR 0044 step 10: a power used on a mind is maintained for that mind
	// alone, so only the same power on the same mind renews its row (its
	// name doesn't either).
	if b.PowerID != "" && b.Target != "" {
		return samePower && strings.EqualFold(b.Target, buff.Target)
	}
	// ADR 0044 step 3: a row holding a mind is named by the player's label;
	// no buff of that name renews it.
	return samePower || replacesDefence(buff, b) ||
		(!stacks && !b.OnOther && !b.linkRow() && strings.EqualFold(b.Name, buff.Name))
}

// replacesDefence reports whether the defence mode's row buff, put on,
// replaces row b (ADR 0044 steps 4 and 5): any raised defence goes (one a
// round, Q2), and a standing one replaces the standing one.
func replacesDefence(buff, b BuffEffect) bool {
	switch buff.Defence {
	case DefenceRaised:
		return b.Defence == DefenceRaised
	case DefenceStanding:
		return b.Defence == DefenceRaised || b.Defence == DefenceStanding
	}
	return false
}

// Remove takes buff buffID off the active list and puts putBack in its
// place, without recording an entry, for a use case that records one entry
// covering the change (a skill check's adjustment that turns its success into
// a failure takes the buff the success put on off and gives back the rows it
// renewed, ADR 0037 step 20.8). Call it inside the caller's transaction and
// record the returned snapshot (StateKeyBuffs) in that entry. A buff no
// longer on the list (it ended, or was dismissed) is not an error: removed is
// false, putBack is dropped and nothing is written.
func (s *service) Remove(ctx context.Context, characterID int64, buffID string, putBack []BuffEffect) (bool, actionlog.Snapshot, error) {
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return false, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	i := indexOf(before, buffID)
	if buffID == "" || i < 0 {
		return false, actionlog.Snapshot{}, nil
	}
	after := make([]BuffEffect, 0, len(before)-1+len(putBack))
	after = append(after, before[:i]...)
	after = append(after, putBack...)
	after = append(after, before[i+1:]...)
	snap, err := buffsSnapshot(before, after)
	if err != nil {
		return false, actionlog.Snapshot{}, err
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, after); err != nil {
		return false, actionlog.Snapshot{}, fmt.Errorf("save active buffs: %w", err)
	}
	return true, snap, nil
}

// EndMaintained takes every maintained power's row off the active list
// without recording an entry (ADR 0043 step 7: a long rest ends them, the
// book allows no maintenance asleep). Call it inside the caller's
// transaction and record the returned snapshot (StateKeyBuffs) in that
// entry. With no maintained row it writes nothing and the snapshot is the
// zero value (ended is empty).
func (s *service) EndMaintained(ctx context.Context, characterID int64) (ended []BuffEffect, snap actionlog.Snapshot, err error) {
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	after := make([]BuffEffect, 0, len(before))
	for _, b := range before {
		if b.Maintained() {
			ended = append(ended, b)
		} else {
			after = append(after, b)
		}
	}
	if len(ended) == 0 {
		return nil, actionlog.Snapshot{}, nil
	}
	if snap, err = buffsSnapshot(before, after); err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, after); err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("save active buffs: %w", err)
	}
	return ended, snap, nil
}
