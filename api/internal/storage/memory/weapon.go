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
		TwoHanded: w.TwoHanded, AmmoKind: w.AmmoKind, Type: w.Type, Specialized: w.Specialized,
	}
}

// sortedWeapons is every weapon of the character in sheet order.
func (t *tables) sortedWeapons(characterID int64) []storedWeapon {
	rows := make([]storedWeapon, 0, len(t.weapons))
	for k, r := range t.weapons {
		if k.characterID == characterID {
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b storedWeapon) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.row, b.row))
	})
	return rows
}

// weapon is the character's stored weapon id; notFound is the error wrapped
// when the id is unknown.
func (t *tables) weapon(characterID int64, id string, notFound error) (storedWeapon, error) {
	r, ok := t.weapons[owned{characterID, id}]
	if !ok {
		return storedWeapon{}, fmt.Errorf("%w: %s", notFound, id)
	}
	return r, nil
}

// Get returns the weapon or an error wrapping combatsvc.ErrWeaponNotFound.
func (p Weapons) Get(ctx context.Context, characterID int64, id string) (combatsvc.Weapon, error) {
	var out combatsvc.Weapon
	err := p.s.read(ctx, func(t *tables) error {
		r, err := t.weapon(characterID, id, combatsvc.ErrWeaponNotFound)
		out = r.combat()
		return err
	})
	return out, err
}

// List returns the character's weapons in sheet order.
func (p Weapons) List(ctx context.Context, characterID int64) ([]combatsvc.Weapon, error) {
	out := []combatsvc.Weapon{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, r := range t.sortedWeapons(characterID) {
			out = append(out, r.combat())
		}
		return nil
	})
	return out, err
}

// List returns the character's weapons in sheet order.
func (p WeaponEditor) List(ctx context.Context, characterID int64) ([]weaponsvc.Weapon, error) {
	out := []weaponsvc.Weapon{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, r := range t.sortedWeapons(characterID) {
			out = append(out, r.edited())
		}
		return nil
	})
	return out, err
}

// Get returns one weapon or an error wrapping weaponsvc.ErrWeaponNotFound.
func (p WeaponEditor) Get(ctx context.Context, characterID int64, id string) (weaponsvc.Weapon, error) {
	var out weaponsvc.Weapon
	err := p.s.read(ctx, func(t *tables) error {
		r, err := t.weapon(characterID, id, weaponsvc.ErrWeaponNotFound)
		out = r.edited()
		return err
	})
	return out, err
}

// Insert writes a whole weapon for the character (a new one, or a void
// restoring a removed one); an id the character holds is refused.
func (p WeaponEditor) Insert(ctx context.Context, characterID int64, w weaponsvc.Weapon) error {
	key := owned{characterID, w.ID}
	return p.s.write(ctx, func(t *tables) error {
		if _, taken := t.weapons[key]; taken {
			return errDuplicate("weapons", w.ID)
		}
		w = clone(w)
		w.Name = stored(w.Name)
		t.weapons[key] = storedWeapon{Weapon: w, row: nextID(&t.rowSeq)}
		return nil
	})
}

// Update overwrites every field of an existing weapon, its name included.
func (p WeaponEditor) Update(ctx context.Context, characterID int64, w weaponsvc.Weapon) error {
	return p.s.write(ctx, func(t *tables) error {
		r, err := t.weapon(characterID, w.ID, weaponsvc.ErrWeaponNotFound)
		if err != nil {
			return err
		}
		w = clone(w)
		w.Name = stored(w.Name)
		t.weapons[owned{characterID, w.ID}] = storedWeapon{Weapon: w, row: r.row}
		return nil
	})
}

// Delete removes a weapon and its name.
func (p WeaponEditor) Delete(ctx context.Context, characterID int64, id string) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, err := t.weapon(characterID, id, weaponsvc.ErrWeaponNotFound); err != nil {
			return err
		}
		delete(t.weapons, owned{characterID, id})
		return nil
	})
}
