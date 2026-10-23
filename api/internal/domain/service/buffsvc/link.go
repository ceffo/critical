package buffsvc

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
		case b.TangentRow() && !strings.EqualFold(b.Target, target):
			ch.Broken = &b
			continue
		case b.linkRow() && strings.EqualFold(b.Target, target):
			at = len(after)
		}
		after = append(after, b)
	}
	if at >= 0 {
		ch.Target = after[at].Target // the row's label, whatever the case typed
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
		after[at].ContactFrom = after[at].RoundsKept
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

// TangentsOn is the tangents rows hold on target (ADR 0044 step 3): 0 with
// no row holding it, MaxTangents+1 when a contact row does. target is
// compared ignoring case, as a renewed name is: trim it first (psionics.NormalizeTarget).
func TangentsOn(rows []BuffEffect, target string) int {
	for _, b := range rows {
		if b.linkRow() && strings.EqualFold(b.Target, target) {
			return held(b)
		}
	}
	return 0
}

// MakeContact is the contact power's success on target (ADR 0044 step 9):
// target's row becomes a contact row, a tangent row on it turned (it keeps
// its id and rounds kept) or a new row put on (maintained at
// LinkMaintenanceCost every round, first charged as a tangent row is). A
// mind already contacted is renewed, never doubled: nothing changes. Another
// mind's tangent row stays (only an attack mode breaks it, step 3.3). It
// records nothing, as AddTangents: record the returned snapshot (zero when
// nothing changed) in the caller's entry. A target that is no label:
// a *domain.FieldError.
func (s *service) MakeContact(ctx context.Context, characterID int64, target string) (TangentsChange, actionlog.Snapshot, error) {
	t, err := psionics.NormalizeTarget(target)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, err
	}
	before, err := s.dal.GetActiveBuffs(ctx, characterID)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("get active buffs: %w", err)
	}
	round, err := s.dal.GetRoundNumber(ctx, characterID)
	if err != nil {
		return TangentsChange{}, actionlog.Snapshot{}, fmt.Errorf("get round: %w", err)
	}
	ch, after := makeContact(before, t, round)
	if ch.Before == ch.After {
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

// makeContact is MakeContact's rule on the rows, at round.
func makeContact(rows []BuffEffect, target string, round int) (TangentsChange, []BuffEffect) {
	ch := TangentsChange{Target: target, After: MaxTangents + 1}
	after := slices.Clone(rows)
	at := slices.IndexFunc(after, func(b BuffEffect) bool { return b.linkRow() && strings.EqualFold(b.Target, target) })
	if at >= 0 {
		ch.Target, ch.Before = after[at].Target, held(after[at])
	}
	if ch.Before == ch.After {
		ch.Row = after[at]
		return ch, after
	}
	if at < 0 {
		after = append(after, linkRow(target, round))
		at = len(after) - 1
	}
	after[at].Tangents, after[at].ContactFrom = 0, after[at].RoundsKept
	ch.Contact, ch.Row = true, after[at]
	return ch, after
}

// ContactBefore is the contact row holding target made in an earlier round
// than the current one (ADR 0044 step 10): kept at least one Next Round (or
// a round of passing time) since it became contact. ok false when no
// contact row holds target, or it was made this round. target is compared
// ignoring case, as TangentsOn's.
func ContactBefore(rows []BuffEffect, target string) (BuffEffect, bool) {
	for _, b := range rows {
		if b.ContactRow() && strings.EqualFold(b.Target, target) {
			return b, b.RoundsKept > b.ContactFrom
		}
	}
	return BuffEffect{}, false
}

// usesContact reports whether row b is a maintained power kept up through
// the contact row c (ADR 0044 step 10).
func usesContact(b, c BuffEffect) bool {
	return b.PowerID != "" && b.Target != "" && strings.EqualFold(b.Target, c.Target)
}
