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

// List returns every item in sheet order.
func (p Items) List(ctx context.Context) ([]itemsvc.Item, error) {
	out := []itemsvc.Item{}
	err := p.s.read(ctx, func(t *tables) error {
		rows := make([]storedItem, 0, len(t.items))
		for _, r := range t.items {
			rows = append(rows, r)
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
func (p Items) Get(ctx context.Context, itemID string) (itemsvc.Item, error) {
	var out itemsvc.Item
	err := p.s.read(ctx, func(t *tables) error {
		r, ok := t.items[itemID]
		if !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		var err error
		out, err = r.item()
		return err
	})
	return out, err
}

// change applies fn to the stored item itemID and returns it as read back;
// an unknown id is an error wrapping itemsvc.ErrItemNotFound.
func (p Items) change(ctx context.Context, itemID string, fn func(r *storedItem) error) (itemsvc.Item, error) {
	var out itemsvc.Item
	err := p.s.write(ctx, func(t *tables) error {
		r, ok := t.items[itemID]
		if !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		if err := fn(&r); err != nil {
			return err
		}
		t.items[itemID] = r
		var err error
		out, err = r.item()
		return err
	})
	return out, err
}

// Update overwrites every field of an existing item, powers and name
// included; the row keeps its place among equal positions.
func (p Items) Update(ctx context.Context, it itemsvc.Item) error {
	_, err := p.change(ctx, it.ID, func(r *storedItem) error {
		next, err := toStored(it, r.row)
		*r = next
		return err
	})
	return err
}

// Decrement spends one count; an item at 0 is refused with an error wrapping
// itemsvc.ErrItemEmpty, so the count never goes below 0.
func (p Items) Decrement(ctx context.Context, itemID string) (itemsvc.Item, error) {
	return p.change(ctx, itemID, func(r *storedItem) error {
		if r.Count <= 0 {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemEmpty, itemID)
		}
		r.Count--
		return nil
	})
}

// Increment adds n to the count and returns the item.
func (p Items) Increment(ctx context.Context, itemID string, n int) (itemsvc.Item, error) {
	return p.change(ctx, itemID, func(r *storedItem) error {
		r.Count += n
		return nil
	})
}

// ResetDaily refills every daily item to its max count.
func (p Items) ResetDaily(ctx context.Context) error {
	return p.s.write(ctx, func(t *tables) error {
		for id, r := range t.items {
			if r.Behaviour == itemsvc.BehaviourDaily {
				r.Count = r.MaxCount
				t.items[id] = r
			}
		}
		return nil
	})
}

// SetCount writes an item's count back (an action-log void restoring a
// snapshot).
func (p Items) SetCount(ctx context.Context, itemID string, count int) error {
	_, err := p.change(ctx, itemID, func(r *storedItem) error {
		r.Count = count
		return nil
	})
	return err
}

// Delete removes an item and its name.
func (p Items) Delete(ctx context.Context, itemID string) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, ok := t.items[itemID]; !ok {
			return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
		}
		delete(t.items, itemID)
		return nil
	})
}

// Insert writes a whole item (a void restoring a removed item), powers
// included; an id the store holds is refused.
func (p Items) Insert(ctx context.Context, it itemsvc.Item) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, taken := t.items[it.ID]; taken {
			return errDuplicate("items", it.ID)
		}
		r, err := toStored(it, nextID(&t.rowSeq))
		if err != nil {
			return err
		}
		t.items[it.ID] = r
		return nil
	})
}
