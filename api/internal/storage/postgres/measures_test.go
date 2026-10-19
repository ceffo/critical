package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0032 step 8 on existing data: the stored free-text heights and weights become numbers, an
// unrecognised one is left unset, the old text stays (the release before may read it), a second run
// changes nothing and an applied migration runs no step.
func TestMigrate_Step8_HeightsAndWeightsFreeTextBecomeNumbers(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	chars := []struct{ name, height, weight string }{
		{"Metric", "1,80 m", "75 kg"},
		{"Imperial", `5'11"`, "165 lb"},
		{"Bare", "172", "68"},
		{"Words", "grand", "costaud"},
		{"Half", "6 ft", "lourd"},
		{"Empty", "", ""},
		{"Beyond the bound", "999 cm", "600 kg"},
	}
	for _, c := range chars {
		id, err := store.InsertCharacter(context.Background(), insertParams(c.name, "human"))
		require.NoError(t, err)
		store.t.Heights[id], store.t.Weights[id] = c.height, c.weight
	}
	var recorded []string
	for _, m := range embedded {
		if m.name != measuresMigration {
			recorded = append(recorded, m.name)
		}
	}
	run := func(recorded []string) {
		f := &fakeMigrations{recorded: recorded, store: store}
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
			return runMigrations(ctx, f, embedded, before, after)
		}))
	}
	run(recorded)

	ids := store.t.Characters
	for i, want := range []struct{ h, w float64 }{
		{180, 75}, {180.3, 74.8}, {172, 68}, {0, 0}, {182.9, 0}, {0, 0}, {0, 0},
	} {
		assert.InDelta(t, want.h, store.t.HeightsCm[ids[i]], 1e-9, "%s height", chars[i].name)
		assert.InDelta(t, want.w, store.t.WeightsKg[ids[i]], 1e-9, "%s weight", chars[i].name)
	}
	assert.Equal(t, "costaud", store.t.Weights[ids[3]], "the old text is left alone")

	before := copyTables(store.t)
	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error { return mapMeasures(ctx, store) }))
	assert.Equal(t, before, store.t, "a second run changes nothing")

	store.t.Heights[ids[0]] = "190"
	run(append(recorded, measuresMigration))
	assert.InDelta(t, 180, store.t.HeightsCm[ids[0]], 1e-9, "an applied migration runs no step")
}
