package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// MaxRestock is the most one restock may add.
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
// increment is one atomic statement; a charges or daily item is rejected
// after it and the transaction rolls the increment back, so its count never
// moves. One item_restock entry with the item's snapshot: popping it takes
// the count back down.
func (s *service) Restock(ctx context.Context, characterID int64, itemID string, n int) (*RestockResult, error) {
	if n < 1 || n > MaxRestock {
		return nil, fmt.Errorf("%w: restock count must be 1 to %d", domain.ErrInvalidInput, MaxRestock)
	}
	var res *RestockResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		item, err := s.dal.Increment(ctx, itemID, n)
		if err != nil {
			return err
		}
		if !item.Behaviour.Restockable() {
			return fmt.Errorf("%w: %s (%s) is only used, never restocked", ErrNotRestockable, item.Name, item.Behaviour)
		}
		snap, err := CountSnapshot(item.ID, item.Count-n, item.Count)
		if err != nil {
			return err
		}
		r := &RestockResult{ItemID: item.ID, Name: item.Name, Type: item.Type, Added: n, Count: item.Count}
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
