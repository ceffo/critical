package combatsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// ADR 0046 step 3.5: the saves owner's terms reach the roll as before, in
// order (race, Wisdom, class, then the buffs), with the target untouched.
// Every value differs (d20, race 4, Wisdom 2, class 3, buff 1), so a term
// read twice, dropped or from the wrong field shows in the total; the d20,
// the buffs and the log are each called once. The grant against another
// tag never applies, flagged or not.
func TestResolveSavingThrow_Step3_ModifiersInOrder(t *testing.T) {
	char := func() charactersvc.View {
		c := withWisdom("psionicist", 16, 2)
		c.Race = "dwarf"
		c.RaceSaveBonus = charactersvc.Saves{RodStaffWand: 7, Spell: 4}
		c.ClassSaveBonuses = []charactersvc.ClassSaveBonus{
			{Class: "psionicist", Against: classes.SaveAgainstMindAffecting, Bonus: 3},
			{Class: "psionicist", Against: "fire", Bonus: 9},
		}
		return c
	}
	race := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModRace), Detail: "dwarf", Value: 4}
	wisdom := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModWisdom), Detail: "16", Value: 2}
	class := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModClass), Detail: "psionicist", Value: 3}
	buff := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModBuff), Detail: "Prayer", Value: 1}
	cases := []struct {
		name          string
		mindAffecting bool
		d20           int
		wantMods      []combatsvc.BreakdownModifier
		wantTotal     int
		wantPassed    bool
		wantMargin    int
	}{
		{"flagged: race, Wisdom, class, buff", true, 5, []combatsvc.BreakdownModifier{race, wisdom, class, buff}, 15, true, 1},
		{"unflagged: race and buff", false, 6, []combatsvc.BreakdownModifier{race, buff}, 11, false, -3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().View(mock.Anything, charID).Return(char(), nil).Once()
			f.dice.EXPECT().RollD20().Return(tc.d20).Once()
			mods := strMods(12)
			mods.SaveBonus = 1
			mods.SaveBonusFrom = []buffsvc.Contribution{{Buff: "Prayer", Value: 1}}
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, buffsvc.WeaponNone).Return(mods, nil).Once()
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell, combatsvc.SaveOptions{MindAffecting: tc.mindAffecting})
			require.NoError(t, err)
			assert.Equal(t, tc.wantMods, res.Modifiers)
			assert.Equal(t, 14, res.Target, "the target is the derived one, unmoved")
			assert.Equal(t, 14, res.AdjustedTarget)
			assert.Equal(t, tc.d20, res.D20Roll)
			assert.Equal(t, tc.wantTotal, res.RollTotal)
			assert.Equal(t, tc.wantPassed, res.Passed)
			assert.Equal(t, tc.wantPassed, res.RolledPassed)
			assert.Equal(t, tc.wantMargin, res.Margin)

			require.Len(t, f.entries, 1, "one save logged")
			var logged combatsvc.SaveBreakdown
			require.NoError(t, json.Unmarshal(f.entries[0].Payload, &logged))
			assert.Equal(t, tc.wantMods, logged.Modifiers, "the log holds the same terms in the same order")
		})
	}
}
