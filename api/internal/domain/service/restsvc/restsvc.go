// Package restsvc owns the long rest: one operation that refills everything
// that resets overnight (spell slots and per-day items) in a single
// transaction, so a failure in one leaves the other untouched. The rest is
// one action-log entry with the slot list and every refilled item count as
// snapshots, so it blocks voiding an older cast or item use (whose "before"
// would undo the rest) and can itself be voided.
package restsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Result is the state after a long rest.
type Result struct {
	Slots []spellsvc.SpellSlot
	Items []itemsvc.Item
	// EntryID is the long rest's action-log entry.
	EntryID int64
}

// SVC is the driver port.
type SVC interface {
	domain.Service
	LongRest(ctx context.Context, characterID int64) (Result, error)
}

// Spells is the narrow port to the spell-slot service.
type Spells interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	ResetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
}

// Items is the narrow port to the item service.
type Items interface {
	List(ctx context.Context) ([]itemsvc.Item, error)
	ResetDaily(ctx context.Context) ([]itemsvc.Item, error)
}

// Log records the rest (actionlogsvc).
type Log interface {
	actionlog.Recorder
}

type service struct {
	spells Spells
	items  Items
	log    Log
	tx     domain.TxRunner
}

// New constructs the service.
func New(spells Spells, items Items, log Log, tx domain.TxRunner) (SVC, error) {
	if spells == nil || items == nil || log == nil || tx == nil {
		return nil, fmt.Errorf("restsvc: nil dependency")
	}
	return &service{spells: spells, items: items, log: log, tx: tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// restRecord is the long rest's log payload. Add fields, never rename.
type restRecord struct {
	SlotsReset    int      `json:"slotsReset"`
	ItemsRefilled []string `json:"itemsRefilled"`
}

// LongRest resets spell slots and per-day items atomically and logs it (ADR
// 0008 steps 11 and 12).
func (s *service) LongRest(ctx context.Context, characterID int64) (Result, error) {
	var res Result
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		slotsBefore, err := s.spells.GetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get slots: %w", err)
		}
		itemsBefore, err := s.items.List(ctx)
		if err != nil {
			return fmt.Errorf("list items: %w", err)
		}
		slots, err := s.spells.ResetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("reset slots: %w", err)
		}
		items, err := s.items.ResetDaily(ctx)
		if err != nil {
			return fmt.Errorf("reset items: %w", err)
		}
		snaps, rec, err := snapshots(slotsBefore, slots, itemsBefore, items)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode long rest payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindLongRest,
			Payload:     payload,
			Snapshots:   snaps,
		})
		if err != nil {
			return fmt.Errorf("record long rest: %w", err)
		}
		res = Result{Slots: slots, Items: items, EntryID: id}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// snapshots always covers the slot list (a rest must block voiding any
// older cast, even when nothing was used) and every item whose count changed.
func snapshots(slotsBefore, slots []spellsvc.SpellSlot, itemsBefore, items []itemsvc.Item) ([]actionlog.Snapshot, restRecord, error) {
	rec := restRecord{ItemsRefilled: []string{}}
	for _, sl := range slotsBefore {
		if sl.Used {
			rec.SlotsReset++
		}
	}
	slotSnap, err := prepsvc.SlotsSnapshot(slotsBefore, slots)
	if err != nil {
		return nil, rec, err
	}
	snaps := []actionlog.Snapshot{slotSnap}
	was := make(map[string]int, len(itemsBefore))
	for _, it := range itemsBefore {
		was[it.ID] = it.Count
	}
	for _, it := range items {
		before, ok := was[it.ID]
		if !ok || before == it.Count {
			continue
		}
		snap, err := itemsvc.CountSnapshot(it.ID, before, it.Count)
		if err != nil {
			return nil, rec, err
		}
		snaps = append(snaps, snap)
		rec.ItemsRefilled = append(rec.ItemsRefilled, it.Name.Any())
	}
	return snaps, rec, nil
}
