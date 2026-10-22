package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0041 step 1: a fresh store has one ruleset, the default, and the catalogue version it names
// is the one the server loaded (the fixture catalogue's).
func TestRuleset_Step1_FreshStoreHasTheDefaultAtTheLoadedCatalogueVersion(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })

	rs, err := bs.Rulesets.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, ruleset.DefaultName, rs.Name)
	assert.Equal(t, "1970-01-01", rs.CatalogueVersion)
	want := ruleset.Default()
	want.ID, want.CatalogueVersion = rs.ID, "1970-01-01"
	assert.Equal(t, want, rs)
}

// ADR 0041 steps 3, 6 and 7 through the real services: an admin's edit changes what the resolver
// gives every character, is audited with its before and after, and is no character's action
// (it lands on no action stack).
func TestRuleset_Steps3_7_AnEditIsReadByTheResolverAndAudited(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)

	_, err = bs.Rulesets.SetCritTable(ctx, s.User.ID, []ruleset.CritRow{{From: 1, To: 100, Multiplier: 3}})
	require.NoError(t, err)
	_, err = bs.Rulesets.SetSuccessEffects(ctx, s.User.ID, nil)
	require.NoError(t, err)

	got, err := bs.Rulesets.ForCharacter(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, ruleset.CritTable{Rows: []ruleset.CritRow{{From: 1, To: 100, Multiplier: 3}}}, got.Crit)
	assert.Empty(t, got.SuccessEffects, "the Tumbling row removed leaves no success effect")

	changes, err := bs.Rulesets.Changes(ctx)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	assert.Equal(t, ruleset.RuleSuccessEffects, changes[0].Rule)
	assert.JSONEq(t, `[{"proficiencyId":"tumbling","ac":-4,"rounds":1}]`, changes[0].Before)
	assert.Equal(t, `[]`, changes[0].After)
	assert.Equal(t, ruleset.RuleCritTable, changes[1].Rule)
	assert.JSONEq(t, `[{"from":1,"to":100,"multiplier":2}]`, changes[1].Before)
	assert.JSONEq(t, `[{"from":1,"to":100,"multiplier":3}]`, changes[1].After)
	assert.Equal(t, s.User.ID, changes[1].By)
}
