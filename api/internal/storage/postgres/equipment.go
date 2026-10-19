package postgres

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// equipmentMigration makes items every piece of equipment (ADR 0033). Right after it the run
// converts every character's items and weapons (convertEquipment).
const equipmentMigration = "014_equipment.sql"

// convertEquipment is the conversion of ADR 0033 step 26, a data step run just after
// equipmentMigration (migrations hold no data, ADR 0025 step 2.1.1): for every character that
// holds items or weapons, the rows as stored before it are converted by itemsvc.Convert (the one
// implementation of the rules, also the seed's) and written back as pieces, each item and weapon
// under its id and name key, with the review rows (step 27) and the renamed item ids (step 26.3).
// The weapons rows stay, unread, until a later migration drops the table. catalogue is the
// catalogue's weapons the conversion matches (none before format 7).
func convertEquipment(ctx context.Context, q storeq.Querier, catalogue []ruletablesvc.Weapon) error {
	owners, err := q.EquipmentOwners(ctx)
	if err != nil {
		return fmt.Errorf("list equipment owners: %w", err)
	}
	for _, owner := range owners {
		if err := convertCharacter(ctx, q, owner, catalogue); err != nil {
			return fmt.Errorf("character %d: %w", owner, err)
		}
	}
	return nil
}

func convertCharacter(ctx context.Context, q storeq.Querier, owner int64, catalogue []ruletablesvc.Weapon) error {
	itemRows, err := q.ListLegacyItems(ctx, owner)
	if err != nil {
		return fmt.Errorf("list items: %w", err)
	}
	weaponRows, err := q.ListLegacyWeapons(ctx, owner)
	if err != nil {
		return fmt.Errorf("list weapons: %w", err)
	}
	// The name keys by id: an item's under its id before a rename, a weapon's under its id.
	itemKeys := map[string]int64{}
	weaponKeys := map[string]int64{}
	items := make([]itemsvc.Item, 0, len(itemRows))
	for _, r := range itemRows {
		powers, err := itemsvc.DecodePowers(r.Powers)
		if err != nil {
			return fmt.Errorf("item %s: %w", r.ID, err)
		}
		itemKeys[r.ID] = r.Name
		items = append(items, itemsvc.Item{
			ID: r.ID, Position: int(r.Position), Type: r.Type, Behaviour: itemsvc.Behaviour(r.Behaviour),
			MagicBonus: int(r.MagicBonus), AmmoKind: r.AmmoKind, Count: int(r.Count), MaxCount: int(r.MaxCount),
			EffectDice: r.EffectDice, EffectSave: r.EffectSave, Notes: r.Notes, Powers: powers,
		})
	}
	weapons := make([]itemsvc.LegacyWeapon, 0, len(weaponRows))
	for _, r := range weaponRows {
		weaponKeys[r.ID] = r.Name
		weapons = append(weapons, itemsvc.LegacyWeapon{
			ID: r.ID, Position: int(r.Position), DamageSM: r.DamageSm, DamageL: r.DamageL,
			MagicToHit: int(r.MagicToHit), MagicDamage: int(r.MagicDamage), Kind: r.Kind, TwoHanded: r.TwoHanded == 1,
			AmmoKind: r.AmmoKind, Type: r.WeaponType, Specialized: r.Specialized == 1,
		})
	}
	conv := itemsvc.Convert(items, weapons, catalogue)
	renamedFrom := map[string]string{}
	for old, id := range conv.Renames {
		renamedFrom[id] = old
	}
	if err := q.DeleteCharacterItems(ctx, owner); err != nil {
		return fmt.Errorf("clear items: %w", err)
	}
	// Convert lists the items' pieces first, in their order, then the weapons'.
	for i, p := range conv.Pieces {
		var key int64
		switch old, renamed := renamedFrom[p.ID]; {
		case i >= len(items):
			key = weaponKeys[p.ID]
		case renamed:
			key = itemKeys[old]
		default:
			key = itemKeys[p.ID]
		}
		if err := insertPiece(ctx, q, owner, p, key); err != nil {
			return err
		}
	}
	for old, id := range conv.Renames {
		if err := q.InsertItemRename(ctx, storeq.InsertItemRenameParams{CharacterID: owner, OldID: old, NewID: id}); err != nil {
			return fmt.Errorf("record rename %s: %w", old, err)
		}
	}
	return insertReviews(ctx, q, owner, conv.Reviews)
}

// insertPiece writes one piece under the name key key.
func insertPiece(ctx context.Context, q storeq.Querier, owner int64, p itemsvc.Item, key int64) error {
	powers, err := itemsvc.EncodePowers(p.Powers)
	if err != nil {
		return fmt.Errorf("piece %s: %w", p.ID, err)
	}
	damageTypes := p.DamageTypes
	if damageTypes == nil {
		damageTypes = []string{}
	}
	if err := q.InsertItem(ctx, storeq.InsertItemParams{
		CharacterID: owner, ID: p.ID, Position: int64(p.Position), Name: key, Type: p.Type,
		Behaviour: string(p.Behaviour), MagicBonus: int64(p.MagicBonus), AmmoKind: p.AmmoKind,
		Count: int64(p.Count), MaxCount: int64(p.MaxCount), EffectDice: p.EffectDice, EffectSave: p.EffectSave,
		Notes: p.Notes, Powers: powers, Category: string(p.Category), MagicKind: p.MagicKind,
		CatalogueCode: p.CatalogueCode, State: string(p.State), WeaponType: p.WeaponType, WeaponKind: p.WeaponKind,
		Size: p.Size, DamageTypes: damageTypes, Speed: int64(p.Speed), DamageSm: p.DamageSM, DamageL: p.DamageL,
		MagicToHit: int64(p.MagicToHit), MagicDamage: int64(p.MagicDamage), TwoHanded: boolInt(p.TwoHanded),
		Fires: p.Fires, Specialized: boolInt(p.Specialized), BaseAc: int64(p.BaseAC), SkillColumn: p.SkillColumn,
		ArmorRow: p.ArmorRow, ShieldKind: p.ShieldKind, AcMelee: int64(p.ACMelee), AcMissile: int64(p.ACMissile),
	}); err != nil {
		return fmt.Errorf("insert piece %s: %w", p.ID, err)
	}
	return nil
}

func insertReviews(ctx context.Context, q storeq.Querier, owner int64, reviews []itemsvc.Review) error {
	for _, r := range reviews {
		if err := q.InsertReview(ctx, storeq.InsertReviewParams{CharacterID: owner, ItemID: r.ItemID, Reason: r.Reason}); err != nil {
			return fmt.Errorf("insert review %s %s: %w", r.ItemID, r.Reason, err)
		}
	}
	return nil
}
