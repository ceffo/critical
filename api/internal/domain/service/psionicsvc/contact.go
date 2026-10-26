package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
// the opponent (step 2: its level gives the cost; no defence), the rows of
// the cost table's modifiers the DM picks (data: each must be a row of the
// table, one per kind at most), and the labelled modifiers of ADR 0043
// step 10. Cost is the DM's cost for a target whose level Table 29 has no row
// for (Q15's fallback), and only then.
type ContactRequest struct {
	Power    string
	Cost     *int
	Opponent psionics.Opponent
	// Picks are the cost table's modifier rows chosen, in the order they are
	// added to the score.
	Picks    []ContactPick
	Labelled []Modifier
}

// ContactPick names one modifier row of the cost table: its kind and code.
type ContactPick struct {
	Kind string `json:"kind"`
	Code string `json:"code"`
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
// with the modifier rows the DM picked,
// then ADR 0043 steps 10 and 11 as Use's (half the cost on a failure). A
// success makes the target's contact row (buffsvc.MakeContact) in the
// power_check entry, its snapshot beside the PSPs, so one undo takes both
// back. Refused before the roll: a *domain.FieldError on the opponent's
// fields, "defence", "level" (not given), "cost"
// (entered for a level the table has, or under 1) or "modifiers" (a pick that
// is no row of the table, a second of a kind, a bad labelled one), ErrNotPsionicist, ErrPowerNotKnown, ErrNotAContactPower,
// ErrMindClosed, ErrLevelOutOfTable, ErrNotEnoughPSP.
func (s *service) Contact(ctx context.Context, characterID int64, req ContactRequest) (*UseResult, error) {
	opp, mods, err := s.contactInput(req)
	if err != nil {
		return nil, err
	}
	var out *UseResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		p, rec, _, err := s.contactCheck(c, req, opp, mods)
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

// contactInput validates what the contact request holds without the
// character: the opponent and the labelled modifiers. Contact and
// ContactPreview share it.
func (s *service) contactInput(req ContactRequest) (psionics.Opponent, []Modifier, error) {
	opp := req.Opponent
	if err := opp.Validate(s.book); err != nil {
		return opp, nil, err
	}
	if opp.Defence != "" {
		return opp, nil, domain.FieldErrorf("defence", "a contact names no defence")
	}
	mods, err := modifiers(req.Labelled)
	return opp, mods, err
}

// contactCheck is the contact's check before the roll for character c: the
// power must be known and the contact power's, the mind open, the cost
// Table 29's (or the DM's past it), the score with the picked modifier rows.
// fromTable says the cost is the table's. Contact rolls from it;
// ContactPreview only reports it.
func (s *service) contactCheck(c *charactersvc.Character, req ContactRequest, opp psionics.Opponent, mods []Modifier) (p catalogue.Power, rec CheckRecord, fromTable bool, err error) {
	if c.Derived.Powers == nil {
		return p, rec, false, ErrNotPsionicist
	}
	p, ok := s.book.Power(req.Power)
	if !ok || !psionics.Knows(c.PsionicPicks, req.Power) {
		return p, rec, false, ErrPowerNotKnown
	}
	if p.InitialCost.Table != ContactCostTable {
		return p, rec, false, ErrNotAContactPower
	}
	if opp.Mind == psionics.MindClosed {
		return p, rec, false, ErrMindClosed
	}
	if opp.Level < 1 {
		return p, rec, false, domain.FieldErrorf("level", "the contact's cost needs the target's level")
	}
	tableCost, tableErr := s.rates.ContactCost(opp.Level)
	cost, err := contactCost(tableCost, tableErr)(req.Cost)
	if err != nil {
		return p, rec, false, err
	}
	rec, err = checkOf(c, p, UseRequest{Cost: &cost}, mods)
	if err != nil {
		return p, rec, false, err
	}
	rec.Opponent, rec.Level = opp.Target, opp.Level
	if err := s.contactLines(&rec, req.Picks); err != nil {
		return p, rec, false, err
	}
	return p, rec, tableErr == nil, nil
}

// ContactPreview is what the contact would cost and score now, from the same
// computation as Contact's, nothing rolled or written.
type ContactPreview struct {
	// Cost is Table 29's for the level, or the DM's past the table.
	Cost int `json:"cost"`
	// FromTable is false when Cost is the DM's (Q15).
	FromTable bool `json:"fromTable"`
	// Target is the score to roll under: the power's own with the lines.
	Target int `json:"target"`
	// Lines are the contact modifiers in the order they are added.
	Lines []ContactLine `json:"lines"`
	// Current is the PSP pool now; Enough says it covers Cost.
	Current int  `json:"current"`
	Enough  bool `json:"enough"`
	// Warnings are Contact's warning codes now (second_power_this_round).
	Warnings []string `json:"warnings,omitempty"`
}

// ContactPreview reports the contact's cost and score for req without a
// roll. Its refusals are Contact's before the roll, except not_enough_psp,
// which Enough carries.
func (s *service) ContactPreview(ctx context.Context, characterID int64, req ContactRequest) (*ContactPreview, error) {
	opp, mods, err := s.contactInput(req)
	if err != nil {
		return nil, err
	}
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, err
	}
	p, rec, fromTable, err := s.contactCheck(c, req, opp, mods)
	if err != nil {
		return nil, err
	}
	second, err := s.secondPowerThisRound(ctx, characterID, p)
	if err != nil {
		return nil, err
	}
	var warnings []string
	if second {
		warnings = append(warnings, WarnSecondPowerThisRound)
	}
	cur := 0
	if c.Derived.PSP != nil {
		cur = c.Derived.PSP.Current
	}
	return &ContactPreview{
		Cost: rec.Cost, FromTable: fromTable, Target: rec.Target, Lines: rec.ContactModifiers,
		Current: cur, Enough: cur >= rec.Cost, Warnings: warnings,
	}, nil
}

// CostTable is a rule table a power's cost names, with the modifier rows the
// web builds its controls from (data, not code): one per kind and code. The
// cost for a level comes from the preview, never from the web.
type CostTable struct {
	Table     string        `json:"table"`
	Modifiers []ContactLine `json:"modifiers"`
}

// CostTable is the rule table named table, as a power's initial cost names it
// (PowerDTO.initial_cost_table); domain.ErrNotFound for a name no power's cost
// carries.
func (s *service) CostTable(_ context.Context, table string) (CostTable, error) {
	if table != ContactCostTable {
		return CostTable{}, fmt.Errorf("cost table %q: %w", table, domain.ErrNotFound)
	}
	rows, err := s.rates.ContactModifierRows()
	if err != nil {
		return CostTable{}, err
	}
	out := CostTable{Table: table, Modifiers: make([]ContactLine, 0, len(rows))}
	for _, r := range rows {
		out.Modifiers = append(out.Modifiers, ContactLine(r))
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

// contactLines adds the picked modifier rows to rec as labelled lines and to
// its target (step 9), in the order picked. A pick that is no row of the table,
// or a second of a kind, is a *domain.FieldError on "modifiers".
func (s *service) contactLines(rec *CheckRecord, picks []ContactPick) error {
	seen := map[string]bool{}
	for _, pk := range picks {
		if seen[pk.Kind] {
			return domain.FieldErrorf("modifiers", "a second %q modifier", pk.Kind)
		}
		seen[pk.Kind] = true
		m, err := s.rates.ContactModifier(pk.Kind, pk.Code)
		if errors.Is(err, ruletablesvc.ErrNoContactModifier) {
			return domain.FieldErrorf("modifiers", "no %s %q modifier in the table", pk.Kind, pk.Code)
		}
		if err != nil {
			return fmt.Errorf("contact modifiers: %w", err)
		}
		rec.ContactModifiers = append(rec.ContactModifiers, ContactLine{Kind: pk.Kind, Code: pk.Code, Modifier: m})
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
