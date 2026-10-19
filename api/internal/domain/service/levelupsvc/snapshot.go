package levelupsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// realized is the state of the StateKey snapshot (realizedSnaps): the
// realized level and the max HP that goes with it. Entries already in the log must
// stay decodable: add fields, never rename.
type realized struct {
	RealizedLevel int `json:"realizedLevel"`
	HPMax         int `json:"hpMax"`
}

// realizedSnaps is the snapshotter of the realized level (ADR 0018 step 8).
var realizedSnaps = actionlog.NewSnapshotter[realized](StateKey)

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

// record writes the level_up entry with the realized level's snapshot, inside the
// caller's transaction (ADR 0030 step 7.3, ADR 0018 step 1). Its HP delta
// is 0: current HP is the max HP plus the ledger (ADR 0007 step 2), so it
// rises with the max HP and the damage already taken is kept.
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after realized) error {
	payload, err := json.Marshal(record{
		Level: r.Level, HitDie: r.HitDie, Rolled: r.Rolled, Counted: r.Counted,
		ConAdjustment: r.ConAdjustment, Gain: r.Gain, Fixed: r.Fixed, Manual: r.Manual,
		HPMaxBefore: r.HPMaxBefore, HPMaxAfter: r.HPMaxAfter,
	})
	if err != nil {
		return fmt.Errorf("encode level_up payload: %w", err)
	}
	snap, err := realizedSnaps.Snapshot("", &before, &after)
	if err != nil {
		return err
	}
	if _, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindLevelUp, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	}); err != nil {
		return fmt.Errorf("record level_up: %w", err)
	}
	return nil
}

// Restorers writes a realized level snapshot back: the undo of a roll gives
// the realized level and its max HP back, so the level-up is pending again
// and the level's benefits go with it; a redo writes the roll's "after" (ADR
// 0019). It runs inside the undo's transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKey: realizedSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, r realized) error {
			if r.HPMax < 1 || r.RealizedLevel < 1 {
				return fmt.Errorf("decode realized level snapshot: level %d with max HP %d", r.RealizedLevel, r.HPMax)
			}
			if err := s.dal.SetRealizedLevel(ctx, characterID, r.RealizedLevel, r.HPMax); err != nil {
				return fmt.Errorf("restore realized level: %w", err)
			}
			return nil
		}, nil),
	}
}
