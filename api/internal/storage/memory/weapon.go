package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
)

// Weapons is the weapon catalogue as combat reads it (combatsvc.Weapons).
type Weapons struct{ s *Store }

// WeaponEditor is the same table as Weapons with the position and the write
// side (weaponsvc.DAL).
type WeaponEditor struct{ s *Store }

var (
	_ combatsvc.Weapons = Weapons{}
	_ weaponsvc.DAL     = WeaponEditor{}
)

// Weapons returns the weapon port of combat.
func (s *Store) Weapons() Weapons { return Weapons{s} }

// WeaponEditor returns the weapon port of the editor.
func (s *Store) WeaponEditor() WeaponEditor { return WeaponEditor{s} }

func (r storedWeapon) edited() weaponsvc.Weapon {
	w := clone(r.Weapon)
	w.Name = named(w.Name)
	return w
}

func (r storedWeapon) combat() combatsvc.Weapon {
	w := r.edited()
	return combatsvc.Weapon{
		ID: w.ID, Name: w.Name, BaseDamage: w.DamageSM, DamageLarge: w.DamageL,
		MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: combatsvc.WeaponKind(w.Kind),
		TwoHanded: w.TwoHanded, AmmoKind: w.AmmoKind,
	}
}

// sorted is every weapon in sheet order.
func (t *tables) sortedWeapons() []storedWeapon {
	rows := make([]storedWeapon, 0, len(t.weapons))
	for _, r := range t.weapons {
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b storedWeapon) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.row, b.row))
	})
	return rows
}

// weapon is the stored weapon id; notFound is the error wrapped when the id
// is unknown.
func (t *tables) weapon(id string, notFound error) (storedWeapon, error) {
	r, ok := t.weapons[id]
	if !ok {
		return storedWeapon{}, fmt.Errorf("%w: %s", notFound, id)
	}
	return r, nil
}

// Get returns the weapon or an error wrapping combatsvc.ErrWeaponNotFound.
func (p Weapons) Get(ctx context.Context, id string) (combatsvc.Weapon, error) {
	var out combatsvc.Weapon
	err := p.s.read(ctx, func(t *tables) error {
		r, err := t.weapon(id, combatsvc.ErrWeaponNotFound)
		out = r.combat()
		return err
	})
	return out, err
}

// List returns every weapon in sheet order.
func (p Weapons) List(ctx context.Context) ([]combatsvc.Weapon, error) {
	out := []combatsvc.Weapon{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, r := range t.sortedWeapons() {
			out = append(out, r.combat())
		}
		return nil
	})
	return out, err
}

// List returns every weapon in sheet order.
func (p WeaponEditor) List(ctx context.Context) ([]weaponsvc.Weapon, error) {
	out := []weaponsvc.Weapon{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, r := range t.sortedWeapons() {
			out = append(out, r.edited())
		}
		return nil
	})
	return out, err
}

// Get returns one weapon or an error wrapping weaponsvc.ErrWeaponNotFound.
func (p WeaponEditor) Get(ctx context.Context, id string) (weaponsvc.Weapon, error) {
	var out weaponsvc.Weapon
	err := p.s.read(ctx, func(t *tables) error {
		r, err := t.weapon(id, weaponsvc.ErrWeaponNotFound)
		out = r.edited()
		return err
	})
	return out, err
}

// Insert writes a whole weapon (a new one, or a void restoring a removed
// one); an id the store holds is refused.
func (p WeaponEditor) Insert(ctx context.Context, w weaponsvc.Weapon) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, taken := t.weapons[w.ID]; taken {
			return errDuplicate("weapons", w.ID)
		}
		w = clone(w)
		w.Name = stored(w.Name)
		t.weapons[w.ID] = storedWeapon{Weapon: w, row: nextID(&t.rowSeq)}
		return nil
	})
}

// Update overwrites every field of an existing weapon, its name included.
func (p WeaponEditor) Update(ctx context.Context, w weaponsvc.Weapon) error {
	return p.s.write(ctx, func(t *tables) error {
		r, err := t.weapon(w.ID, weaponsvc.ErrWeaponNotFound)
		if err != nil {
			return err
		}
		w = clone(w)
		w.Name = stored(w.Name)
		t.weapons[w.ID] = storedWeapon{Weapon: w, row: r.row}
		return nil
	})
}

// Delete removes a weapon and its name.
func (p WeaponEditor) Delete(ctx context.Context, id string) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, err := t.weapon(id, weaponsvc.ErrWeaponNotFound); err != nil {
			return err
		}
		delete(t.weapons, id)
		return nil
	})
}
