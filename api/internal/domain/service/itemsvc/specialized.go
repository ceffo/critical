package itemsvc

import (
	"context"
	"slices"
)

// specializedDAL is the DAL as the service reads it: every weapon piece read carries its
// Specialized derived from the character's weapon picks (ADR 0037 step 9, amending ADR 0030
// step 12: a weapon is specialized when its type has the fighter's specialization pick). The
// stored per-weapon flag is ignored: the item DAL neither reads nor writes it. A read with no
// weapon piece reads no character.
type specializedDAL struct {
	DAL
	chars Characters
}

func (d specializedDAL) List(ctx context.Context, characterID int64) ([]Item, error) {
	items, err := d.DAL.List(ctx, characterID)
	if err != nil || !slices.ContainsFunc(items, isWeapon) {
		return items, err
	}
	return items, d.derive(ctx, characterID, items)
}

func (d specializedDAL) Get(ctx context.Context, characterID int64, itemID string) (Item, error) {
	return d.one(ctx, characterID)(d.DAL.Get(ctx, characterID, itemID))
}

func (d specializedDAL) Decrement(ctx context.Context, characterID int64, itemID string) (Item, error) {
	return d.one(ctx, characterID)(d.DAL.Decrement(ctx, characterID, itemID))
}

func (d specializedDAL) Increment(ctx context.Context, characterID int64, itemID string, n int) (Item, error) {
	return d.one(ctx, characterID)(d.DAL.Increment(ctx, characterID, itemID, n))
}

// one derives a single piece read.
func (d specializedDAL) one(ctx context.Context, characterID int64) func(Item, error) (Item, error) {
	return func(it Item, err error) (Item, error) {
		if err != nil || !isWeapon(it) {
			return it, err
		}
		items := []Item{it}
		if err := d.derive(ctx, characterID, items); err != nil {
			return Item{}, err
		}
		return items[0], nil
	}
}

// derive sets every weapon piece's Specialized from the character's picks.
func (d specializedDAL) derive(ctx context.Context, characterID int64, items []Item) error {
	c, err := d.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].Specialized = isWeapon(items[i]) && c.Specialized(items[i].WeaponType)
	}
	return nil
}

func isWeapon(it Item) bool { return it.Category == CategoryWeapon }

// specialized is a piece the service built (a creation, an edit) as a read serves it: a weapon's
// Specialized derived from the character's picks for its weapon type.
func (s *service) specialized(ctx context.Context, characterID int64, it Item) (Item, error) {
	return specializedDAL{chars: s.chars}.one(ctx, characterID)(it, nil)
}
