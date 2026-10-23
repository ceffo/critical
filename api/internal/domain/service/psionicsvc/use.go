package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Refusals of a power's use (ADR 0043 steps 10 and 11), all before the roll:
// nothing is rolled or written.
var (
	// ErrPowerNotKnown: the power is not one the character knows (step 9),
	// or no power of the catalogue.
	ErrPowerNotKnown = errors.New("the character does not know that power")
	// ErrNeedsContact: the power's maintenance is paid on contact (no power
	// of the catalogue has one; a power whose initial cost is contact is ADR
	// 0044 step 10's).
	ErrNeedsContact = errors.New("the power needs contact")
	// ErrNotEnoughPSP: the current PSPs are under the power's full initial
	// cost (step 11, Q6).
	ErrNotEnoughPSP = errors.New("not enough PSPs for the power's initial cost")
	// ErrPowerNotMaintained: Stop of a power no buff row keeps up (step 16).
	ErrPowerNotMaintained = errors.New("the power is not maintained")
	// ErrRecoveryWhileMaintaining: a PSP recovery while a power is
	// maintained (step 6: the book recovers only in hours with no PSP spent).
	ErrRecoveryWhileMaintaining = errors.New("no PSP recovery while a power is maintained")
)

// WarnSecondPowerThisRound is the warning of a power initiated in a combat
// round that already had one (step 12). A warning, never a refusal.
const WarnSecondPowerThisRound = "second_power_this_round"

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
// MaintenanceCost is the maintenance of a power whose maintenance cost is
// `variable`, and only then (step 13). Target is the mind a power whose
// cost is contact is used on (ADR 0044 step 10), and only then.
type UseRequest struct {
	Modifiers       []Modifier
	Cost            *int
	Score           *int
	MaintenanceCost *int
	Target          string
}

// UseResult is a power's check as logged, with the pool after it. BuffID is
// the row a successful check of a power with maintenance put on or renewed
// (step 13), "" otherwise.
type UseResult struct {
	Check      CheckRecord
	PSP        actionlog.PSPState
	LogEntryID int64
	BuffID     string
}

// CheckRecord is the power_check entry's payload: the check's breakdown
// (step 10), its outcome and the PSPs it spent (step 11). The UI words it.
// Entries already in the log must stay decodable: add fields, never rename.
type CheckRecord struct {
	Power string `json:"power"`
	// Name is the power's catalogue name at write time, in every language
	// the catalogue has (ADR 0023: names recorded in payloads stay as
	// written), so the journal words it with no catalogue read.
	Name localized.Text `json:"name,omitempty"`
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
	// MaintenanceCost and MaintenancePeriod are the maintenance of a power
	// that has one (step 13): a success keeps it up at that cost.
	MaintenanceCost   int `json:"maintenanceCost,omitempty"`
	MaintenancePeriod int `json:"maintenancePeriod,omitempty"`
	// Warnings are the check's warnings (step 12: WarnSecondPowerThisRound).
	Warnings []string `json:"warnings,omitempty"`
	// Opponent is the mind the check was on (ADR 0044 steps 9 and 10): the
	// contact power's target, or the contacted mind of a power whose cost is
	// contact; "" for every other power. Level is the contact power's
	// target's level (Table 29's row) and ContactModifiers its score's
	// lines (Tables 30 and 31, resisting).
	Opponent         string        `json:"opponent,omitempty"`
	Level            int           `json:"level,omitempty"`
	ContactModifiers []ContactLine `json:"contactModifiers,omitempty"`
	// Contact: the contact power's success made (or renewed) the
	// opponent's contact row (step 9).
	Contact bool `json:"contact,omitempty"`
}

// Use is ADR 0043 steps 10 and 11: a power check of a known power, one d20
// through the dice port against the power's target, the PSPs spent as the
// entry's PSP delta (all of the initial cost on a success, half rounded up
// on a failure), in one power_check entry and one transaction, so one undo
// gives the PSPs back. A success of a power with maintenance puts its buff
// row on (or renews it) in the same entry, its snapshot beside the PSPs, so
// the undo takes both back (step 13). Refused before the roll:
// ErrNotPsionicist, ErrPowerNotKnown, ErrDefenceModeIsRaised (a defence mode
// is raised, ADR 0044 step 4), ErrNoContact (a power whose cost is contact
// names a target no contact row made in an earlier round holds, ADR 0044
// step 10: its initial cost is then its plus amount, its row carries the
// target), ErrNeedsContact, a *domain.FieldError on "modifiers", "cost",
// "maintenanceCost", "score" or "target", ErrNotEnoughPSP.
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
		if p.Kind == catalogue.KindDefenceMode {
			return ErrDefenceModeIsRaised
		}
		target, err := s.contactTarget(ctx, characterID, p, req.Target)
		if err != nil {
			return err
		}
		rec, err := checkOf(c, p, req, mods)
		if err != nil {
			return err
		}
		rec.Opponent = target
		pool := charactersvc.PSPPool{}
		if c.Derived.PSP != nil {
			pool = *c.Derived.PSP
		}
		if pool.Current < rec.Cost {
			return ErrNotEnoughPSP
		}
		second, err := s.secondPowerThisRound(ctx, characterID, p)
		if err != nil {
			return err
		}
		if second {
			rec.Warnings = append(rec.Warnings, WarnSecondPowerThisRound)
		}
		rec.Roll = s.dice.RollDie(psionics.CheckDie)
		rec.Outcome = psionics.Check(rec.Target, rec.Roll)
		rec.Success = psionics.Succeeded(rec.Outcome)
		rec.Spent = psionics.Spent(rec.Outcome, rec.Cost)
		var snaps []actionlog.Snapshot
		var buffID string
		if rec.Success && rec.MaintenanceCost > 0 {
			row, snap, err := s.buffs.Add(ctx, characterID, buffsvc.BuffEffect{
				Name: rowName(p), PowerID: p.ID, PowerName: p.Name, MaintenanceCost: rec.MaintenanceCost, MaintenancePeriod: rec.MaintenancePeriod,
				Target: target,
			})
			if err != nil {
				return fmt.Errorf("maintain power: %w", err)
			}
			snaps, buffID = []actionlog.Snapshot{snap}, row.ID
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode power check payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindPowerCheck, Payload: payload, PSPDelta: -rec.Spent, Snapshots: snaps,
		})
		if err != nil {
			return fmt.Errorf("record power check: %w", err)
		}
		out = &UseResult{
			Check: rec, LogEntryID: id, BuffID: buffID,
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
	rec := CheckRecord{Power: p.ID, Name: p.Name, Relearns: psionics.Relearns(c.PsionicPicks, p.ID), Modifiers: mods}
	switch p.InitialCost.Kind {
	case catalogue.CostContact:
		// ADR 0044 step 10 (Q12): the contact paid for the rest.
		if req.Cost != nil {
			return CheckRecord{}, domain.FieldErrorf("cost", "the power's cost is contact, not entered")
		}
		rec.Cost = p.InitialCost.Plus
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
	if err := maintenanceOf(&rec, p, req); err != nil {
		return CheckRecord{}, err
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

// maintenanceOf sets the maintenance of power p on rec (step 13): the
// catalogue's, or the request's for a `variable` cost, at least 1.
func maintenanceOf(rec *CheckRecord, p catalogue.Power, req UseRequest) error {
	m := p.Maintenance
	if m == nil {
		if req.MaintenanceCost != nil {
			return domain.FieldErrorf("maintenanceCost", "the power has no maintenance")
		}
		return nil
	}
	switch m.Cost.Kind {
	case catalogue.CostContact:
		return ErrNeedsContact
	case catalogue.CostVariable:
		if req.MaintenanceCost == nil || *req.MaintenanceCost < 1 {
			return domain.FieldErrorf("maintenanceCost", "the power's maintenance is variable: give it, at least 1")
		}
		rec.MaintenanceCost = *req.MaintenanceCost
	default:
		if req.MaintenanceCost != nil {
			return domain.FieldErrorf("maintenanceCost", "the power's maintenance is %d, not entered", m.Cost.Amount)
		}
		rec.MaintenanceCost = m.Cost.Amount
	}
	rec.MaintenancePeriod = max(m.Period, 1)
	return nil
}

// rowName is the stored name of a maintained power's row: the catalogue's
// English name (the client names the row from its power id).
func rowName(p catalogue.Power) string {
	if n := p.Name.Any(); n != "" {
		return n
	}
	return p.ID
}

// Stop is ADR 0043 step 16: the maintained power's row is dismissed, which
// logs one entry and costs nothing; the undo puts it back. A defence mode's
// row (ADR 0044 step 5) is stopped the same way, its maintenance cost being 0.
func (s *service) Stop(ctx context.Context, characterID int64, powerID string) ([]buffsvc.BuffEffect, error) {
	var out []buffsvc.BuffEffect
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		if c.Derived.Powers == nil {
			return ErrNotPsionicist
		}
		rows, err := s.buffs.ListActive(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list buffs: %w", err)
		}
		i := slices.IndexFunc(rows, func(b buffsvc.BuffEffect) bool {
			return b.PowerID == powerID && (b.Maintained() || b.Defence != buffsvc.DefenceNone)
		})
		if powerID == "" || i < 0 {
			return ErrPowerNotMaintained
		}
		out, _, err = s.buffs.Dismiss(ctx, characterID, rows[i].ID, "")
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// powerCheckLookback is how many of the newest power checks the step 12
// warning reads: defence modes do not count, so it looks past a few.
const powerCheckLookback = 20

// roundMoves are the entry kinds that move the round counter (step 12).
var roundMoves = []actionlog.Kind{
	actionlog.KindRoundAdvance, actionlog.KindRoundReset, actionlog.KindRoundSet, actionlog.KindTimePassed,
}

// secondPowerThisRound is step 12: in combat (the round counter above 0) a
// power that is not a defence mode is the round's second when a power check
// of one that is not a defence mode, or an attack mode's use (ADR 0044 step
// 7.1), is logged in the same round, after the
// round's start (its Next Round, or the End combat, Set round or passing
// time that last moved the counter). Out of combat it is never.
func (s *service) secondPowerThisRound(ctx context.Context, characterID int64, p catalogue.Power) (bool, error) {
	if p.Kind == catalogue.KindDefenceMode {
		return false, nil
	}
	round, err := s.buffs.GetRound(ctx, characterID)
	if err != nil {
		return false, fmt.Errorf("get round: %w", err)
	}
	if round < 1 {
		return false, nil
	}
	since, err := s.roundStart(ctx, characterID)
	if err != nil {
		return false, err
	}
	// An attack mode's use counts (ADR 0044 step 7.1).
	attacks, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindPsionicAttack, 1)
	if err != nil {
		return false, fmt.Errorf("psionic attacks: %w", err)
	}
	if len(attacks) > 0 && attacks[0].ID > since && attacks[0].Round == round {
		return true, nil
	}
	checks, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindPowerCheck, powerCheckLookback)
	if err != nil {
		return false, fmt.Errorf("power checks: %w", err)
	}
	for _, e := range checks {
		if e.ID <= since || e.Round != round {
			return false, nil
		}
		var rec CheckRecord
		if err := json.Unmarshal(e.Payload, &rec); err != nil {
			return false, fmt.Errorf("decode power check %d: %w", e.ID, err)
		}
		if q, ok := s.book.Power(rec.Power); ok && q.Kind == catalogue.KindDefenceMode {
			continue
		}
		return true, nil
	}
	return false, nil
}

// roundStart is the id of the newest entry that moved the round counter (a
// Next Round, or End combat, a Set round or passing time): the round's
// entries come after it. 0 when none.
func (s *service) roundStart(ctx context.Context, characterID int64) (int64, error) {
	var since int64
	for _, kind := range roundMoves {
		moves, err := s.log.ActiveEntries(ctx, characterID, kind, 1)
		if err != nil {
			return 0, fmt.Errorf("round moves: %w", err)
		}
		if len(moves) > 0 {
			since = max(since, moves[0].ID)
		}
	}
	return since, nil
}
