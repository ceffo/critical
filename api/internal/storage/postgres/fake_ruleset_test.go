package postgres

import (
	"context"
	"database/sql"
	"fmt"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// rulesetTables are the tables of migration 033 and the legacy crit_table_rows the default
// ruleset's data step copies, as the fake keeps them.
type rulesetTables struct {
	// LegacyCrit is crit_table_rows: the table an upgraded database was edited in.
	LegacyCrit []critRow
	// Rows are the rulesets by name.
	Rows    map[string]rulesetRow
	Crit    map[int64][]critRow
	Effects map[int64][]effectRow
}

type critRow struct {
	From, To   int64
	Multiplier float64
}

type effectRow struct {
	Proficiency string
	AC, Rounds  int64
}

type rulesetRow struct {
	ID                          int64
	CritPlaceholder             bool
	MulticlassAny, DualclassAny bool
	LevelLimits                 string
}

func (f *fakeStore) CountLegacyCritRows(context.Context) (int64, error) {
	return int64(len(f.t.Rulesets.LegacyCrit)), nil
}

func (f *fakeStore) InsertRuleset(_ context.Context, arg storeq.InsertRulesetParams) (int64, error) {
	r := &f.t.Rulesets
	if _, ok := r.Rows[arg.Name]; ok {
		return 0, sql.ErrNoRows // ON CONFLICT DO NOTHING returns no row
	}
	if r.Rows == nil {
		r.Rows, r.Crit, r.Effects = map[string]rulesetRow{}, map[int64][]critRow{}, map[int64][]effectRow{}
	}
	id := int64(len(r.Rows) + 1)
	r.Rows[arg.Name] = rulesetRow{
		ID: id, CritPlaceholder: arg.CritPlaceholder, MulticlassAny: arg.MulticlassAnyRace,
		DualclassAny: arg.DualclassAnyRace, LevelLimits: arg.LevelLimits,
	}
	return id, nil
}

func (f *fakeStore) CopyLegacyCritRows(_ context.Context, rulesetID int64) error {
	f.t.Rulesets.Crit[rulesetID] = append(f.t.Rulesets.Crit[rulesetID], f.t.Rulesets.LegacyCrit...)
	return nil
}

func (f *fakeStore) InsertRulesetCritRow(_ context.Context, arg storeq.InsertRulesetCritRowParams) error {
	r := &f.t.Rulesets
	for _, c := range r.Crit[arg.RulesetID] {
		if c.From == arg.RollFrom {
			return fmt.Errorf("duplicate key ruleset_crit_rows %d", arg.RollFrom)
		}
	}
	r.Crit[arg.RulesetID] = append(r.Crit[arg.RulesetID], critRow{From: arg.RollFrom, To: arg.RollTo, Multiplier: arg.Multiplier})
	return nil
}

func (f *fakeStore) InsertRulesetSuccessEffect(_ context.Context, arg storeq.InsertRulesetSuccessEffectParams) error {
	r := &f.t.Rulesets
	for _, e := range r.Effects[arg.RulesetID] {
		if e.Proficiency == arg.ProficiencyID {
			return fmt.Errorf("duplicate key ruleset_success_effects %s", arg.ProficiencyID)
		}
	}
	r.Effects[arg.RulesetID] = append(r.Effects[arg.RulesetID], effectRow{Proficiency: arg.ProficiencyID, AC: arg.Ac, Rounds: arg.Rounds})
	return nil
}
