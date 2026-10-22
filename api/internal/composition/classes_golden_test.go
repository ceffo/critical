package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0040 step 11.3 (no rule moves for the five classes) and step 6: one
// human of each class, realized level 5 with the XP of level 6, derives what
// the book prints, now that the class, its group and its prime requisites
// come from the catalogue's definitions. Every expected number is read in the
// PHB, not in the catalogue:
//   - THAC0 at level 5, Table 53 (p.91): warrior 16, wizard 19, priest 18, rogue 18.
//   - Saves at level 5, Table 60 (p.101; para/poison, rod, petrification, breath,
//     spell): warrior row 5-6 11 13 12 13 14, wizard 1-5 14 11 13 15 12, priest
//     4-6 9 13 12 15 14, rogue 5-8 12 12 11 15 13.
//   - The XP of levels 5 and 6, Tables 14, 20, 23, 25 (pp.26, 30, 33, 38): fighter
//     16,000 / 32,000, mage 20,000 / 40,000, cleric 13,000 / 27,500, thief and bard
//     10,000 / 20,000; the level-6 hit die d10, d4, d8, d6, d6 (chapter 3).
//   - Constitution 17, Table 3 (p.15): +2 hit points a die, +3 for warriors.
//   - Spell slots at level 5: mage Table 21 (p.30) 4 2 1; cleric Table 24 (p.33)
//     3 3 1 plus Wisdom 16's cumulative bonus spells, Table 5 (p.17), two 1st and
//     two 2nd: 5 5 1; bard Table 32 (p.42) 3 1; fighter and thief none.
//   - The 10% XP bonus, chapter 3 "Prime Requisite": 16 or more in all the
//     class's prime requisites (fighter Str, mage Int, cleric Wis, thief Dex,
//     bard Dex and Cha).
func TestClasses_Golden_FiveClassesDeriveTheBooksFigures(t *testing.T) {
	type figures struct {
		thac0 int
		saves charactersvc.Saves
		next  int // the XP of level 6
		die   int
		gain  int // a rolled 3 with Constitution 17
		bonus bool
		slots map[int]int
	}
	tests := []struct {
		name string
		char charactersvc.Character
		want figures
	}{
		{
			"a fighter of Str 16",
			charactersvc.Character{Class: "fighter", Str: 16, Dex: 10, Con: 17, Int: 10, Wis: 10, Cha: 10},
			figures{16, charactersvc.Saves{ParaPoison: 11, RodStaffWand: 13, PetriPoly: 12, Breath: 13, Spell: 14}, 32_000, 10, 3 + 3, true, map[int]int{}},
		},
		{
			"a mage of Int 15",
			charactersvc.Character{Class: "mage", Str: 10, Dex: 10, Con: 17, Int: 15, Wis: 10, Cha: 10},
			figures{19, charactersvc.Saves{ParaPoison: 14, RodStaffWand: 11, PetriPoly: 13, Breath: 15, Spell: 12}, 40_000, 4, 3 + 2, false, map[int]int{1: 4, 2: 2, 3: 1}},
		},
		{
			"a cleric of Wis 16",
			charactersvc.Character{Class: "cleric", Str: 10, Dex: 10, Con: 17, Int: 10, Wis: 16, Cha: 10},
			figures{18, charactersvc.Saves{ParaPoison: 9, RodStaffWand: 13, PetriPoly: 12, Breath: 15, Spell: 14}, 27_500, 8, 3 + 2, true, map[int]int{1: 5, 2: 5, 3: 1}},
		},
		{
			"a thief of Dex 16",
			charactersvc.Character{Class: "thief", Str: 10, Dex: 16, Con: 17, Int: 10, Wis: 10, Cha: 10},
			figures{18, charactersvc.Saves{ParaPoison: 12, RodStaffWand: 12, PetriPoly: 11, Breath: 15, Spell: 13}, 20_000, 6, 3 + 2, true, map[int]int{}},
		},
		{
			"a bard of Dex 16 and Cha 15",
			charactersvc.Character{Class: "bard", Str: 10, Dex: 16, Con: 17, Int: 13, Wis: 10, Cha: 15},
			figures{18, charactersvc.Saves{ParaPoison: 12, RodStaffWand: 12, PetriPoly: 11, Breath: 15, Spell: 13}, 20_000, 6, 3 + 2, false, map[int]int{1: 3, 2: 1}},
		},
	}
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.char
			c.Name, c.Race, c.RealizedLevel, c.HPMax = "Golden", "human", 5, 20
			c.XPTotal = tc.want.next - 1
			short, err := m.AddCharacter(ctx, c)
			require.NoError(t, err)
			c.XPTotal = tc.want.next
			id, err := m.AddCharacter(ctx, c)
			require.NoError(t, err)

			below, err := bs.Character.GetCharacter(ctx, short)
			require.NoError(t, err)
			assert.Equal(t, 5, below.Derived.TargetLevel, "one XP short of level 6")

			got, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			d := got.Derived
			assert.Equal(t, 5, d.RealizedLevel)
			assert.Equal(t, 6, d.TargetLevel, "the XP of level 6 reaches it")
			assert.Equal(t, tc.want.thac0, d.THAC0, "Table 53")
			assert.Equal(t, tc.want.saves, d.Saves, "Table 60")
			assert.Equal(t, tc.want.slots, d.SpellSlots, "Tables 21, 24 with Table 5, 32")
			l, err := bs.XP.Get(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tc.want.bonus, l.BonusEligible, "every prime requisite at 16")

			rolled := 3
			res, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 6, Die: &rolled})
			require.NoError(t, err)
			assert.Equal(t, tc.want.die, res.HitDie, "the class's hit die")
			assert.Equal(t, tc.want.gain, res.Gain, "the die and Table 3")
		})
	}
}
