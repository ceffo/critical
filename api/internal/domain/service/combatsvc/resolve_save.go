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
	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
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

// SaveOptions are the save request's flags, set by the player before the
// roll.
type SaveOptions struct {
	// MindAffecting marks a save against magic that attacks the mind (ADR
	// 0006 step 9.3, the DM's call): the roll gets the Wisdom modifier.
	MindAffecting bool
	// SpellID names the spell the save is against (optional, a catalogue
	// id): one the character is immune to rolls nothing (ADR 0006 step
	// 10.2).
	SpellID string
}

// Immunity says why a save or a spell received rolled nothing (ADR 0006
// step 10): Source is the rule ("wisdom": PHB Table 5's spell immunity),
// Score the ability score that gives it. The web words it.
type Immunity struct {
	Source string `json:"source"`
	Score  int    `json:"score"`
}

// ImmunitySourceWisdom is PHB Table 5's spell immunity (ADR 0006 step 10).
const ImmunitySourceWisdom = "wisdom"

// ImmunityTo is ADR 0006 step 10.1: the immunity of char to the spell with
// catalogue id spellID (nil: none, or no spell named). It reads the derived
// immunities, never a spell id in the code.
func ImmunityTo(char charactersvc.View, spellID string) *Immunity {
	if spellID == "" || !slices.Contains(char.SpellImmunities, spellID) {
		return nil
	}
	return &Immunity{Source: ImmunitySourceWisdom, Score: char.Wis}
}

// ResolveSavingThrow rolls a D20 against the character's per-category target,
// applying the save's terms the saves owner builds from the view's figure
// and the request's flags (ADR 0046 step 3.3: the race's bonus, the Wisdom
// adjustment on a mind-affecting save, the class save_bonus grants) and the
// buffs' SaveBonus modifiers, and records the save in the action log (kind
// save, no snapshots: it changes no state). A natural 1 or 20 is plain
// arithmetic.
func (s *service) ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory, opts SaveOptions) (*SaveBreakdown, error) {
	if err := saves.Check(string(category)); err != nil {
		return nil, err
	}
	var res *SaveBreakdown
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		char, err := s.chars.View(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character view: %w", err)
		}
		// The target is the derived one (ADR 0030 step 6, amending ADR 0006
		// step 1): PHB Table 60 for the class group and level.
		figure := char.SaveInputs()
		target, err := saves.Target(figure, string(category))
		if err != nil {
			return err
		}

		rec := SaveBreakdown{Category: string(category), Target: target, Modifiers: []BreakdownModifier{}, MindAffecting: opts.MindAffecting, SpellID: opts.SpellID}
		// ADR 0006 step 10.2: against a spell the character is immune to,
		// nothing is rolled; the save passes and the breakdown names why.
		// The adjusted target is the target (nothing adjusts it); the roll
		// side (RollTotal, Total, Margin) stays 0: there is no roll.
		if rec.Immunity = ImmunityTo(char, opts.SpellID); rec.Immunity != nil {
			rec.Passed, rec.RolledPassed = true, true
			rec.AdjustedTarget = rec.Target
			return s.recordSave(ctx, characterID, rec, &res)
		}
		rec.D20Roll = s.dice.RollD20()
		mods, err := s.buffs.Aggregate(ctx, characterID, buffsvc.BaseStrengthOf(char), buffsvc.WeaponNone)
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		// The owner's terms come first, in its order; each is one named
		// modifier on the roll, never the target.
		for _, t := range saves.Terms(figure, string(category), saves.Flags{MindAffecting: opts.MindAffecting}) {
			rec.Modifiers = append(rec.Modifiers, BreakdownModifier{Kind: t.Code, Detail: t.Source, Value: t.Value})
		}
		// One modifier per buff, named like attack buff modifiers (Detail =
		// buff name), so the card labels each one.
		for _, c := range mods.SaveBonusFrom {
			rec.Modifiers = append(rec.Modifiers, BreakdownModifier{Kind: string(ModBuff), Detail: c.Buff, ItemID: c.ItemID, Value: c.Value})
		}
		rec.settle()
		rec.RolledPassed = rec.Passed
		return s.recordSave(ctx, characterID, rec, &res)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// recordSave logs rec (kind save, no snapshots) and sets *res to its answer.
func (s *service) recordSave(ctx context.Context, characterID int64, rec SaveBreakdown, res **SaveBreakdown) error {
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
	*res = rec.answer(id)
	return nil
}

// AdjustSave sets the adjustment of the latest save entry (replacing any
// earlier one, 0 removes it) and recomputes its adjusted target, verdict and
// margin from the stored d20, modifiers and target (ADR 0006 step 5). The change is an audited amend on the
// entry. Refused for an entry that is not a save, is voided or is not the
// newest save.
func (s *service) AdjustSave(ctx context.Context, characterID, entryID int64, adj SaveAdjustment) (*SaveBreakdown, error) {
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
	var rec SaveBreakdown
	e, err := s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindSave, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
		if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("decode save #%d: %w", e.ID, err)
		}
		if rec.Immunity != nil {
			return actionlog.PayloadAmendment{}, fmt.Errorf("save #%d: %w", e.ID, ErrSaveImmune)
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
	return rec.answer(e.ID), nil
}

// SaveBreakdown is the one wire shape of a saving throw: the payload of its
// action-log entry (an adjustment amend carries the recomputed one) and the
// answer of /api/roll/save and its adjust route are this struct. Entries
// already in the log must stay decodable: add fields, never rename them (ADR
// 0010, ADR 0018).
// Saves logged before the UI named the category also carry an English
// "label", no longer read.
type SaveBreakdown struct {
	Category string `json:"category"`
	D20Roll  int    `json:"d20Roll"`
	// Modifiers are the ones applied when rolling (the race's bonus, the
	// Wisdom adjustment, buffs), not the adjustment.
	Modifiers []BreakdownModifier `json:"modifiers"`
	// MindAffecting is the request's flag (ADR 0006 step 9.5), fixed at the
	// roll; an entry logged without it reads as false.
	MindAffecting bool `json:"mindAffecting"`
	// SpellID is the spell the save was against (ADR 0006 step 10.2; empty:
	// none named).
	SpellID string `json:"spellId,omitempty"`
	// Immunity is set when the character was immune to that spell (step
	// 10.2): no d20 (0), no modifiers, passed, nothing to adjust;
	// AdjustedTarget is Target and RollTotal, Total and Margin are 0.
	Immunity *Immunity `json:"immunity,omitempty"`
	// Target is the category's derived target (ADR 0030 step 6), never adjusted.
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
	// LogEntryID is the entry recording the save: set on the endpoints'
	// answer, never in the payload (the entry carries its own id). Last, so
	// an answer is its entry's payload plus this one key.
	LogEntryID int64 `json:"logEntryId,omitempty"`
}

// settle computes the roll total, the adjusted target, the verdict and the
// margin: the adjustment moves the target (ADR 0006 step 5), and the save
// passes when d20 + modifiers >= target - adjustment, a natural 1 or 20
// included (steps 2 and 3).
func (r *SaveBreakdown) settle() {
	r.RollTotal = r.D20Roll + r.modTotal()
	r.AdjustedTarget = r.Target - r.Adjustment
	r.Passed = r.RollTotal >= r.AdjustedTarget
	r.Margin = r.RollTotal - r.AdjustedTarget
	r.Total = r.RollTotal + r.Adjustment
}

func (r *SaveBreakdown) modTotal() int {
	sum := 0
	for _, m := range r.Modifiers {
		sum += m.Value
	}
	return sum
}

// answer is the save as its endpoints return it: the payload plus the id of
// its entry. Every field is settled (an old entry's roll total and adjusted
// target included, see SavePayloadView), so the answer needs no view.
func (r SaveBreakdown) answer(entryID int64) *SaveBreakdown {
	r.LogEntryID = entryID
	return &r
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
	var rec SaveBreakdown
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
