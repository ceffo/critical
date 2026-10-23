package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ContactCostTable is the rule table a power's initial cost names when it is
// the contact power's (ADR 0044 step 1.4).
const ContactCostTable = "contact_cost"

// Refusals of the contact power and of a power whose cost is contact (ADR
// 0044 steps 9 and 10), all before the roll: nothing is rolled or written.
var (
	// ErrNotAContactPower: Contact of a power whose cost does not name
	// ContactCostTable.
	ErrNotAContactPower = errors.New("the power's cost is not the contact table's")
	// ErrMindClosed: the contact power never works on a closed mind (step 9).
	ErrMindClosed = errors.New("the contact power does not work on a closed mind")
	// ErrLevelOutOfTable: Table 29 has no row for the target's level (Q15).
	ErrLevelOutOfTable = ruletablesvc.ErrLevelOutOfTable
	// ErrNoContact: a power whose cost is contact names a target no contact
	// row made in an earlier round holds (step 10).
	ErrNoContact = errors.New("no contact with the target from an earlier round")
	// ErrUseContactRoute: Use of a power whose cost names a rule table (the
	// contact power): its cost comes from the target, so it goes through
	// Contact, which also makes the contact row (step 9, Q15).
	ErrUseContactRoute = errors.New("the power's cost names a table: use it through Contact")
)

// ContactRequest is the contact power's use (ADR 0044 step 9): the power,
// the opponent (step 2: its level gives the cost; no defence), the target's
// distance and life order (Table 30 and 31 codes), whether the DM says the
// target resists, and the labelled modifiers of ADR 0043 step 10. Cost is
// the DM's cost for a target whose level Table 29 has no row for (Q15's
// fallback), and only then.
type ContactRequest struct {
	Power     string
	Cost      *int
	Opponent  psionics.Opponent
	Distance  string
	LifeOrder string
	Resisting bool
	Modifiers []Modifier
}

// ContactLine is one of the contact power's score modifiers (step 9), read
// from the contact_modifiers table: the web words the code.
type ContactLine struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Modifier int    `json:"modifier"`
}

// Contact is ADR 0044 step 9: the contact power's check against an open
// mind, its initial cost Table 29's row for the opponent's level, its score
// with the distance, life-order and (when the DM says so) resisting lines,
// then ADR 0043 steps 10 and 11 as Use's (half the cost on a failure). A
// success makes the target's contact row (buffsvc.MakeContact) in the
// power_check entry, its snapshot beside the PSPs, so one undo takes both
// back. Refused before the roll: a *domain.FieldError on the opponent's
// fields, "defence", "distance", "lifeOrder", "level" (not given), "cost"
// (entered for a level the table has, or under 1) or "modifiers", ErrNotPsionicist, ErrPowerNotKnown, ErrNotAContactPower,
// ErrMindClosed, ErrLevelOutOfTable, ErrNotEnoughPSP.
func (s *service) Contact(ctx context.Context, characterID int64, req ContactRequest) (*UseResult, error) {
	opp := req.Opponent
	if err := opp.Validate(s.book); err != nil {
		return nil, err
	}
	if opp.Defence != "" {
		return nil, domain.FieldErrorf("defence", "a contact names no defence")
	}
	if !slices.Contains(ruletablesvc.ContactModifierCodes[ruletablesvc.ContactDistance], req.Distance) {
		return nil, domain.FieldErrorf("distance", "unknown distance %q", req.Distance)
	}
	if !slices.Contains(ruletablesvc.ContactModifierCodes[ruletablesvc.ContactLifeOrder], req.LifeOrder) {
		return nil, domain.FieldErrorf("lifeOrder", "unknown life order %q", req.LifeOrder)
	}
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
		p, ok := s.book.Power(req.Power)
		if !ok || !psionics.Knows(c.PsionicPicks, req.Power) {
			return ErrPowerNotKnown
		}
		if p.InitialCost.Table != ContactCostTable {
			return ErrNotAContactPower
		}
		if opp.Mind == psionics.MindClosed {
			return ErrMindClosed
		}
		if opp.Level < 1 {
			return domain.FieldErrorf("level", "the contact's cost needs the target's level")
		}
		cost, err := contactCost(s.rates.ContactCost(opp.Level))(req.Cost)
		if err != nil {
			return err
		}
		rec, err := checkOf(c, p, UseRequest{Cost: &cost}, mods)
		if err != nil {
			return err
		}
		rec.Opponent, rec.Level = opp.Target, opp.Level
		if err := s.contactLines(&rec, req); err != nil {
			return err
		}
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
		if rec.Success {
			ch, snap, err := s.buffs.MakeContact(ctx, characterID, opp.Target)
			if err != nil {
				return fmt.Errorf("make contact: %w", err)
			}
			if ch.Target != "" {
				rec.Opponent = ch.Target // an existing row's label, whatever the case typed
			}
			if snap.Key != "" {
				snaps = []actionlog.Snapshot{snap}
			}
			rec.Contact, buffID = true, ch.Row.ID
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

// contactCost is the contact's initial cost from Table 29's answer (cost,
// err) and the request's entered cost: the table's when it has the level's
// row (no cost entered then), the entered one, at least 1, when it has not
// (Q15: the DM gives the cost instead); ErrLevelOutOfTable when neither.
func contactCost(cost int, err error) func(entered *int) (int, error) {
	return func(entered *int) (int, error) {
		switch {
		case errors.Is(err, ErrLevelOutOfTable) && entered != nil:
			if *entered < 1 {
				return 0, domain.FieldErrorf("cost", "the contact's cost must be at least 1")
			}
			return *entered, nil
		case err != nil:
			return 0, err
		case entered != nil:
			return 0, domain.FieldErrorf("cost", "the contact's cost is the table's for the target's level, not entered")
		}
		return cost, nil
	}
}

// contactLines adds the request's contact modifiers to rec as labelled lines
// and to its target (step 9): the distance, the life order, and resisting
// when the DM says so.
func (s *service) contactLines(rec *CheckRecord, req ContactRequest) error {
	lines := [][2]string{{ruletablesvc.ContactDistance, req.Distance}, {ruletablesvc.ContactLifeOrder, req.LifeOrder}}
	if req.Resisting {
		lines = append(lines, [2]string{ruletablesvc.ContactResistance, "resisting"})
	}
	for _, l := range lines {
		m, err := s.rates.ContactModifier(l[0], l[1])
		if err != nil {
			return fmt.Errorf("contact modifiers: %w", err)
		}
		rec.ContactModifiers = append(rec.ContactModifiers, ContactLine{Kind: l[0], Code: l[1], Modifier: m})
		rec.Target += m
	}
	return nil
}

// contactTarget is step 10 for power p named on target: the label of the
// contact row made in an earlier round that holds target, ErrNoContact when
// none does. A power whose cost is not contact takes no target.
func (s *service) contactTarget(ctx context.Context, characterID int64, p catalogue.Power, target string) (string, error) {
	if p.InitialCost.Kind != catalogue.CostContact {
		if target != "" {
			return "", domain.FieldErrorf("target", "only a power whose cost is contact names a target")
		}
		return "", nil
	}
	t, err := psionics.NormalizeTarget(target)
	if err != nil {
		return "", err
	}
	rows, err := s.buffs.ListActive(ctx, characterID)
	if err != nil {
		return "", fmt.Errorf("list buffs: %w", err)
	}
	row, ok := buffsvc.ContactBefore(rows, t)
	if !ok {
		return "", ErrNoContact
	}
	return row.Target, nil
}
