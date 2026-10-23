package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ErrNotAnAttackMode: an attack of a power that is not an attack mode (ADR
// 0044 step 7), refused before anything is rolled or written.
var ErrNotAnAttackMode = errors.New("the power is not an attack mode")

// AttackRequest is an attack mode's use (step 7): the mode, the opponent the
// request names (step 2: nothing of it is stored but the tangent and contact
// rows) and the labelled modifiers of ADR 0043 step 10 (the mode's range
// steps among them, read from its text by the player, Q13).
type AttackRequest struct {
	Mode      string
	Opponent  psionics.Opponent
	Modifiers []Modifier
}

// AttackRecord is the psionic_attack entry's payload (step 7.5). The UI
// words it. Add fields, never rename.
type AttackRecord struct {
	Power string `json:"power"`
	// Name is the mode's catalogue name at write time, in every language
	// (ADR 0023), as CheckRecord's.
	Name localized.Text `json:"name,omitempty"`
	// Ability, AbilityScore, PowerModifier, Relearns and Modifiers are the
	// mode's score's parts, as CheckRecord's; Score their sum, before
	// Table 14's cell.
	Ability       string     `json:"ability,omitempty"`
	AbilityScore  int        `json:"abilityScore,omitempty"`
	PowerModifier int        `json:"powerModifier,omitempty"`
	Relearns      int        `json:"relearns,omitempty"`
	Modifiers     []Modifier `json:"modifiers,omitempty"`
	Score         int        `json:"score"`
	// Target and Mind are the opponent as the request named it (step 2).
	Target string `json:"target"`
	Mind   string `json:"mind"`
	// Defence is the opponent's raised defence mode, "" for none; then
	// nothing is contested and a successful check wins (Q14).
	Defence     string         `json:"defence,omitempty"`
	DefenceName localized.Text `json:"defenceName,omitempty"`
	// AttackVersusDefence is Table 14's cell for the mode against Defence,
	// added to each check's target. Absent with no defence.
	AttackVersusDefence *int `json:"attackVersusDefence,omitempty"`
	// Checks are the checks rolled: two, or one when the first made full
	// contact (Q11).
	Checks []AttackCheck `json:"checks"`
	// Cost is the mode's initial cost; Spent what the checks spent of it
	// (the entry's PSP delta, negated): half rounded up when every check
	// failed, the whole cost otherwise (Q5).
	Cost  int `json:"cost"`
	Spent int `json:"spent"`
	// TangentsBefore and TangentsAfter are the tangents the target's row
	// held (buffsvc.TangentsChange: 3 for a contact row).
	TangentsBefore int  `json:"tangentsBefore"`
	TangentsAfter  int  `json:"tangentsAfter"`
	Contact        bool `json:"contact,omitempty"`
	// Broken is the target of another mind's tangent row the attack ended
	// (step 3.3).
	Broken string `json:"broken,omitempty"`
	// Warnings are ADR 0043 step 12's (WarnSecondPowerThisRound).
	Warnings []string `json:"warnings,omitempty"`
}

// AttackCheck is one of the attack's checks and its contest (step 7.2).
type AttackCheck struct {
	// Check is the mode's check: its target is the score plus Table 14's
	// cell.
	Check psionics.ContestCheck `json:"check"`
	// Defence is the DM's roll for the opponent's defence; absent with none.
	Defence *psionics.ContestSide `json:"defence,omitempty"`
	Winner  string                `json:"winner"`
	// Effect: the contest was won against an open or contacted mind: the
	// mode's effect, the DM's to apply (step 7.4), no tangent.
	Effect bool `json:"effect,omitempty"`
}

// AttackResult is the attack as logged, with the pool after it.
type AttackResult struct {
	Attack     AttackRecord
	PSP        actionlog.PSPState
	LogEntryID int64
}

// Attack is ADR 0044 step 7: a known attack mode's one cost buys two d20
// checks through the dice port, each against the mode's score plus Table
// 14's cell for the opponent's defence plus the modifiers, each decided by
// the contest (step 6) against the DM's roll for that defence, or won by a
// success with no defence. Against a closed mind each won contest is a
// tangent; the third makes contact and the second check is not rolled. Against an
// open or contacted mind a won contest is the mode's effect. One
// psionic_attack entry carries the PSPs spent and the snapshot of
// buffsvc.AddTangents (which also ends another mind's tangent row), so one
// undo gives both back. Refused before the roll: a *domain.FieldError on
// the opponent's fields or "modifiers", ErrNotPsionicist, ErrPowerNotKnown,
// ErrNotAnAttackMode, ErrNeedsContact, ErrNotEnoughPSP.
func (s *service) Attack(ctx context.Context, characterID int64, req AttackRequest) (*AttackResult, error) {
	opp := req.Opponent
	if err := opp.Validate(s.book); err != nil {
		return nil, err
	}
	mods, err := modifiers(req.Modifiers)
	if err != nil {
		return nil, err
	}
	var out *AttackResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		if c.Derived.Powers == nil {
			return ErrNotPsionicist
		}
		p, ok := s.book.Power(req.Mode)
		if !ok || !psionics.Knows(c.PsionicPicks, req.Mode) {
			return ErrPowerNotKnown
		}
		if p.Kind != catalogue.KindAttackMode {
			return ErrNotAnAttackMode
		}
		check, err := checkOf(c, p, UseRequest{Modifiers: mods}, mods)
		if err != nil {
			return err
		}
		rec := AttackRecord{
			Power: p.ID, Name: p.Name, Ability: check.Ability, AbilityScore: check.AbilityScore,
			PowerModifier: check.PowerModifier, Relearns: check.Relearns, Modifiers: mods,
			Score: check.Target, Cost: check.Cost, Target: opp.Target, Mind: opp.Mind,
		}
		pool := charactersvc.PSPPool{}
		if c.Derived.PSP != nil {
			pool = *c.Derived.PSP
		}
		if pool.Current < rec.Cost {
			return ErrNotEnoughPSP
		}
		if err := s.attackVersus(&rec, opp.Defence); err != nil {
			return err
		}
		second, err := s.secondPowerThisRound(ctx, characterID, p)
		if err != nil {
			return err
		}
		if second {
			rec.Warnings = append(rec.Warnings, WarnSecondPowerThisRound)
		}
		rows, err := s.buffs.ListActive(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list buffs: %w", err)
		}
		won := s.rollChecks(&rec, opp, buffsvc.TangentsOn(rows, opp.Target))
		ch, snap, err := s.buffs.AddTangents(ctx, characterID, opp.Target, won)
		if err != nil {
			return fmt.Errorf("add tangents: %w", err)
		}
		rec.TangentsBefore, rec.TangentsAfter, rec.Contact = ch.Before, ch.After, ch.Contact
		if ch.Broken != nil {
			rec.Broken = ch.Broken.Target
		}
		var snaps []actionlog.Snapshot
		if snap.Key != "" {
			snaps = []actionlog.Snapshot{snap}
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode psionic attack payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindPsionicAttack, Payload: payload, PSPDelta: -rec.Spent, Snapshots: snaps,
		})
		if err != nil {
			return fmt.Errorf("record psionic attack: %w", err)
		}
		out = &AttackResult{Attack: rec, LogEntryID: id, PSP: actionlog.PSPState{Current: pool.Current - rec.Spent, Max: pool.Max}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// attackVersus sets Table 14's cell for rec's mode against defence on rec
// (step 7.1); none with no defence (Q14).
func (s *service) attackVersus(rec *AttackRecord, defence string) error {
	if defence == "" {
		return nil
	}
	d, ok := s.book.Power(defence)
	if !ok {
		return fmt.Errorf("defence %q is not in the catalogue", defence)
	}
	m, err := s.rates.AttackVersusDefence(rec.Power, defence)
	if err != nil {
		return fmt.Errorf("table 14: %w", err)
	}
	rec.Defence, rec.DefenceName, rec.AttackVersusDefence = defence, d.Name, &m
	return nil
}

// rollChecks rolls rec's checks against opp, the target's row holding held
// tangents before (step 7.2 to 7.4), and sets what they spent (Q5). It
// returns the tangents won: a won contest against a closed mind not yet
// contacted. The check that makes full contact is the last rolled (Q11).
func (s *service) rollChecks(rec *AttackRecord, opp psionics.Opponent, held int) int {
	target := rec.Score
	if rec.AttackVersusDefence != nil {
		target += *rec.AttackVersusDefence
	}
	won, succeeded := 0, false
	for i := range psionics.AttackChecks {
		ac := AttackCheck{Check: psionics.CheckContest(target, s.dice.RollDie(psionics.CheckDie), false)}
		var defence psionics.ContestSide // no defence: a success wins (Q14)
		if opp.Defence != "" {
			d := opp.Rolls[i]
			ac.Defence, defence = &d, d
		}
		ac.Winner = psionics.Contest(ac.Check.Side(), defence)
		succeeded = succeeded || psionics.Succeeded(ac.Check.Outcome)
		contact := false
		if ac.Winner == psionics.SideAttacker {
			if opp.Mind == psionics.MindClosed && held <= buffsvc.MaxTangents {
				held++
				won++
				contact = held > buffsvc.MaxTangents
			} else {
				ac.Effect = true
			}
		}
		rec.Checks = append(rec.Checks, ac)
		if contact {
			break
		}
	}
	rec.Spent = rec.Cost
	if !succeeded {
		rec.Spent = psionics.Spent(psionics.OutcomeFailure, rec.Cost)
	}
	return won
}
