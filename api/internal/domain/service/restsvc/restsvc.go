// Package restsvc owns the long rest: one operation that refills everything
// that resets overnight (spell slots and per-day items) in a single
// transaction, so a failure in one leaves the other untouched. A psionic
// character also recovers a night's sleep of PSPs (ADR 0043 step 7), carried
// as the entry's PSP delta. The rest is
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
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Result is the state after a long rest.
type Result struct {
	Slots []spellsvc.SpellSlot
	Items []itemsvc.Item
	// Warnings are the class warnings of Items (ADR 0033 step 20).
	Warnings []itemsvc.Warning
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
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
	ResetDaily(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
	Warnings(ctx context.Context, characterID int64, items []itemsvc.Item) ([]itemsvc.Warning, error)
}

// PSP is the narrow port to the PSP pool (charactersvc).
type PSP interface {
	// NightsPSPRecovery is the PSPs a night's sleep recovers, before the
	// cap at the maximum; 0 for a character with no psionics.
	NightsPSPRecovery(ctx context.Context, characterID int64) (int, error)
}

// Buffs ends the maintained powers (buffsvc, ADR 0043 step 7).
type Buffs interface {
	// EndMaintained takes the maintained powers' rows off inside the ctx
	// transaction, without recording: the rest's entry records the snapshot.
	EndMaintained(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, actionlog.Snapshot, error)
}

// Log records the rest (actionlogsvc).
type Log interface {
	actionlog.Recorder
}

type service struct {
	spells Spells
	items  Items
	psp    PSP
	buffs  Buffs
	log    Log
	tx     domain.TxRunner
}

// New constructs the service.
func New(spells Spells, items Items, psp PSP, buffs Buffs, log Log, tx domain.TxRunner) (SVC, error) {
	if spells == nil || items == nil || psp == nil || buffs == nil || log == nil || tx == nil {
		return nil, fmt.Errorf("restsvc: nil dependency")
	}
	return &service{spells: spells, items: items, psp: psp, buffs: buffs, log: log, tx: tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares no codec: the slots and the items a rest refills are
// encoded by prepsvc's and itemsvc's helpers and restored by their codecs.
func (s *service) Module() domain.Module { return domain.Module{Name: "restsvc"} }

// restRecord is the long rest's log payload. Add fields, never rename.
type restRecord struct {
	SlotsReset    int      `json:"slotsReset"`
	ItemsRefilled []string `json:"itemsRefilled"`
	// PowersEnded are the maintained powers the rest ended (ADR 0043 step
	// 7), by catalogue id; their rows are in the entry's buff snapshot.
	PowersEnded []string `json:"powersEnded,omitempty"`
	// PowerNames names them by id in every language (ADR 0023).
	PowerNames map[string]localized.Text `json:"powerNames,omitempty"`
	// TargetsEnded are the minds whose tangent or contact row the rest
	// ended (ADR 0044 step 3), by the player's label.
	TargetsEnded []string `json:"targetsEnded,omitempty"`
}

// LongRest resets spell slots and per-day items atomically and logs it (ADR
// 0008 steps 11 and 12). It ends every maintained power, their rows
// snapshotted in the entry, and the entry's PSP delta is a night's sleep (ADR
// 0043 step 7); the recording path caps it at the PSPs spent (step 5), so
// undoing the rest takes back what was applied and puts the rows back.
func (s *service) LongRest(ctx context.Context, characterID int64) (Result, error) {
	var res Result
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		slotsBefore, err := s.spells.GetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get slots: %w", err)
		}
		itemsBefore, err := s.items.List(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list items: %w", err)
		}
		slots, err := s.spells.ResetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("reset slots: %w", err)
		}
		items, err := s.items.ResetDaily(ctx, characterID)
		if err != nil {
			return fmt.Errorf("reset items: %w", err)
		}
		snaps, rec, err := snapshots(slotsBefore, slots, itemsBefore, items)
		if err != nil {
			return err
		}
		// ADR 0043 step 7: no maintenance asleep; the rows come back on undo.
		ended, buffSnap, err := s.buffs.EndMaintained(ctx, characterID)
		if err != nil {
			return fmt.Errorf("end maintained powers: %w", err)
		}
		for _, b := range ended {
			switch {
			case b.PowerID != "":
				rec.PowersEnded = append(rec.PowersEnded, b.PowerID)
			case b.Target != "":
				rec.TargetsEnded = append(rec.TargetsEnded, b.Target)
			}
		}
		rec.PowerNames = buffsvc.PowerNames(ended)
		if len(ended) > 0 {
			snaps = append(snaps, buffSnap)
		}
		psp, err := s.psp.NightsPSPRecovery(ctx, characterID)
		if err != nil {
			return fmt.Errorf("psp recovery: %w", err)
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
			PSPDelta:    psp,
		})
		if err != nil {
			return fmt.Errorf("record long rest: %w", err)
		}
		warnings, err := s.items.Warnings(ctx, characterID, items)
		if err != nil {
			return fmt.Errorf("class warnings: %w", err)
		}
		res = Result{Slots: slots, Items: items, Warnings: warnings, EntryID: id}
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
