package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// A new store holds the default ruleset, as a migrated database does (ADR 0041 step 1).
func TestRulesets_NewStoreHoldsTheDefault(t *testing.T) {
	got, err := memory.New().Rulesets().Default(context.Background())
	require.NoError(t, err)
	want := ruleset.Default()
	want.ID = got.ID
	assert.Equal(t, want, got)
	assert.NotZero(t, got.ID)
}

func TestRulesets_EditsAreStoredSortedAndAudited(t *testing.T) {
	ctx := context.Background()
	p := memory.New().Rulesets()
	rs, err := p.Default(ctx)
	require.NoError(t, err)

	rows := []ruleset.CritRow{{From: 1, To: 50, Multiplier: 2}, {From: 51, To: 100, Multiplier: 1.5}}
	require.NoError(t, p.ReplaceCritTable(ctx, rs.ID, []ruleset.CritRow{rows[1], rows[0]}))
	require.NoError(t, p.SetRacialLift(ctx, rs.ID, ruleset.RacialLift{}))
	require.NoError(t, p.ReplaceSuccessEffects(ctx, rs.ID, []ruleset.SuccessEffect{{ProficiencyID: "b", Rounds: 1}, {ProficiencyID: "a", Rounds: 2}}))
	require.NoError(t, p.SetCatalogueVersion(ctx, rs.ID, "v5"))

	got, err := p.Default(ctx)
	require.NoError(t, err)
	assert.Equal(t, ruleset.CritTable{Rows: rows}, got.Crit, "sorted, and no longer the placeholder")
	assert.Equal(t, ruleset.RacialLift{}, got.RacialLift)
	assert.Equal(t, []ruleset.SuccessEffect{{ProficiencyID: "a", Rounds: 2}, {ProficiencyID: "b", Rounds: 1}}, got.SuccessEffects)
	assert.Equal(t, "v5", got.CatalogueVersion)

	by := uuid.New()
	require.NoError(t, p.AddChange(ctx, ruleset.Change{RulesetID: rs.ID, By: by, Rule: ruleset.RuleCritTable, Before: "1", After: "2"}))
	require.NoError(t, p.AddChange(ctx, ruleset.Change{RulesetID: rs.ID, By: by, Rule: ruleset.RuleRacialLift, Before: "3", After: "4"}))
	changes, err := p.ListChanges(ctx, rs.ID)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	assert.Equal(t, ruleset.RuleRacialLift, changes[0].Rule, "newest first")
	assert.Greater(t, changes[0].ID, changes[1].ID)
	assert.False(t, changes[0].At.IsZero())
}

// A failed transaction puts the ruleset and its audit back, as the database does.
func TestRulesets_ATransactionRollsBack(t *testing.T) {
	ctx := context.Background()
	s := memory.New()
	p := s.Rulesets()
	rs, err := p.Default(ctx)
	require.NoError(t, err)

	err = s.InTx(ctx, func(ctx context.Context) error {
		require.NoError(t, p.ReplaceCritTable(ctx, rs.ID, []ruleset.CritRow{{From: 1, To: 100, Multiplier: 9}}))
		require.NoError(t, p.AddChange(ctx, ruleset.Change{RulesetID: rs.ID, Rule: ruleset.RuleCritTable}))
		return errors.New("boom")
	})
	require.Error(t, err)

	got, err := p.Default(ctx)
	require.NoError(t, err)
	assert.Equal(t, rs.Crit, got.Crit)
	changes, err := p.ListChanges(ctx, rs.ID)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestRulesets_AnUnknownRulesetIsRefused(t *testing.T) {
	p := memory.New().Rulesets()
	assert.Error(t, p.SetRacialLift(context.Background(), 99, ruleset.RacialLift{}))
}
