package buffsvc

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ResetRound is End combat: the round counter goes to 0.
func (s *service) ResetRound(ctx context.Context, characterID int64) ([]BuffEffect, int, error) {
	return s.moveRound(ctx, characterID, actionlog.KindRoundReset, "end_combat", func(int) (int, error) { return 0, nil })
}

// SetRound sets the round counter to an arbitrary value >= 0.
func (s *service) SetRound(ctx context.Context, characterID int64, to int) ([]BuffEffect, int, error) {
	if to < 0 {
		return nil, 0, fmt.Errorf("%w: round must be >= 0, got %d", domain.ErrInvalidInput, to)
	}
	return s.moveRound(ctx, characterID, actionlog.KindRoundSet, "set_round", func(int) (int, error) { return to, nil })
}

// moveRound logs and applies a round change that leaves buffs alone, with a
// snapshot of the round key only, except that moving the counter off a
// maintained power's activation round closes that round (ADR 0043 step 14):
// the row counts one round kept, charging nothing, so the next Next Round
// charges it; and a defence mode raised for the round ends (ADR 0044 step 4).
// Without such a row the buff list is neither snapshotted nor written.
func (s *service) moveRound(ctx context.Context, characterID int64, kind actionlog.Kind, action string, target func(from int) (int, error)) ([]BuffEffect, int, error) {
	var active []BuffEffect
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, from int) (change, error) {
		to, err := target(from)
		if err != nil {
			return change{}, err
		}
		active = buffs
		closed := false
		if to != from {
			active, closed = closeActivationRounds(buffs)
			var lowered bool
			active, lowered = lowerRaisedDefences(active)
			closed = closed || lowered
		}
		return change{
			kind:      kind,
			payload:   logPayload{Action: action, FromRound: &from, ToRound: &to},
			round:     &to,
			buffs:     active,
			roundOnly: !closed,
			// Round 0 is out of combat: no initiative (ADR 0013 step 5). A
			// set to another round keeps the round's initiative.
			clearInitiative: to == 0,
		}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return active, *ch.round, nil
}

// closeActivationRounds counts one round kept on every maintained row still
// in its activation round (RoundsKept 0), and reports whether there was one.
func closeActivationRounds(buffs []BuffEffect) ([]BuffEffect, bool) {
	closed := false
	for i, b := range buffs {
		if b.Maintained() && b.RoundsKept == 0 {
			buffs[i].RoundsKept = 1
			closed = true
		}
	}
	return buffs, closed
}

// lowerRaisedDefences drops every defence mode raised for the round (ADR
// 0044 step 4: the row ends at the next move of the round counter), and
// reports whether there was one. Next Round and passing time end it as a
// one-round row.
func lowerRaisedDefences(buffs []BuffEffect) ([]BuffEffect, bool) {
	out := slices.DeleteFunc(slices.Clone(buffs), func(b BuffEffect) bool { return b.Defence == DefenceRaised })
	return out, len(out) != len(buffs)
}
