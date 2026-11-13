package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// pendingConversion is the campaign's seeded database as it is before the conversion: every item's
// abilities NULL, the Power Gem used today (count 0) and its hammer on.
func pendingConversion(t *testing.T) (*fakeStore, int64) {
	t.Helper()
	f := newFakeStore()
	require.NoError(t, seedOn(f, campaignSeeds()))
	owner := f.t.Characters[0]
	clear(f.t.Abilities)
	gem := f.t.Items[rowKey(owner, "power_gem_dispel")]
	gem.Count = 0
	f.t.Items[rowKey(owner, "power_gem_dispel")] = gem
	return f, owner
}

func abilitiesOf(t *testing.T, f *fakeStore, owner int64, id string) []itemsvc.Ability {
	t.Helper()
	raw, ok := f.t.Abilities[rowKey(owner, id)]
	require.True(t, ok, "%s converted", id)
	var js []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &js))
	got, err := itemsvc.DecodeAbilities(raw, true)
	require.NoError(t, err)
	return got
}

// ADR 0049 step 18: the boot step converts every row with NULL abilities in one transaction, keeps
// count and the old columns as stored, names the Power Gem's ability from the seed row's abilities,
// puts the review reasons on the list, and a second run converts nothing.
func TestConvertItems_ADR0049Step18_ConvertsOnceAndKeepsTheOldColumns(t *testing.T) {
	f, owner := pendingConversion(t)
	before := map[string]int64{}
	for k, it := range f.t.Items {
		before[k] = it.Count
	}
	n := len(f.t.Items)

	got, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	assert.Equal(t, n, got.Converted)
	assert.Positive(t, got.Review, "the wand's charges total is on the review list")

	for k, it := range f.t.Items {
		assert.Equal(t, before[k], it.Count, "%s keeps its count: the old meaning stays until Use moves", k)
	}
	gem := abilitiesOf(t, f, owner, "power_gem_dispel")
	require.Len(t, gem, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 0}, gem[0].Limit, "used today stays used")
	assert.Equal(t, "Dispel Magic", gem[0].Name["en"])
	assert.Equal(t, "Dissipation de la magie", gem[0].Name["fr"])
	wand := abilitiesOf(t, f, owner, "wand_lightning")
	require.Len(t, wand, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}, wand[0].Limit)
	assert.True(t, f.t.Reviews[rowKey(owner, "wand_lightning")+"|"+itemsvc.ReasonChargesTotal])
	flask := abilitiesOf(t, f, owner, "fire_flask")
	require.Len(t, flask, 1)
	assert.Equal(t, itemsvc.LimitWithItem, flask[0].Limit.Kind)
	assert.Empty(t, abilitiesOf(t, f, owner, "arrows_plus1"))

	again, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again, "a second boot converts nothing")
}

// ADR 0049 step 18: a row an older release wrote after a rollback (NULL again) is converted by the
// next start, and the rows already converted are not touched.
func TestConvertItems_ADR0049Step18_ConvertsOnlyTheRowsStillNull(t *testing.T) {
	f, owner := pendingConversion(t)
	_, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	delete(f.t.Abilities, rowKey(owner, "fire_flask"))
	f.t.Abilities[rowKey(owner, "arrows_plus1")] = `[{"id":"edited","limit":{"kind":"unlimited"},"effects":[]}]`

	got, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	assert.Equal(t, 1, got.Converted)
	assert.Equal(t, `[{"id":"edited","limit":{"kind":"unlimited"},"effects":[]}]`, f.t.Abilities[rowKey(owner, "arrows_plus1")])
}

// ADR 0049 step 23: a fresh database is seeded in the new shape, abilities written, nothing left
// for the boot step; the old columns mirror the abilities for the previous release.
func TestSeed_ADR0049Step23_FreshDatabaseIsSeededConverted(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, campaignSeeds()))
	owner := f.t.Characters[0]
	// Only the worn armour the skill armour carry-over adds (written by the migration-time insert,
	// which predates the column) is left to the boot step; every seeded item is converted already.
	got, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{Converted: 1}, got)
	again, err := convertItems(context.Background(), f, f, campaignSeeds())
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again)
	gem := abilitiesOf(t, f, owner, "power_gem_dispel")
	require.Len(t, gem, 1)
	assert.Equal(t, "Dispel Magic", gem[0].Name["en"])
	row := f.t.Items[rowKey(owner, "power_gem_dispel")]
	assert.Equal(t, "daily", row.Behaviour)
	assert.Equal(t, int64(1), row.Count)
	assert.Equal(t, int64(1), row.MaxCount)
	wand := f.t.Items[rowKey(owner, "wand_lightning")]
	assert.Equal(t, "charges", wand.Behaviour)
	assert.Equal(t, int64(4), wand.Count)
	assert.Equal(t, "6d6", wand.EffectDice)
}
