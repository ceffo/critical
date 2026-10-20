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
	after := make([]BuffEffect, 0, len(before)+1)
	// ADR 0004 step 20.5: a row on someone else or on an area never renews
	// and is never renewed (two Webs on two areas are two rows).
	stacks := isStrengthBonus(buff.Effects) || buff.OnOther
	for _, b := range before {
		if stacks || b.OnOther || !strings.EqualFold(b.Name, buff.Name) {
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

// Remove takes buff buffID off the active list without recording an entry,
// for a use case that records one entry covering the change (a skill check's
// adjustment that turns its success into a failure takes the buff the
// success put on off, ADR 0037 step 20.8). Call it inside the caller's
// transaction and record the returned snapshot (StateKeyBuffs) in that
// entry. A buff no longer on the list (it ended, or was dismissed) is not an
// error: removed is false and nothing is written.
func (s *service) Remove(ctx context.Context, characterID int64, buffID string) (bool, actionlog.Snapshot, error) {
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return false, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	i := indexOf(before, buffID)
	if buffID == "" || i < 0 {
		return false, actionlog.Snapshot{}, nil
	}
	after := without(before, i)
	snap, err := buffsSnapshot(before, after)
	if err != nil {
		return false, actionlog.Snapshot{}, err
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, after); err != nil {
		return false, actionlog.Snapshot{}, fmt.Errorf("save active buffs: %w", err)
	}
	return true, snap, nil
}
