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
