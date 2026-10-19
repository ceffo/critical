package combatsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// MaxTargetAC bounds a corrected target AC (±MaxTargetAC): AD&D armor
// classes run from 10 down to about -10, anything far beyond is a typo.
const MaxTargetAC = 20

// AdjustAttackAC corrects the target AC of the latest attack (log entry
// entryID) after the roll, when the DM gives or changes it (ADR 0002 step
// 11). Hit, margin and
// crit are recomputed with the same d20, crit-check d100 and multiplier
// d100 (ADR 0001 step 6; the multiplier is the crit row the multiplier d100
// fell in when rolling), and the damage from the dice rolled with the
// attack: no die is rolled and no ammunition is spent. The change is an
// audited amend on the entry; voiding the entry still gives the ammunition
// back. Refused for an entry that is not an attack, is voided or is not the
// latest roll, and for an attack logged before corrections existed.
//
// On an attack rolled against an unknown AC, giving the AC is ADR 0012 step
// 5: the same recomputation, and it replaces any GM verdict. There is no AC
// "as rolled" to keep then.
func (s *service) AdjustAttackAC(ctx context.Context, characterID, entryID int64, targetAC int) (*RollResult, error) {
	if targetAC < -MaxTargetAC || targetAC > MaxTargetAC {
		return nil, fmt.Errorf("%w: target AC must be between -%d and %d", domain.ErrInvalidInput, MaxTargetAC, MaxTargetAC)
	}
	var res *RollResult
	_, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindAttack, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		r, err := decodeCorrectable(e)
		if err != nil {
			return actionlog.PayloadAmendment{}, err
		}
		// The first correction keeps the target AC and outcome as rolled;
		// later ones keep those, so the line always reads rolled → now. An
		// attack rolled without the AC has none to keep.
		change := actionlog.Change{Field: ChangeTargetAC, From: knownAC(r), To: targetAC}
		if !r.RolledACUnknown && (r.RolledTargetAC == nil || r.RolledHit == nil || r.RolledCrit == nil) {
			ac, hit, crit := r.TargetAC, r.Hit, r.IsCrit
			r.RolledTargetAC, r.RolledHit, r.RolledCrit = &ac, &hit, &crit
		}
		r.TargetAC, r.ACKnown, r.Verdict = targetAC, true, VerdictNone
		settle(r)
		// Back to the rolled AC: the attack is as rolled again (still an
		// audited amend), its line the plain one.
		if r.RolledTargetAC != nil && *r.RolledTargetAC == targetAC {
			r.RolledTargetAC, r.RolledHit, r.RolledCrit = nil, nil, nil
		}
		res = r
		return encodeAmendment(r, change)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SetAttackVerdict records the GM's hit or miss on the latest attack (log
// entry entryID) when its target AC is unknown (ADR 0012 step 4): the attack
// hits or misses as told, never crits (x1), and its damage is the one rolled
// with it. Nothing is rolled and no ammunition is spent; it is an audited
// amend like a correction, replacing an earlier verdict. Refused like
// AdjustAttackAC, and for an attack whose AC is known (correct the AC
// instead).
func (s *service) SetAttackVerdict(ctx context.Context, characterID, entryID int64, hit bool) (*RollResult, error) {
	var res *RollResult
	_, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindAttack, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		r, err := decodeCorrectable(e)
		if err != nil {
			return actionlog.PayloadAmendment{}, err
		}
		if r.ACKnown {
			return actionlog.PayloadAmendment{}, fmt.Errorf("%w: the target AC of attack #%d is known, correct the AC instead", domain.ErrInvalidInput, e.ID)
		}
		change := actionlog.Change{Field: ChangeVerdict, From: verdictValue(r.Verdict)}
		r.Verdict = VerdictMiss
		if hit {
			r.Verdict = VerdictHit
		}
		change.To = verdictValue(r.Verdict)
		settle(r)
		res = r
		return encodeAmendment(r, change)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ClearAttackAC makes the target AC of the latest attack (log entry entryID)
// unknown again after it was given (ADR 0012 step 8): no hit, no crit, no
// verdict, the damage dimmed as "if it hits". Nothing is rolled and no
// ammunition is spent; it is an audited amend like a correction, refused like
// AdjustAttackAC and for an attack whose AC is already unknown.
func (s *service) ClearAttackAC(ctx context.Context, characterID, entryID int64) (*RollResult, error) {
	var res *RollResult
	_, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindAttack, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		r, err := decodeCorrectable(e)
		if err != nil {
			return actionlog.PayloadAmendment{}, err
		}
		if !r.ACKnown {
			return actionlog.PayloadAmendment{}, fmt.Errorf("%w: the target AC of attack #%d is already unknown", domain.ErrInvalidInput, e.ID)
		}
		change := actionlog.Change{Field: ChangeTargetAC, From: r.TargetAC, To: nil}
		// Keep the AC and outcome as rolled, like the first correction does.
		if !r.RolledACUnknown && (r.RolledTargetAC == nil || r.RolledHit == nil || r.RolledCrit == nil) {
			ac, hit, crit := r.TargetAC, r.Hit, r.IsCrit
			r.RolledTargetAC, r.RolledHit, r.RolledCrit = &ac, &hit, &crit
		}
		r.TargetAC, r.ACKnown, r.Verdict = 0, false, VerdictNone
		settle(r)
		res = r
		return encodeAmendment(r, change)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// decodeCorrectable decodes an attack entry that stored every die its
// outcome depends on; older entries are refused.
func decodeCorrectable(e actionlog.Entry) (*RollResult, error) {
	var rec AttackBreakdown
	if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
		return nil, fmt.Errorf("decode attack #%d: %w", e.ID, err)
	}
	if rec.Rolled == nil || rec.CritRow == nil || rec.D100MultiplierRoll == 0 {
		return nil, fmt.Errorf("%w: attack #%d was logged without its damage dice, undo it and roll again", domain.ErrInvalidInput, e.ID)
	}
	return rec.result(e.ID), nil
}

func encodeAmendment(r *RollResult, c actionlog.Change) (actionlog.PayloadAmendment, error) {
	payload, err := json.Marshal(toBreakdown(r))
	if err != nil {
		return actionlog.PayloadAmendment{}, fmt.Errorf("encode attack payload: %w", err)
	}
	return actionlog.PayloadAmendment{Payload: payload, Change: c}, nil
}

// The fields a correction step names (actionlog.CorrectionStep.Field).
const (
	// ChangeTargetAC is a target AC correction: from the AC before (null
	// when it was unknown) to the AC given.
	ChangeTargetAC = "targetAc"
	// ChangeVerdict is a GM verdict: from the verdict before (null: none)
	// to "hit" or "miss".
	ChangeVerdict = "verdict"
	// ChangeSaveAdjustment is a save adjustment: from the adjustment
	// before to the new one (0: none).
	ChangeSaveAdjustment = "saveAdjustment"
)

// knownAC is the attack's target AC, nil while it is unknown.
func knownAC(r *RollResult) any {
	if !r.ACKnown {
		return nil
	}
	return r.TargetAC
}

// verdictValue is the verdict as a correction step encodes it, nil for none.
func verdictValue(v Verdict) any {
	if v == VerdictNone {
		return nil
	}
	return string(v)
}
