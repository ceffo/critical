package postgres

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// picksMigration adds characters.proficiency_picks. Right after it the run turns every stored
// proficiencies row into a pick (pickProficiencies, ADR 0037 step 16).
const picksMigration = "025_proficiency_picks.sql"

// pickProficiencies is ADR 0037 step 16.1, the data step after migration 025: each character's
// proficiencies rows, in sheet order, become its non-weapon picks, each at its catalogue cost for
// the character's class (step 4: Table 37's slots, one more outside the class's Table 38
// groups), so every check target stays what it was (no extra slot). A row whose id the catalogue
// does not hold is kept as a pick of 1 slot, never dropped: the derived warnings report it
// (proficiency_unknown) and the player may remove it. Nothing is refused: a character left over
// budget sees the slots_over_budget warning (step 16.4). The rows stay in their table, read by
// nothing. book is the catalogue (seeds/proficiencies.json).
func pickProficiencies(ctx context.Context, q storeq.Querier, book *ruletablesvc.Proficiencies) error {
	rows, err := q.ProficiencyRows(ctx)
	if err != nil {
		return fmt.Errorf("list proficiencies: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	chars, err := q.CharacterClasses(ctx)
	if err != nil {
		return fmt.Errorf("list characters: %w", err)
	}
	class := make(map[int64]string, len(chars))
	for _, c := range chars {
		class[c.ID] = c.Class
	}
	picks := map[int64]*charactersvc.Picks{}
	var order []int64
	for _, r := range rows {
		p, ok := picks[r.CharacterID]
		if !ok {
			p = &charactersvc.Picks{}
			picks[r.CharacterID] = p
			order = append(order, r.CharacterID)
		}
		slots := 1
		if row, known := book.Get(r.ID); known {
			slots = ruletablesvc.NonweaponCost(row, book.GroupsOf(class[r.CharacterID]))
		}
		p.Nonweapon = append(p.Nonweapon, charactersvc.Pick{ID: r.ID, Slots: slots})
	}
	for _, id := range order {
		raw, err := characterdal.EncodePicks(*picks[id])
		if err != nil {
			return err
		}
		if err := q.SetCharacterPicks(ctx, storeq.SetCharacterPicksParams{ProficiencyPicks: raw, ID: id}); err != nil {
			return fmt.Errorf("character %d picks: %w", id, err)
		}
	}
	return nil
}

// pickProficienciesFrom reads the catalogue of seedsDir, then runs pickProficiencies.
func pickProficienciesFrom(ctx context.Context, q storeq.Querier, seedsDir string) error {
	book, err := ruletablesadpt.ReadProficiencies(seedsDir)
	if err != nil {
		return err
	}
	return pickProficiencies(ctx, q, book)
}
