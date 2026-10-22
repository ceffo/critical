package levelupsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// realized is the state of the StateKey snapshot (realizedSnaps): the
// realized level and the max HP that goes with it. Entries already in the log must
// stay decodable: add fields, never rename.
type realized struct {
	// Class is the class entry whose level it is (ADR 0042 step 7); "" in
	// a snapshot written before the entries: the first active one.
	Class         string `json:"class,omitempty"`
	RealizedLevel int    `json:"realizedLevel"`
	HPMax         int    `json:"hpMax"`
}

// realizedSnaps is the snapshotter of the realized level (ADR 0018 step 8).
var realizedSnaps = actionlog.NewSnapshotter[realized](StateKey)

// record is the payload of a level_up entry, the roll's breakdown in the
// /api/character/levelup/roll wire names. The UI words it. Add fields, never
// rename.
type record struct {
	// Class is the entry the level was taken in and Classes the divisor
	// (ADR 0042 step 10.4); DieShare and ConShare the die's and the
	// Constitution's shares. A multi-class character's only: a single-class
	// record (and one written before them) has none, its one class the first
	// active one.
	Class         string `json:"class,omitempty"`
	Classes       int    `json:"classes,omitempty"`
	DieShare      int    `json:"dieShare,omitempty"`
	ConShare      int    `json:"conShare,omitempty"`
	Level         int    `json:"level"`
	HitDie        int    `json:"hitDie"`
	Rolled        *int   `json:"rolled"`
	Counted       *int   `json:"counted"`
	ConAdjustment int    `json:"conAdjustment"`
	Gain          int    `json:"gain"`
	Fixed         bool   `json:"fixed"`
	Manual        bool   `json:"manual"`
	HPMaxBefore   int    `json:"hpMaxBefore"`
	HPMaxAfter    int    `json:"hpMaxAfter"`
}

// record writes the level_up entry with the realized level's snapshot, inside the
// caller's transaction (ADR 0030 step 7.3, ADR 0018 step 1). Its HP delta
// is 0: current HP is the max HP plus the ledger (ADR 0007 step 2), so it
// rises with the max HP and the damage already taken is kept.
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after realized) error {
	rec := record{
		Level: r.Level, HitDie: r.HitDie, Rolled: r.Rolled, Counted: r.Counted,
		ConAdjustment: r.ConAdjustment, Gain: r.Gain, Fixed: r.Fixed, Manual: r.Manual,
		HPMaxBefore: r.HPMaxBefore, HPMaxAfter: r.HPMaxAfter,
	}
	if r.Classes > 1 {
		// A single-class record stays as it was written before the entries.
		rec.Class, rec.Classes, rec.DieShare, rec.ConShare = r.Class, r.Classes, r.DieShare, r.ConShare
	}
	payload, err := json.Marshal(rec)
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

// Module declares the codec that writes a realized level snapshot back: the
// undo of a roll gives the realized level and its max HP back, so the level-up
// is pending again and the level's benefits go with it; a redo writes the
// roll's "after" (ADR 0019). It runs inside the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "levelupsvc", Codecs: []actionlog.Codec{
		realizedSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, r realized) error {
			if r.HPMax < 1 || r.RealizedLevel < 1 {
				return fmt.Errorf("decode realized level snapshot: level %d with max HP %d", r.RealizedLevel, r.HPMax)
			}
			if err := s.dal.SetRealizedLevel(ctx, characterID, r.Class, r.RealizedLevel, r.HPMax); err != nil {
				return fmt.Errorf("restore realized level: %w", err)
			}
			return nil
		}, nil),
	}}
}

// LevelReached is the class entry ("" for a record written before the
// entries: the first active one) and the level a level_up entry's payload
// (record) reached: a level taken in the app (ADR 0030 step 20.8 reads them).
func LevelReached(payload json.RawMessage) (class string, level int, err error) {
	var r record
	if err := json.Unmarshal(payload, &r); err != nil {
		return "", 0, fmt.Errorf("decode level_up payload: %w", err)
	}
	if r.Level < 1 {
		return "", 0, fmt.Errorf("decode level_up payload: level %d", r.Level)
	}
	return r.Class, r.Level, nil
}
