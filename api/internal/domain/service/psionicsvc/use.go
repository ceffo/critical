package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Refusals of a power's use (ADR 0043 steps 10 and 11), all before the roll:
// nothing is rolled or written.
var (
	// ErrPowerNotKnown: the power is not one the character knows (step 9),
	// or no power of the catalogue.
	ErrPowerNotKnown = errors.New("the character does not know that power")
	// ErrNeedsContact: the power's cost is paid on contact, telepathic
	// combat's (layer 2, step 21).
	ErrNeedsContact = errors.New("the power needs contact")
	// ErrNotEnoughPSP: the current PSPs are under the power's full initial
	// cost (step 11, Q6).
	ErrNotEnoughPSP = errors.New("not enough PSPs for the power's initial cost")
)

// Bounds of a power use's request.
const (
	// MaxModifiers is the most labelled modifiers on one check.
	MaxModifiers = 10
	// MaxModifierReasonLen is a modifier's reason's longest, in characters
	// (ADR 0014's adjustment reason).
	MaxModifierReasonLen = 80
	// MaxEnteredScore is the highest score entered for a power whose score is
	// a formula in its text (the highest ability score, 25).
	MaxEnteredScore = 25
)

// Modifier is a labelled modifier on a power check (step 10): a value the
// player gives before the roll, + easier, - harder, with the power's or the
// DM's reason.
type Modifier struct {
	Value  int    `json:"value"`
	Reason string `json:"reason,omitempty"`
}

// UseRequest is what the player gives with a power's use. Cost is the
// initial cost of a power whose cost is `variable` (the power's text or
// table gives it), and only then. Score is the power score of a power whose
// score is a formula in its text (the catalogue has none), and only then.
type UseRequest struct {
	Modifiers []Modifier
	Cost      *int
	Score     *int
}

// UseResult is a power's check as logged, with the pool after it.
type UseResult struct {
	Check      CheckRecord
	PSP        actionlog.PSPState
	LogEntryID int64
}

// CheckRecord is the power_check entry's payload: the check's breakdown
// (step 10), its outcome and the PSPs it spent (step 11). The UI words it.
// Entries already in the log must stay decodable: add fields, never rename.
type CheckRecord struct {
	Power string `json:"power"`
	// Ability, AbilityScore and PowerModifier are the catalogue score's
	// parts; EnteredScore replaces them for a power with no score.
	Ability       string `json:"ability,omitempty"`
	AbilityScore  int    `json:"abilityScore,omitempty"`
	PowerModifier int    `json:"powerModifier,omitempty"`
	EnteredScore  *int   `json:"enteredScore,omitempty"`
	// Relearns are +1 each (step 9).
	Relearns  int        `json:"relearns,omitempty"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
	// Target is the power score with every part and modifier (step 10).
	Target  int    `json:"target"`
	Roll    int    `json:"d20"`
	Outcome string `json:"outcome"`
	Success bool   `json:"success"`
	// Cost is the initial cost; Spent what the outcome spent of it (the
	// entry's PSP delta, negated).
	Cost  int `json:"cost"`
	Spent int `json:"spent"`
}

// Use is ADR 0043 steps 10 and 11: a power check of a known power, one d20
// through the dice port against the power's target, the PSPs spent as the
// entry's PSP delta (all of the initial cost on a success, half rounded up
// on a failure), in one power_check entry and one transaction, so one undo
// gives the PSPs back. Refused before the roll: ErrNotPsionicist,
// ErrPowerNotKnown, ErrNeedsContact, a *domain.FieldError on "modifiers",
// "cost" or "score", ErrNotEnoughPSP.
func (s *service) Use(ctx context.Context, characterID int64, powerID string, req UseRequest) (*UseResult, error) {
	mods, err := modifiers(req.Modifiers)
	if err != nil {
		return nil, err
	}
	var out *UseResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		if c.Derived.Powers == nil {
			return ErrNotPsionicist
		}
		p, ok := s.book.Power(powerID)
		if !ok || !psionics.Knows(c.PsionicPicks, powerID) {
			return ErrPowerNotKnown
		}
		rec, err := checkOf(c, p, req, mods)
		if err != nil {
			return err
		}
		pool := charactersvc.PSPPool{}
		if c.Derived.PSP != nil {
			pool = *c.Derived.PSP
		}
		if pool.Current < rec.Cost {
			return ErrNotEnoughPSP
		}
		rec.Roll = s.dice.RollDie(psionics.CheckDie)
		rec.Outcome = psionics.Check(rec.Target, rec.Roll)
		rec.Success = psionics.Succeeded(rec.Outcome)
		rec.Spent = psionics.Spent(rec.Outcome, rec.Cost)
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode power check payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindPowerCheck, Payload: payload, PSPDelta: -rec.Spent,
		})
		if err != nil {
			return fmt.Errorf("record power check: %w", err)
		}
		out = &UseResult{
			Check: rec, LogEntryID: id,
			PSP: actionlog.PSPState{Current: pool.Current - rec.Spent, Max: pool.Max},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkOf is the check's breakdown before the roll: the target (step 10) and
// the initial cost (step 11) of power p for character c.
func checkOf(c *charactersvc.Character, p catalogue.Power, req UseRequest, mods []Modifier) (CheckRecord, error) {
	rec := CheckRecord{Power: p.ID, Relearns: psionics.Relearns(c.PsionicPicks, p.ID), Modifiers: mods}
	switch p.InitialCost.Kind {
	case catalogue.CostContact:
		return CheckRecord{}, ErrNeedsContact
	case catalogue.CostVariable:
		if req.Cost == nil || *req.Cost < 1 {
			return CheckRecord{}, domain.FieldErrorf("cost", "the power's cost is variable: give it, at least 1")
		}
		rec.Cost = *req.Cost
	default:
		if req.Cost != nil {
			return CheckRecord{}, domain.FieldErrorf("cost", "the power's cost is %d, not entered", p.InitialCost.Amount)
		}
		rec.Cost = p.InitialCost.Amount
	}
	if p.Score == nil {
		if req.Score == nil || *req.Score < 1 || *req.Score > MaxEnteredScore {
			return CheckRecord{}, domain.FieldErrorf("score", "the power's score is in its text: give it, 1 to %d", MaxEnteredScore)
		}
		v := *req.Score
		rec.EnteredScore = &v
		rec.Target = v
	} else {
		if req.Score != nil {
			return CheckRecord{}, domain.FieldErrorf("score", "the power has its own score")
		}
		score, ok := c.Score(charactersvc.Ability(p.Score.Ability))
		if !ok {
			return CheckRecord{}, fmt.Errorf("power %s: unknown ability %q", p.ID, p.Score.Ability)
		}
		rec.Ability, rec.AbilityScore, rec.PowerModifier = p.Score.Ability, score, p.Score.Modifier
		rec.Target = score + p.Score.Modifier
	}
	rec.Target += rec.Relearns
	for _, m := range mods {
		rec.Target += m.Value
	}
	return rec, nil
}

// modifiers checks the labelled modifiers (step 10: -20 to +20 each, as ADR
// 0014 step 5): a reason trimmed and at most MaxModifierReasonLen
// characters; a 0 adds nothing and is dropped.
func modifiers(in []Modifier) ([]Modifier, error) {
	if len(in) > MaxModifiers {
		return nil, domain.FieldErrorf("modifiers", "at most %d modifiers", MaxModifiers)
	}
	var out []Modifier
	for _, m := range in {
		if m.Value < -psionics.MaxModifier || m.Value > psionics.MaxModifier {
			return nil, domain.FieldErrorf("modifiers", "a modifier must be between -%d and +%d", psionics.MaxModifier, psionics.MaxModifier)
		}
		m.Reason = strings.TrimSpace(m.Reason)
		if utf8.RuneCountInString(m.Reason) > MaxModifierReasonLen {
			return nil, domain.FieldErrorf("modifiers", "a reason must be at most %d characters", MaxModifierReasonLen)
		}
		if m.Value != 0 {
			out = append(out, m)
		}
	}
	return out, nil
}
