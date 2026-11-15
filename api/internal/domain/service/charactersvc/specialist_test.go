package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// readWizard reads, on the fixture's real tables, a wizard of class realized
// at 3 with the XP of level 5 (PHB Table 20) at Int 17, whose log took level
// 3: Table 21 opens the 3rd spell level at 5 (pending) and the 2nd at 3
// (owed).
func readWizard(t *testing.T, class string) *charactersvc.Character {
	t.Helper()
	e := newEnv(t)
	free := &freeSpells{taken: map[int64][]int{9: {3}}, claimed: map[int64][]spellbook.FreeSpell{}}
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: edited(t, keep), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
		Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book, FreeSpells: free,
	})
	require.NoError(t, err)
	c := &charactersvc.Character{
		ID: 9, Name: "Test", Class: class, Race: "human", Str: 10, Dex: 16, Con: 11, Int: 17, Wis: 12, Cha: 10,
		HPMax: 9, RealizedLevel: 3, XPTotal: 20_000,
	}
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(9)).Return(c, nil).Once()
	e.xp.EXPECT().Awards(mock.Anything, int64(9)).Return(nil, nil).Once()
	got, err := svc.GetCharacter(context.Background(), 9)
	require.NoError(t, err)
	return got
}

// ADR 0050 step 8: a specialist owes the mage's one free spell per new spell
// level (Q5), and the grant says ofSchool, pending and owed alike; a mage's
// does not.
func TestGetCharacter_Step8_TheGrantSaysOfSchool(t *testing.T) {
	for _, tc := range []struct {
		class         string
		pending, owed []spellbook.FreeSpell
	}{
		{"transmuter", []spellbook.FreeSpell{{Level: 5, SpellLevel: 3, OfSchool: true}}, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2, OfSchool: true}}},
		{"mage", []spellbook.FreeSpell{{Level: 5, SpellLevel: 3}}, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}},
	} {
		t.Run(tc.class, func(t *testing.T) {
			got := readWizard(t, tc.class)
			var pending []spellbook.FreeSpell
			for _, p := range got.Derived.LevelUp.Pending {
				pending = append(pending, p.FreeSpells...)
			}
			assert.Equal(t, tc.pending, pending)
			assert.Equal(t, tc.owed, got.Derived.LevelUp.FreeSpellsOwed)
		})
	}
}

// ADR 0050 step 7: the derived spellbook block carries a specialist's school
// (PHB Table 22: a transmuter's alteration, against abjuration and
// necromancy) and the learn chance's adjustments, +15 and -15, beside Table
// 4's chance (Int 17: 75%); a mage's block has none.
func TestGetCharacter_Step7_TheSpellbookBlockCarriesTheAdjustments(t *testing.T) {
	got := readWizard(t, "transmuter")
	require.NotNil(t, got.Derived.Spellbook)
	assert.Equal(t, 75, got.Derived.Spellbook.LearnChance)
	assert.Equal(t, 15, got.Derived.Spellbook.SchoolAdjustment)
	assert.Equal(t, -15, got.Derived.Spellbook.OtherAdjustment)
	assert.Equal(t, spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}},
		got.Derived.Spellbook.Specialization)

	mage := readWizard(t, "mage")
	require.NotNil(t, mage.Derived.Spellbook)
	assert.Equal(t, 75, mage.Derived.Spellbook.LearnChance)
	assert.Zero(t, mage.Derived.Spellbook.SchoolAdjustment)
	assert.Zero(t, mage.Derived.Spellbook.OtherAdjustment)
	assert.False(t, mage.Derived.Spellbook.Specialization.Specialist())
}

// ADR 0050 step 10: a specialist has one school slot "for each spell level
// whose allotment (ADR 0030 step 8, Table 21 at the entry's realized level) is
// 1 or more" (PHB p.32, 06-mage.md l.56), carried beside the slots, not in
// them. Table 21 (p.30) at level 3: two 1st and one 2nd; the 3rd opens at 5,
// the XP's level, which is not realized yet. A mage has none.
func TestDerive_Step10_TheSchoolSlotPerOpenLevel(t *testing.T) {
	got := readWizard(t, "transmuter")
	assert.Equal(t, map[int]int{1: 2, 2: 1}, got.Derived.SpellSlots)
	assert.Equal(t, map[int]int{1: 1, 2: 1}, got.Derived.SchoolSlots)
	for _, e := range got.Derived.Classes {
		assert.Equal(t, map[int]int{1: 1, 2: 1}, e.SchoolSlots, "the entry's own")
	}

	mage := readWizard(t, "mage")
	assert.Equal(t, map[int]int{1: 2, 2: 1}, mage.Derived.SpellSlots)
	assert.Nil(t, mage.Derived.SchoolSlots)
}
