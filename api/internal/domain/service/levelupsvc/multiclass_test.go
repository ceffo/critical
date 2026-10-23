package levelupsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
)

// ADR 0042's level-up of one class entry of a multi-class character.

// owing is an entry of class realized at level owing pending (each named in
// the entry, out of n classes, as Derive sends them); state is active unless
// frozen.
type owing struct {
	class   string
	level   int
	frozen  bool
	pending []charactersvc.PendingLevel
}

// multi is a character of Constitution con and max HP hpMax with the
// entries es, in order: the first one is the mirror's class.
func multi(con, hpMax int, es ...owing) charactersvc.View {
	active := 0
	opts := []viewtest.Option{viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(10, 10, con, 10, 10, 10)}
	for _, e := range es {
		if e.frozen {
			opts = append(opts, viewtest.WithFrozen(e.class, e.level))
		} else {
			active++
			opts = append(opts, viewtest.WithEntry(e.class, e.level))
		}
	}
	v := viewtest.Multi(opts...)
	v.HPMax = hpMax
	all := []charactersvc.PendingLevel{}
	for i, e := range es {
		v.Entries[i].Group = groupOf[e.class]
		v.Entries[i].LevelUp = charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}}
		for _, p := range e.pending {
			p.Class, p.Classes = e.class, active
			v.Entries[i].LevelUp.Pending = append(v.Entries[i].LevelUp.Pending, p)
			all = append(all, p)
		}
	}
	v.LevelUp = charactersvc.LevelUp{Pending: all}
	return v
}

// fixed is a pending level above the name level with the group's gain.
func fixed(level, gain int) charactersvc.PendingLevel {
	return charactersvc.PendingLevel{Level: level, FixedGain: gain}
}

func d4(level int) charactersvc.PendingLevel { return charactersvc.PendingLevel{Level: level, Die: 4} }

func d8(level int) charactersvc.PendingLevel { return charactersvc.PendingLevel{Level: level, Die: 8} }

// ADR 0042 step 7: "The level-up request names the entry's class with the
// level (level must be that entry's lowest pending one)". A cleric/fighter
// whose fighter owes 5: the fighter's 5 is rolled and written to the
// fighter's entry, the snapshot naming it; the cleric's 5 (none pending), an
// unknown class's and a frozen entry's are refused with level_up_not_pending
// before any roll or write.
func TestRoll_Step7_TheRequestNamesTheEntry(t *testing.T) {
	cleric := owing{class: "cleric", level: 5}
	fighter := owing{class: "fighter", level: 4, pending: []charactersvc.PendingLevel{d10(5)}}
	t.Run("the named entry's level is rolled and written to it", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().View(mock.Anything, charID).Return(multi(14, 30, cleric, fighter), nil).Once()
		f.dice.EXPECT().RollDie(10).Return(7).Once()
		f.rules.EXPECT().ConstitutionFor(14).Return(con(0, 0, 0), nil).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "fighter", 5, 33).Return(nil).Once()
		rec := f.recorded()

		got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "fighter", Level: 5})
		require.NoError(t, err)
		assert.Equal(t, "fighter", got.Class)
		assert.Equal(t, 2, got.Classes)
		assert.Empty(t, got.Pending)
		require.Len(t, *rec, 1)
		e := (*rec)[0]
		assert.JSONEq(t, `{"class":"fighter","classes":2,"dieShare":3,"level":5,"hitDie":10,"rolled":7,"counted":7,
			"conAdjustment":0,"gain":3,"fixed":false,"manual":false,"hpMaxBefore":30,"hpMaxAfter":33}`, string(e.Payload))
		assert.JSONEq(t, `{"class":"fighter","realizedLevel":4,"hpMax":30}`, snaptest.State(t, e.Snapshots[0].Before))
		assert.JSONEq(t, `{"class":"fighter","realizedLevel":5,"hpMax":33}`, snaptest.State(t, e.Snapshots[0].After))
	})
	for _, tc := range []struct {
		name  string
		c     charactersvc.View
		class string
	}{
		{"another entry's level", multi(14, 30, cleric, fighter), "cleric"},
		{"no request class is the first entry's", multi(14, 30, cleric, fighter), ""},
		{"a class the character does not have", multi(14, 30, cleric, fighter), "mage"},
		{"a frozen entry", multi(14, 30, owing{class: "fighter", level: 4, frozen: true}, owing{class: "mage", level: 6}), "fighter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t) // no dice, DAL or log call expected
			f.chars.EXPECT().View(mock.Anything, charID).Return(tc.c, nil).Once()
			_, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: tc.class, Level: 5})
			require.ErrorIs(t, err, levelupsvc.ErrNotPending)
		})
	}
}

// ADR 0042 step 10.2: "A level-up of one entry: that class's hit die (with
// Table 3's raised low dice) divided by the number of the character's
// classes, rounded down, at least 1 [...]; plus that class's share of the
// Constitution adjustment, the adjustment divided by the number of classes
// (Q7: the share rounded down, a warrior class's share using the warrior
// column)." The total stays at least 1 (one class is ADR 0030 step 7.1's).
// Each case rolls one die, a distinct value, through the dice port once.
func TestRoll_Step10_2_DieDividedByTheClasses(t *testing.T) {
	three := func(p charactersvc.PendingLevel) []owing {
		return []owing{{class: "mage", level: 5}, {class: "thief", level: 5, pending: []charactersvc.PendingLevel{p}}, {class: "fighter", level: 5}}
	}
	for _, tc := range []struct {
		name                     string
		con                      int
		adj, warrior             int
		es                       []owing
		class                    string
		die, rolled              int
		dieShare, conShare, gain int
	}{
		{"a thief's d6 of 4 in three classes gives 1", 16, 2, 2, three(d6(6)), "thief", 6, 4, 1, 0, 1},
		{"a thief's d6 of 1 in three classes gives 1, never 0", 16, 2, 2, three(d6(6)), "thief", 6, 1, 1, 0, 1},
		{
			"a fighter's d10 of 9 in two classes, the warrior's +3 shared", 17, 2, 3,
			[]owing{{class: "cleric", level: 5}, {class: "fighter", level: 5, pending: []charactersvc.PendingLevel{d10(6)}}},
			"fighter", 10, 9, 4, 1, 5,
		},
		{
			"a cleric's d8 of 5 in two classes, the plain +2 shared", 17, 2, 3,
			[]owing{{class: "cleric", level: 5, pending: []charactersvc.PendingLevel{d8(6)}}, {class: "fighter", level: 5}},
			"cleric", 8, 5, 2, 1, 3,
		},
		{
			"a mage's d4 of 3 in two classes, a -1 shared down to -1, the gain at least 1", 5, -1, -1,
			[]owing{{class: "fighter", level: 5}, {class: "mage", level: 5, pending: []charactersvc.PendingLevel{d4(6)}}},
			"mage", 4, 3, 1, -1, 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			c := multi(tc.con, 30, tc.es...)
			f.chars.EXPECT().View(mock.Anything, charID).Return(c, nil).Once()
			f.dice.EXPECT().RollDie(tc.die).Return(tc.rolled).Once()
			f.rules.EXPECT().ConstitutionFor(tc.con).Return(con(tc.adj, tc.warrior, 0), nil).Once()
			f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, tc.class, 6, 30+tc.gain).Return(nil).Once()
			f.recorded()

			got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: tc.class, Level: 6})
			require.NoError(t, err)
			assert.Equal(t, tc.rolled, *got.Rolled)
			assert.Equal(t, len(tc.es), got.Classes)
			assert.Equal(t, tc.dieShare, got.DieShare, "die share")
			assert.Equal(t, tc.conShare, got.ConShare, "Constitution share")
			assert.Equal(t, tc.gain, got.Gain, "gain")
			assert.Equal(t, 30+tc.gain, got.HPMaxAfter)
		})
	}
}

// ADR 0042 step 10.3: "Above the entry's name level: the class's fixed gain,
// divided by the number of classes like a die (Q8), no Constitution share"
// (Q8: at least 1). No die is rolled and Table 3 is not read.
func TestRoll_Step10_3_FixedGainAboveTheNameLevel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		es    []owing
		class string
		gain  int
	}{
		{"a fighter's +3 in two classes gives 1", []owing{{class: "cleric", level: 9}, {class: "fighter", level: 9, pending: []charactersvc.PendingLevel{fixed(10, 3)}}}, "fighter", 1},
		{"a thief's +2 in three classes gives 1, never 0", []owing{
			{class: "mage", level: 9}, {class: "fighter", level: 9}, {class: "thief", level: 10, pending: []charactersvc.PendingLevel{fixed(11, 2)}},
		}, "thief", 1},
		{"one class keeps its whole +3 (ADR 0030 step 7.2)", []owing{{class: "fighter", level: 9, pending: []charactersvc.PendingLevel{fixed(10, 3)}}}, "fighter", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t) // no dice or Table 3 call expected
			lvl := tc.es[len(tc.es)-1].pending[0].Level
			f.chars.EXPECT().View(mock.Anything, charID).Return(multi(18, 60, tc.es...), nil).Once()
			class := tc.class
			if len(tc.es) == 1 {
				class = "" // a single-class write names no class
			}
			f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, class, lvl, 60+tc.gain).Return(nil).Once()
			f.recorded()

			got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: tc.class, Level: lvl})
			require.NoError(t, err)
			assert.True(t, got.Fixed)
			assert.Nil(t, got.Rolled)
			assert.Zero(t, got.ConShare)
			assert.Equal(t, tc.gain, got.DieShare)
			assert.Equal(t, tc.gain, got.Gain)
		})
	}
}
