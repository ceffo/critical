package buffsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/google/uuid"
)

// Add puts buff on the active list without recording an entry, for a use
// case that records one entry covering several states (a spell cast: the
// slot and the buff). Call it inside the caller's transaction (ctx carries
// it) and record the returned snapshot (StateKeyBuffs) in that entry.
//
// An active buff with the same name is replaced: spell effects are not
// cumulative with themselves, so casting one again renews it. A Strength
// bonus (not Improved) is the exception: it stacks with itself, so a second
// cast is a second buff (ADR 0004 steps 6 and 7).
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
	stacks := isStrengthBonus(buff.Effects)
	for _, b := range before {
		if stacks || !strings.EqualFold(b.Name, buff.Name) {
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
