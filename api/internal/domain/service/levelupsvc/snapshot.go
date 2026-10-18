package levelupsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// hitPoints is the JSON encoding of the StateKey snapshot: the max HP and
// the level it is set for. Entries already in the log must stay decodable:
// add fields, never rename.
type hitPoints struct {
	HPMax   int `json:"hpMax"`
	HPLevel int `json:"hpLevel"`
}

// record is the payload of a level_up entry, the roll's breakdown in the
// /api/character/levelup/roll wire names. The UI words it. Add fields, never
// rename.
type record struct {
	Level         int  `json:"level"`
	HitDie        int  `json:"hitDie"`
	Rolled        *int `json:"rolled"`
	Counted       *int `json:"counted"`
	ConAdjustment int  `json:"conAdjustment"`
	Gain          int  `json:"gain"`
	Fixed         bool `json:"fixed"`
	Manual        bool `json:"manual"`
	HPMaxBefore   int  `json:"hpMaxBefore"`
	HPMaxAfter    int  `json:"hpMaxAfter"`
}

// record writes the level_up entry with the hit points' snapshot, inside the
// caller's transaction (ADR 0030 step 7.3, ADR 0018 step 1). Its HP delta
// is 0: current HP is the max HP plus the ledger (ADR 0007 step 2), so it
// rises with the max HP and the damage already taken is kept.
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after hitPoints) error {
	payload, err := json.Marshal(record{
		Level: r.Level, HitDie: r.HitDie, Rolled: r.Rolled, Counted: r.Counted,
		ConAdjustment: r.ConAdjustment, Gain: r.Gain, Fixed: r.Fixed, Manual: r.Manual,
		HPMaxBefore: r.HPMaxBefore, HPMaxAfter: r.HPMaxAfter,
	})
	if err != nil {
		return fmt.Errorf("encode level_up payload: %w", err)
	}
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode hit points snapshot: %w", err)
	}
	a, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode hit points snapshot: %w", err)
	}
	if _, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindLevelUp, Payload: payload,
		Snapshots: []actionlog.Snapshot{{Key: StateKey, Before: b, After: a}},
	}); err != nil {
		return fmt.Errorf("record level_up: %w", err)
	}
	return nil
}

type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes a hit points snapshot back: the undo of a roll gives the
// max HP and its level back, so the level-up is pending again; a redo writes
// the roll's "after" (ADR 0019). It runs inside the undo's transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKey: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var hp hitPoints
			if err := json.Unmarshal(state, &hp); err != nil {
				return fmt.Errorf("decode hit points snapshot: %w", err)
			}
			if hp.HPMax < 1 || hp.HPLevel < 1 {
				return fmt.Errorf("decode hit points snapshot: max HP %d at level %d", hp.HPMax, hp.HPLevel)
			}
			if err := s.dal.SetHitPoints(ctx, characterID, hp.HPMax, hp.HPLevel); err != nil {
				return fmt.Errorf("restore hit points: %w", err)
			}
			return nil
		}),
	}
}
