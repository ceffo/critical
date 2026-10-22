package combatsvc_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// ADR 0043 step 3: a class's save_bonus (the psionicist's +2 against the
// mind-affecting tag, Q12) is one modifier of kind class (detail: the class
// code) on the roll of a save whose request carries the tag, beside the
// Wisdom adjustment of ADR 0006 step 9: two labelled lines, both added, the
// target untouched, in any category. Unflagged, there is none; a class
// without the grant (a fighter) gets the Wisdom line only. Every roll is one
// d20 with its own value.
func TestResolveSavingThrow_Step3_ThePsionicistsTwoAgainstCharm(t *testing.T) {
	psionicist := func() *charactersvc.Character {
		c := withWisdom("psionicist", 17, 3)
		c.Derived.ClassSaveBonuses = []charactersvc.ClassSaveBonus{{Class: "psionicist", Against: classes.SaveAgainstMindAffecting, Bonus: 2}}
		return c
	}
	wisdom := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModWisdom), Detail: strconv.Itoa(17), Value: 3}
	class := combatsvc.BreakdownModifier{Kind: string(combatsvc.ModClass), Detail: "psionicist", Value: 2}
	cases := []struct {
		name          string
		char          *charactersvc.Character
		category      combatsvc.SaveCategory
		mindAffecting bool
		d20           int
		wantMods      []combatsvc.BreakdownModifier
		wantTotal     int
		wantPassed    bool
	}{
		{
			"a psionicist's flagged save: +3 and +2, exactly at the target", psionicist(), combatsvc.SaveSpell, true, 9,
			[]combatsvc.BreakdownModifier{wisdom, class},
			14, true,
		},
		{
			"a psionicist's flagged save: one below the target", psionicist(), combatsvc.SaveSpell, true, 8,
			[]combatsvc.BreakdownModifier{wisdom, class},
			13, false,
		},
		{
			"a psionicist's flagged Rod/Staff/Wand save", psionicist(), combatsvc.SaveRodStaffWand, true, 7,
			[]combatsvc.BreakdownModifier{wisdom, class},
			12, false,
		},
		{
			"a psionicist's unflagged save: neither", psionicist(), combatsvc.SaveSpell, false, 13,
			[]combatsvc.BreakdownModifier{},
			13, false,
		},
		{
			"a fighter's flagged save: the Wisdom line only", withWisdom("fighter", 17, 3), combatsvc.SaveSpell, true, 11,
			[]combatsvc.BreakdownModifier{wisdom},
			14, true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(c.char)
			f.dice.EXPECT().RollD20().Return(c.d20).Once()
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, buffsvc.BaseStrengthOf(c.char), buffsvc.WeaponNone).Return(strMods(12), nil).Once()
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, c.category, combatsvc.SaveOptions{MindAffecting: c.mindAffecting})
			require.NoError(t, err)
			target := savesOf(c.char)[c.category]
			assert.Equal(t, c.wantMods, res.Modifiers)
			assert.Equal(t, c.d20, res.D20Roll)
			assert.Equal(t, c.wantTotal, res.RollTotal)
			assert.Equal(t, target, res.Target, "the bonus never moves the target")
			assert.Equal(t, c.wantPassed, res.Passed)

			// Logged with the roll: the modifier is in the save entry's payload.
			require.Len(t, f.entries, 1)
			p := payloadOf(t, f.entries[0].Payload)
			assert.Len(t, p["modifiers"], len(c.wantMods))
			assert.Equal(t, float64(c.wantTotal), p["rollTotal"])
		})
	}
}
