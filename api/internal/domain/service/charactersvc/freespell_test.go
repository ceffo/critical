package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// table21 is PHB Table 21 (p.30), a mage's spells per day, levels 1 to 5.
var table21 = map[int]map[int]int{1: {1: 1}, 2: {1: 2}, 3: {1: 2, 2: 1}, 4: {1: 3, 2: 2}, 5: {1: 4, 2: 2, 3: 1}}

// ADR 0030 step 20.8 on a read: "Free spells owed = the new spell levels
// opened by the levels taken in the app ... minus the level_grant entries for
// those levels." A level 4 mage whose log holds the rolls of 2, 3 (twice: a
// redone roll is one level) and a stale 5 (above the realized level, not
// his) owes level 3's 2nd-level spell, once; claimed, he owes none. Table 21
// is read through the rules port.
func TestGetCharacter_Step20_8_OwedIsOpenedLevelsMinusGrants(t *testing.T) {
	for _, tc := range []struct {
		name    string
		claimed []spellbook.FreeSpell
		want    []spellbook.FreeSpell
	}{
		{"none claimed", nil, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}},
		{"claimed", []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			c := &charactersvc.Character{ID: 4, Class: "mage", Race: "human", Int: 14, HPMax: 9, RealizedLevel: 4, XPTotal: 10_000}
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(4)).Return(c, nil).Once()
			e.xp.EXPECT().EntriesTotal(mock.Anything, int64(4)).Return(0, nil).Once()
			expectMageRules(e.rules, 4)
			e.free.taken[4] = []int{3, 5, 2, 3}
			e.free.claimed[4] = tc.claimed

			got, err := e.svc.GetCharacter(context.Background(), 4)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Derived.LevelUp.FreeSpellsOwed)
			assert.Equal(t, 1, e.free.reads)
		})
	}
}

// ADR 0030 step 20.8: "A bard gets none": a bard's read never asks for the
// levels taken or the claims (nor would any class but the mage's).
func TestGetCharacter_Step20_8_ABardReadsNoFreeSpells(t *testing.T) {
	e := newEnv(t)
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
	expectKelsenRules(e.rules, 1_293_418, 15)
	e.free.taken[3] = []int{15}
	got, err := e.svc.GetCharacter(context.Background(), 3)
	require.NoError(t, err)
	assert.Nil(t, got.Derived.LevelUp.FreeSpellsOwed)
	assert.Zero(t, e.free.reads)
}

// expectMageRules answers a mage's lookups at level (target and realized),
// Table 21 from table21.
func expectMageRules(r *mocks.MockRules, level int) {
	r.EXPECT().LevelFor("mage", mock.Anything).Return(level, nil).Once()
	r.EXPECT().XPThreshold("mage", level+1).Return(20_000, nil).Once()
	r.EXPECT().THAC0For("wizard", level).Return(20, nil).Once()
	r.EXPECT().SavesFor("wizard", level).Return(ruletablesvc.SavingThrowRow{}, nil).Once()
	r.EXPECT().SpellsPerDay(classDef("mage"), mock.Anything, mock.Anything).RunAndReturn(func(_ classes.Class, l, _ int) (map[int]int, error) {
		return table21[l], nil
	})
}
