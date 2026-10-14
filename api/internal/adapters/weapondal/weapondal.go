// Package weapondal is the storage adapter for the weapons catalog consumed by
// combatsvc (combatsvc.Weapons). It delegates to bob-generated typed queries.
package weapondal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/weapondal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of combatsvc.Weapons.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

type row = queries.ListWeaponsRow

func toWeapon(r row) combatsvc.Weapon {
	return combatsvc.Weapon{
		ID: r.ID, Name: r.Name,
		BaseDamage: r.DamageSM, DamageLarge: r.DamageL,
		MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage),
		Kind: combatsvc.WeaponKind(r.Kind), TwoHanded: r.TwoHanded != 0,
		AmmoKind: r.AmmoKind,
	}
}

// Get returns the weapon or an error wrapping combatsvc.ErrWeaponNotFound.
func (d *DAL) Get(ctx context.Context, id string) (combatsvc.Weapon, error) {
	r, err := queries.GetWeapon(id).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return combatsvc.Weapon{}, fmt.Errorf("%w: %s", combatsvc.ErrWeaponNotFound, id)
	}
	if err != nil {
		return combatsvc.Weapon{}, fmt.Errorf("get weapon %s: %w", id, err)
	}
	return toWeapon(r), nil
}

// List returns every weapon in sheet order.
func (d *DAL) List(ctx context.Context) ([]combatsvc.Weapon, error) {
	rows, err := queries.ListWeapons().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list weapons: %w", err)
	}
	out := make([]combatsvc.Weapon, 0, len(rows))
	for _, r := range rows {
		out = append(out, toWeapon(r))
	}
	return out, nil
}

// Editor is the SQLite-backed implementation of weaponsvc.DAL: the same
// table as DAL, with the position and the write side.
type Editor struct {
	q sqlite.Querier
}

// NewEditor returns an Editor bound to q.
func NewEditor(q sqlite.Querier) *Editor { return &Editor{q: q} }

func (e *Editor) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, e.q)}
}

func toEdited(r queries.GetWeaponFullRow) weaponsvc.Weapon {
	return weaponsvc.Weapon{
		ID: r.ID, Position: int(r.Position), Name: r.Name, DamageSM: r.DamageSM, DamageL: r.DamageL,
		MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage), Kind: r.Kind,
		TwoHanded: r.TwoHanded != 0, AmmoKind: r.AmmoKind,
	}
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// List returns every weapon in sheet order.
func (e *Editor) List(ctx context.Context) ([]weaponsvc.Weapon, error) {
	rows, err := queries.ListWeaponsFull().All(ctx, e.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list weapons: %w", err)
	}
	out := make([]weaponsvc.Weapon, 0, len(rows))
	for _, r := range rows {
		out = append(out, toEdited(r))
	}
	return out, nil
}

// Get returns one weapon or an error wrapping weaponsvc.ErrWeaponNotFound.
func (e *Editor) Get(ctx context.Context, id string) (weaponsvc.Weapon, error) {
	r, err := queries.GetWeaponFull(id).One(ctx, e.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return weaponsvc.Weapon{}, fmt.Errorf("%w: %s", weaponsvc.ErrWeaponNotFound, id)
	}
	if err != nil {
		return weaponsvc.Weapon{}, fmt.Errorf("get weapon %s: %w", id, err)
	}
	return toEdited(r), nil
}

// Insert writes a whole weapon (a new one, or a void restoring a removed one).
func (e *Editor) Insert(ctx context.Context, w weaponsvc.Weapon) error {
	_, err := queries.InsertWeapon([]queries.InsertWeapon_Weapons{{
		ID: w.ID, Position: int64(w.Position), Name: w.Name, DamageSM: w.DamageSM, DamageL: w.DamageL,
		MagicToHit: int64(w.MagicToHit), MagicDamage: int64(w.MagicDamage), Kind: w.Kind,
		TwoHanded: b2i(w.TwoHanded), AmmoKind: w.AmmoKind,
	}}).Exec(ctx, e.exec(ctx))
	if err != nil {
		return fmt.Errorf("insert weapon %s: %w", w.ID, err)
	}
	return nil
}

// Update overwrites every field of an existing weapon.
func (e *Editor) Update(ctx context.Context, w weaponsvc.Weapon) error {
	ids, err := queries.UpdateWeapon(int64(w.Position), w.Name, w.DamageSM, w.DamageL,
		int64(w.MagicToHit), int64(w.MagicDamage), w.Kind, b2i(w.TwoHanded), w.AmmoKind, w.ID).All(ctx, e.exec(ctx))
	if err != nil {
		return fmt.Errorf("update weapon %s: %w", w.ID, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s", weaponsvc.ErrWeaponNotFound, w.ID)
	}
	return nil
}

// Delete removes a weapon; ErrWeaponNotFound when the id is unknown.
func (e *Editor) Delete(ctx context.Context, id string) error {
	ids, err := queries.DeleteWeapon(id).All(ctx, e.exec(ctx))
	if err != nil {
		return fmt.Errorf("delete weapon %s: %w", id, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s", weaponsvc.ErrWeaponNotFound, id)
	}
	return nil
}
