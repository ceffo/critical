package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// MaxRestock is the most one restock may add; the stock itself stops at MaxQuantity.
const MaxRestock = 999

// RestockResult is one restock of a stock item.
type RestockResult struct {
	ItemID     string
	Name       string
	Type       string
	Added      int
	Count      int // the count after the restock
	LogEntryID int64
}

// restockRecord is the item_restock log payload, the same camelCase names as
// the /api/items/{id}/restock response. Add fields, never rename.
type restockRecord struct {
	ItemID string `json:"itemId"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Added  int    `json:"added"`
	Count  int    `json:"count"`
}

// Restock adds n to a stock item (the inventory's +, buying arrows). The
// increment is one atomic conditional statement (the DAL refuses past MaxQuantity), after the refusals are read. One item_restock entry with the item's snapshot: popping it takes
// the count back down.
func (s *service) Restock(ctx context.Context, characterID int64, itemID string, n int) (*RestockResult, error) {
	if n < 1 || n > MaxRestock {
		return nil, fmt.Errorf("%w: restock count must be 1 to %d", domain.ErrInvalidInput, MaxRestock)
	}
	var res *RestockResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		// Read first and refuse before any write: only a stock goes up (ADR 0049
		// step 2), and not the one an item with a per_day or charges ability is
		// (its counter belongs to the one object, step 4).
		before, err := s.dal.Get(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		if !before.Usable() || before.holdsPerObjectLimit() {
			return fmt.Errorf("%w: %s (%s) has no stock to restock", ErrNotRestockable, before.Name.Any(), before.Category)
		}
		if before.Count+n > MaxQuantity {
			return fmt.Errorf("%w: a quantity is at most %d (%d held, %d added)", domain.ErrInvalidInput, MaxQuantity, before.Count, n)
		}
		item, err := s.dal.Increment(ctx, characterID, itemID, n)
		if err != nil {
			return err
		}
		snap, err := CountSnapshot(item.ID, item.Count-n, item.Count)
		if err != nil {
			return err
		}
		r := &RestockResult{ItemID: item.ID, Name: item.Name.Any(), Type: item.Type, Added: n, Count: item.Count}
		payload, err := json.Marshal(restockRecord{ItemID: r.ItemID, Name: r.Name, Type: r.Type, Added: r.Added, Count: r.Count})
		if err != nil {
			return fmt.Errorf("encode restock payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindItemRestock,
			Payload:     payload,
			Snapshots:   []actionlog.Snapshot{snap},
		})
		if err != nil {
			return fmt.Errorf("record restock: %w", err)
		}
		r.LogEntryID = id
		res = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
