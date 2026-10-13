package combatsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Adjustment limits: an adjustment is a bonus learnt after the roll (+4 vs
// fire), so anything beyond a d20's range is a typo.
const (
	MaxSaveAdjustment      = 20
	MaxAdjustmentReasonLen = 80
)

// SaveAdjustment is a signed bonus or penalty applied to the latest save
// after the roll, with an optional reason ("Gaia fire"). It changes the
// verdict and the margin, never the d20.
type SaveAdjustment struct {
	Value  int
	Reason string
}

var saveLabels = map[SaveCategory]string{
	SaveParaPoison:   "Paralyze/Poison",
	SaveRodStaffWand: "Rod/Staff/Wand",
	SavePetriPoly:    "Petrify/Polymorph",
	SaveBreath:       "Breath Weapon",
	SaveSpell:        "Spells",
}

// ResolveSavingThrow rolls a D20 against the character's per-category target,
// applying buff SaveBonus modifiers, and records the save in the action log
// (kind save, no snapshots: it changes no state). A natural 1 or 20 is plain
// arithmetic.
func (s *service) ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory) (*RollResult, error) {
	label, ok := saveLabels[category]
	if !ok {
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

		rec := saveRecord{Category: string(category), Label: label, Target: target, Modifiers: []modifierRecord{}}
		rec.D20Roll = s.dice.RollD20()
		mods, err := s.buffs.Aggregate(ctx, characterID, char.Str)
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		if mods.SaveBonus != 0 {
			rec.Modifiers = append(rec.Modifiers, modifierRecord{Kind: string(ModBuff), Source: "Buffs", Value: mods.SaveBonus})
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
			Summary:     rec.summary(),
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
// earlier one, 0 removes it) and recomputes its verdict and margin from the
// stored d20, modifiers and target. The change is an audited amend on the
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
		return actionlog.PayloadAmendment{Summary: rec.summary(), Payload: payload}, nil
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
type saveRecord struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	D20Roll  int    `json:"d20Roll"`
	// Modifiers are the ones applied when rolling (buffs), not the adjustment.
	Modifiers []modifierRecord `json:"modifiers"`
	Target    int              `json:"target"`
	// RolledPassed is the verdict as rolled, before any adjustment.
	RolledPassed     bool   `json:"rolledPassed"`
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	Total            int    `json:"total"`
	Passed           bool   `json:"passed"`
	Margin           int    `json:"margin"`
	// PreviousPassed and PreviousAdjustment are the verdict and adjustment
	// before the latest adjustment; set on adjustment amends only.
	PreviousPassed     *bool `json:"previousPassed,omitempty"`
	PreviousAdjustment *int  `json:"previousAdjustment,omitempty"`
}

// settle computes total, verdict and margin: the save passes when
// d20 + modifiers + adjustment >= target.
func (r *saveRecord) settle() {
	r.Total = r.D20Roll + r.modTotal() + r.Adjustment
	r.Passed = r.Total >= r.Target
	r.Margin = r.Total - r.Target
}

func (r *saveRecord) modTotal() int {
	sum := 0
	for _, m := range r.Modifiers {
		sum += m.Value
	}
	return sum
}

func verdict(passed bool) string {
	if passed {
		return "PASSED"
	}
	return "FAILED"
}

// summary is the English log line; the UI renders the payload instead.
func (r *saveRecord) summary() string {
	head := fmt.Sprintf("Save vs %s: d20 %d vs %d", r.Label, r.D20Roll, r.Target)
	if r.PreviousPassed == nil {
		return fmt.Sprintf("%s, %s", head, verdict(r.Passed))
	}
	adj := fmt.Sprintf("adjusted %+d", r.Adjustment)
	if r.Adjustment == 0 {
		adj = "adjustment removed"
	}
	if r.AdjustmentReason != "" {
		adj += " " + r.AdjustmentReason
	}
	return fmt.Sprintf("%s, %s: %s → %s", head, adj, verdict(*r.PreviousPassed), verdict(r.Passed))
}

func (r *saveRecord) breakdown() string {
	mod := r.modTotal()
	if r.Adjustment == 0 {
		return fmt.Sprintf("Roll: %d + Mod: %d = %d vs Target: %d (%s)", r.D20Roll, mod, r.Total, r.Target, r.Label)
	}
	return fmt.Sprintf("Roll: %d + Mod: %d + Adj: %+d = %d vs Target: %d (%s)",
		r.D20Roll, mod, r.Adjustment, r.Total, r.Target, r.Label)
}

// result is the RollResult of the save: the target is BaseThac0 and
// EffectiveThac0, the verdict Hit, the margin HitMargin; the adjustment is
// listed among the modifiers.
func (r *saveRecord) result(entryID int64) *RollResult {
	mods := make([]Modifier, 0, len(r.Modifiers)+1)
	for _, m := range r.Modifiers {
		mods = append(mods, Modifier{Kind: ModifierKind(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value})
	}
	if r.Adjustment != 0 {
		mods = append(mods, Modifier{Kind: ModAdjustment, Source: "Adjustment", Detail: r.AdjustmentReason, Value: r.Adjustment})
	}
	return &RollResult{
		D20Roll: r.D20Roll, BaseThac0: r.Target, EffectiveThac0: r.Target, Modifiers: mods,
		Hit: r.Passed, HitMargin: r.Margin, DamageBreakdown: r.breakdown(),
		SaveCategory: SaveCategory(r.Category), Adjustment: r.Adjustment, AdjustmentReason: r.AdjustmentReason,
		LogEntryID: entryID,
	}
}
