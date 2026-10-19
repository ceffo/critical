package combatsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ItemLister lists a character's equipment (itemsvc.SVC).
type ItemLister interface {
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
}

// ItemWeapons is the Weapons port over the equipment (ADR 0033 step 1): the
// weapon pieces, read through weaponOf. The composition root wires it.
func ItemWeapons(items ItemLister) Weapons { return itemWeapons{items} }

type itemWeapons struct{ items ItemLister }

func (w itemWeapons) List(ctx context.Context, characterID int64) ([]Weapon, error) {
	items, err := w.items.List(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := []Weapon{}
	for _, it := range items {
		if it.Category == itemsvc.CategoryWeapon {
			out = append(out, weaponOf(it))
		}
	}
	return out, nil
}

// Get is one weapon piece; a piece of another category is not a weapon
// (ErrWeaponNotFound).
func (w itemWeapons) Get(ctx context.Context, characterID int64, id string) (Weapon, error) {
	weapons, err := w.List(ctx, characterID)
	if err != nil {
		return Weapon{}, err
	}
	if found, ok := findWeapon(weapons, id); ok {
		return found, nil
	}
	return Weapon{}, fmt.Errorf("%w: %s", ErrWeaponNotFound, id)
}
