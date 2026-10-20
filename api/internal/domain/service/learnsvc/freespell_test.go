package learnsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The level-up free spell, ADR 0030 step 20.8: "it is added through the
// roll-free add of 20.6 (the same limits and refusals) with the origin
// level_grant and the realized level that granted it (20.5)." The dice mock
// has no expectation in these tests: a roll fails them.

// owingMage is a level 5 mage at Intelligence 14 who owes the free spells
// owed.
func owingMage(owed ...spellbook.FreeSpell) *charactersvc.Character {
	c := mage(5, &int14Mage)
	c.Derived.LevelUp.FreeSpellsOwed = owed
	return c
}

func TestAdd_Step20_8_ClaimingStoresTheLevelGrantOrigin(t *testing.T) {
	f := newFixture(t)
	f.catalogue(map[string]learnsvc.Spell{"web": {Level: 2, Wizard: true}})
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).
		Return(owingMage(spellbook.FreeSpell{Level: 3, SpellLevel: 2}, spellbook.FreeSpell{Level: 5, SpellLevel: 3}), nil).Once()
	f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
	f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
	f.dal.EXPECT().LearnFailure(mock.Anything, charID, "web").Return(0, false, nil)
	f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "web", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}).Return(nil).Once()
	f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "web").Return(nil).Once()
	var got actionlog.NewEntry
	f.recordAdd(&got)

	res, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 3)
	require.NoError(t, err)
	assert.Equal(t, &learnsvc.Added{SpellID: "web", SpellLevel: 2, Origin: spellbook.OriginLevelGrant, GrantLevel: 3}, res)
	assert.Equal(t, 1, f.tx.n, "one transaction")
	assert.Equal(t, actionlog.KindSpellAdd, got.Kind)
	assert.JSONEq(t, `{"spellId":"web","spellLevel":2,"origin":"level_grant","grantLevel":3}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.JSONEq(t, `{"known":false,"failedAtLevel":0}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"known":true,"failedAtLevel":0,"origin":"level_grant","grantLevel":3}`, snaptest.State(t, got.Snapshots[0].After))
}

// One row per refusal code of a claim: nothing written or logged (the mocks
// have no such expectation), no die.
func TestAdd_Step20_8_RefusedWhenNotOwed(t *testing.T) {
	cat := map[string]learnsvc.Spell{"web": {Level: 2, Wizard: true}, "sleep": {Level: 1, Wizard: true}}
	third := spellbook.FreeSpell{Level: 3, SpellLevel: 2}
	for _, tc := range []struct {
		code       string
		c          *charactersvc.Character
		spell      string
		grantLevel int
		want       error
	}{
		{"no_free_spell_owed: nothing owed", owingMage(), "web", 3, learnsvc.ErrNoFreeSpellOwed},
		{"no_free_spell_owed: another level owes it", owingMage(third), "web", 4, learnsvc.ErrNoFreeSpellOwed},
		{"free_spell_wrong_level", owingMage(third), "sleep", 3, learnsvc.ErrFreeSpellWrongLevel},
	} {
		t.Run(tc.code, func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(cat)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.c, nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
			_, err := f.svc.AddFreeSpell(context.Background(), charID, tc.spell, tc.grantLevel)
			require.ErrorIs(t, err, tc.want)
		})
	}
	t.Run("the limits of 20.1 come first", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(cat)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(owingMage(third), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return([]string{"web"}, nil)
		_, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 3)
		require.ErrorIs(t, err, learnsvc.ErrAlreadyKnown)
	})
	t.Run("a grant level under 1 reads nothing", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 0)
		require.ErrorIs(t, err, learnsvc.ErrNoFreeSpellOwed)
	})
}

// The claim's undo takes the spell out (the free spell is owed again, read
// from the book); its redo writes it back with its origin and level.
func TestAdd_Step20_8_RedoRestoresTheLevelGrant(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().RestoreEntry(mock.Anything, charID, spellbook.Entry{Key: "web", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}).Return(nil).Once()
	f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "web").Return(nil).Once()
	require.NoError(t, f.svc.Module().Restorer(learnsvc.StateFamily).Restore(context.Background(), charID,
		snaptest.EnvID("web", `{"known":true,"failedAtLevel":0,"origin":"level_grant","grantLevel":3}`)))
}
