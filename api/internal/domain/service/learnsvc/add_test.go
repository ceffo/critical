package learnsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// Ajouter, ADR 0030 step 20.6: "The player adds a wizard spell to a mage's or
// bard's spellbook without a roll ... No die is rolled (the dice port is not
// called). It is refused with the codes of 20.1 and the Learn endpoint, before
// any write ... It is not refused for a spell failed at this level ... One
// transaction and one undoable journal entry whose undo takes the spell out
// again; the entry's origin is given (20.5). A spell goes at the end of the
// book." The dice mock has no expectation in these tests: a roll fails them.

// recordAdd captures the journal entry the add records.
func (f fixture) recordAdd(got *actionlog.NewEntry) {
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, e actionlog.NewEntry) (int64, error) {
		*got = e
		return 1, nil
	}).Once()
}

func TestAdd_Step20_6_NoDieAndTheSpellIsInTheBook(t *testing.T) {
	f := newFixture(t)
	f.catalogue(map[string]learnsvc.Spell{"new3": {Level: 3, Wizard: true}, "old1": {Level: 1, Wizard: true}})
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
	f.book.EXPECT().Keys(mock.Anything, charID).Return([]string{"old1"}, nil)
	f.book.EXPECT().Entries(mock.Anything, charID).Return([]spellbook.Entry{{Key: "old1", Origin: spellbook.OriginStarting}}, nil)
	f.dal.EXPECT().LearnFailure(mock.Anything, charID, "new3").Return(0, false, nil)
	f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "new3", Origin: spellbook.OriginGiven}).Return(nil).Once()
	f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
	var got actionlog.NewEntry
	f.recordAdd(&got)

	res, err := f.svc.Add(context.Background(), charID, "new3")
	require.NoError(t, err)
	assert.Equal(t, &learnsvc.Added{SpellID: "new3", SpellLevel: 3, Origin: spellbook.OriginGiven}, res)
	assert.Equal(t, 1, f.tx.n, "one transaction")
	assert.Equal(t, actionlog.KindSpellAdd, got.Kind)
	assert.JSONEq(t, `{"spellId":"new3","spellLevel":3,"origin":"given"}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "spellbook:new3", got.Snapshots[0].Key)
	assert.Equal(t, "new3", snaptest.ID(t, got.Snapshots[0].After), "the envelope names the spell")
	assert.JSONEq(t, `{"known":false,"failedAtLevel":0}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"known":true,"failedAtLevel":0,"origin":"given"}`, snaptest.State(t, got.Snapshots[0].After))
}

// One row per refusal code of step 20.6 (the codes of 20.1 and the Learn
// endpoint): nothing is read past the refusal, nothing written or logged.
func TestAdd_Step20_6_RefusedLikeLearn(t *testing.T) {
	cat := map[string]learnsvc.Spell{
		"new3": {Level: 3, Wizard: true}, "new8": {Level: 8, Wizard: true}, "new7": {Level: 7, Wizard: true},
		"cure": {Level: 1, Wizard: false}, "known": {Level: 3, Wizard: true},
	}
	full := make([]string, 0, 9)
	for i := range 9 {
		k := string(rune('a'+i)) + "_3"
		full = append(full, k)
		cat[k] = learnsvc.Spell{Level: 3, Wizard: true}
	}
	bard := charactersvc.SpellbookRules{ClassMaxSpellLevel: 6, IntMaxSpellLevel: 9, LearnChance: 85, MaxSpellsPerLevel: 18}
	for _, tc := range []struct {
		code  string
		c     *charactersvc.Character
		spell string
		book  []string
		want  error
	}{
		{"no_spellbook", mage(5, nil), "new3", nil, learnsvc.ErrNoSpellbook},
		{"not_a_wizard_spell", mage(5, &int14Mage), "cure", nil, learnsvc.ErrNotAWizardSpell},
		{"already_in_spellbook", mage(5, &int14Mage), "known", []string{"known"}, learnsvc.ErrAlreadyKnown},
		{"spell_level_above_class", mage(5, &bard), "new7", nil, learnsvc.ErrAboveClass},
		{"spell_level_above_intelligence", mage(5, &int14Mage), "new8", nil, learnsvc.ErrAboveIntelligence},
		{"spellbook_level_full", mage(5, &int14Mage), "new3", full, learnsvc.ErrLevelFull},
	} {
		t.Run(tc.code, func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(cat)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.c, nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(tc.book, nil).Maybe()
			_, err := f.svc.Add(context.Background(), charID, tc.spell)
			require.ErrorIs(t, err, tc.want)
			// No die, no write, no entry: the mocks have no such expectation.
		})
	}
	t.Run("a spell the catalogue does not hold", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(cat)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
		_, err := f.svc.Add(context.Background(), charID, "nope")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// A spell failed at this level (and at a higher one, after an undone
// level-up) is added: the add lifts the block, its snapshot's "before" keeps
// it for the undo.
func TestAdd_Step20_6_NotRefusedForASpellFailedAtThisLevel(t *testing.T) {
	for _, failedAt := range []int{5, 6} {
		t.Run(itoa(failedAt), func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(map[string]learnsvc.Spell{"new3": {Level: 3, Wizard: true}})
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
			f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
			f.dal.EXPECT().LearnFailure(mock.Anything, charID, "new3").Return(failedAt, true, nil)
			f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "new3", Origin: spellbook.OriginGiven}).Return(nil).Once()
			f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
			var got actionlog.NewEntry
			f.recordAdd(&got)

			_, err := f.svc.Add(context.Background(), charID, "new3")
			require.NoError(t, err)
			assert.JSONEq(t, `{"known":false,"failedAtLevel":`+itoa(failedAt)+`}`, snaptest.State(t, got.Snapshots[0].Before))
			assert.JSONEq(t, `{"known":true,"failedAtLevel":0,"origin":"given"}`, snaptest.State(t, got.Snapshots[0].After))
		})
	}
}

// Undoing an add writes its "before" back: the spell out of the book, and the
// block it lifted back; a redo writes it in again, given.
func TestAdd_Step20_6_UndoTakesTheSpellOut(t *testing.T) {
	t.Run("undo", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().RemoveEntry(mock.Anything, charID, "new3").Return(nil).Once()
		f.dal.EXPECT().SetLearnFailure(mock.Anything, charID, "new3", 5).Return(nil).Once()
		require.NoError(t, f.svc.Module().Restorer(learnsvc.StateFamily).Restore(context.Background(), charID,
			snaptest.EnvID("new3", `{"known":false,"failedAtLevel":5}`)))
	})
	t.Run("redo", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "new3", Origin: spellbook.OriginGiven}).Return(nil).Once()
		f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
		require.NoError(t, f.svc.Module().Restorer(learnsvc.StateFamily).Restore(context.Background(), charID,
			snaptest.EnvID("new3", `{"known":true,"failedAtLevel":0,"origin":"given"}`)))
	})
}
