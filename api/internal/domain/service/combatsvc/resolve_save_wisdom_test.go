package combatsvc_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// withWisdom is the test bard as a character of class with Wisdom wis and
// Table 5's magical defence adjustment md at that Wisdom (derived by
// charactersvc, ADR 0006 step 9.1). Targets: Spells 14, Rod/Staff/Wand 13.
func withWisdom(class string, wis, md int) *charactersvc.Character {
	c := bard()
	c.Class, c.Wis = class, wis
	c.Derived.MagicDefence = md
	return c
}

// ADR 0006 step 9: on a save the player flags as against magic that attacks
// the mind, Table 5's adjustment at the sheet's Wisdom is one modifier of kind
// wisdom (detail: the Wisdom score, value: the signed adjustment) on the roll,
// never on the target, in any category and for any class. Without the flag,
// or with an adjustment of 0, there is none. Every roll is one d20 with its
// own value.
func TestResolveSavingThrow_Step9_WisdomMagicDefence(t *testing.T) {
	wisdom := func(wis, v int) combatsvc.BreakdownModifier {
		return combatsvc.BreakdownModifier{Kind: string(combatsvc.ModWisdom), Detail: strconv.Itoa(wis), Value: v}
	}
	cases := []struct {
		name          string
		char          *charactersvc.Character
		category      combatsvc.SaveCategory
		mindAffecting bool
		d20, buff     int
		wantMods      []combatsvc.BreakdownModifier
		wantTotal     int
		wantPassed    bool
		wantMargin    int
	}{
		{
			"Wisdom 17 flagged: +3, exactly at the target", withWisdom("cleric", 17, 3), combatsvc.SaveSpell, true, 11, 0,
			[]combatsvc.BreakdownModifier{wisdom(17, 3)},
			14, true, 0,
		},
		{
			"Wisdom 17 flagged: +3, one below the target", withWisdom("cleric", 17, 3), combatsvc.SaveSpell, true, 10, 0,
			[]combatsvc.BreakdownModifier{wisdom(17, 3)},
			13, false, -1,
		},
		{
			"Wisdom 3 flagged: -3, exactly at the target", withWisdom("cleric", 3, -3), combatsvc.SaveSpell, true, 17, 0,
			[]combatsvc.BreakdownModifier{wisdom(3, -3)},
			14, true, 0,
		},
		{
			"Wisdom 3 flagged: -3, one below the target", withWisdom("cleric", 3, -3), combatsvc.SaveSpell, true, 16, 0,
			[]combatsvc.BreakdownModifier{wisdom(3, -3)},
			13, false, -1,
		},
		{
			"Wisdom 12 flagged: an adjustment of 0 is no modifier", withWisdom("cleric", 12, 0), combatsvc.SaveSpell, true, 9, 0,
			[]combatsvc.BreakdownModifier{},
			9, false, -5,
		},
		{
			"Wisdom 17 unflagged: no modifier", withWisdom("cleric", 17, 3), combatsvc.SaveSpell, false, 12, 0,
			[]combatsvc.BreakdownModifier{},
			12, false, -2,
		},
		{
			"a flagged Rod/Staff/Wand save (a fear wand)", withWisdom("cleric", 17, 3), combatsvc.SaveRodStaffWand, true, 10, 0,
			[]combatsvc.BreakdownModifier{wisdom(17, 3)},
			13, true, 0,
		},
		{
			"a fighter gets it like a cleric", withWisdom("fighter", 17, 3), combatsvc.SaveSpell, true, 6, 0,
			[]combatsvc.BreakdownModifier{wisdom(17, 3)},
			9, false, -5,
		},
		{
			"a mage gets it like a cleric", withWisdom("mage", 3, -3), combatsvc.SaveSpell, true, 19, 0,
			[]combatsvc.BreakdownModifier{wisdom(3, -3)},
			16, true, 2,
		},
		{
			"before the buffs, both added", withWisdom("cleric", 16, 2), combatsvc.SaveSpell, true, 7, 5,
			[]combatsvc.BreakdownModifier{wisdom(16, 2), {Kind: string(combatsvc.ModBuff), Detail: "Prayer", Value: 5}},
			14, true, 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(c.char)
			f.dice.EXPECT().RollD20().Return(c.d20).Once()
			mods := strMods(12)
			if c.buff != 0 {
				mods.SaveBonus = c.buff
				mods.SaveBonusFrom = []buffsvc.Contribution{{Buff: "Prayer", Value: c.buff}}
			}
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, buffsvc.BaseStrengthOf(c.char), buffsvc.WeaponNone).Return(mods, nil).Once()
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, c.category, combatsvc.SaveOptions{MindAffecting: c.mindAffecting})
			require.NoError(t, err)
			target := savesOf(c.char)[c.category]
			assert.Equal(t, c.wantMods, res.Modifiers)
			assert.Equal(t, c.d20, res.D20Roll)
			assert.Equal(t, c.wantTotal, res.RollTotal)
			assert.Equal(t, target, res.Target, "the adjustment never moves the target")
			assert.Equal(t, target, res.AdjustedTarget)
			assert.Equal(t, c.wantPassed, res.Passed)
			assert.Equal(t, c.wantPassed, res.RolledPassed)
			assert.Equal(t, c.wantMargin, res.Margin)
			assert.Equal(t, c.mindAffecting, res.MindAffecting)

			// Step 9.5: the flag and the modifier are logged with the roll.
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindSave, f.entries[0].Kind)
			p := payloadOf(t, f.entries[0].Payload)
			assert.Equal(t, c.mindAffecting, p["mindAffecting"])
			assert.Len(t, p["modifiers"], len(c.wantMods))
			assert.Equal(t, float64(c.wantTotal), p["rollTotal"])
		})
	}
}

// ADR 0006 step 9.5: the wisdom modifier is stored among the modifiers, so the
// DM's adjustment (step 5) keeps it and the flag: the roll total stays d20 +
// Wisdom, only the target moves.
func TestAdjustSave_Step9_TheWisdomModifierIsKept(t *testing.T) {
	f := newFixture(t)
	f.withChar(withWisdom("cleric", 17, 3))
	f.dice.EXPECT().RollD20().Return(10).Once()
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, buffsvc.WeaponNone).Return(strMods(12), nil).Once()
	f.records()
	rolled, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell, combatsvc.SaveOptions{MindAffecting: true})
	require.NoError(t, err)
	require.False(t, rolled.Passed, "10 + 3 is one below 14")
	require.Len(t, f.entries, 1)
	e := &actionlog.Entry{ID: 42, CharacterID: charID, Kind: f.entries[0].Kind, Payload: f.entries[0].Payload}
	amends(f, e)

	res, err := f.svc.AdjustSave(context.Background(), charID, 42, combatsvc.SaveAdjustment{Value: 1, Reason: "Gaia"})
	require.NoError(t, err)
	wisdom := []combatsvc.BreakdownModifier{{Kind: string(combatsvc.ModWisdom), Detail: "17", Value: 3}}
	assert.Equal(t, wisdom, res.Modifiers)
	assert.True(t, res.MindAffecting)
	assert.Equal(t, 13, res.RollTotal, "d20 10 + Wisdom 3, the adjustment left out")
	assert.Equal(t, 13, res.AdjustedTarget)
	assert.True(t, res.Passed)
	assert.False(t, res.RolledPassed)
	assert.Equal(t, 0, res.Margin)

	p := payloadOf(t, e.EffectivePayload())
	assert.Equal(t, true, p["mindAffecting"])
	assert.Equal(t, []any{map[string]any{"kind": "wisdom", "detail": "17", "value": 3.0}}, p["modifiers"])
	assert.Equal(t, 13.0, p["rollTotal"])
}

// savesOf keys the character's derived targets by save category.
func savesOf(c *charactersvc.Character) map[combatsvc.SaveCategory]int {
	s := c.Derived.Saves
	return map[combatsvc.SaveCategory]int{
		combatsvc.SaveParaPoison: s.ParaPoison, combatsvc.SaveRodStaffWand: s.RodStaffWand,
		combatsvc.SavePetriPoly: s.PetriPoly, combatsvc.SaveBreath: s.Breath, combatsvc.SaveSpell: s.Spell,
	}
}
