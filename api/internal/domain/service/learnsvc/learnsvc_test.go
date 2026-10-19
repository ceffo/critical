package learnsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc/mocks"
)

// Learning a spell, ADR 0030 step 20. 20.1: "Refused (400, before any die)
// when its level is above the Intelligence's maximum spell level (PHB Table
// 4), above the class's highest spell level (bard 6th, mage 9th), or when the
// spellbook already holds the Intelligence's maximum number of spells of that
// level (Table 4; "All" has no maximum)." 20.2: "Otherwise one d100 through
// the dice port; at or under the Intelligence's chance to learn (Table 4) the
// spell is written in the spellbook; above it, the failure is logged and that
// spell is refused for this character until his level is higher than at the
// failure." 20.3: "One transaction, success or failure; undoing a failure
// lifts its block."

type recordingTx struct{ n int }

func (r *recordingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	r.n++
	return fn(ctx)
}

type fixture struct {
	svc    learnsvc.SVC
	dal    *mocks.MockDAL
	book   *mocks.MockBook
	spells *mocks.MockSpells
	chars  *mocks.MockCharacters
	dice   *mocks.MockDice
	log    *mocks.MockLog
	tx     *recordingTx
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		dal: mocks.NewMockDAL(t), book: mocks.NewMockBook(t), spells: mocks.NewMockSpells(t),
		chars: mocks.NewMockCharacters(t), dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), tx: &recordingTx{},
	}
	svc, err := learnsvc.New(learnsvc.Deps{
		DAL: f.dal, Book: f.book, Spells: f.spells, Characters: f.chars, Dice: f.dice, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

const charID = int64(4)

// Intelligence 14 (Table 4: 7th level, 60%, 9 per level) for a mage.
var int14Mage = charactersvc.SpellbookRules{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9}

func mage(level int, rules *charactersvc.SpellbookRules) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Class: "mage", RealizedLevel: level,
		Derived: charactersvc.Derived{RealizedLevel: level, Spellbook: rules},
	}
}

// catalogue answers the catalogue's lookups from spells.
func (f fixture) catalogue(spells map[string]learnsvc.Spell) {
	f.spells.EXPECT().Spell(mock.Anything).RunAndReturn(func(id string) (learnsvc.Spell, bool) {
		s, ok := spells[id]
		return s, ok
	}).Maybe()
}

func TestNew_RejectsNilDependencies(t *testing.T) {
	_, err := learnsvc.New(learnsvc.Deps{})
	require.Error(t, err)
}

func TestLearn_Step20_1_RefusedBeforeAnyDie(t *testing.T) {
	full := make([]string, 0, 9)
	cat := map[string]learnsvc.Spell{
		"new3": {Level: 3, Wizard: true}, "new8": {Level: 8, Wizard: true}, "new7": {Level: 7, Wizard: true},
		"cure": {Level: 1, Wizard: false}, "known": {Level: 3, Wizard: true},
	}
	for i := range 9 {
		k := string(rune('a'+i)) + "_3"
		full = append(full, k)
		cat[k] = learnsvc.Spell{Level: 3, Wizard: true}
	}
	bard := charactersvc.SpellbookRules{ClassMaxSpellLevel: 6, IntMaxSpellLevel: 9, LearnChance: 85, MaxSpellsPerLevel: 18}
	for _, tc := range []struct {
		name     string
		c        *charactersvc.Character
		spell    string
		book     []string
		failedAt int
		want     error
	}{
		{"a fighter has no spellbook", mage(5, nil), "new3", nil, 0, learnsvc.ErrNoSpellbook},
		{"a priest spell", mage(5, &int14Mage), "cure", nil, 0, learnsvc.ErrNotAWizardSpell},
		{"already in the spellbook", mage(5, &int14Mage), "known", []string{"known"}, 0, learnsvc.ErrAlreadyKnown},
		{"above the Intelligence's 7th level", mage(5, &int14Mage), "new8", nil, 0, learnsvc.ErrAboveIntelligence},
		{"above the bard's 6th level", mage(5, &bard), "new7", nil, 0, learnsvc.ErrAboveClass},
		{"nine 3rd-level spells at Intelligence 14", mage(5, &int14Mage), "new3", full, 0, learnsvc.ErrLevelFull},
		{"failed at this level", mage(5, &int14Mage), "new3", nil, 5, learnsvc.ErrFailedAtLevel},
		{"failed at a higher level (an undone level-up)", mage(5, &int14Mage), "new3", nil, 6, learnsvc.ErrFailedAtLevel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(cat)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.c, nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(tc.book, nil).Maybe()
			f.dal.EXPECT().LearnFailure(mock.Anything, charID, tc.spell).Return(tc.failedAt, tc.failedAt > 0, nil).Maybe()
			_, err := f.svc.Learn(context.Background(), charID, tc.spell)
			require.ErrorIs(t, err, tc.want)
			// No die (the dice mock has no expectation), nothing written.
		})
	}
	t.Run("a spell the catalogue does not hold", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(cat)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
		_, err := f.svc.Learn(context.Background(), charID, "nope")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// "All" (Intelligence 19 and up) has no maximum; a failure at a lower level
// no longer blocks; the other levels' spells do not count.
func TestLearn_Step20_1_WhatIsNotRefused(t *testing.T) {
	cat := map[string]learnsvc.Spell{"new3": {Level: 3, Wizard: true}}
	book := []string{}
	for i := range 20 {
		k := string(rune('a'+i)) + "_3"
		book = append(book, k)
		cat[k] = learnsvc.Spell{Level: 3, Wizard: true}
	}
	all := charactersvc.SpellbookRules{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 9, LearnChance: 95, NoMaximum: true}
	nine := []string{"o1", "o2", "o3", "o4", "o5", "o6", "o7", "o8", "o9"}
	for _, k := range nine {
		cat[k] = learnsvc.Spell{Level: 2, Wizard: true}
	}
	for _, tc := range []struct {
		name     string
		c        *charactersvc.Character
		book     []string
		failedAt int
	}{
		{"twenty 3rd-level spells at Intelligence 19", mage(5, &all), book, 0},
		{"failed at level 4, now level 5", mage(5, &int14Mage), nil, 4},
		{"nine 2nd-level spells do not fill the 3rd", mage(5, &int14Mage), nine, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(cat)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.c, nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(tc.book, nil)
			f.dal.EXPECT().LearnFailure(mock.Anything, charID, "new3").Return(tc.failedAt, tc.failedAt > 0, nil)
			f.dice.EXPECT().RollD100().Return(1).Once()
			f.dal.EXPECT().AddEntry(mock.Anything, charID, "new3").Return(nil).Once()
			f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil).Once()
			res, err := f.svc.Learn(context.Background(), charID, "new3")
			require.NoError(t, err)
			assert.True(t, res.Learned)
		})
	}
}

// One d100 against the chance: 60 at a chance of 60 learns, 61 fails and
// blocks the spell at the realized level. One transaction, one entry, its
// snapshot the spell's place before and after.
func TestLearn_Step20_2_OneD100AtOrUnderTheChance(t *testing.T) {
	for _, tc := range []struct {
		roll     int
		learned  bool
		before   string
		after    string
		failedAt int
		retry    int
	}{
		{60, true, `{"known":false,"failedAtLevel":0}`, `{"known":true,"failedAtLevel":0}`, 0, 0},
		// A failure at level 5 may be tried again from level 6, the next one.
		{61, false, `{"known":false,"failedAtLevel":0}`, `{"known":false,"failedAtLevel":5}`, 5, 6},
	} {
		t.Run(map[bool]string{true: "learned", false: "failed"}[tc.learned], func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(map[string]learnsvc.Spell{"new3": {Level: 3, Wizard: true}})
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return([]string{}, nil)
			f.dal.EXPECT().LearnFailure(mock.Anything, charID, "new3").Return(0, false, nil)
			f.dice.EXPECT().RollD100().Return(tc.roll).Once()
			if tc.learned {
				f.dal.EXPECT().AddEntry(mock.Anything, charID, "new3").Return(nil).Once()
				f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
			} else {
				f.dal.EXPECT().RemoveEntry(mock.Anything, charID, "new3").Return(nil).Once()
				f.dal.EXPECT().SetLearnFailure(mock.Anything, charID, "new3", 5).Return(nil).Once()
			}
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, e actionlog.NewEntry) (int64, error) {
				got = e
				return 1, nil
			}).Once()
			res, err := f.svc.Learn(context.Background(), charID, "new3")
			require.NoError(t, err)
			assert.Equal(t, &learnsvc.Result{
				SpellID: "new3", SpellLevel: 3, Roll: tc.roll, Chance: 60, Learned: tc.learned, CharacterLevel: 5,
				RetryLevel: tc.retry,
			}, res)
			assert.Equal(t, 1, f.tx.n, "one transaction")
			assert.Equal(t, actionlog.KindSpellLearn, got.Kind)
			assert.JSONEq(t, `{"spellId":"new3","spellLevel":3,"roll":`+itoa(tc.roll)+`,"chance":60,"learned":`+
				map[bool]string{true: "true", false: "false"}[tc.learned]+`,"characterLevel":5,"retryLevel":`+itoa(tc.retry)+`}`, string(got.Payload))
			require.Len(t, got.Snapshots, 1)
			assert.Equal(t, "spellbook:new3", got.Snapshots[0].Key)
			assert.Equal(t, "new3", snaptest.ID(t, got.Snapshots[0].Before), "the envelope names the spell")
			assert.JSONEq(t, tc.before, snaptest.State(t, got.Snapshots[0].Before))
			assert.JSONEq(t, tc.after, snaptest.State(t, got.Snapshots[0].After))
		})
	}
}

// Undoing a success takes the spell out; undoing a failure lifts its block
// (or puts back an older one, from a lower level); a redo writes the "after".
func TestRestore_Step20_3_UndoLiftsTheBlock(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		expect      func(f fixture)
	}{
		{"before a success", `{"known":false,"failedAtLevel":0}`, func(f fixture) {
			f.dal.EXPECT().RemoveEntry(mock.Anything, charID, "new3").Return(nil).Once()
			f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
		}},
		{"before a failure at level 6, an older one at 4", `{"known":false,"failedAtLevel":4}`, func(f fixture) {
			f.dal.EXPECT().RemoveEntry(mock.Anything, charID, "new3").Return(nil).Once()
			f.dal.EXPECT().SetLearnFailure(mock.Anything, charID, "new3", 4).Return(nil).Once()
		}},
		{"a redo of a success", `{"known":true,"failedAtLevel":0}`, func(f fixture) {
			f.dal.EXPECT().AddEntry(mock.Anything, charID, "new3").Return(nil).Once()
			f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "new3").Return(nil).Once()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.expect(f)
			r := f.svc.Restorers()[learnsvc.StateFamily]
			require.NotNil(t, r)
			require.NoError(t, r.Restore(context.Background(), charID, snaptest.EnvID("new3", tc.state)))
		})
	}
	f := newFixture(t)
	require.Error(t, f.svc.Restorers()[learnsvc.StateFamily].Restore(context.Background(), charID, snaptest.Env(`{"known":true}`)),
		"a snapshot that names no spell")
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
