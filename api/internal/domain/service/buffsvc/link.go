package buffsvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// TangentsChange is what AddTangents did to the rows holding minds (ADR
// 0044 step 3).
type TangentsChange struct {
	// Target is the mind the attack named, its label trimmed.
	Target string
	// Before and After are the tangents the target's row held before and
	// after the change: 0 with no tangent row, MaxTangents+1 for a contact
	// row (full contact is the third tangent).
	Before, After int
	// Contact: the change turned the target's row into a contact row.
	Contact bool
	// Broken is the tangent row of another mind the change ended, if any.
	Broken *BuffEffect
	// Row is the target's row after the change; zero when it holds none.
	Row BuffEffect
}

// AddTangents is an attack mode's use on target (ADR 0044 step 3, for step
// 7.3): it ends the tangent row of any other mind (one mind at a time, the
// book's breaking by another target; contact rows stay), then adds won
// tangents to target's row, putting one on with the first (maintained at
// LinkMaintenanceCost every round, first charged on the Next Round after
// the round it is made). The third tangent turns the row into a contact
// row and the rest count for nothing; a mind already contacted takes none.
// won 0 only breaks. It records nothing: call it inside the caller's
// transaction (ctx carries it) and record the returned snapshot
// (StateKeyBuffs) in the attack's entry; the snapshot is the zero value
// when nothing changed. A target that is no label, or a negative won:
// ErrInvalidInput.
func (s *service) AddTangents(ctx context.Context, characterID int64, target string, won int) (TangentsChange, actionlog.Snapshot, error) {
	t, err := psionics.NormalizeTarget(target)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, err
	}
	if won < 0 {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("%w: won tangents cannot be negative", domain.ErrInvalidInput)
	}
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	round, err := s.dal.GetRoundNumber(ctx, characterID)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("get round: %w", err)
	}
	ch, after := addTangents(before, t, won, round)
	if ch.Broken == nil && ch.Before == ch.After {
		return ch, actionlog.Snapshot{}, nil
	}
	snap, err := buffsSnapshot(before, after)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, err
	}
	if err := s.dal.SaveActiveBuffs(ctx, characterID, after); err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("save active buffs: %w", err)
	}
	return ch, snap, nil
}

// addTangents is AddTangents' rule on the rows, at round: the change and the
// rows after it.
func addTangents(rows []BuffEffect, target string, won, round int) (TangentsChange, []BuffEffect) {
	ch := TangentsChange{Target: target}
	after := make([]BuffEffect, 0, len(rows)+1)
	at := -1
	for _, b := range rows {
		switch {
		case b.TangentRow() && b.Target != target:
			ch.Broken = &b
			continue
		case b.linkRow() && b.Target == target:
			at = len(after)
		}
		after = append(after, b)
	}
	if at >= 0 {
		ch.Before = held(after[at])
	}
	ch.After = min(ch.Before+won, MaxTangents+1)
	if ch.Before == MaxTangents+1 || ch.After == ch.Before {
		if at >= 0 {
			ch.Row = after[at]
		}
		ch.After = ch.Before
		return ch, after
	}
	if at < 0 {
		after = append(after, linkRow(target, round))
		at = len(after) - 1
	}
	// The row keeps its id, start and rounds kept: the tangents made in an
	// earlier round were paid for, the new ones join them.
	after[at].Tangents = ch.After
	if ch.After > MaxTangents {
		after[at].Tangents = 0 // step 3.1: the third tangent is full contact
		ch.Contact = true
	}
	ch.Row = after[at]
	return ch, after
}

// held is the tangents a row holding a mind counts: MaxTangents+1 for full
// contact.
func held(b BuffEffect) int {
	if b.ContactRow() {
		return MaxTangents + 1
	}
	return b.Tangents
}

// linkRow is a new row holding target at round, before its tangents are
// set: maintained every round, untimed (ADR 0044 step 3). Its name is the
// target's label, the player's text.
func linkRow(target string, round int) BuffEffect {
	return BuffEffect{
		ID: uuid.New().String(), Name: target, Target: target, StartRound: round,
		MaintenanceCost: LinkMaintenanceCost, MaintenancePeriod: 1,
	}
}
