package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ItemInput is a new item. MaxCount is the daily number (daily items only;
// for the others it is stored as the starting count, informational).
type ItemInput struct {
	Name       string
	Type       string
	Behaviour  Behaviour
	Count      int
	MaxCount   int
	MagicBonus int
	AmmoKind   string
	EffectDice string
	EffectSave string
	Notes      string
}

// ItemPatch is an edit: only the set (non-nil) fields change.
type ItemPatch struct {
	Name       *string
	Type       *string
	Behaviour  *Behaviour
	Count      *int
	MaxCount   *int
	MagicBonus *int
	AmmoKind   *string
	EffectDice *string
	EffectSave *string
	Notes      *string
}

// ChangeResult is one add, edit or remove: the item as it is now (as it was,
// for a remove) and the log entry written.
type ChangeResult struct {
	Item       Item
	LogEntryID int64
}

// FieldChange is one edited field in an item_edit payload (values as text).
type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// changeRecord is the item_add / item_edit / item_remove log payload (camelCase,
// the /api/items wire names). Changes only in item_edit. Add fields, never rename.
type changeRecord struct {
	ItemID     string        `json:"itemId"`
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Behaviour  string        `json:"behaviour"`
	Count      int           `json:"count"`
	MaxCount   int           `json:"maxCount"`
	MagicBonus int           `json:"magicalBonus"`
	AmmoKind   string        `json:"ammoKind"`
	EffectDice string        `json:"effectDice"`
	EffectSave string        `json:"effectSave"`
	Notes      string        `json:"notes"`
	Changes    []FieldChange `json:"changes,omitempty"`
}

func record(i Item, changes []FieldChange) changeRecord {
	return changeRecord{
		ItemID: i.ID, Name: i.Name, Type: i.Type, Behaviour: string(i.Behaviour), Count: i.Count,
		MaxCount: i.MaxCount, MagicBonus: i.MagicBonus, AmmoKind: i.AmmoKind,
		EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Changes: changes,
	}
}

// slug turns a name into an id: lower-case letters and digits, "_" between.
func slug(name string) string {
	var b strings.Builder
	sep := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte('_')
			}
			sep = false
			b.WriteRune(r)
		} else {
			sep = true
		}
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

// logChange records one entry with the item's snapshot.
func (s *service) logChange(ctx context.Context, characterID int64, kind actionlog.Kind, rec changeRecord, before, after *Item) (int64, error) {
	snap, err := itemSnapshot(rec.ItemID, before, after)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", kind, err)
	}
	id, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: kind, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	})
	if err != nil {
		return 0, fmt.Errorf("record %s: %w", kind, err)
	}
	return id, nil
}

func (s *service) Create(ctx context.Context, characterID int64, in ItemInput) (*ChangeResult, error) {
	item := Item{
		Name: strings.TrimSpace(in.Name), Type: strings.TrimSpace(in.Type), Behaviour: in.Behaviour,
		Count: in.Count, MaxCount: in.MaxCount, MagicBonus: in.MagicBonus,
		AmmoKind: strings.TrimSpace(in.AmmoKind), EffectDice: strings.TrimSpace(in.EffectDice),
		EffectSave: in.EffectSave, Notes: strings.TrimSpace(in.Notes),
	}
	if item.Behaviour != BehaviourDaily {
		item.MaxCount = item.Count
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.dal.List(ctx)
		if err != nil {
			return err
		}
		taken, pos := map[string]bool{}, 0
		for _, o := range all {
			taken[o.ID] = true
			if o.Position >= pos {
				pos = o.Position + 1
			}
		}
		base := slug(item.Name)
		item.ID = base
		for n := 2; taken[item.ID]; n++ {
			item.ID = base + "_" + strconv.Itoa(n)
		}
		item.Position = pos
		if err := s.dal.Insert(ctx, item); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemAdd, record(item, nil), nil, &item)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: item, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) Update(ctx context.Context, characterID int64, itemID string, p ItemPatch) (*ChangeResult, error) {
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, itemID)
		if err != nil {
			return err
		}
		item := old
		setStr := func(dst *string, v *string) {
			if v != nil {
				*dst = strings.TrimSpace(*v)
			}
		}
		setStr(&item.Name, p.Name)
		setStr(&item.Type, p.Type)
		setStr(&item.AmmoKind, p.AmmoKind)
		setStr(&item.EffectDice, p.EffectDice)
		setStr(&item.Notes, p.Notes)
		if p.EffectSave != nil {
			item.EffectSave = *p.EffectSave
		}
		if p.Behaviour != nil {
			item.Behaviour = *p.Behaviour
		}
		if p.Count != nil {
			item.Count = *p.Count
		}
		if p.MaxCount != nil {
			item.MaxCount = *p.MaxCount
		}
		if p.MagicBonus != nil {
			item.MagicBonus = *p.MagicBonus
		}
		// Becoming daily needs an explicit daily number: the stored max of a stock or
		// charges item is informational.
		if item.Behaviour == BehaviourDaily && old.Behaviour != BehaviourDaily && p.MaxCount == nil {
			item.MaxCount = 0
		}
		// A non-daily item's max is informational: keep it from going below the count.
		if item.Behaviour != BehaviourDaily && p.MaxCount == nil && item.Count > item.MaxCount {
			item.MaxCount = item.Count
		}
		if err := item.Validate(); err != nil {
			return err
		}
		changes := diff(old, item)
		if len(changes) == 0 {
			return fmt.Errorf("%w: nothing changed", domain.ErrInvalidInput)
		}
		if err := s.dal.Update(ctx, item); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemEdit, record(item, changes), &old, &item)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: item, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) Delete(ctx context.Context, characterID int64, itemID string) (*ChangeResult, error) {
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, itemID)
		if err != nil {
			return err
		}
		if err := s.dal.Delete(ctx, itemID); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemRemove, record(old, nil), &old, nil)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: old, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// diff lists the fields that differ, in form order.
func diff(a, b Item) []FieldChange {
	var out []FieldChange
	add := func(field, x, y string) {
		if x != y {
			out = append(out, FieldChange{Field: field, Before: x, After: y})
		}
	}
	add("name", a.Name, b.Name)
	add("type", a.Type, b.Type)
	add("behaviour", string(a.Behaviour), string(b.Behaviour))
	add("count", strconv.Itoa(a.Count), strconv.Itoa(b.Count))
	add("maxCount", strconv.Itoa(a.MaxCount), strconv.Itoa(b.MaxCount))
	add("magicalBonus", strconv.Itoa(a.MagicBonus), strconv.Itoa(b.MagicBonus))
	add("ammoKind", a.AmmoKind, b.AmmoKind)
	add("effectDice", a.EffectDice, b.EffectDice)
	add("effectSave", a.EffectSave, b.EffectSave)
	add("notes", a.Notes, b.Notes)
	return out
}
