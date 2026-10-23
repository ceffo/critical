package composition_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// table2 is PHB Table 2 as the app gave it before the Dexterity columns moved
// to the view (committed values, ADR 0046 step 7): the missile attack and the
// defensive adjustment per score.
var table2 = map[int][2]int{
	1: {-6, 5}, 2: {-4, 5}, 3: {-3, 4}, 4: {-2, 3}, 5: {-1, 2}, 6: {0, 1},
	7: {0, 0}, 8: {0, 0}, 9: {0, 0}, 10: {0, 0}, 11: {0, 0}, 12: {0, 0}, 13: {0, 0}, 14: {0, 0},
	15: {0, -1}, 16: {1, -2}, 17: {2, -3}, 18: {2, -4}, 19: {3, -4}, 20: {3, -4},
	21: {4, -5}, 22: {4, -5}, 23: {4, -5}, 24: {5, -6}, 25: {5, -6},
}

func dexTable(t *testing.T) (*composition.BackingServices, *memory.Store) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	return bs, m
}

func dexCharacter(dex int) charactersvc.Character {
	return charactersvc.Character{
		Name: "Dex", Class: "fighter", Str: 10, Dex: dex, Con: 10, Int: 10, Wis: 10, Cha: 10, HPMax: 10,
	}
}

// ADR 0046 step 2.3: every score of Table 2 gives the same AC and the same
// Dexterity row through acsvc.Derive, fed by the view, and the view carries
// both columns, as the committed table says.
func TestDerive_Step2_DexterityRowsMatchTheTable(t *testing.T) {
	ctx := context.Background()
	bs, m := dexTable(t)
	for dex := 1; dex <= 25; dex++ {
		t.Run(fmt.Sprint(dex), func(t *testing.T) {
			id, err := m.AddCharacter(ctx, dexCharacter(dex))
			require.NoError(t, err)
			want := table2[dex]

			v, err := bs.Character.View(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, want[0], v.DexMissile)
			assert.Equal(t, want[1], v.DexDefence)

			ac, err := bs.AC.Get(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, 10+want[1], ac.Value, "base 10 and the defensive adjustment")
			assert.Contains(t, ac.Rows, acsvc.Row{Term: acsvc.TermDexterity, Value: want[1], Dexterity: dex})
		})
	}
}

// ADR 0046 step 2.1 (Q2): a Dexterity outside Table 2 is refused where the
// table is read, with the table's message, so the whole character cannot be
// read, not only its AC.
func TestDerive_Step2_DexterityOutsideTheTableRefusesTheRead(t *testing.T) {
	ctx := context.Background()
	bs, m := dexTable(t)
	for _, dex := range []int{0, 26} {
		id, err := m.AddCharacter(ctx, dexCharacter(dex))
		require.NoError(t, err, "the store must accept the score so the read is what refuses it")
		_, err = bs.Character.View(ctx, id)
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("dexterity %d", dex))
		assert.Contains(t, err.Error(), "out of range 1..25")
		_, err = bs.AC.Get(ctx, id)
		require.Error(t, err)
	}
}

// ADR 0046 step 2.3: acsvc has no rule table, so the AC of Dexterity 17 is
// the view's column and nothing else (the one-read count is
// charactersvc's TestDerive_Step2_TheDexterityTableIsReadOnce).
func TestDerive_Step2_TheACTakesTheViewsColumn(t *testing.T) {
	ctx := context.Background()
	bs, m := dexTable(t)
	id, err := m.AddCharacter(ctx, dexCharacter(17))
	require.NoError(t, err)
	v, err := bs.Character.View(ctx, id)
	require.NoError(t, err)
	ac, err := bs.AC.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 10+v.DexDefence, ac.Value)
}
