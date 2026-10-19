package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// Items is itemsvc.DAL over the store. Powers are kept in itemsvc's stored
// encoding and decoded on read, as the database adapter does.
type Items struct{ s *Store }

var _ itemsvc.DAL = Items{}

// Items returns the item port.
func (s *Store) Items() Items { return Items{s} }

func (r storedItem) item() (itemsvc.Item, error) {
	it := clone(r.Item)
	it.Name = named(it.Name)
	powers, err := itemsvc.DecodePowers(r.powers)
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("item %s: %w", it.ID, err)
	}
	it.Powers = powers
	return it, nil
}

// toStored is it as the store keeps it, its row order taken from seq.
func toStored(it itemsvc.Item, row int64) (storedItem, error) {
	powers, err := itemsvc.EncodePowers(it.Powers)
	if err != nil {
		return storedItem{}, err
	}
	it = clone(it)
	it.Name = stored(it.Name)
	it.Powers = nil
	return storedItem{Item: it, powers: powers, row: row}, nil
}

// List returns the character's items in sheet order.
func (p Items) List(ctx context.Context, characterID int64) ([]itemsvc.Item, error) {
	out := []itemsvc.Item{}
	err := p.s.read(ctx, func(t *tables) error {
		rows := make([]storedItem, 0, len(t.items))
		for k, r := range t.items {
			if k.characterID == characterID {
				rows = append(rows, r)
			}
		}
		slices.SortFunc(rows, func(a, b storedItem) int {
			return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.row, b.row))
		})
		for _, r := range rows {
			it, err := r.item()
			if err != nil {
				return err
			}
			out = append(out, it)
		}
		return nil
	})
	return out, err
}

// Get returns one item, or an error wrapping itemsvc.ErrItemNotFound.
func (p Items) Get(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error) {
	var out itemsvc.Item
	err := p.s.read(ctx, func(t *tables) error {
		r, ok := t.items[owned{characterID, itemID}]
		if !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		var err error
		out, err = r.item()
		return err
	})
	return out, err
}

// change applies fn to the character's stored item itemID and returns it as
// read back; an unknown id is an error wrapping itemsvc.ErrItemNotFound.
func (p Items) change(ctx context.Context, characterID int64, itemID string, fn func(r *storedItem) error) (itemsvc.Item, error) {
	key := owned{characterID, itemID}
	var out itemsvc.Item
	err := p.s.write(ctx, func(t *tables) error {
		r, ok := t.items[key]
		if !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		if err := fn(&r); err != nil {
			return err
		}
		t.items[key] = r
		var err error
		out, err = r.item()
		return err
	})
	return out, err
}

// Update overwrites every field of an existing item, powers and name
// included; the row keeps its place among equal positions.
func (p Items) Update(ctx context.Context, characterID int64, it itemsvc.Item) error {
	_, err := p.change(ctx, characterID, it.ID, func(r *storedItem) error {
		next, err := toStored(it, r.row)
		*r = next
		return err
	})
	return err
}

// Decrement spends one count; an item at 0 is refused with an error wrapping
// itemsvc.ErrItemEmpty, so the count never goes below 0.
func (p Items) Decrement(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error) {
	return p.change(ctx, characterID, itemID, func(r *storedItem) error {
		if r.Count <= 0 {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemEmpty, itemID)
		}
		r.Count--
		return nil
	})
}

// Increment adds n to the count and returns the item.
func (p Items) Increment(ctx context.Context, characterID int64, itemID string, n int) (itemsvc.Item, error) {
	return p.change(ctx, characterID, itemID, func(r *storedItem) error {
		r.Count += n
		return nil
	})
}

// ResetDaily refills every daily item of the character to its max count.
func (p Items) ResetDaily(ctx context.Context, characterID int64) error {
	return p.s.write(ctx, func(t *tables) error {
		for k, r := range t.items {
			if k.characterID == characterID && r.Behaviour == itemsvc.BehaviourDaily {
				r.Count = r.MaxCount
				t.items[k] = r
			}
		}
		return nil
	})
}

// SetCount writes an item's count back (an action-log void restoring a
// snapshot).
func (p Items) SetCount(ctx context.Context, characterID int64, itemID string, count int) error {
	_, err := p.change(ctx, characterID, itemID, func(r *storedItem) error {
		r.Count = count
		return nil
	})
	return err
}

// Delete removes an item and its name.
func (p Items) Delete(ctx context.Context, characterID int64, itemID string) error {
	key := owned{characterID, itemID}
	return p.s.write(ctx, func(t *tables) error {
		if _, ok := t.items[key]; !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		delete(t.items, key)
		return nil
	})
}

// Insert writes a whole item for the character (a new one, or a void
// restoring a removed one), powers included; an id the character holds is
// refused.
func (p Items) Insert(ctx context.Context, characterID int64, it itemsvc.Item) error {
	key := owned{characterID, it.ID}
	return p.s.write(ctx, func(t *tables) error {
		if _, taken := t.items[key]; taken {
			return errDuplicate("items", it.ID)
		}
		r, err := toStored(it, nextID(&t.rowSeq))
		if err != nil {
			return err
		}
		t.items[key] = r
		return nil
	})
}

// Reviews returns the character's review rows (ADR 0033 step 27).
func (p Items) Reviews(ctx context.Context, characterID int64) ([]itemsvc.Review, error) {
	out := []itemsvc.Review{}
	err := p.s.read(ctx, func(t *tables) error {
		for r := range t.reviews {
			if r.characterID == characterID {
				out = append(out, itemsvc.Review{ItemID: r.itemID, Reason: r.reason})
			}
		}
		return nil
	})
	slices.SortFunc(out, func(a, b itemsvc.Review) int {
		return cmp.Or(cmp.Compare(a.ItemID, b.ItemID), cmp.Compare(a.Reason, b.Reason))
	})
	return out, err
}

// DeleteReviews removes every review row of a piece.
func (p Items) DeleteReviews(ctx context.Context, characterID int64, itemID string) error {
	return p.s.write(ctx, func(t *tables) error {
		for r := range t.reviews {
			if r.characterID == characterID && r.itemID == itemID {
				delete(t.reviews, r)
			}
		}
		return nil
	})
}
