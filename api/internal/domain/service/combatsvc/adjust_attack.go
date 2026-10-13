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
// entryID) after the roll, when the DM gives or changes it. Hit, margin and
// crit are recomputed with the same d20 and crit d100 against the crit row
// the d100 fell in when rolling, and the damage from the dice rolled with the
// attack: no die is rolled and no ammunition is spent. The change is an
// audited amend on the entry; voiding the entry still gives the ammunition
// back. Refused for an entry that is not an attack, is voided or is not the
// latest roll, and for an attack logged before corrections existed.
func (s *service) AdjustAttackAC(ctx context.Context, characterID, entryID int64, targetAC int) (*RollResult, error) {
	if targetAC < -MaxTargetAC || targetAC > MaxTargetAC {
		return nil, fmt.Errorf("%w: target AC must be between -%d and %d", domain.ErrInvalidInput, MaxTargetAC, MaxTargetAC)
	}
	var res *RollResult
	_, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindAttack, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		var rec attackRecord
		if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("decode attack #%d: %w", e.ID, err)
		}
		if rec.Rolled == nil || rec.CritRow == nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("%w: attack #%d was logged without its damage dice, void it and roll again", domain.ErrInvalidInput, e.ID)
		}
		r := rec.result(e.ID)
		prevAC, prevHit, prevCrit := r.TargetAC, r.Hit, r.IsCrit
		r.TargetAC = targetAC
		settle(r)
		r.PreviousTargetAC, r.PreviousHit, r.PreviousCrit = &prevAC, &prevHit, &prevCrit
		payload, err := json.Marshal(toRecord(r))
		if err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("encode attack payload: %w", err)
		}
		res = r
		return actionlog.PayloadAmendment{Summary: summary(r), Payload: payload}, nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
