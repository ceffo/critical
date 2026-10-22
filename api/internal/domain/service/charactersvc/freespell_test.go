package charactersvc_test

import (
	"context"
	"slices"
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
			e.xp.EXPECT().Awards(mock.Anything, int64(4)).Return(nil, nil).Once()
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
	e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
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

// ADR 0040 step 3: the free spells of step 20.8 are the spellbook feature's
// free_spell_per_new_spell_level, not the class id. On the fixture's real
// tables, a mage of Int 17 realized at 3 with the XP of level 5 (PHB Table
// 20) whose log took level 3: Table 21 opens the 3rd spell level at 5
// (pending) and the 2nd at 3 (owed). A mage whose spellbook has the param
// false gets neither and pays no read; a test class with it true gets both.
func TestGetCharacter_Step20_8_FreeSpellsFromTheSpellbookParam(t *testing.T) {
	withFree := func(id string, free bool) func([]classes.Class) []classes.Class {
		return func(cs []classes.Class) []classes.Class {
			for i := range cs {
				if cs[i].ID != id {
					continue
				}
				fs := slices.Clone(cs[i].Features)
				for j := range fs {
					if fs[j].Kind == classes.KindSpellbook {
						sb := *fs[j].Spellbook
						sb.FreeSpellPerNewSpellLevel = free
						fs[j].Spellbook = &sb
					}
				}
				cs[i].Features = fs
			}
			return cs
		}
	}
	scribe := add("test_scribe", "mage", ruletablesvc.TableWizardSpells,
		classes.Feature{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 9, FreeSpellPerNewSpellLevel: true}})
	for _, tc := range []struct {
		name    string
		edit    func([]classes.Class) []classes.Class
		class   string
		pending []spellbook.FreeSpell
		owed    []spellbook.FreeSpell
		reads   int
	}{
		{"the book mage gets them", keep, "mage", []spellbook.FreeSpell{{Level: 5, SpellLevel: 3}}, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}, 1},
		{"a mage with the param false gets none", withFree("mage", false), "mage", nil, nil, 0},
		{"a test class with the param true gets them", scribe, "test_scribe", []spellbook.FreeSpell{{Level: 5, SpellLevel: 3}}, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			free := &freeSpells{taken: map[int64][]int{9: {3}}, claimed: map[int64][]spellbook.FreeSpell{}}
			svc, err := charactersvc.New(charactersvc.Deps{
				Reader: e.reader, XP: e.xp, Rules: edited(t, tc.edit), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
				Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book, FreeSpells: free,
			})
			require.NoError(t, err)
			c := &charactersvc.Character{
				ID: 9, Name: "Test", Class: tc.class, Race: "human", Str: 10, Dex: 12, Con: 11, Int: 17, Wis: 12, Cha: 10,
				HPMax: 9, RealizedLevel: 3, XPTotal: 20_000,
			}
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(9)).Return(c, nil).Once()
			e.xp.EXPECT().Awards(mock.Anything, int64(9)).Return(nil, nil).Once()

			got, err := svc.GetCharacter(context.Background(), 9)
			require.NoError(t, err)
			pending := map[int][]spellbook.FreeSpell{}
			var all []spellbook.FreeSpell
			for _, p := range got.Derived.LevelUp.Pending {
				pending[p.Level] = p.FreeSpells
				all = append(all, p.FreeSpells...)
			}
			require.Contains(t, pending, 5, "level 5 is pending")
			assert.Equal(t, tc.pending, all)
			assert.Equal(t, tc.owed, got.Derived.LevelUp.FreeSpellsOwed)
			assert.Equal(t, tc.reads, free.reads)
		})
	}
}
