package combatsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// Adjustment limits: an adjustment is a ruling learnt after the roll (+4 vs
// fire), so anything beyond a d20's range is a typo.
const (
	MaxSaveAdjustment      = 20
	MaxAdjustmentReasonLen = 80
)

// SaveAdjustment is a signed adjustment set on the latest save after the
// roll, with an optional reason ("Gaia fire"). It moves the target, + easier
// and - harder (ADR 0006 step 5): it changes the verdict and the margin,
// never the roll.
type SaveAdjustment struct {
	Value  int
	Reason string
}

// saveCategories are the categories a save can be rolled in; the UI names
// them.
var saveCategories = []SaveCategory{SaveParaPoison, SaveRodStaffWand, SavePetriPoly, SaveBreath, SaveSpell}

// ResolveSavingThrow rolls a D20 against the character's per-category target,
// applying buff SaveBonus modifiers, and records the save in the action log
// (kind save, no snapshots: it changes no state). A natural 1 or 20 is plain
// arithmetic.
func (s *service) ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory) (*RollResult, error) {
	if !slices.Contains(saveCategories, category) {
		return nil, fmt.Errorf("%w: unknown save category %q", domain.ErrInvalidInput, category)
	}
	var res *RollResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		target := map[SaveCategory]int{
			SaveParaPoison:   char.SaveParaPoison,
			SaveRodStaffWand: char.SaveRodStaffWand,
			SavePetriPoly:    char.SavePetriPoly,
			SaveBreath:       char.SaveBreath,
			SaveSpell:        char.SaveSpell,
		}[category]

		rec := saveRecord{Category: string(category), Target: target, Modifiers: []modifierRecord{}}
		rec.D20Roll = s.dice.RollD20()
		mods, err := s.buffs.Aggregate(ctx, characterID, char.Str, buffsvc.WeaponNone)
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		// One modifier per buff, named like attack buff modifiers (Detail =
		// buff name), so the card labels each one.
		for _, c := range mods.SaveBonusFrom {
			rec.Modifiers = append(rec.Modifiers, modifierRecord{Kind: string(ModBuff), Detail: c.Buff, ItemID: c.ItemID, Value: c.Value})
		}
		rec.settle()
		rec.RolledPassed = rec.Passed

		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode save payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindSave,
			Payload:     payload,
		})
		if err != nil {
			return fmt.Errorf("record save: %w", err)
		}
		res = rec.result(id)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// AdjustSave sets the adjustment of the latest save entry (replacing any
// earlier one, 0 removes it) and recomputes its adjusted target, verdict and
// margin from the stored d20, modifiers and target (ADR 0006 step 5). The change is an audited amend on the
// entry. Refused for an entry that is not a save, is voided or is not the
// newest save.
func (s *service) AdjustSave(ctx context.Context, characterID, entryID int64, adj SaveAdjustment) (*RollResult, error) {
	adj.Reason = strings.TrimSpace(adj.Reason)
	switch {
	case adj.Value < -MaxSaveAdjustment || adj.Value > MaxSaveAdjustment:
		return nil, fmt.Errorf("%w: adjustment must be between -%d and +%d", domain.ErrInvalidInput, MaxSaveAdjustment, MaxSaveAdjustment)
	case utf8.RuneCountInString(adj.Reason) > MaxAdjustmentReasonLen:
		return nil, fmt.Errorf("%w: reason must be at most %d characters", domain.ErrInvalidInput, MaxAdjustmentReasonLen)
	}
	if adj.Value == 0 {
		adj.Reason = "" // no adjustment, nothing to explain
	}
	var rec saveRecord
	e, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindSave, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("decode save #%d: %w", e.ID, err)
		}
		prevPassed, prevAdj := rec.Passed, rec.Adjustment
		rec.Adjustment, rec.AdjustmentReason = adj.Value, adj.Reason
		rec.settle()
		rec.PreviousPassed, rec.PreviousAdjustment = &prevPassed, &prevAdj
		payload, err := json.Marshal(rec)
		if err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("encode save payload: %w", err)
		}
		return actionlog.PayloadAmendment{Payload: payload, Change: actionlog.Change{
			Field: ChangeSaveAdjustment, From: prevAdj, To: rec.Adjustment,
		}}, nil
	})
	if err != nil {
		return nil, err
	}
	return rec.result(e.ID), nil
}

// saveRecord is combatsvc's JSON encoding of a saving throw inside its
// action-log entry (payload); an adjustment amend carries the recomputed
// record. Entries already in the log must stay decodable: add fields, never
// rename them.
// Saves logged before the UI named the category also carry an English
// "label", no longer read.
type saveRecord struct {
	Category string `json:"category"`
	D20Roll  int    `json:"d20Roll"`
	// Modifiers are the ones applied when rolling (buffs), not the adjustment.
	Modifiers []modifierRecord `json:"modifiers"`
	// Target is the category's target on the sheet, never adjusted.
	Target int `json:"target"`
	// RolledPassed is the verdict as rolled, before any adjustment.
	RolledPassed bool `json:"rolledPassed"`
	// Adjustment moves the target, + easier, - harder (ADR 0006 step 5).
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	// RollTotal is d20 + modifiers, the adjustment left out; AdjustedTarget
	// is target - adjustment. Saves logged before the adjustment moved the
	// target lack both: savePayloadView fills them on read.
	RollTotal      int `json:"rollTotal"`
	AdjustedTarget int `json:"adjustedTarget"`
	// Total is RollTotal + Adjustment, the roll side of the verdict before
	// the adjustment moved the target; kept so every entry has the same
	// fields. Margin is RollTotal - AdjustedTarget (= Total - Target).
	Total  int  `json:"total"`
	Passed bool `json:"passed"`
	Margin int  `json:"margin"`
	// PreviousPassed and PreviousAdjustment are the verdict and adjustment
	// before the latest adjustment; set on adjustment amends only.
	PreviousPassed     *bool `json:"previousPassed,omitempty"`
	PreviousAdjustment *int  `json:"previousAdjustment,omitempty"`
}

// settle computes the roll total, the adjusted target, the verdict and the
// margin: the adjustment moves the target (ADR 0006 step 5), and the save
// passes when d20 + modifiers >= target - adjustment, a natural 1 or 20
// included (steps 2 and 3).
func (r *saveRecord) settle() {
	r.RollTotal = r.D20Roll + r.modTotal()
	r.AdjustedTarget = r.Target - r.Adjustment
	r.Passed = r.RollTotal >= r.AdjustedTarget
	r.Margin = r.RollTotal - r.AdjustedTarget
	r.Total = r.RollTotal + r.Adjustment
}

func (r *saveRecord) modTotal() int {
	sum := 0
	for _, m := range r.Modifiers {
		sum += m.Value
	}
	return sum
}

// result is the RollResult of the save: the sheet's target is BaseThac0,
// the adjusted target EffectiveThac0 and SaveAdjustedTarget, the verdict Hit,
// the margin HitMargin. The modifiers are the roll's (buffs): the adjustment
// moves the target, so it is not one of them.
func (r *saveRecord) result(entryID int64) *RollResult {
	mods := make([]Modifier, 0, len(r.Modifiers))
	for _, m := range r.Modifiers {
		mods = append(mods, Modifier{Kind: ModifierKind(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value})
	}
	return &RollResult{
		D20Roll: r.D20Roll, BaseThac0: r.Target, EffectiveThac0: r.AdjustedTarget, Modifiers: mods,
		Hit: r.Passed, HitMargin: r.Margin, SaveBonus: r.modTotal(), SaveTotal: r.Total,
		SaveRollTotal: r.RollTotal, SaveAdjustedTarget: r.AdjustedTarget,
		SaveCategory: SaveCategory(r.Category), Adjustment: r.Adjustment, AdjustmentReason: r.AdjustmentReason,
		LogEntryID: entryID,
	}
}

// SavePayloadView is the actionlog.PayloadView of save entries (ADR 0006
// step 8): a save logged before the adjustment moved the target is served
// with its roll total (d20 + modifiers) and adjusted target (target -
// adjustment) computed from its stored numbers. Every other field, the
// verdict and the margin included, is served as stored.
func SavePayloadView(payload json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("decode save payload: %w", err)
	}
	_, hasRoll := fields["rollTotal"]
	_, hasTarget := fields["adjustedTarget"]
	if hasRoll && hasTarget {
		return payload, nil
	}
	var rec saveRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return nil, fmt.Errorf("decode save payload: %w", err)
	}
	for key, v := range map[string]int{"rollTotal": rec.D20Roll + rec.modTotal(), "adjustedTarget": rec.Target - rec.Adjustment} {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", key, err)
		}
		fields[key] = raw
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode save payload: %w", err)
	}
	return out, nil
}
