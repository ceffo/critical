// Package weapondal is the storage adapter for the weapons catalog consumed by
// combatsvc (combatsvc.Weapons) and edited by weaponsvc (weaponsvc.DAL). It
// delegates to sqlc-generated queries.
package weapondal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/textdal"
	"github.com/ceffo/critical/api/internal/adapters/weapondal/queries"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
)

// DAL is the implementation of combatsvc.Weapons over the sqlc-generated
// queries, each the given character's (weapons.character_id). A weapon's name is a key into localized_texts (ADR 0023 step 9),
// read through texts.
type DAL struct {
	q     queries.Querier
	texts textdal.Store
}

var _ combatsvc.Weapons = (*DAL)(nil)

// New returns a DAL running its queries on q and its names on texts. The
// composition root builds both over the pool, routed to the caller's
// transaction (ADR 0025 step 5.2).
func New(q queries.Querier, texts textdal.Store) *DAL { return &DAL{q: q, texts: texts} }

// names reads the name of every weapon of the character, by key.
func names(ctx context.Context, q queries.Querier, characterID int64) (map[int64]localized.Text, error) {
	rows, err := q.WeaponNames(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list weapon names: %w", err)
	}
	out := map[int64]localized.Text{}
	for _, t := range rows {
		if out[t.TextID] == nil {
			out[t.TextID] = localized.Text{}
		}
		out[t.TextID][t.Lang] = t.Body
	}
	return out, nil
}

func toWeapon(r queries.Weapon, name localized.Text) combatsvc.Weapon {
	return combatsvc.Weapon{
		ID: r.ID, Name: name,
		BaseDamage: r.DamageSm, DamageLarge: r.DamageL,
		MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage),
		Kind: combatsvc.WeaponKind(r.Kind), TwoHanded: r.TwoHanded != 0,
		AmmoKind: r.AmmoKind, Type: r.WeaponType, Specialized: r.Specialized != 0,
	}
}

// get reads one weapon row and its name; notFound is the error wrapped when
// the id is unknown.
func get(ctx context.Context, q queries.Querier, texts textdal.Store, characterID int64, id string, notFound error) (queries.Weapon, localized.Text, error) {
	r, err := q.GetWeapon(ctx, queries.GetWeaponParams{CharacterID: characterID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return queries.Weapon{}, nil, fmt.Errorf("%w: %s", notFound, id)
	}
	if err != nil {
		return queries.Weapon{}, nil, fmt.Errorf("get weapon %s: %w", id, err)
	}
	name, err := texts.Get(ctx, r.Name)
	if err != nil {
		return queries.Weapon{}, nil, fmt.Errorf("weapon %s: %w", id, err)
	}
	return r, name, nil
}

// list reads every weapon row of the character in sheet order and the names
// by key.
func list(ctx context.Context, q queries.Querier, characterID int64) ([]queries.Weapon, map[int64]localized.Text, error) {
	rows, err := q.ListWeapons(ctx, characterID)
	if err != nil {
		return nil, nil, fmt.Errorf("list weapons: %w", err)
	}
	byKey, err := names(ctx, q, characterID)
	if err != nil {
		return nil, nil, err
	}
	return rows, byKey, nil
}

// Get returns the weapon or an error wrapping combatsvc.ErrWeaponNotFound.
func (d *DAL) Get(ctx context.Context, characterID int64, id string) (combatsvc.Weapon, error) {
	r, name, err := get(ctx, d.q, d.texts, characterID, id, combatsvc.ErrWeaponNotFound)
	if err != nil {
		return combatsvc.Weapon{}, err
	}
	return toWeapon(r, name), nil
}

// List returns the character's weapons in sheet order.
func (d *DAL) List(ctx context.Context, characterID int64) ([]combatsvc.Weapon, error) {
	rows, byKey, err := list(ctx, d.q, characterID)
	if err != nil {
		return nil, err
	}
	out := make([]combatsvc.Weapon, 0, len(rows))
	for _, r := range rows {
		out = append(out, toWeapon(r, byKey[r.Name]))
	}
	return out, nil
}

// Editor is the implementation of weaponsvc.DAL: the same table as DAL, with
// the position and the write side.
type Editor struct {
	q     queries.Querier
	texts textdal.Store
}

var _ weaponsvc.DAL = (*Editor)(nil)

// NewEditor returns an Editor running its queries on q and its names on
// texts (wired as New's).
func NewEditor(q queries.Querier, texts textdal.Store) *Editor {
	return &Editor{q: q, texts: texts}
}

func toEdited(r queries.Weapon, name localized.Text) weaponsvc.Weapon {
	return weaponsvc.Weapon{
		ID: r.ID, Position: int(r.Position), Name: name, DamageSM: r.DamageSm, DamageL: r.DamageL,
		MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage), Kind: r.Kind,
		TwoHanded: r.TwoHanded != 0, AmmoKind: r.AmmoKind, Type: r.WeaponType, Specialized: r.Specialized != 0,
	}
}

// two_handed is stored as 0 or 1.
func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// List returns the character's weapons in sheet order.
func (e *Editor) List(ctx context.Context, characterID int64) ([]weaponsvc.Weapon, error) {
	rows, byKey, err := list(ctx, e.q, characterID)
	if err != nil {
		return nil, err
	}
	out := make([]weaponsvc.Weapon, 0, len(rows))
	for _, r := range rows {
		out = append(out, toEdited(r, byKey[r.Name]))
	}
	return out, nil
}

// Get returns one weapon or an error wrapping weaponsvc.ErrWeaponNotFound.
func (e *Editor) Get(ctx context.Context, characterID int64, id string) (weaponsvc.Weapon, error) {
	r, name, err := get(ctx, e.q, e.texts, characterID, id, weaponsvc.ErrWeaponNotFound)
	if err != nil {
		return weaponsvc.Weapon{}, err
	}
	return toEdited(r, name), nil
}

// Insert writes a whole weapon (a new one, or a void restoring a removed
// one), its name under a new key.
func (e *Editor) Insert(ctx context.Context, characterID int64, w weaponsvc.Weapon) error {
	key, err := e.texts.Add(ctx, w.Name)
	if err != nil {
		return fmt.Errorf("insert weapon %s: %w", w.ID, err)
	}
	err = e.q.InsertWeapon(ctx, queries.InsertWeaponParams{
		CharacterID: characterID, ID: w.ID, Position: int64(w.Position), Name: key, DamageSm: w.DamageSM, DamageL: w.DamageL,
		MagicToHit: int64(w.MagicToHit), MagicDamage: int64(w.MagicDamage), Kind: w.Kind,
		TwoHanded: b2i(w.TwoHanded), AmmoKind: w.AmmoKind, WeaponType: w.Type, Specialized: b2i(w.Specialized),
	})
	if err != nil {
		return fmt.Errorf("insert weapon %s: %w", w.ID, err)
	}
	return nil
}

// Update overwrites every field of an existing weapon and its name's rows
// (the key stays).
func (e *Editor) Update(ctx context.Context, characterID int64, w weaponsvc.Weapon) error {
	key, err := e.q.UpdateWeapon(ctx, queries.UpdateWeaponParams{
		Position: int64(w.Position), DamageSm: w.DamageSM, DamageL: w.DamageL,
		MagicToHit: int64(w.MagicToHit), MagicDamage: int64(w.MagicDamage), Kind: w.Kind,
		TwoHanded: b2i(w.TwoHanded), AmmoKind: w.AmmoKind, WeaponType: w.Type, Specialized: b2i(w.Specialized), CharacterID: characterID, ID: w.ID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", weaponsvc.ErrWeaponNotFound, w.ID)
	}
	if err != nil {
		return fmt.Errorf("update weapon %s: %w", w.ID, err)
	}
	return e.texts.Put(ctx, key, w.Name)
}

// Delete removes a weapon and its name's rows; ErrWeaponNotFound when the id
// is unknown.
func (e *Editor) Delete(ctx context.Context, characterID int64, id string) error {
	key, err := e.q.DeleteWeapon(ctx, queries.DeleteWeaponParams{CharacterID: characterID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", weaponsvc.ErrWeaponNotFound, id)
	}
	if err != nil {
		return fmt.Errorf("delete weapon %s: %w", id, err)
	}
	return e.texts.Delete(ctx, key)
}
