package charactersvc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
)

// ADR 0032 step 5: the options carry PHB Tables 10 and 11 by race, the dice a roll draws and the
// ranges a value warns outside, read from the race definitions alone.
func TestRaceIdentity_Step5_TheDiceAndRangesOfTheHalfElf(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	got, ok := raceIdentity(defs, "half_elf")
	require.True(t, ok)
	assert.Equal(t, "half_elf", got.Race)
	// Table 10: 60/58 in + 2d6, 110/85 lb + 3d12; Table 11: 15 + 1d6, max 125 + 3d20.
	require.Len(t, got.Height, len(Sexes))
	require.Len(t, got.Weight, len(Sexes))
	for i, sex := range Sexes {
		assert.Equal(t, sex, got.Height[i].Sex)
		assert.Equal(t, [2]int{2, 6}, [2]int{got.Height[i].Count, got.Height[i].Sides})
		assert.Equal(t, sex, got.Weight[i].Sex)
		assert.Equal(t, [2]int{3, 12}, [2]int{got.Weight[i].Count, got.Weight[i].Sides})
	}
	assert.InDelta(t, 157.4, got.Height[0].Range.Min, 0.05)
	assert.InDelta(t, 152.4, got.Height[1].Range.Min, 0.05)
	assert.Equal(t, [2]int{1, 6}, [2]int{got.Age.Count, got.Age.Sides})
	assert.InDelta(t, 16, got.StartAge.Min, 0)
	assert.InDelta(t, 21, got.StartAge.Max, 0)
}

func TestRaceIdentity_AnUnknownRaceHasNone(t *testing.T) {
	_, ok := raceIdentity(cataloguetest.Definitions(t), "nope")
	assert.False(t, ok)
}
