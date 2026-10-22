package charactersvc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0043 step 4.4: Create stores a new character with the PSP maximum entered from the sheet,
// kept as entered, else step 4's at its psionics entry's starting level, on the fixture's real
// Table 5. The party's human thief/psionicist (psionicist 3, Wis 17 Int 15 Con 16) computes
// 25 + 2 x 12 = 49, its gain not divided by its classes (Q2); a single-class psionicist 1
// computes 25. Create's own gate refuses the psionicist until every kind it lists is computed
// (ADR 0040 step 8), so this reads the started character Create passes.
func TestCreate_Step4_4_PSPMaxEnteredIsKept(t *testing.T) {
	rules := cataloguetest.Tables(t)
	thiefPsi := func() *Character {
		c := &Character{Race: "human", Wis: 17, Int: 15, Con: 16}
		c.SetClasses([]classes.Entry{
			{Class: "thief", RealizedLevel: 3, StartingXP: 2_500, State: classes.EntryActive},
			{Class: "psionicist", RealizedLevel: 3, StartingXP: 4_400, State: classes.EntryActive},
		})
		return c
	}
	single := func() *Character {
		return &Character{Race: "human", Class: "psionicist", RealizedLevel: 1, Wis: 17, Int: 15, Con: 16}
	}
	for _, tc := range []struct {
		name    string
		c       *Character
		entered int
		want    int
	}{
		{"entered, kept as entered", thiefPsi(), 90, 90},
		{"not entered, the multi-class gain undivided", thiefPsi(), 0, 49},
		{"a single-class psionicist at level 1", single(), 0, 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := startingPSPMax(tc.c, tc.entered, rules)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
