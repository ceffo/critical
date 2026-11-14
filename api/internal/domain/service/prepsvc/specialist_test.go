package prepsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The transmuter's spellbook: his school is alteration, his opposition
// abjuration and necromancy (PHB Table 22, p.31). The school codes are the
// catalogue's (ADR 0050 step 1).
var transmuterBook = []prepsvc.SpellbookEntry{
	{ID: "enlarge", Name: "Enlarge", Level: 1, Wizard: true, Schools: []string{"alteration"}},
	{ID: "magic_missile", Name: "Magic Missile", Level: 1, Wizard: true, Schools: []string{"invocation_evocation"}},
	{ID: "identify", Name: "Identify", Level: 1, Wizard: true, Schools: []string{"lesser_divination"}},
	{ID: "alarm", Name: "Alarm", Level: 1, Wizard: true, Schools: []string{"abjuration", "invocation_evocation"}},
	{ID: "levitate", Name: "Levitate", Level: 2, Wizard: true, Schools: []string{"alteration"}},
	{ID: "mirror_image", Name: "Mirror Image", Level: 2, Wizard: true, Schools: []string{"illusion"}},
	{ID: "slow", Name: "Slow", Level: 3, Wizard: true, Schools: []string{"alteration"}},
}

// level3 is PHB Table 21 (p.30) at level 3: two 1st-level spells and one
// 2nd-level, no 3rd.
var level3 = map[int]int{1: 2, 2: 1}

// wizardAt3 is a wizard of class at level 3 with the given specialization and
// school slots, as charactersvc derives them (the view states them; no rule
// here).
func wizardAt3(class string, spec spellbook.Specialization, school map[int]int) charactersvc.View {
	v := viewtest.Single(class, 3)
	v.Entries[0].Casts, v.Entries[0].SpellSlots, v.Entries[0].SchoolSlots = true, level3, school
	v.SpellSlots, v.SchoolSlots = level3, school
	v.Spellbook = &charactersvc.SpellbookRules{Specialization: spec}
	return v
}

// transmuter is a transmuter at level 3 with ADR 0050 step 10's school slot
// at each spell level he has slots of: one at the 1st, one at the 2nd.
func transmuter() charactersvc.View {
	return wizardAt3("transmuter", spellbook.Specialization{
		Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"},
	}, map[int]int{1: 1, 2: 1})
}

// mage is a mage at level 3: no specialist, no school slot.
func mage() charactersvc.View { return wizardAt3("mage", spellbook.Specialization{}, nil) }

func newWizardFixture(t *testing.T, c charactersvc.View) *fixture {
	t.Helper()
	f := &fixture{slots: mocks.NewMockSlots(t), log: mocks.NewMockLog(t), items: mocks.NewMockItems(t), tx: &passTx{}}
	b := mocks.NewMockSpellbook(t)
	b.EXPECT().Of(mock.Anything, int64(1), mock.Anything).Return(transmuterBook, nil).Maybe()
	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().View(mock.Anything, int64(1)).Return(c, nil).Maybe()
	svc, err := prepsvc.New(prepsvc.Deps{
		Slots: f.slots, Spellbook: b, Effects: catalog(t), Items: f.items, Characters: chars, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

// accepts expects the preparation written and returns what was saved.
func (f *fixture) accepts() *[]spellsvc.SpellSlot {
	var saved []spellsvc.SpellSlot
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).
		Run(func(_ context.Context, _ int64, s []spellsvc.SpellSlot) { saved = s }).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Return(int64(1), nil)
	return &saved
}

// ADR 0050 step 10: "a specialist entry has one more slot of that level that
// holds only a spell of his school (PHB p.32) ... accepted when its spells
// number at most the allotment + 1 and its spells that are not of his school
// number at most the allotment; otherwise it is refused with nothing written".
// PHB p.32 (06-mage.md l.56): "one additional spell per spell level, provided
// the additional spell is taken in the specialist's school". The slot list is
// unchanged: no slot is marked.
func TestPrepare_Step10_TheSchoolSlotTakesOnlyASchoolSpell(t *testing.T) {
	t.Run("allotment + 1 with one school spell is accepted", func(t *testing.T) {
		f := newWizardFixture(t, transmuter())
		saved := f.accepts()
		_, err := f.svc.Prepare(context.Background(), 1, []string{"magic_missile", "identify", "enlarge", "mirror_image", "levitate"})
		require.NoError(t, err)
		assert.Equal(t, []spellsvc.SpellSlot{
			{Level: 1, SpellID: "magic_missile", PreparedSpellName: "Magic Missile"},
			{Level: 1, SpellID: "identify", PreparedSpellName: "Identify"},
			{Level: 1, SpellID: "enlarge", PreparedSpellName: "Enlarge"},
			{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
			{Level: 2, SpellID: "levitate", PreparedSpellName: "Levitate"},
		}, *saved)
	})
	t.Run("school spells fill any slot of the level", func(t *testing.T) {
		f := newWizardFixture(t, transmuter())
		f.accepts()
		_, err := f.svc.Prepare(context.Background(), 1, []string{"enlarge", "enlarge", "enlarge"})
		require.NoError(t, err)
	})
	for _, tc := range []struct {
		name string
		ids  []string
		want error
	}{
		{"allotment + 1 with no school spell", []string{"magic_missile", "identify", "magic_missile"}, prepsvc.ErrSchoolSlot},
		{"allotment + 1 with no school spell at the 2nd", []string{"mirror_image", "mirror_image"}, prepsvc.ErrSchoolSlot},
		{"allotment + 2", []string{"magic_missile", "identify", "enlarge", "enlarge"}, prepsvc.ErrOverLimit},
		{"a level with no allotment has no school slot", []string{"slow"}, prepsvc.ErrOverLimit},
	} {
		t.Run(tc.name+" is refused, nothing written", func(t *testing.T) {
			f := newWizardFixture(t, transmuter()) // no slot store or log call expected
			_, err := f.svc.Prepare(context.Background(), 1, tc.ids)
			require.ErrorIs(t, err, tc.want)
			assert.Zero(t, f.tx.calls)
		})
	}
	t.Run("a mage has no school slot", func(t *testing.T) {
		f := newWizardFixture(t, mage())
		_, err := f.svc.Prepare(context.Background(), 1, []string{"magic_missile", "identify", "enlarge"})
		require.ErrorIs(t, err, prepsvc.ErrOverLimit)
		assert.Zero(t, f.tx.calls)
	})
}

// ADR 0050 step 11: "A preparation holding an opposed spell is refused,
// spell_school_opposed (step 6's code), nothing written" (PHB p.32: an
// opposition school's spells cannot be learned; one is in the book only after
// a switch). Alarm is abjuration and evocation: opposed for a transmuter (Q4:
// opposed wins), any spell for a mage.
func TestPrepare_Step11_AnOpposedSpellIsRefused(t *testing.T) {
	t.Run("a transmuter's preparation with Alarm", func(t *testing.T) {
		f := newWizardFixture(t, transmuter())
		_, err := f.svc.Prepare(context.Background(), 1, []string{"enlarge", "alarm"})
		require.ErrorIs(t, err, spellbook.ErrSchoolOpposed)
		assert.Zero(t, f.tx.calls)
	})
	t.Run("a mage prepares Alarm", func(t *testing.T) {
		f := newWizardFixture(t, mage())
		f.accepts()
		_, err := f.svc.Prepare(context.Background(), 1, []string{"enlarge", "alarm"})
		require.NoError(t, err)
	})
}

// ADR 0050 step 10: the preparation editor's limits carry the school slot
// separately ("the web shows 2 + 1 and never adds it itself"): the allotment
// as Max, the school slot as School, so Max + School is what PrepareFor
// accepts. A mage's limits have no school slot.
func TestInfo_Step10_TheLimitsCarryTheSchoolSlot(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    charactersvc.View
		want []prepsvc.LevelLimit
	}{
		{"transmuter", transmuter(), []prepsvc.LevelLimit{{Level: 1, Max: 2, School: 1}, {Level: 2, Max: 1, School: 1}}},
		{"mage", mage(), []prepsvc.LevelLimit{{Level: 1, Max: 2}, {Level: 2, Max: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWizardFixture(t, tc.c)
			f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
			info, err := f.svc.Info(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, tc.want, info.Limits)
		})
	}
}

// ADR 0050 step 15.3: the transmuter's book serves the abjuration spell (Alarm) as opposed; the
// mage's book, the same spells, flags none.
func TestInfo_Step15_AnOpposedSpellIsFlagged(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    charactersvc.View
		want []string
	}{
		{"transmuter", transmuter(), []string{"alarm"}},
		{"mage", mage(), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWizardFixture(t, tc.c)
			f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
			info, err := f.svc.Info(context.Background(), 1)
			require.NoError(t, err)
			assert.Len(t, info.Spellbook, len(transmuterBook), "the book keeps every spell")
			var opposed []string
			for _, e := range info.Spellbook {
				if e.Opposed {
					opposed = append(opposed, e.ID)
				}
			}
			assert.Equal(t, tc.want, opposed)
		})
	}
}
