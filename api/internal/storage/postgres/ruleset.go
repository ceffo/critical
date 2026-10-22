package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/ruleset"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// rulesetsMigration is the migration whose data step creates the default ruleset (ADR 0041 step
// 10). A fresh database and an upgraded one both get it, right after the tables exist.
const rulesetsMigration = "033_rulesets.sql"

// defaultRuleset creates the default ruleset with the content of ruleset.Default, except the crit
// table: a database that stored crit_table_rows (an edited table) keeps them as the default's
// rows, a database that never stored any gets the default table and its placeholder flag (ADR
// 0041 step 10). It is the data step after rulesetsMigration; a default ruleset already there
// (a second run) is left as it is.
func defaultRuleset(ctx context.Context, q storeq.Querier) error {
	def := ruleset.Default()
	stored, err := q.CountLegacyCritRows(ctx)
	if err != nil {
		return fmt.Errorf("count the stored crit rows: %w", err)
	}
	id, ok, err := insertRuleset(ctx, q, storeq.InsertRulesetParams{
		Name: def.Name, CritPlaceholder: stored == 0,
		MulticlassAnyRace: def.RacialLift.MulticlassAnyRace, DualclassAnyRace: def.RacialLift.DualclassAnyRace,
		LevelLimits: string(def.LevelLimits),
	})
	if err != nil || !ok {
		return err
	}
	if stored > 0 {
		if err := q.CopyLegacyCritRows(ctx, id); err != nil {
			return fmt.Errorf("copy the stored crit rows: %w", err)
		}
	} else {
		for _, r := range def.Crit.Rows {
			err := q.InsertRulesetCritRow(ctx, storeq.InsertRulesetCritRowParams{
				RulesetID: id, RollFrom: int64(r.From), RollTo: int64(r.To), Multiplier: r.Multiplier,
			})
			if err != nil {
				return fmt.Errorf("insert the default crit row: %w", err)
			}
		}
	}
	for _, e := range def.SuccessEffects {
		err := q.InsertRulesetSuccessEffect(ctx, storeq.InsertRulesetSuccessEffectParams{
			RulesetID: id, ProficiencyID: e.ProficiencyID, Ac: int64(e.AC), Rounds: int64(e.Rounds),
		})
		if err != nil {
			return fmt.Errorf("insert the %s success effect: %w", e.ProficiencyID, err)
		}
	}
	return nil
}

// insertRuleset inserts the row; ok is false when a ruleset of that name already exists.
func insertRuleset(ctx context.Context, q storeq.Querier, p storeq.InsertRulesetParams) (id int64, ok bool, err error) {
	id, err = q.InsertRuleset(ctx, p)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("insert the default ruleset: %w", err)
	}
	return id, true, nil
}
