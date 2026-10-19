package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0032 step 8.1: a stored height names centimetres, metres, or feet and inches, en and fr; a
// bare one under 3 is metres and otherwise centimetres; anything else names nothing.
func TestParseHeight_Step8_1_EachUnit(t *testing.T) {
	for raw, want := range map[string]float64{
		"180": 180, "180 cm": 180, "180cm": 180, "182,5 cm": 182.5, "1.80": 180, "1,80": 180, "1.8 m": 180,
		"1,75 m": 175, "1m80": 180, "1 m 80": 180, "2 mètres": 200, "1 metre": 100,
		`5'11"`: 180.3, "5'11": 180.3, "5' 11\"": 180.3, "5 ft 11 in": 180.3, "5ft 11in": 180.3,
		"6 feet 2 inches": 188, "5 pieds 11 pouces": 180.3, "5'": 152.4, "6 ft": 182.9,
		"71 in": 180.3, "71 inches": 180.3, "5′11″": 180.3, "  170 CM ": 170,
	} {
		got, ok := parseHeight(raw)
		assert.True(t, ok, "%q", raw)
		assert.InDelta(t, want, got, 0.05, "%q", raw)
	}
	for _, raw := range []string{"", "tall", "grand", "average", "5'11 or so", "0", "0 cm", "3000", "-10", "huge 180"} {
		_, ok := parseHeight(raw)
		assert.False(t, ok, "%q is not a height", raw)
	}
}

// ADR 0032 step 8.1: a stored weight names kilograms or pounds; a bare one is kilograms.
func TestParseWeight_Step8_1_EachUnit(t *testing.T) {
	for raw, want := range map[string]float64{
		"75": 75, "75 kg": 75, "75kg": 75, "75,5 kg": 75.5, "70 kilos": 70, "70 kilogrammes": 70,
		"165 lb": 74.8, "165lbs": 74.8, "165 pounds": 74.8, "  80 KG ": 80,
	} {
		got, ok := parseWeight(raw)
		assert.True(t, ok, "%q", raw)
		assert.InDelta(t, want, got, 0.05, "%q", raw)
	}
	for _, raw := range []string{"", "heavy", "lourd", "0", "0 kg", "9999 kg", "12 stone", "-5"} {
		_, ok := parseWeight(raw)
		assert.False(t, ok, "%q is not a weight", raw)
	}
}

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
			before, after := migrationDataSteps(repoSeeds())
			return runMigrations(ctx, f, embedded, before, after)
		}))
	}
	run(recorded)

	ids := store.t.Characters
	for i, want := range []struct{ h, w float64 }{
		{180, 75}, {180.3, 74.8}, {172, 68}, {0, 0}, {182.9, 0}, {0, 0},
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
