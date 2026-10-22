package postgres

import (
	"context"
	"fmt"
	"github.com/ceffo/critical/api/internal/domain/classes"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// skillArmourMigration retires the entered skill armour (ADR 0033 step 14). Right after it the run
// carries every thief's and bard's entered armour over to a worn body armour (carrySkillArmour).
const skillArmourMigration = "020_skill_armour_worn.sql"

// carrySkillArmour is the carry-over of ADR 0033 step 14, a data step run just after
// skillArmourMigration (migrations hold no data, ADR 0025 step 2.1.1): every character is carried
// over by carryOver. armours are the catalogue's body armours; a carry-over that needs a row
// the catalogue does not hold (before format 7) fails the run, which rolls back and is retried
// on the next start, as the equipment conversion does (ErrCatalogueWithoutEquipment).
func carrySkillArmour(ctx context.Context, q storeq.Querier, armours []ruletablesvc.Armour, defs *classes.Registry) error {
	rows, err := q.SkillArmourCharacters(ctx)
	if err != nil {
		return fmt.Errorf("list characters: %w", err)
	}
	for _, r := range rows {
		if err := carryOver(ctx, q, r.ID, r.Class, r.SkillArmor, armours, defs); err != nil {
			return fmt.Errorf("character %d: %w", r.ID, err)
		}
	}
	return nil
}

// carryCharacter carries the character owner's entered skill armour over to a worn body armour
// (itemsvc.CarrySkillArmour): the seed's, once its equipment is seeded.
func carryCharacter(ctx context.Context, q storeq.Querier, owner int64, armours []ruletablesvc.Armour, defs *classes.Registry) error {
	c, err := q.CharacterSkillArmour(ctx, owner)
	if err != nil {
		return fmt.Errorf("read the skill armour: %w", err)
	}
	return carryOver(ctx, q, owner, c.Class, c.SkillArmor, armours, defs)
}

// carryOver applies itemsvc.CarrySkillArmour to one character: it wears the piece it names, or
// inserts the new piece under a new name key (its name in every language of the catalogue row)
// with its review row. Nothing is logged: the carry-over is no move of the game. A class the
// catalogue does not define has no features: nothing to carry.
func carryOver(ctx context.Context, q storeq.Querier, owner int64, class, skillArmor string, armours []ruletablesvc.Armour, defs *classes.Registry) error {
	def, _ := defs.Class(class)
	rows, err := q.CarryOverItems(ctx, owner)
	if err != nil {
		return fmt.Errorf("list items: %w", err)
	}
	pieces := make([]itemsvc.Item, 0, len(rows))
	for _, r := range rows {
		pieces = append(pieces, itemsvc.Item{
			ID: r.ID, Position: int(r.Position), Category: itemsvc.Category(r.Category),
			State: itemsvc.State(r.State), SkillColumn: r.SkillColumn,
		})
	}
	co, err := itemsvc.CarrySkillArmour(def, skillArmor, pieces, armours)
	if err != nil {
		return err
	}
	switch {
	case co.Wear != "":
		if err := q.WearItem(ctx, storeq.WearItemParams{CharacterID: owner, ID: co.Wear}); err != nil {
			return fmt.Errorf("wear %s: %w", co.Wear, err)
		}
	case co.Add != nil:
		key, err := addSeedText(ctx, q, co.Add.Name)
		if err != nil {
			return fmt.Errorf("piece %s: %w", co.Add.ID, err)
		}
		if err := insertPiece(ctx, q, owner, *co.Add, key); err != nil {
			return err
		}
		if co.Review != nil {
			return insertReviews(ctx, q, owner, []itemsvc.Review{*co.Review})
		}
	}
	return nil
}
