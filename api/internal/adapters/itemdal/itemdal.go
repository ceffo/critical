// Package itemdal is the storage adapter for itemsvc. It delegates to
// sqlc-generated queries in the queries/ subpackage.
package itemdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/itemdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/textdal"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// DAL is the implementation of itemsvc.DAL over the sqlc-generated queries.
// Every query is the given character's (items.character_id).
// An item's name is a key into localized_texts (ADR 0023 step 9), written
// with the item through texts.
type DAL struct {
	q     queries.Querier
	texts textdal.Store
}

var _ itemsvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q and its names on texts. The
// composition root builds both over the pool, routed to the caller's
// transaction (ADR 0025 step 5.2).
func New(q queries.Querier, texts textdal.Store) *DAL { return &DAL{q: q, texts: texts} }

func toItem(r queries.Item, name localized.Text) (itemsvc.Item, error) {
	powers, err := itemsvc.DecodePowers(r.Powers)
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("item %s: %w", r.ID, err)
	}
	return itemsvc.Item{
		ID: r.ID, Position: int(r.Position), Name: name, Type: r.Type,
		Category: itemsvc.Category(r.Category), MagicKind: r.MagicKind, CatalogueCode: r.CatalogueCode,
		Component:  r.Component,
		State:      itemsvc.State(r.State),
		Behaviour:  itemsvc.Behaviour(r.Behaviour),
		MagicBonus: int(r.MagicBonus), AmmoKind: r.AmmoKind,
		Count: int(r.Count), MaxCount: int(r.MaxCount),
		EffectDice: r.EffectDice, EffectSave: r.EffectSave, Notes: r.Notes,
		Powers:      powers,
		DamageTypes: r.DamageTypes, DamageSM: r.DamageSm, DamageL: r.DamageL,
		WeaponType: r.WeaponType, WeaponKind: r.WeaponKind, Size: r.Size, Speed: int(r.Speed),
		MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage), TwoHanded: r.TwoHanded == 1,
		Fires: r.Fires, Specialized: r.Specialized == 1,
		BaseAC: int(r.BaseAc), SkillColumn: r.SkillColumn, ArmorRow: r.ArmorRow,
		ShieldKind: r.ShieldKind, ACMelee: int(r.AcMelee), ACMissile: int(r.AcMissile),
	}, nil
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// types is never nil: the column is NOT NULL.
func types(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

// List returns the character's items in sheet order.
func (d *DAL) List(ctx context.Context, characterID int64) ([]itemsvc.Item, error) {
	rows, err := d.q.ListItems(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	texts, err := d.q.ItemNames(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list item names: %w", err)
	}
	names := map[int64]localized.Text{}
	for _, t := range texts {
		if names[t.TextID] == nil {
			names[t.TextID] = localized.Text{}
		}
		names[t.TextID][t.Lang] = t.Body
	}
	out := make([]itemsvc.Item, 0, len(rows))
	for _, r := range rows {
		it, err := toItem(r, names[r.Name])
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// Decrement spends one count with a single conditional UPDATE, so two
// concurrent uses of the last charge cannot both succeed and the count never
// goes below 0. When no row was updated, a follow-up read tells an unknown id
// from an empty item.
func (d *DAL) Decrement(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error) {
	r, err := d.q.DecrementItem(ctx, queries.DecrementItemParams{CharacterID: characterID, ID: itemID})
	if err == nil {
		return d.named(ctx, r)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("decrement item %s: %w", itemID, err)
	}
	_, err = d.q.GetItem(ctx, queries.GetItemParams{CharacterID: characterID, ID: itemID})
	if errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("get item %s: %w", itemID, err)
	}
	return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemEmpty, itemID)
}

// Increment adds n with a single UPDATE and returns the updated item;
// ErrItemNotFound when the id is unknown.
func (d *DAL) Increment(ctx context.Context, characterID int64, itemID string, n int) (itemsvc.Item, error) {
	r, err := d.q.IncrementItem(ctx, queries.IncrementItemParams{N: int64(n), CharacterID: characterID, ID: itemID})
	if errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("increment item %s: %w", itemID, err)
	}
	return d.named(ctx, r)
}

// named is the item of r with its name read.
func (d *DAL) named(ctx context.Context, r queries.Item) (itemsvc.Item, error) {
	name, err := d.texts.Get(ctx, r.Name)
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("item %s: %w", r.ID, err)
	}
	return toItem(r, name)
}

// ResetDaily refills every daily item of the character to its max count.
func (d *DAL) ResetDaily(ctx context.Context, characterID int64) error {
	if err := d.q.ResetDailyItems(ctx, characterID); err != nil {
		return fmt.Errorf("reset daily items: %w", err)
	}
	return nil
}

// Delete removes an item (a charges item spent to 0) and its name's rows.
// ErrItemNotFound when the id is unknown.
func (d *DAL) Delete(ctx context.Context, characterID int64, itemID string) error {
	key, err := d.q.DeleteItem(ctx, queries.DeleteItemParams{CharacterID: characterID, ID: itemID})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return fmt.Errorf("delete item %s: %w", itemID, err)
	}
	return d.texts.Delete(ctx, key)
}

// Insert writes a whole item (a void restoring a removed item), powers
// included, its name under a new key.
func (d *DAL) Insert(ctx context.Context, characterID int64, it itemsvc.Item) error {
	powers, err := itemsvc.EncodePowers(it.Powers)
	if err != nil {
		return err
	}
	key, err := d.texts.Add(ctx, it.Name)
	if err != nil {
		return fmt.Errorf("insert item %s: %w", it.ID, err)
	}
	err = d.q.InsertItem(ctx, queries.InsertItemParams{
		CharacterID: characterID, ID: it.ID, Position: int64(it.Position), Name: key, Type: it.Type,
		Behaviour: string(it.Behaviour), MagicBonus: int64(it.MagicBonus), AmmoKind: it.AmmoKind,
		Count: int64(it.Count), MaxCount: int64(it.MaxCount),
		EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes, Powers: powers,
		Category: string(it.Category), MagicKind: it.MagicKind, CatalogueCode: it.CatalogueCode, Component: it.Component,
		State: string(it.State), WeaponType: it.WeaponType, WeaponKind: it.WeaponKind, Size: it.Size,
		DamageTypes: types(it.DamageTypes), Speed: int64(it.Speed), DamageSm: it.DamageSM, DamageL: it.DamageL,
		MagicToHit: int64(it.MagicToHit), MagicDamage: int64(it.MagicDamage), TwoHanded: flag(it.TwoHanded),
		Fires: it.Fires, Specialized: flag(it.Specialized), BaseAc: int64(it.BaseAC),
		SkillColumn: it.SkillColumn, ArmorRow: it.ArmorRow, ShieldKind: it.ShieldKind,
		AcMelee: int64(it.ACMelee), AcMissile: int64(it.ACMissile),
	})
	if err != nil {
		return fmt.Errorf("insert item %s: %w", it.ID, err)
	}
	return nil
}

// SetCount writes an item's count back (an action-log void restoring a
// snapshot). ErrItemNotFound when the id is unknown.
func (d *DAL) SetCount(ctx context.Context, characterID int64, itemID string, count int) error {
	n, err := d.q.SetItemCount(ctx, queries.SetItemCountParams{Count: int64(count), CharacterID: characterID, ID: itemID})
	if err != nil {
		return fmt.Errorf("set item %s count: %w", itemID, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	return nil
}

// Get returns one item; ErrItemNotFound when the id is unknown.
func (d *DAL) Get(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error) {
	r, err := d.q.GetItem(ctx, queries.GetItemParams{CharacterID: characterID, ID: itemID})
	if errors.Is(err, sql.ErrNoRows) {
		return itemsvc.Item{}, fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, itemID)
	}
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("get item %s: %w", itemID, err)
	}
	return d.named(ctx, r)
}

// Update overwrites every field of an existing item, powers included, and
// its name's rows (the key stays). ErrItemNotFound when the id is unknown.
func (d *DAL) Update(ctx context.Context, characterID int64, it itemsvc.Item) error {
	powers, err := itemsvc.EncodePowers(it.Powers)
	if err != nil {
		return err
	}
	key, err := d.q.UpdateItem(ctx, queries.UpdateItemParams{
		Position: int64(it.Position), Type: it.Type, Behaviour: string(it.Behaviour),
		MagicBonus: int64(it.MagicBonus), AmmoKind: it.AmmoKind, Count: int64(it.Count),
		MaxCount: int64(it.MaxCount), EffectDice: it.EffectDice, EffectSave: it.EffectSave,
		Notes: it.Notes, Powers: powers, CharacterID: characterID, ID: it.ID,
		Category: string(it.Category), MagicKind: it.MagicKind, CatalogueCode: it.CatalogueCode, Component: it.Component,
		State: string(it.State), WeaponType: it.WeaponType, WeaponKind: it.WeaponKind, Size: it.Size,
		DamageTypes: types(it.DamageTypes), Speed: int64(it.Speed), DamageSm: it.DamageSM, DamageL: it.DamageL,
		MagicToHit: int64(it.MagicToHit), MagicDamage: int64(it.MagicDamage), TwoHanded: flag(it.TwoHanded),
		Fires: it.Fires, Specialized: flag(it.Specialized), BaseAc: int64(it.BaseAC),
		SkillColumn: it.SkillColumn, ArmorRow: it.ArmorRow, ShieldKind: it.ShieldKind,
		AcMelee: int64(it.ACMelee), AcMissile: int64(it.ACMissile),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", itemsvc.ErrItemNotFound, it.ID)
	}
	if err != nil {
		return fmt.Errorf("update item %s: %w", it.ID, err)
	}
	return d.texts.Put(ctx, key, it.Name)
}

// Reviews returns the character's review rows (ADR 0033 step 27).
func (d *DAL) Reviews(ctx context.Context, characterID int64) ([]itemsvc.Review, error) {
	rows, err := d.q.ListReviews(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	out := make([]itemsvc.Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, itemsvc.Review{ItemID: r.ItemID, Reason: r.Reason})
	}
	return out, nil
}

// DeleteReviews removes every review row of a piece.
func (d *DAL) DeleteReviews(ctx context.Context, characterID int64, itemID string) error {
	if err := d.q.DeleteReviews(ctx, queries.DeleteReviewsParams{CharacterID: characterID, ItemID: itemID}); err != nil {
		return fmt.Errorf("delete reviews of %s: %w", itemID, err)
	}
	return nil
}
