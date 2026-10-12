package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
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
type snapCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// UseSnapshot is the snapshot of one Use: after is the item Use returned,
// the "before" count is one more (Use always spends exactly one).
func UseSnapshot(after Item) (actionlog.Snapshot, error) {
	before, err := json.Marshal([]snapCount{{ID: after.ID, Count: after.Count + 1}})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	now, err := json.Marshal([]snapCount{{ID: after.ID, Count: after.Count}})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	return actionlog.Snapshot{Key: StateKey(after.ID), Before: before, After: now}, nil
}

// restoreFunc adapts a function to actionlog.Restorer.
type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes the counts of an items snapshot back (registered for the
// StateKeyItems family, so it serves every "items:<id>" key). It runs inside the
// void's transaction (ctx carries it). Items are not per character, so the
// character id is unused.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyItems: restoreFunc(func(ctx context.Context, _ int64, state json.RawMessage) error {
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
				if err := s.dal.SetCount(ctx, c.ID, c.Count); err != nil {
					return fmt.Errorf("restore item %s: %w", c.ID, err)
				}
			}
			return nil
		}),
	}
}
