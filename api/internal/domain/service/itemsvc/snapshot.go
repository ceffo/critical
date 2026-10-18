package itemsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// StateKeyItems is the action-log state-key family for item counts. Each
// item has its own key, StateKey(id) = "items:<id>", so voiding a use is
// only blocked by later uses of the same item. Services that spend items (an
// attack firing an arrow) record a snapshot with UseSnapshot; the Restorer
// registered for the family writes the "before" counts back on void.
const StateKeyItems = "items"

// StateKey is the action-log state key of one item's count.
func StateKey(itemID string) string { return StateKeyItems + actionlog.KeySeparator + itemID }

// snapCount is itemsvc's JSON encoding of one item count inside a snapshot.
// Entries already in the log must stay decodable: add fields, never rename.
// Item is the whole item as it was (a use's "before"), so a pop can put back
// an item the use removed. Removed marks an "after" in which the item is gone.
type snapCount struct {
	ID      string    `json:"id"`
	Count   int       `json:"count"`
	Item    *snapItem `json:"item,omitempty"`
	Removed bool      `json:"removed,omitempty"`
	// Absent marks a "before" in which the item did not exist yet (an item
	// added by hand): restoring it deletes the item.
	Absent bool `json:"absent,omitempty"`
	// Replace marks a snapshot whose Item is the whole truth (edit, remove):
	// restoring overwrites every field, or inserts the item when it is gone.
	Replace bool `json:"replace,omitempty"`
}

// snapItem is the whole item inside a snapshot (add fields, never rename).
type snapItem struct {
	Position int `json:"position"`
	// Name is the item's name by language (ADR 0023 step 11); a snapshot
	// taken before names were localized holds a plain string.
	Name       localized.Snap `json:"name"`
	Type       string         `json:"type"`
	Behaviour  string         `json:"behaviour"`
	MagicBonus int            `json:"magicalBonus"`
	AmmoKind   string         `json:"ammoKind"`
	MaxCount   int            `json:"maxCount"`
	EffectDice string         `json:"effectDice"`
	EffectSave string         `json:"effectSave"`
	Notes      string         `json:"notes"`
	// Powers are the item's own powers (ADR 0016 step 15), always written
	// ([] for none); nil in a snapshot taken before they existed, which then
	// leaves the stored powers as they are (see keepCurrent).
	Powers *[]jsonPower `json:"powers,omitempty"`
}

func toSnapItem(i Item) *snapItem {
	powers := toJSONPowers(i.Powers)
	return &snapItem{
		Position: i.Position, Name: localized.Snap{Text: i.Name}, Type: i.Type, Behaviour: string(i.Behaviour),
		MagicBonus: i.MagicBonus, AmmoKind: i.AmmoKind, MaxCount: i.MaxCount,
		EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: &powers,
	}
}

func (c snapCount) item() Item {
	it := c.Item
	out := Item{
		ID: c.ID, Position: it.Position, Name: it.Name.Text, Type: it.Type, Behaviour: Behaviour(it.Behaviour),
		MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind, Count: c.Count, MaxCount: it.MaxCount,
		EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes,
	}
	if it.Powers != nil {
		out.Powers = fromJSONPowers(*it.Powers)
	}
	return out
}

// keepCurrent is the item a Replace snapshot writes back: when the snapshot
// predates powers (no "powers" key), the item keeps the powers it has now,
// so popping an old edit never strips the ones a migration gave it; when it
// predates localized names (a plain string), that string is the fallback
// body and the other languages the item has now are kept (ADR 0023 step
// 11). An item that is gone has nothing to keep (ADR 0016 Consequences).
func (s *service) keepCurrent(ctx context.Context, characterID int64, c snapCount) (Item, error) {
	item := c.item()
	if c.Item.Powers != nil && !c.Item.Name.Plain {
		return item, nil
	}
	cur, err := s.dal.Get(ctx, characterID, c.ID)
	switch {
	case errors.Is(err, ErrItemNotFound):
		return item, nil
	case err != nil:
		return Item{}, err
	}
	if c.Item.Powers == nil {
		item.Powers = cur.Powers
	}
	if c.Item.Name.Plain {
		item.Name = c.Item.Name.Restored(cur.Name)
	}
	return item, nil
}

// UseSnapshot is the snapshot of one Use: after is the item Use returned,
// the "before" count is one more (Use always spends exactly one). When the
// use removed the item (a charges item spent to 0), "before" carries the
// whole item, so popping the use brings it back with 1 charge.
func UseSnapshot(after Item) (actionlog.Snapshot, error) {
	before := snapCount{ID: after.ID, Count: after.Count + 1}
	if after.Removed() {
		before.Item = toSnapItem(after)
	}
	return encodeSnapshot(after.ID, before, snapCount{ID: after.ID, Count: after.Count, Removed: after.Removed()})
}

// CountSnapshot is the snapshot of one item's count going from before to
// after (a long rest refilling a daily item, a restock).
func CountSnapshot(itemID string, before, after int) (actionlog.Snapshot, error) {
	return encodeSnapshot(itemID, snapCount{ID: itemID, Count: before}, snapCount{ID: itemID, Count: after})
}

// itemSnapshot is the snapshot of an item going from before to after by hand
// (nil = it did not exist / no longer exists). Restoring "before" undoes it:
// a nil before deletes the item, any other before overwrites it or puts it
// back.
func itemSnapshot(itemID string, before, after *Item) (actionlog.Snapshot, error) {
	return encodeSnapshot(itemID, wholeItem(itemID, before), wholeItem(itemID, after))
}

func wholeItem(id string, i *Item) snapCount {
	if i == nil {
		return snapCount{ID: id, Absent: true, Removed: true}
	}
	return snapCount{ID: id, Count: i.Count, Item: toSnapItem(*i), Replace: true}
}

func encodeSnapshot(itemID string, before, after snapCount) (actionlog.Snapshot, error) {
	b, err := json.Marshal([]snapCount{before})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	a, err := json.Marshal([]snapCount{after})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	return actionlog.Snapshot{Key: StateKey(itemID), Before: b, After: a}, nil
}

// restoreFunc adapts a function to actionlog.Restorer.
type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes the counts of an items snapshot back (registered for the
// StateKeyItems family, so it serves every "items:<id>" key). An item that is
// gone (a charges item its use removed) is inserted back whole when the
// snapshot carries it; a state in which the item is gone (Absent or Removed)
// deletes it. It runs inside the void's transaction (ctx carries
// it). The restored item is the entry's character's.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyItems: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var counts []snapCount
			if err := json.Unmarshal(state, &counts); err != nil {
				return fmt.Errorf("decode items snapshot: %w", err)
			}
			for _, c := range counts {
				if c.ID == "" || c.Count < 0 {
					return fmt.Errorf("decode items snapshot: invalid count %+v", c)
				}
			}
			for _, c := range counts {
				var err error
				switch {
				case c.Absent, c.Removed:
					// Gone in this state: an add's "before", a remove's
					// "after", the "after" of a use that spent the last
					// charge (written back by a redo, ADR 0019 step 1).
					err = s.dal.Delete(ctx, characterID, c.ID)
				case c.Replace && c.Item != nil:
					var item Item
					if item, err = s.keepCurrent(ctx, characterID, c); err != nil {
						break
					}
					err = s.dal.Update(ctx, characterID, item)
					if errors.Is(err, ErrItemNotFound) {
						err = s.dal.Insert(ctx, characterID, item)
					}
				default:
					// The item is inserted only when it is gone, so there
					// are no stored powers to keep: a snapshot from before
					// powers brings it back without them (ADR 0016
					// Consequences).
					err = s.dal.SetCount(ctx, characterID, c.ID, c.Count)
					if errors.Is(err, ErrItemNotFound) && c.Item != nil {
						err = s.dal.Insert(ctx, characterID, c.item())
					}
				}
				if err != nil {
					return fmt.Errorf("restore item %s: %w", c.ID, err)
				}
			}
			return nil
		}),
	}
}
