package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// Spells is the spell-effect catalogue (spelleffectsvc), read for the end
// effects of a row's spell (ADR 0004 step 21). Wired in the composition root.
type Spells interface {
	Get(id string) (spelleffectsvc.Entry, bool)
}

// DieRoll is one die an end effect threw.
type DieRoll struct {
	Sides int
	Value int
}

// EndEffectRoll is one end effect applied when a row ended (ADR 0004 step
// 21): the row's spell and name, the cause, the kind, the dice rolled with
// every die, and the total.
type EndEffectRoll struct {
	SpellID  string
	Buff     string
	Cause    spelleffectsvc.EndCause
	Kind     spelleffectsvc.EndKind
	Dice     string
	Rolls    []DieRoll
	Constant int
	Total    int
}

// EndResult is what the end of one or more rows did: the effects applied
// and, when any applied, the current HP before and after the action (the
// heal capped at max HP by the ledger, ADR 0007 step 4). Zero when nothing
// applied.
type EndResult struct {
	Effects  []EndEffectRoll
	HPBefore int
	HPAfter  int
}

// endEffectPayload is an EndEffectRoll in a log entry, worded by the UI.
type endEffectPayload struct {
	SpellID  string           `json:"spellId"`
	Buff     string           `json:"buff"`
	Cause    string           `json:"cause"`
	Kind     string           `json:"kind"`
	Dice     string           `json:"dice"`
	Rolls    []dieRollPayload `json:"rolls"`
	Constant int              `json:"constant,omitempty"`
	Total    int              `json:"total"`
}

type dieRollPayload struct {
	Sides int `json:"sides"`
	Value int `json:"value"`
}

// asksEndCause reports whether stopping b asks how it ends: a row on the
// bearer whose spell has end effects. A row on someone else applies none
// (ADR 0004 step 20.5: it counts for nothing on the Bard).
func (s *service) asksEndCause(b BuffEffect) bool {
	if b.SpellID == "" || b.OnOther {
		return false
	}
	e, ok := s.spells.Get(b.SpellID)
	return ok && e.AsksEndCause()
}

// rollEndEffects is the one applier of ADR 0004 step 21: for each ended row
// on the bearer whose spell has end effects for cause, it rolls their dice
// through the dice port, in row order. heal is the HP the action requests.
func (s *service) rollEndEffects(ended []BuffEffect, cause spelleffectsvc.EndCause) (rolls []EndEffectRoll, heal int, err error) {
	if cause == "" {
		return nil, 0, nil
	}
	for _, b := range ended {
		if !s.asksEndCause(b) {
			continue
		}
		entry, _ := s.spells.Get(b.SpellID)
		for _, x := range entry.EndEffectsOn(cause) {
			r, err := s.rollEndEffect(b, cause, x)
			if err != nil {
				return nil, 0, err
			}
			rolls = append(rolls, r)
			if x.Kind == spelleffectsvc.EndHeal {
				heal += r.Total
			}
		}
	}
	return rolls, heal, nil
}

func (s *service) rollEndEffect(b BuffEffect, cause spelleffectsvc.EndCause, x spelleffectsvc.EndEffect) (EndEffectRoll, error) {
	// The loader refused dice that read the level or are not a sum, so the
	// level given here changes nothing.
	sum, err := x.Dice.Expand(0)
	if err != nil {
		return EndEffectRoll{}, fmt.Errorf("end effect of %s: %w", b.SpellID, err)
	}
	r := EndEffectRoll{
		SpellID: b.SpellID, Buff: b.Name, Cause: cause, Kind: x.Kind,
		Dice: x.Dice.String(), Constant: sum.Constant, Total: sum.Constant,
	}
	for _, d := range sum.Dice {
		for range d.Count {
			v := s.dice.RollDie(d.Sides)
			r.Rolls = append(r.Rolls, DieRoll{Sides: d.Sides, Value: v})
			r.Total += v
		}
	}
	return r, nil
}

// endResult reads the HP around a heal of the action's entry, inside its
// transaction: before is the current HP now, after what the ledger's cap
// (actionlog.CapHeal, the rule Record applies) leaves.
func (s *service) endResult(ctx context.Context, characterID int64, rolls []EndEffectRoll, heal int) (EndResult, error) {
	total, err := s.log.HPTotal(ctx, characterID)
	if err != nil {
		return EndResult{}, fmt.Errorf("hp total: %w", err)
	}
	ch, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return EndResult{}, fmt.Errorf("get character: %w", err)
	}
	return EndResult{
		Effects:  rolls,
		HPBefore: actionlog.CurrentHP(ch.HPMax, total),
		HPAfter:  actionlog.CurrentHP(ch.HPMax, total+actionlog.CapHeal(total, heal)),
	}, nil
}

func endEffectPayloads(rolls []EndEffectRoll) []endEffectPayload {
	if len(rolls) == 0 {
		return nil
	}
	out := make([]endEffectPayload, len(rolls))
	for i, r := range rolls {
		dice := make([]dieRollPayload, len(r.Rolls))
		for j, d := range r.Rolls {
			dice[j] = dieRollPayload(d)
		}
		out[i] = endEffectPayload{
			SpellID: r.SpellID, Buff: r.Buff, Cause: string(r.Cause), Kind: string(r.Kind),
			Dice: r.Dice, Rolls: dice, Constant: r.Constant, Total: r.Total,
		}
	}
	return out
}
