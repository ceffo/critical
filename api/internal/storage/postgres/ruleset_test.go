package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRulesetStep runs the data step after the rulesets migration, the way a migration run does.
func runRulesetStep(t *testing.T, f *fakeStore) {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	require.NoError(t, f.InTx(context.Background(), func(ctx context.Context) error {
		return after[rulesetsMigration](ctx, f)
	}))
}

// ADR 0041 step 10 (the migration's data step) and step 2 (the default's values): a database that
// never edited the crit table gets one default ruleset with the placeholder table 1-100 -> x2, both
// racial lifts on, level limits as warnings and the Tumbling success effect (AC -4, 1 round).
func TestMigrate_Step10_FreshDatabaseGetsTheDefaultRuleset(t *testing.T) {
	f := newFakeStore()
	runRulesetStep(t, f)

	require.Len(t, f.t.Rulesets.Rows, 1)
	def := f.t.Rulesets.Rows["default"]
	assert.True(t, def.CritPlaceholder, "no stored table: the placeholder flag stays")
	assert.True(t, def.MulticlassAny)
	assert.True(t, def.DualclassAny)
	assert.Equal(t, "warn", def.LevelLimits)
	assert.Equal(t, []critRow{{From: 1, To: 100, Multiplier: 2}}, f.t.Rulesets.Crit[def.ID])
	assert.Equal(t, []effectRow{{Proficiency: "tumbling", AC: -4, Rounds: 1}}, f.t.Rulesets.Effects[def.ID])
}

// ADR 0041 step 10: the crit table a database stored (edited in the app) is the default ruleset's
// table, every row and decimal kept, and it is no longer the placeholder: nothing is lost.
func TestMigrate_Step10_StoredCritTableMovesToTheDefaultRuleset(t *testing.T) {
	f := newFakeStore()
	stored := []critRow{{From: 1, To: 10, Multiplier: 3}, {From: 11, To: 90, Multiplier: 1.5}, {From: 91, To: 100, Multiplier: 4}}
	f.t.Rulesets.LegacyCrit = stored

	runRulesetStep(t, f)

	def := f.t.Rulesets.Rows["default"]
	assert.False(t, def.CritPlaceholder)
	assert.Equal(t, stored, f.t.Rulesets.Crit[def.ID])
	assert.Equal(t, stored, f.t.Rulesets.LegacyCrit, "the legacy table is read, not changed")
	assert.Equal(t, []effectRow{{Proficiency: "tumbling", AC: -4, Rounds: 1}}, f.t.Rulesets.Effects[def.ID])
}

// ADR 0041 step 10 ("the seed loader is idempotent"): a second run finds the default ruleset and
// writes nothing, so an admin's later edits are never overwritten.
func TestMigrate_Step10_SecondRunWritesNothing(t *testing.T) {
	f := newFakeStore()
	runRulesetStep(t, f)
	f.t.Rulesets.Crit[1] = []critRow{{From: 1, To: 100, Multiplier: 5}}
	f.t.Rulesets.Effects[1] = nil
	want := copyTables(f.t)

	runRulesetStep(t, f)

	assert.Equal(t, want.Rulesets, f.t.Rulesets)
}

// The data step is reached through runMigrations from the embedded file: a typo in the step's key
// would leave it silently never run.
func TestMigrate_Step10_RulesetStepIsWiredToTheEmbeddedFile(t *testing.T) {
	f := newFakeStore()
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != rulesetsMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+rulesetsMigration)
	assert.Contains(t, f.t.Rulesets.Rows, "default")
}
