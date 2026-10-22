package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ADR 0043 step 3 through the class entries (ADR 0042 step 12): the fixture's
// psionicist lists save_bonus (+2 against the mind-affecting tag) from level
// 1, so a psionicist alone and a thief/psionicist each have that one bonus,
// named by the class that grants it; a fighter has none. A dual-class's old
// psionicist counts where its saves do (ADR 0042 step 9): not before the
// crossover, from it on.
func TestDerive_Step3_ThePsionicistsSaveBonus(t *testing.T) {
	two := []charactersvc.ClassSaveBonus{{Class: "psionicist", Against: classes.SaveAgainstMindAffecting, Bonus: 2}}
	for _, tc := range []struct {
		name string
		char *charactersvc.Character
		want []charactersvc.ClassSaveBonus
	}{
		{"a psionicist 1", multi("human", entry("psionicist", 1, 0)), two},
		{"a thief 15/psionicist 14", multi("human", entry("thief", 15, 0), entry("psionicist", 14, 0)), two},
		{"a fighter", multi("human", entry("fighter", 5, 16_000)), nil},
		{"a dual-class fighter 5 before the crossover", dualPsionicist(5, 16_000), nil},
		{"a dual-class fighter 6 past the crossover", dualPsionicist(6, 32_000), two},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, derived(t, tc.char).ClassSaveBonuses)
		})
	}
}

// dualPsionicist is a human psionicist left at 5 (16,500 XP), then a fighter
// at fighterLevel with fighterXP.
func dualPsionicist(fighterLevel, fighterXP int) *charactersvc.Character {
	c := multi("human", frozen("psionicist", 5, 16_500), entry("fighter", fighterLevel, fighterXP))
	c.Class, c.RealizedLevel, c.XPTotal = "fighter", fighterLevel, fighterXP
	return c
}
