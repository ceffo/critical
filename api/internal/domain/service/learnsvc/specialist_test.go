package learnsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The specialist wizard's spellbook, ADR 0050 steps 6 to 8. A transmuter
// (PHB Table 22: school alteration, opposition abjuration and necromancy) at
// Intelligence 14 (Table 4: 7th level, 60%, 9 per level).
var int14Transmuter = charactersvc.SpellbookRules{
	ClassMaxSpellLevel: 9, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9,
	Specialization: spellbook.Specialization{
		Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"},
	},
	SchoolAdjustment: 15, OtherAdjustment: -15,
}

// specialistSpells: 2nd-level wizard spells of his school (levitate), of
// another school (web), of none (no code), opposed (spectral_hand,
// necromancy) and of two schools, his and an opposed one (opposed wins, Q4).
var specialistSpells = map[string]learnsvc.Spell{
	"levitate":      {Level: 2, Wizard: true, Schools: []string{"alteration"}},
	"web":           {Level: 2, Wizard: true, Schools: []string{"conjuration_summoning"}},
	"nocode":        {Level: 2, Wizard: true},
	"spectral_hand": {Level: 2, Wizard: true, Schools: []string{"necromancy"}},
	"both":          {Level: 2, Wizard: true, Schools: []string{"alteration", "necromancy"}},
}

func transmuter(owed ...spellbook.FreeSpell) charactersvc.View {
	c := mage(5, &int14Transmuter)
	c.LevelUp.FreeSpellsOwed = owed
	return c
}

// ADR 0050 step 6: Learn, Ajouter and the free spell refuse an opposed spell
// with spell_school_opposed before any die (the dice mock has no
// expectation) and write nothing (no write or log expectation).
func TestLearn_Step6_OpposedRefusedBeforeAnyDie(t *testing.T) {
	owed := spellbook.FreeSpell{Level: 3, SpellLevel: 2}
	for _, tc := range []struct {
		name  string
		spell string
		do    func(learnsvc.SVC, string) error
	}{
		{"learn", "spectral_hand", func(s learnsvc.SVC, id string) error { _, err := s.Learn(context.Background(), charID, id); return err }},
		{"learn, a spell of his school and an opposed one", "both", func(s learnsvc.SVC, id string) error {
			_, err := s.Learn(context.Background(), charID, id)
			return err
		}},
		{"add", "spectral_hand", func(s learnsvc.SVC, id string) error { _, err := s.Add(context.Background(), charID, id); return err }},
		{"free spell", "spectral_hand", func(s learnsvc.SVC, id string) error {
			_, err := s.AddFreeSpell(context.Background(), charID, id, 3)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.catalogue(specialistSpells)
			f.chars.EXPECT().View(mock.Anything, charID).Return(transmuter(owed), nil).Once()
			f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
			require.ErrorIs(t, tc.do(f.svc, tc.spell), learnsvc.ErrSchoolOpposed)
		})
	}
	t.Run("a mage learns the same spell", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		f.chars.EXPECT().View(mock.Anything, charID).Return(mage(5, &int14Mage), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
		f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
		f.dal.EXPECT().LearnFailure(mock.Anything, charID, "spectral_hand").Return(0, false, nil)
		f.dice.EXPECT().RollD100().Return(7).Once()
		f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "spectral_hand", Origin: spellbook.OriginLearned}).Return(nil).Once()
		f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "spectral_hand").Return(nil).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil).Once()
		res, err := f.svc.Learn(context.Background(), charID, "spectral_hand")
		require.NoError(t, err)
		assert.True(t, res.Learned)
	})
}

// learnOnce is one Learn of spell by c with the d100 showing roll, once;
// the write the result asks for is accepted.
func learnOnce(t *testing.T, c charactersvc.View, spell string, roll int) (*learnsvc.Result, actionlog.NewEntry) {
	t.Helper()
	f := newFixture(t)
	f.catalogue(specialistSpells)
	f.chars.EXPECT().View(mock.Anything, charID).Return(c, nil).Once()
	f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
	f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
	f.dal.EXPECT().LearnFailure(mock.Anything, charID, spell).Return(0, false, nil)
	f.dice.EXPECT().RollD100().Return(roll).Once()
	f.dal.EXPECT().AddEntry(mock.Anything, charID, mock.Anything).Return(nil).Maybe()
	f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, spell).Return(nil).Maybe()
	f.dal.EXPECT().RemoveEntry(mock.Anything, charID, spell).Return(nil).Maybe()
	f.dal.EXPECT().SetLearnFailure(mock.Anything, charID, spell, 5).Return(nil).Maybe()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, e actionlog.NewEntry) (int64, error) {
		got = e
		return 1, nil
	}).Once()
	res, err := f.svc.Learn(context.Background(), charID, spell)
	require.NoError(t, err)
	return res, got
}

// ADR 0050 step 7 (PHB p.32): a spell of his school is learned at Table 4's
// chance +15. A 75 on the d100 fails Intelligence 14's 60 and succeeds at
// 75; the breakdown carries the chance, the adjustment and the chance used,
// and so does the journal entry.
func TestLearn_Step7_SchoolSpellPlusFifteen(t *testing.T) {
	res, entry := learnOnce(t, transmuter(), "levitate", 75)
	assert.Equal(t, &learnsvc.Result{
		SpellID: "levitate", SpellLevel: 2, Roll: 75, Chance: 60, Adjustment: 15, ChanceUsed: 75, Learned: true, CharacterLevel: 5,
	}, res)
	assert.JSONEq(t, `{"spellId":"levitate","spellLevel":2,"roll":75,"chance":60,"adjustment":15,"chanceUsed":75,`+
		`"learned":true,"characterLevel":5,"retryLevel":0}`, string(entry.Payload))
}

// ADR 0050 step 7: any other spell, one of another school or of no school,
// is learned at Table 4's chance -15: a 46 succeeds at 60 and fails at 45.
func TestLearn_Step7_OtherSpellMinusFifteen(t *testing.T) {
	for _, spell := range []string{"web", "nocode"} {
		t.Run(spell, func(t *testing.T) {
			res, _ := learnOnce(t, transmuter(), spell, 46)
			assert.Equal(t, &learnsvc.Result{
				SpellID: spell, SpellLevel: 2, Roll: 46, Chance: 60, Adjustment: -15, ChanceUsed: 45, Learned: false,
				CharacterLevel: 5, RetryLevel: 6,
			}, res)
		})
	}
}

// ADR 0050 step 7: a mage is unchanged, whatever the spell's school: a 61
// fails his 60 on an alteration spell (it would succeed for a transmuter)
// and a 46 succeeds on a conjuration one (it would fail for a transmuter).
func TestLearn_Step7_AMageIsUnchanged(t *testing.T) {
	res, _ := learnOnce(t, mage(5, &int14Mage), "levitate", 61)
	assert.Equal(t, &learnsvc.Result{
		SpellID: "levitate", SpellLevel: 2, Roll: 61, Chance: 60, Adjustment: 0, ChanceUsed: 60, Learned: false,
		CharacterLevel: 5, RetryLevel: 6,
	}, res)
	res, _ = learnOnce(t, mage(5, &int14Mage), "web", 46)
	assert.True(t, res.Learned)
	assert.Equal(t, 60, res.ChanceUsed)
}

// ADR 0050 step 8: a specialist's free spell is of his school: a claim with
// web (conjuration) is refused with free_spell_not_of_school while levitate
// (alteration, 2nd level) is not in his book, nothing written; levitate is
// accepted with the origin level_grant.
func TestAddFreeSpell_Step8_ASpecialistsFreeSpellIsOfHisSchool(t *testing.T) {
	owed := spellbook.FreeSpell{Level: 3, SpellLevel: 2}
	t.Run("another school refused", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		f.chars.EXPECT().View(mock.Anything, charID).Return(transmuter(owed), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
		f.spells.EXPECT().WizardSpellsOfLevel(2).Return([]string{"both", "levitate", "nocode", "spectral_hand", "web"}).Once()
		_, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 3)
		require.ErrorIs(t, err, learnsvc.ErrFreeSpellNotOfSchool)
	})
	t.Run("his school accepted", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		f.chars.EXPECT().View(mock.Anything, charID).Return(transmuter(owed), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
		f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
		f.dal.EXPECT().LearnFailure(mock.Anything, charID, "levitate").Return(0, false, nil)
		f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "levitate", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}).Return(nil).Once()
		f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "levitate").Return(nil).Once()
		var got actionlog.NewEntry
		f.recordAdd(&got)
		res, err := f.svc.AddFreeSpell(context.Background(), charID, "levitate", 3)
		require.NoError(t, err)
		assert.Equal(t, &learnsvc.Added{SpellID: "levitate", SpellLevel: 2, Origin: spellbook.OriginLevelGrant, GrantLevel: 3}, res)
	})
	t.Run("a mage's free spell is any", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		c := mage(5, &int14Mage)
		c.LevelUp.FreeSpellsOwed = []spellbook.FreeSpell{owed}
		f.chars.EXPECT().View(mock.Anything, charID).Return(c, nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return(nil, nil)
		f.book.EXPECT().Entries(mock.Anything, charID).Return(nil, nil)
		f.dal.EXPECT().LearnFailure(mock.Anything, charID, "web").Return(0, false, nil)
		f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "web", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}).Return(nil).Once()
		f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "web").Return(nil).Once()
		var got actionlog.NewEntry
		f.recordAdd(&got)
		_, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 3)
		require.NoError(t, err)
	})
}

// ADR 0050 step 8 (DMG p.41, "if there is a spell available"): when every
// 2nd-level wizard spell of his school is already in his book, the claim
// takes any spell he may add; an opposed one stays refused (step 6).
func TestAddFreeSpell_Step8_AnySpellWhenNoneOfHisSchoolIsLeft(t *testing.T) {
	owed := spellbook.FreeSpell{Level: 3, SpellLevel: 2}
	t.Run("another school accepted", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		f.chars.EXPECT().View(mock.Anything, charID).Return(transmuter(owed), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return([]string{"levitate"}, nil)
		f.spells.EXPECT().WizardSpellsOfLevel(2).Return([]string{"both", "levitate", "nocode", "spectral_hand", "web"}).Once()
		f.book.EXPECT().Entries(mock.Anything, charID).Return([]spellbook.Entry{{Key: "levitate", Origin: spellbook.OriginLearned}}, nil)
		f.dal.EXPECT().LearnFailure(mock.Anything, charID, "web").Return(0, false, nil)
		f.dal.EXPECT().AddEntry(mock.Anything, charID, spellbook.Entry{Key: "web", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}).Return(nil).Once()
		f.dal.EXPECT().DeleteLearnFailure(mock.Anything, charID, "web").Return(nil).Once()
		var got actionlog.NewEntry
		f.recordAdd(&got)
		res, err := f.svc.AddFreeSpell(context.Background(), charID, "web", 3)
		require.NoError(t, err)
		assert.Equal(t, spellbook.OriginLevelGrant, res.Origin)
	})
	t.Run("an opposed spell stays refused", func(t *testing.T) {
		f := newFixture(t)
		f.catalogue(specialistSpells)
		f.chars.EXPECT().View(mock.Anything, charID).Return(transmuter(owed), nil).Once()
		f.book.EXPECT().Keys(mock.Anything, charID).Return([]string{"levitate"}, nil)
		_, err := f.svc.AddFreeSpell(context.Background(), charID, "spectral_hand", 3)
		require.ErrorIs(t, err, learnsvc.ErrSchoolOpposed)
	})
}
