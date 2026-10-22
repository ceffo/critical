package skillsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// The buff a successful proficiency check puts on (ADR 0037 step 20): the
// rules data says which proficiency puts which buff on
// (the ruleset's success effects, ADR 0041 step 6); this file only follows it. The buff follows
// the verdict in effect: the roll's verdict puts it on in the check's entry
// (step 20.4), a post-roll adjustment that flips the verdict puts it on or
// takes it off in its correction (step 20.8).

// Buffs puts a buff on and takes it off inside the caller's transaction,
// returning the snapshot to record (buffsvc satisfies it).
type Buffs interface {
	ListActive(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	Add(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error)
	Remove(ctx context.Context, characterID int64, buffID string, putBack []buffsvc.BuffEffect) (bool, actionlog.Snapshot, error)
}

// AppliedBuff is the buff a check's verdict in effect put on (ADR 0037 step
// 20.4): the buff as put on, and Renewed when it replaced an active one of
// the same name (step 20.3).
type AppliedBuff struct {
	Buff    buffsvc.BuffEffect
	Renewed bool
}

// EffectsByKey is a check's buff's effects keyed like the buff API ("ac"),
// the one shape of the payload's buff and of the check's answer.
func EffectsByKey(fx buffsvc.BuffEffects) map[string]int {
	out := map[string]int{}
	if fx.AC != nil {
		out["ac"] = *fx.AC
	}
	return out
}

// buffRecord is skillsvc's JSON encoding of the buff a check put on, inside
// its skill_check payload (ADR 0037 step 20.4: name, effects, rounds; the
// row's id, so a later adjustment takes that row off, and the proficiency it
// is named from). Effects are keyed like the buff API ("ac").
type buffRecord struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	ProficiencyID string         `json:"proficiencyId"`
	Effects       map[string]int `json:"effects"`
	Rounds        int            `json:"rounds"`
}

func toBuffRecord(b buffsvc.BuffEffect) *buffRecord {
	return &buffRecord{ID: b.ID, Name: b.Name, ProficiencyID: b.ProficiencyID, Effects: EffectsByKey(b.Effects), Rounds: b.Duration}
}

func (r *buffRecord) applied(renewed bool) *AppliedBuff {
	if r == nil {
		return nil
	}
	b := buffsvc.BuffEffect{ID: r.ID, Name: r.Name, ProficiencyID: r.ProficiencyID, Duration: r.Rounds}
	if ac, ok := r.Effects["ac"]; ok {
		b.Effects.AC = &ac
	}
	return &AppliedBuff{Buff: b, Renewed: renewed}
}

// successBuff is the buff a successful check of rec puts on, per the rules
// data (ADR 0037 steps 20.1 and 20.6); false for a check that puts none (any
// other proficiency, a bard or thieving skill). The buff is named from the
// proficiency's id, which the client turns into its name (step 20.1); the
// stored name is the id too, the fallback label and the renewal key (ADR
// 0004 step 6).
func (s *service) successBuff(ctx context.Context, characterID int64, rec checkRecord) (buffsvc.BuffEffect, bool, error) {
	if rec.Kind != KindProficiency {
		return buffsvc.BuffEffect{}, false, nil
	}
	rs, err := s.rulesets.ForCharacter(ctx, characterID)
	if err != nil {
		return buffsvc.BuffEffect{}, false, fmt.Errorf("resolve the ruleset: %w", err)
	}
	rule, ok := rs.SuccessEffectFor(rec.ID)
	if !ok {
		return buffsvc.BuffEffect{}, false, nil
	}
	b := buffsvc.BuffEffect{Name: rec.ID, ProficiencyID: rec.ID, Duration: rule.Rounds}
	if rule.AC != 0 {
		ac := rule.AC
		b.Effects.AC = &ac
	}
	return b, true, nil
}

// putBuff puts on the buff the success of rec gives, if any (ADR 0037 steps
// 20.1 and 20.3: a buff of the same name already on is renewed, never
// stacked), recording it in rec with the rows it replaced (so a later
// failure gives them back, step 20.8), and returns the buff list's snapshot
// (nil: none put on). It runs inside the check's or the correction's
// transaction. No die: the effect and the rounds are fixed (step 20.5).
func (s *service) putBuff(ctx context.Context, characterID int64, rec *checkRecord) (*actionlog.Snapshot, error) {
	buff, ok, err := s.successBuff(ctx, characterID, *rec)
	if err != nil || !ok {
		return nil, err
	}
	active, err := s.buffs.ListActive(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("active buffs: %w", err)
	}
	var replaced []buffsvc.BuffEffect
	for _, b := range active {
		// buffsvc.Add's renewal: a row on the character of the same name.
		if !b.OnOther && strings.EqualFold(b.Name, buff.Name) {
			replaced = append(replaced, b)
		}
	}
	added, snap, err := s.buffs.Add(ctx, characterID, buff)
	if err != nil {
		return nil, fmt.Errorf("put the %s buff on: %w", rec.ID, err)
	}
	rec.Buff, rec.Renewed, rec.Replaced = toBuffRecord(added), len(replaced) > 0, nil
	if len(replaced) > 0 {
		if rec.Replaced, err = buffsvc.EncodeRows(replaced); err != nil {
			return nil, err
		}
	}
	return &snap, nil
}

// takeBuff takes off the buff rec's success put on (ADR 0037 step 20.8: the
// verdict turned to a failure), clearing it from rec, and gives back the rows
// it renewed when giveBack (the check's round still runs), so the list reads
// as if the check had put nothing on. It returns the buff list's snapshot
// (nil when the buff is no longer on the list: it ended or was dismissed;
// nothing is given back then). It runs inside the correction's transaction.
func (s *service) takeBuff(ctx context.Context, characterID int64, rec *checkRecord, giveBack bool) (*actionlog.Snapshot, error) {
	if rec.Buff == nil {
		return nil, nil
	}
	var putBack []buffsvc.BuffEffect
	if giveBack && len(rec.Replaced) > 0 {
		var err error
		if putBack, err = buffsvc.DecodeRows(rec.Replaced); err != nil {
			return nil, err
		}
	}
	id := rec.Buff.ID
	rec.Buff, rec.Renewed, rec.Replaced = nil, false, nil
	removed, snap, err := s.buffs.Remove(ctx, characterID, id, putBack)
	if err != nil {
		return nil, fmt.Errorf("take the %s buff off: %w", rec.ID, err)
	}
	if !removed {
		return nil, nil
	}
	return &snap, nil
}

// followVerdict puts the buff on or takes it off when an adjustment changed
// rec's verdict from wasSuccess (ADR 0037 step 20.8); an unchanged verdict
// touches nothing. checkRound is the round the check was rolled in: once a
// Next Round has passed it, the buff's one round is over, so a success puts
// nothing on and a failure only takes off what the check still has on,
// giving nothing back. The snapshots go on the correction.
func (s *service) followVerdict(ctx context.Context, characterID int64, checkRound int, rec *checkRecord, wasSuccess bool) ([]actionlog.Snapshot, error) {
	if rec.Success == wasSuccess {
		return nil, nil
	}
	_, ok, err := s.successBuff(ctx, characterID, *rec)
	if err != nil {
		return nil, err
	}
	if (rec.Success && !ok) || (!rec.Success && rec.Buff == nil) {
		return nil, nil // no buff to put on or take off
	}
	round, err := s.buffs.GetRound(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("current round: %w", err)
	}
	current := round == checkRound
	var snap *actionlog.Snapshot
	switch {
	case rec.Success && current:
		snap, err = s.putBuff(ctx, characterID, rec)
	case rec.Success:
		// The check's round is over: no buff (ADR 0037 step 20.8).
	default:
		snap, err = s.takeBuff(ctx, characterID, rec, current)
	}
	if err != nil || snap == nil {
		return nil, err
	}
	return []actionlog.Snapshot{*snap}, nil
}
