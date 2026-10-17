package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// AbilityDie is the die of an ability check (ADR 0028 step 1).
const AbilityDie = 20

// MaxAbilityAdjustment is the largest adjustment, either sign, of an ability
// check (ADR 0028 step 8).
const MaxAbilityAdjustment = 20

// ChangeAbilityAdjustment names an adjustment of an ability check in its
// correction step (actionlog.CorrectionStep.Field): from the adjustment in
// effect to the new one (ADR 0028 step 7).
const ChangeAbilityAdjustment = "abilityAdjustment"

// AbilityCheckResult is one ability check with the adjustment in effect.
// AbilityScore is the sheet's score, Target the score in effect when it was
// rolled, AdjustedTarget = Target + Adjustment the one the roll is judged
// against; Success is the verdict in effect, RolledSuccess the verdict as
// rolled (adjustment 0).
type AbilityCheckResult struct {
	Ability          Ability
	Die              int
	Roll             int
	AbilityScore     int
	Target           int
	Adjustment       int
	AdjustmentReason string
	AdjustedTarget   int
	Success          bool
	RolledSuccess    bool
	LogEntryID       int64
}

// abilityRecord is the payload of an ability_check entry (ADR 0028 step 5);
// an adjustment amend carries the recomputed record. Entries in the log must
// stay decodable: add fields, never rename them.
type abilityRecord struct {
	Ability          Ability `json:"ability"`
	Die              int     `json:"die"`
	Roll             int     `json:"roll"`
	AbilityScore     int     `json:"abilityScore"`
	Target           int     `json:"target"`
	Adjustment       int     `json:"adjustment"`
	AdjustmentReason string  `json:"adjustmentReason,omitempty"`
	AdjustedTarget   int     `json:"adjustedTarget"`
	Success          bool    `json:"success"`
	RolledSuccess    bool    `json:"rolledSuccess"`
	// PreviousSuccess and PreviousAdjustment are the verdict and adjustment
	// before the latest adjustment; set on adjustment amends only (step 7).
	PreviousSuccess    *bool `json:"previousSuccess,omitempty"`
	PreviousAdjustment *int  `json:"previousAdjustment,omitempty"`
}

// AbilityCheckSuccess is the verdict of an ability check: the roll at or
// under the adjusted target (ADR 0028 step 3, plain arithmetic: a target at
// or under 0 always fails), a natural 20 always failing (step 4). It is the
// proficiency check's rule with no proficiency modifier.
func AbilityCheckSuccess(roll, adjustedTarget int) bool {
	return ProficiencySuccess(roll, adjustedTarget)
}

// settle moves the target by the adjustment, + easier, - harder, and judges
// the stored roll against it (ADR 0028 steps 3, 4 and 6): the roll never
// changes.
func (r *abilityRecord) settle() {
	r.AdjustedTarget = AdjustedTarget(r.Target, r.Adjustment)
	r.Success = AbilityCheckSuccess(r.Roll, r.AdjustedTarget)
}

func (r abilityRecord) result(entryID int64) *AbilityCheckResult {
	return &AbilityCheckResult{
		Ability: r.Ability, Die: r.Die, Roll: r.Roll, AbilityScore: r.AbilityScore, Target: r.Target,
		Adjustment: r.Adjustment, AdjustmentReason: r.AdjustmentReason, AdjustedTarget: r.AdjustedTarget,
		Success: r.Success, RolledSuccess: r.RolledSuccess, LogEntryID: entryID,
	}
}

// parseAbility is the ability key of the URL, refused unless it is one of
// the six (ADR 0028 step 1).
func parseAbility(key string) (Ability, error) {
	for _, a := range charactersvc.Abilities {
		if string(a) == key {
			return a, nil
		}
	}
	return "", fmt.Errorf("%w: unknown ability %q", domain.ErrInvalidInput, key)
}

// RollAbility rolls one d20 through the dice port (ADR 0028 step 1) against
// the score in effect (step 2) and records the check as one ability_check
// entry with no snapshots and no summary, in the check's transaction (step
// 5). An unknown ability is refused before anything is read, rolled or
// logged.
func (s *service) RollAbility(ctx context.Context, characterID int64, ability string) (*AbilityCheckResult, error) {
	a, err := parseAbility(ability)
	if err != nil {
		return nil, err
	}
	var res *AbilityCheckResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		score, err := s.abils.GetAbilityScore(ctx, characterID, a)
		if err != nil {
			return fmt.Errorf("get ability score: %w", err)
		}
		rec := abilityRecord{
			Ability: a, Die: AbilityDie, AbilityScore: score.Base, Target: score.Effective,
			Roll: s.dice.RollDie(AbilityDie),
		}
		rec.settle()
		rec.RolledSuccess = rec.Success
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode ability check payload: %w", err)
		}
		entryID, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindAbilityCheck,
			Payload:     payload,
		})
		if err != nil {
			return fmt.Errorf("record ability check: %w", err)
		}
		res = rec.result(entryID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// AdjustAbility sets the adjustment of the latest ability check entryID
// (ADR 0028 step 6): it replaces the one in effect, 0 removes it and drops
// the reason, and the verdict is recomputed from the stored roll and target,
// the verdict as rolled kept. An adjustment out of range or a too long reason
// is refused before the log is asked (step 8). The change is an amend
// correction, its own undoable step (step 7, ADR 0010 step 6); AmendLatest
// refuses an entry that is not an ability check, is voided or is not the
// latest roll.
func (s *service) AdjustAbility(ctx context.Context, characterID, entryID int64, adj Adjustment) (*AbilityCheckResult, error) {
	adj, err := normalize(adj)
	if err != nil {
		return nil, err
	}
	if adj.Value < -MaxAbilityAdjustment || adj.Value > MaxAbilityAdjustment {
		return nil, fmt.Errorf("%w: adjustment must be between -%d and +%d", domain.ErrInvalidInput, MaxAbilityAdjustment, MaxAbilityAdjustment)
	}
	var rec abilityRecord
	e, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindAbilityCheck, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("decode ability check #%d: %w", e.ID, err)
		}
		prevSuccess, prevAdj := rec.Success, rec.Adjustment
		rec.Adjustment, rec.AdjustmentReason = adj.Value, adj.Reason
		rec.settle()
		rec.PreviousSuccess, rec.PreviousAdjustment = &prevSuccess, &prevAdj
		payload, err := json.Marshal(rec)
		if err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("encode ability check payload: %w", err)
		}
		return actionlog.PayloadAmendment{Payload: payload, Change: actionlog.Change{
			Field: ChangeAbilityAdjustment, From: prevAdj, To: rec.Adjustment,
		}}, nil
	})
	if err != nil {
		return nil, err
	}
	return rec.result(e.ID), nil
}
