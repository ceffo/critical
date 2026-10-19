package combatsvc

import (
	"context"
	"fmt"
)

// InHand is the weapon in hand outside an attack (ADR 0033 step 8): the
// attack form's weapon as the form resolves it (the stored one while it is
// wielded, none once it was put away, else the first wielded weapon). ok is
// false when no weapon is in hand. The initiative reads it (ADR 0035 item
// 6) through the composition root, without the service.
func InHand(ctx context.Context, forms FormStore, weapons Weapons, characterID int64) (weapon Weapon, ok bool, err error) {
	form, stored, err := forms.Get(ctx, characterID)
	if err != nil {
		return Weapon{}, false, fmt.Errorf("get attack form: %w", err)
	}
	if !stored {
		form = DefaultAttackForm()
	}
	list, err := weapons.List(ctx, characterID)
	if err != nil {
		return Weapon{}, false, fmt.Errorf("list weapons: %w", err)
	}
	held, ok := resolveWeapon(list, form.WeaponID)
	if !ok {
		return Weapon{}, false, nil
	}
	return *held, true, nil
}
