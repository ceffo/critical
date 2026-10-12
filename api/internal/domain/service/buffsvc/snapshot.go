package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// snapBuff is buffsvc's own JSON encoding of a buff inside action-log
// snapshots (StateKeyBuffs). It must stay decodable for entries already in
// the log: add fields, never rename them.
type snapBuff struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Charges  int         `json:"charges"`
	Duration int         `json:"duration"`
	Effects  snapEffects `json:"effects"`
}

type snapEffects struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrSet     *int     `json:"str_set,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
}

func encodeBuffs(buffs []BuffEffect) (json.RawMessage, error) {
	out := make([]snapBuff, len(buffs))
	for i, b := range buffs {
		out[i] = snapBuff{
			ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
			Effects: snapEffects(b.Effects),
		}
	}
	return json.Marshal(out)
}

func decodeBuffs(raw json.RawMessage) ([]BuffEffect, error) {
	var in []snapBuff
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode buffs snapshot: %w", err)
	}
	out := make([]BuffEffect, len(in))
	for i, b := range in {
		out[i] = BuffEffect{
			ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
			Effects: BuffEffects(b.Effects),
		}
	}
	return out, nil
}

// restoreFunc adapts a function to actionlog.Restorer.
type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes the buff list or the round number back from a snapshot.
// They run inside the void's transaction (ctx carries it).
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyBuffs: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			buffs, err := decodeBuffs(state)
			if err != nil {
				return err
			}
			return s.dal.SaveActiveBuffs(ctx, characterID, buffs)
		}),
		StateKeyRound: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var round int
			if err := json.Unmarshal(state, &round); err != nil {
				return fmt.Errorf("decode round snapshot: %w", err)
			}
			if round < 0 {
				return fmt.Errorf("decode round snapshot: negative round %d", round)
			}
			return s.dal.SaveRoundNumber(ctx, characterID, round)
		}),
	}
}
