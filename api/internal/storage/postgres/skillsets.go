package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// skillSetsMigration adds characters.skill_sets (ADR 0040 step 3.5). Right after it the run copies
// every character's skill points under the skill set its class grants (carrySkillSets).
const skillSetsMigration = "034_skill_sets.sql"

// carrySkillSets is the copy of the flat characters.skill_points into characters.skill_sets, a data
// step run just after skillSetsMigration (migrations hold no data, ADR 0025 step 2.1.1): the
// points of a character are entered in the first skill set its class lists (skills.FirstSet), as
// they were before a class could grant several. Only characters that have points and no skill sets
// yet are carried, so a rerun changes nothing; a class the catalogue does not define, or one
// without a skill set, carries nothing (its points could not be read before either).
func carrySkillSets(ctx context.Context, q storeq.Querier, defs *classes.Registry) error {
	rows, err := q.ListLegacySkillPoints(ctx)
	if err != nil {
		return fmt.Errorf("list characters: %w", err)
	}
	for _, r := range rows {
		def, ok := defs.Class(r.Class)
		if !ok {
			continue
		}
		set := skills.FirstSet(def)
		if set == "" {
			continue
		}
		var flat map[string]int
		if err := json.Unmarshal([]byte(r.SkillPoints), &flat); err != nil {
			return fmt.Errorf("character %d: decode skill points: %w", r.ID, err)
		}
		raw, err := skillSetsJSON(skills.SetPoints(nil).With(set, flat))
		if err != nil {
			return fmt.Errorf("character %d: %w", r.ID, err)
		}
		if err := q.SetSkillSets(ctx, storeq.SetSkillSetsParams{SkillSets: raw, ID: r.ID}); err != nil {
			return fmt.Errorf("character %d: set skill sets: %w", r.ID, err)
		}
	}
	return nil
}
