package buffsvc

import (
	"context"
	"fmt"
	"math"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// MaxPassHours caps one fast-forward (30 days): a typo like 8000 instead of
// 8 is refused rather than wiping every buff.
const MaxPassHours = 720

// RoundsPerHour is the rounds in one game hour: an AD&D round is one minute.
const RoundsPerHour = 60

// RoundsForHours converts game hours to whole rounds, rounding down (1.5 h =
// 90 rounds, 0.02 h = 1 round). hours must be > 0 and <= MaxPassHours, and
// give at least one round; otherwise ErrInvalidInput.
func RoundsForHours(hours float64) (int, error) {
	if math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 {
		return 0, fmt.Errorf("%w: hours must be more than 0", domain.ErrInvalidInput)
	}
	if hours > MaxPassHours {
		return 0, fmt.Errorf("%w: at most %d hours at a time, got %g", domain.ErrInvalidInput, MaxPassHours, hours)
	}
	// The epsilon absorbs binary fractions: 1.15 h is 68.99999999999999 in
	// floating point but 69 rounds.
	rounds := int(math.Floor(hours*RoundsPerHour + 1e-9))
	if rounds < 1 {
		return 0, fmt.Errorf("%w: %g hours is less than one round (1 minute)", domain.ErrInvalidInput, hours)
	}
	return rounds, nil
}

// PassTime fast-forwards hours of game time: every timed buff loses
// RoundsForHours(hours) rounds and expires at 0 or below; charge-only and
// untimed buffs are untouched; the round counter goes to 0 (out of combat).
// One time_passed entry snapshots the buffs and the round, so voiding it
// restores both.
func (s *service) PassTime(ctx context.Context, characterID int64, hours float64) (active, expired []BuffEffect, rounds, round int, end EndResult, err error) {
	rounds, err = RoundsForHours(hours)
	if err != nil {
		return nil, nil, 0, 0, EndResult{}, err
	}
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, from int) (change, error) {
		to := 0
		active, expired = elapseBuffs(buffs, rounds)
		names, spellIDs, profIDs := expiredNames(expired)
		return change{
			kind:    actionlog.KindTimePassed,
			payload: logPayload{Action: "pass_time", Hours: hours, Rounds: rounds, FromRound: &from, ToRound: &to, Expired: names, ExpiredSpellIDs: spellIDs, ExpiredProficiencyIDs: profIDs},
			buffs:   active,
			round:   &to,
			// Out of combat: no initiative (ADR 0013 step 5).
			clearInitiative: true,
			// ADR 0004 step 21: the rows that run out apply their
			// "expired" end effects in this entry.
			ended:    expired,
			endCause: spelleffectsvc.EndExpired,
		}, nil
	})
	if err != nil {
		return nil, nil, 0, 0, EndResult{}, err
	}
	return active, expired, rounds, *ch.round, ch.end, nil
}

// PreviewPassTime runs the PassTime rules on the current buffs and returns
// the rounds and the buffs that would expire, writing nothing (ADR 0005
// step 8): the time-passes form lists them before the player confirms.
func (s *service) PreviewPassTime(ctx context.Context, characterID int64, hours float64) (int, []BuffEffect, error) {
	rounds, err := RoundsForHours(hours)
	if err != nil {
		return 0, nil, err
	}
	buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return 0, nil, fmt.Errorf("get active buffs: %w", err)
	}
	_, expiring := elapseBuffs(buffs, rounds)
	return rounds, expiring, nil
}
