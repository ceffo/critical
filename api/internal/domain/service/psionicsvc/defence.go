package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Refusals of a defence mode's raise and of its use (ADR 0044 step 4), all
// before anything is written.
var (
	// ErrNotInCombat: a defence mode raised for the round with the round
	// counter at 0 (Q3: a round-scoped power needs a round). A standing
	// defence (step 5) is not refused.
	ErrNotInCombat = errors.New("a defence mode is raised for a combat round: no round is running")
	// ErrNotADefenceMode: a raise of a power that is not a defence mode.
	ErrNotADefenceMode = errors.New("the power is not a defence mode")
	// ErrDefenceModeIsRaised: a use (power check) of a defence mode: it is
	// raised, with no roll (step 4 amends ADR 0043 step 10 for the kind).
	ErrDefenceModeIsRaised = errors.New("a defence mode is raised, not used")
)

// Bounds of a contest request (step 8).
const (
	// MaxIncomingRolls: an attack mode is checked at most twice per cost
	// (step 7).
	MaxIncomingRolls = 2
	// defenceLookback is how many of the newest entries of a kind the
	// defence reads back for the round's kept roll and the raise's round.
	defenceLookback = 20
)

// RaiseRecord is the defence_raised entry's payload (step 4). The UI words
// it. Add fields, never rename.
type RaiseRecord struct {
	Power string `json:"power"`
	// Name is the mode's catalogue name at write time, in every language
	// (ADR 0023), as CheckRecord's.
	Name localized.Text `json:"name,omitempty"`
	// Defence is how the row lasts: raised for the round, or standing.
	Defence buffsvc.DefenceKind `json:"defence"`
	// Cost is the initial cost paid (the entry's PSP delta, negated).
	Cost int `json:"cost"`
}

// RaiseResult is a raise as logged, with the pool after it and the row put.
type RaiseResult struct {
	Raise      RaiseRecord
	PSP        actionlog.PSPState
	LogEntryID int64
	BuffID     string
}

// standing reports whether defence mode p is kept at no cost (step 5): its
// catalogue maintenance is a number, 0. A rule on the cost, never the id.
func standing(p catalogue.Power) bool {
	return p.Maintenance != nil && p.Maintenance.Cost.Kind == "" && p.Maintenance.Cost.Amount == 0
}

// RaiseDefence is ADR 0044 steps 4 and 5: a known defence mode is raised
// with no roll, paying its initial cost, in one defence_raised entry whose
// snapshot carries its buff row, so one undo gives both back. A mode kept at
// no cost is a standing row, untimed and raised in or out of combat; any
// other is the round's raised row (one a round: it replaces the one
// before). Refused before anything is written: ErrNotPsionicist,
// ErrPowerNotKnown, ErrNotADefenceMode, ErrNeedsContact, ErrNotInCombat
// (a raised mode at round 0), ErrNotEnoughPSP.
func (s *service) RaiseDefence(ctx context.Context, characterID int64, powerID string) (*RaiseResult, error) {
	var out *RaiseResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
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
		if p.Kind != catalogue.KindDefenceMode {
			return ErrNotADefenceMode
		}
		switch p.InitialCost.Kind {
		case "":
		case catalogue.CostContact:
			return ErrNeedsContact
		default:
			return fmt.Errorf("defence mode %s: initial cost %q is not a number", p.ID, p.InitialCost.Kind)
		}
		rec := RaiseRecord{Power: p.ID, Name: p.Name, Defence: buffsvc.DefenceRaised, Cost: p.InitialCost.Amount}
		row := buffsvc.BuffEffect{Name: rowName(p), PowerID: p.ID, PowerName: p.Name, Defence: buffsvc.DefenceRaised, Duration: 1}
		if standing(p) {
			rec.Defence, row.Defence, row.Duration = buffsvc.DefenceStanding, buffsvc.DefenceStanding, 0
		} else {
			round, err := s.buffs.GetRound(ctx, characterID)
			if err != nil {
				return fmt.Errorf("get round: %w", err)
			}
			if round < 1 {
				return ErrNotInCombat
			}
		}
		pool := charactersvc.PSPPool{}
		if c.Derived.PSP != nil {
			pool = *c.Derived.PSP
		}
		if pool.Current < rec.Cost {
			return ErrNotEnoughPSP
		}
		put, snap, err := s.buffs.Add(ctx, characterID, row)
		if err != nil {
			return fmt.Errorf("raise defence: %w", err)
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode defence raised payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindDefenceRaised, Payload: payload, PSPDelta: -rec.Cost,
			Snapshots: []actionlog.Snapshot{snap},
		})
		if err != nil {
			return fmt.Errorf("record defence raised: %w", err)
		}
		out = &RaiseResult{
			Raise: rec, LogEntryID: id, BuffID: put.ID,
			PSP: actionlog.PSPState{Current: pool.Current - rec.Cost, Max: pool.Max},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DefendRequest is a contest against the character (step 8): the DM's
// attack mode, any of the catalogue's, and its one or two rolls. Keep is the
// index of the contest whose check the player keeps for the round's next
// contests of the same defence; nil keeps none.
type DefendRequest struct {
	Attack string
	Rolls  []psionics.ContestSide
	Keep   *int
}

// DefenceRecord is the psionic_defence entry's payload (step 8). The UI
// words it. Add fields, never rename.
type DefenceRecord struct {
	Attack string `json:"attack"`
	// AttackName is the attack mode's catalogue name at write time.
	AttackName localized.Text `json:"attackName,omitempty"`
	// Defence is the current defence's power id, "" when none: then nothing
	// is rolled and the attacker's successes win.
	Defence     string              `json:"defence,omitempty"`
	DefenceName localized.Text      `json:"defenceName,omitempty"`
	DefenceKind buffsvc.DefenceKind `json:"defenceKind,omitempty"`
	// Ability, AbilityScore, PowerModifier and Relearns are the defence
	// score's parts; Score their sum, before the kept +1.
	Ability       string `json:"ability,omitempty"`
	AbilityScore  int    `json:"abilityScore,omitempty"`
	PowerModifier int    `json:"powerModifier,omitempty"`
	Relearns      int    `json:"relearns,omitempty"`
	Score         int    `json:"score,omitempty"`
	// Kept: the defence is the standing row raised in an earlier round (+1,
	// a failure counts as 1, step 6).
	Kept bool `json:"kept,omitempty"`
	// Modifier is Table 14's cell for the attack against the defence, shown
	// beside the DM's roll (he applies it to his score). Absent with no
	// defence.
	Modifier *int            `json:"modifier,omitempty"`
	Contests []ContestRecord `json:"contests"`
	// KeptCheck is the check the round's next contests of this defence take
	// with no die, once the player kept one.
	KeptCheck *psionics.ContestCheck `json:"keptCheck,omitempty"`
}

// ContestRecord is one contest: the DM's roll, the defence's check (absent
// with no defence) and the winner (psionics.SideAttacker or SideDefender).
type ContestRecord struct {
	Attack psionics.ContestSide   `json:"attack"`
	Check  *psionics.ContestCheck `json:"check,omitempty"`
	// FromKept: the check is the round's kept one, no die rolled.
	FromKept bool   `json:"fromKept,omitempty"`
	Winner   string `json:"winner"`
}

// DefendResult is the contest as logged.
type DefendResult struct {
	Defence    DefenceRecord
	LogEntryID int64
}

// Defend is ADR 0044 step 8: against the current defence (step 5) one check
// per incoming roll, each decided by the contest (step 6), in one
// psionic_defence entry with no PSP delta (the raise paid). A check kept
// earlier in the round for the same defence is taken with no die. With no
// current defence nothing is rolled. Refused before the roll:
// ErrNotPsionicist, a *domain.FieldError on "attack", "rolls" or "keep".
func (s *service) Defend(ctx context.Context, characterID int64, req DefendRequest) (*DefendResult, error) {
	attack, ok := s.book.Power(req.Attack)
	if !ok || attack.Kind != catalogue.KindAttackMode {
		return nil, domain.FieldErrorf("attack", "not an attack mode: %q", req.Attack)
	}
	if len(req.Rolls) < 1 || len(req.Rolls) > MaxIncomingRolls {
		return nil, domain.FieldErrorf("rolls", "one or %d rolls", MaxIncomingRolls)
	}
	for _, r := range req.Rolls {
		if r.Roll < 1 || r.Roll > psionics.CheckDie {
			return nil, domain.FieldErrorf("rolls", "a roll is 1 to %d", psionics.CheckDie)
		}
	}
	if req.Keep != nil && (*req.Keep < 0 || *req.Keep >= len(req.Rolls)) {
		return nil, domain.FieldErrorf("keep", "keep names one of the rolls")
	}
	var out *DefendResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		if c.Derived.Powers == nil {
			return ErrNotPsionicist
		}
		rec := DefenceRecord{Attack: attack.ID, AttackName: attack.Name}
		rows, err := s.buffs.ListActive(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list buffs: %w", err)
		}
		row, has := buffsvc.CurrentDefence(rows)
		if !has {
			if req.Keep != nil {
				return domain.FieldErrorf("keep", "no defence: nothing is rolled")
			}
			for _, r := range req.Rolls {
				rec.Contests = append(rec.Contests, ContestRecord{Attack: r, Winner: psionics.Contest(r, psionics.ContestSide{})})
			}
			return s.recordDefence(ctx, characterID, rec, &out)
		}
		if err := s.defenceOf(ctx, characterID, c, row, &rec); err != nil {
			return err
		}
		if rec.KeptCheck != nil && req.Keep != nil {
			return domain.FieldErrorf("keep", "a check is already kept this round")
		}
		for i, r := range req.Rolls {
			cr := ContestRecord{Attack: r}
			if rec.KeptCheck != nil {
				k := *rec.KeptCheck
				cr.Check, cr.FromKept = &k, true
			} else {
				ch := psionics.CheckContest(rec.Score, s.dice.RollDie(psionics.CheckDie), rec.Kept)
				cr.Check = &ch
				if req.Keep != nil && *req.Keep == i {
					k := ch
					rec.KeptCheck = &k
				}
			}
			cr.Winner = psionics.Contest(r, cr.Check.Side())
			rec.Contests = append(rec.Contests, cr)
		}
		return s.recordDefence(ctx, characterID, rec, &out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// defenceOf fills rec with the current defence row's mode: its score, the
// kept +1 of a standing row raised before this round, Table 14's cell and
// the round's kept check.
func (s *service) defenceOf(ctx context.Context, characterID int64, c *charactersvc.Character, row buffsvc.BuffEffect, rec *DefenceRecord) error {
	p, ok := s.book.Power(row.PowerID)
	if !ok || p.Score == nil {
		return fmt.Errorf("defence row %s: power %q has no score in the catalogue", row.ID, row.PowerID)
	}
	score, ok := c.Score(charactersvc.Ability(p.Score.Ability))
	if !ok {
		return fmt.Errorf("power %s: unknown ability %q", p.ID, p.Score.Ability)
	}
	rec.Defence, rec.DefenceName, rec.DefenceKind = p.ID, p.Name, row.Defence
	rec.Ability, rec.AbilityScore, rec.PowerModifier = p.Score.Ability, score, p.Score.Modifier
	rec.Relearns = psionics.Relearns(c.PsionicPicks, p.ID)
	rec.Score = score + p.Score.Modifier + rec.Relearns
	m, err := s.rates.AttackVersusDefence(rec.Attack, p.ID)
	if err != nil {
		return fmt.Errorf("table 14: %w", err)
	}
	rec.Modifier = &m
	round, err := s.buffs.GetRound(ctx, characterID)
	if err != nil {
		return fmt.Errorf("get round: %w", err)
	}
	since, err := s.roundStart(ctx, characterID)
	if err != nil {
		return err
	}
	if row.Defence == buffsvc.DefenceStanding {
		raisedThisRound, err := s.raisedSince(ctx, characterID, p.ID, since)
		if err != nil {
			return err
		}
		rec.Kept = !raisedThisRound
	}
	contests, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindPsionicDefence, defenceLookback)
	if err != nil {
		return fmt.Errorf("psionic defences: %w", err)
	}
	for _, e := range contests {
		if e.ID <= since || e.Round != round {
			break
		}
		var prev DefenceRecord
		if err := json.Unmarshal(e.Payload, &prev); err != nil {
			return fmt.Errorf("decode psionic defence %d: %w", e.ID, err)
		}
		if prev.Defence == p.ID {
			rec.KeptCheck = prev.KeptCheck
			break
		}
	}
	return nil
}

// raisedSince reports whether defence mode power was last raised after the
// entry since (in the current round).
func (s *service) raisedSince(ctx context.Context, characterID int64, power string, since int64) (bool, error) {
	raises, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindDefenceRaised, defenceLookback)
	if err != nil {
		return false, fmt.Errorf("defence raises: %w", err)
	}
	for _, e := range raises {
		var r RaiseRecord
		if err := json.Unmarshal(e.Payload, &r); err != nil {
			return false, fmt.Errorf("decode defence raised %d: %w", e.ID, err)
		}
		if r.Power == power {
			return e.ID > since, nil
		}
	}
	return false, nil
}

func (s *service) recordDefence(ctx context.Context, characterID int64, rec DefenceRecord, out **DefendResult) error {
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode psionic defence payload: %w", err)
	}
	id, err := s.log.Record(ctx, actionlog.NewEntry{CharacterID: characterID, Kind: actionlog.KindPsionicDefence, Payload: payload})
	if err != nil {
		return fmt.Errorf("record psionic defence: %w", err)
	}
	*out = &DefendResult{Defence: rec, LogEntryID: id}
	return nil
}
