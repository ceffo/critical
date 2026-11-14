package combatsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// catalogueSpells is the spell catalogue port as a map: a test adds the
// spells its saves name.
type catalogueSpells map[string]spellbook.Spell

func (c catalogueSpells) Spell(id string) (spellbook.Spell, bool) {
	s, ok := c[id]
	return s, ok
}

// The spells the saves below name, with the school codes of ADR 0050 step 1.
var (
	polymorphOther = spellbook.Spell{Level: 4, Wizard: true, Schools: []string{"alteration"}}
	fireball       = spellbook.Spell{Level: 3, Wizard: true, Schools: []string{"invocation_evocation"}}
	// A wizard spell of two schools, one the transmuter's opposition (PHB
	// p.32's Table 22: necromancy): opposed wins (ADR 0050 step 5, Q4).
	alterNecro = spellbook.Spell{Level: 2, Wizard: true, Schools: []string{"necromancy", "alteration"}}
	// A priest spell carries school codes too (ADR 0050 step 1.3); no rule
	// reads them.
	priestAlteration = spellbook.Spell{Level: 3, Wizard: false, Schools: []string{"alteration"}}
)

// wizard14 is a single-class wizard of level 14 with PHB Table 60's
// wizard row for levels 11 to 15 (Para/Poison 11, Rod/Staff/Wand 7,
// Petri/Poly 9, Breath 11, Spell 8) and spec as its specialization (the
// zero value: a mage).
func wizard14(class string, spec spellbook.Specialization) charactersvc.View {
	v := viewtest.Single(class, 14, viewtest.WithIdentity(charID, "Test", "human"), viewtest.WithScores(10, 10, 0, 0, 0, 0))
	v.Entries[0].Group = "wizard"
	v.Saves = charactersvc.Saves{ParaPoison: 11, RodStaffWand: 7, PetriPoly: 9, Breath: 11, Spell: 8}
	v.Spellbook = &charactersvc.SpellbookRules{Specialization: spec}
	return v
}

// The transmuter of PHB p.31's Table 22: alteration, opposed to abjuration
// and necromancy.
func transmuter() charactersvc.View {
	return wizard14("transmuter", spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}})
}

func mage() charactersvc.View { return wizard14("mage", spellbook.Specialization{}) }

var (
	specialistTerm       = combatsvc.BreakdownModifier{Kind: "specialist", Value: 1}
	casterSpecialistTerm = combatsvc.BreakdownModifier{Kind: "caster_specialist", Value: -1}
)

// saveCase is one save of the specialist tests: who saves, in which
// category, against what, the d20 (a distinct value per case) and the book's
// expected modifiers, roll total and verdict.
type saveCase struct {
	name       string
	char       charactersvc.View
	category   combatsvc.SaveCategory
	opts       combatsvc.SaveOptions
	d20        int
	wantMods   []combatsvc.BreakdownModifier
	wantTotal  int
	wantTarget int
	wantPassed bool
}

func runSaveCase(t *testing.T, c saveCase) {
	t.Helper()
	f := newFixture(t)
	f.spells["polymorph_other"] = polymorphOther
	f.spells["fireball"] = fireball
	f.spells["alter_necro"] = alterNecro
	f.spells["priest_alteration"] = priestAlteration
	f.withChar(c.char)
	f.dice.EXPECT().RollD20().Return(c.d20).Once()
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, buffsvc.BaseStrengthOf(c.char), buffsvc.WeaponNone).Return(strMods(10), nil).Once()
	f.records()

	res, err := f.svc.ResolveSavingThrow(context.Background(), charID, c.category, c.opts)
	require.NoError(t, err)
	assert.Equal(t, c.wantMods, res.Modifiers)
	assert.Equal(t, c.d20, res.D20Roll)
	assert.Equal(t, c.wantTotal, res.RollTotal)
	assert.Equal(t, c.wantTarget, res.Target, "the term moves the roll, never the target")
	assert.Equal(t, c.wantPassed, res.Passed)
	require.Len(t, f.entries, 1)
}

// ADR 0050 step 12 (PHB p.32): a specialist saving against a wizard spell of
// his school, cast by someone else, gets a named term specialist +1 on the
// roll. A transmuter's save vs Petrify/Polymorph (target 9) against
// Polymorph Other (alteration) with a d20 of 8: 8 + 1 = 9, passed (without
// the term, failed). Against Fireball (evocation), a spell of two schools one
// of which he is opposed to, a priest spell of the school, an unknown spell
// or none: no term. A mage against Polymorph Other: none.
func TestResolveSavingThrow_Step12_PlusOneAgainstHisSchool(t *testing.T) {
	for _, c := range []saveCase{
		{
			"a transmuter against Polymorph Other: +1, passes at the target", transmuter(), combatsvc.SavePetriPoly,
			combatsvc.SaveOptions{SpellID: "polymorph_other"},
			8,
			[]combatsvc.BreakdownModifier{specialistTerm},
			9, 9, true,
		},
		{
			"a transmuter against Polymorph Other, one short even with it", transmuter(), combatsvc.SavePetriPoly,
			combatsvc.SaveOptions{SpellID: "polymorph_other"},
			7,
			[]combatsvc.BreakdownModifier{specialistTerm},
			8, 9, false,
		},
		{
			"a transmuter against Fireball (evocation): none", transmuter(), combatsvc.SaveSpell,
			combatsvc.SaveOptions{SpellID: "fireball"},
			7,
			[]combatsvc.BreakdownModifier{},
			7, 8, false,
		},
		{
			"a transmuter against an alteration and necromancy spell: opposed wins, none", transmuter(), combatsvc.SaveSpell,
			combatsvc.SaveOptions{SpellID: "alter_necro"},
			6,
			[]combatsvc.BreakdownModifier{},
			6, 8, false,
		},
		{
			"a transmuter against a priest spell of alteration: none", transmuter(), combatsvc.SaveSpell,
			combatsvc.SaveOptions{SpellID: "priest_alteration"},
			5,
			[]combatsvc.BreakdownModifier{},
			5, 8, false,
		},
		{
			"a transmuter against a spell the catalogue does not hold: none", transmuter(), combatsvc.SaveSpell,
			combatsvc.SaveOptions{SpellID: "no_such_spell"},
			4,
			[]combatsvc.BreakdownModifier{},
			4, 8, false,
		},
		{
			"a transmuter with no spell named: none", transmuter(), combatsvc.SavePetriPoly,
			combatsvc.SaveOptions{},
			8,
			[]combatsvc.BreakdownModifier{},
			8, 9, false,
		},
		{
			"a mage against Polymorph Other: none", mage(), combatsvc.SavePetriPoly,
			combatsvc.SaveOptions{SpellID: "polymorph_other"},
			8,
			[]combatsvc.BreakdownModifier{},
			8, 9, false,
		},
	} {
		t.Run(c.name, func(t *testing.T) { runSaveCase(t, c) })
	}
}

// ADR 0050 step 12 with ADR 0006 step 10.2: a Wisdom immunity to the spell
// still rolls nothing, so no term either.
func TestResolveSavingThrow_Step12_AnImmunityStillRollsNothing(t *testing.T) {
	f := newFixture(t)
	f.spells["polymorph_other"] = polymorphOther
	c := transmuter()
	c.Wis, c.SpellImmunities = 19, []string{"polymorph_other"}
	f.withChar(c)
	f.records()

	res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SavePetriPoly, combatsvc.SaveOptions{SpellID: "polymorph_other"})
	require.NoError(t, err)
	assert.Equal(t, &combatsvc.Immunity{Source: combatsvc.ImmunitySourceWisdom, Score: 19}, res.Immunity)
	assert.Empty(t, res.Modifiers)
	assert.Zero(t, res.D20Roll)
	assert.True(t, res.Passed)
}

// ADR 0050 step 13 (PHB p.32): the request's casterSpecialist flag says the
// wizard spell named was cast by a specialist of its school: a term
// caster_specialist -1 on the roll. A mage's save vs Petrify/Polymorph
// (target 9) against Polymorph Other with a d20 of 9: 9 - 1 = 8, failed
// (without the term, passed). A transmuter (his school) gets both terms,
// which cancel: 9 + 1 - 1 = 9, passed, the +1 listed first.
func TestResolveSavingThrow_Step13_MinusOneAgainstASpecialistCaster(t *testing.T) {
	flagged := func(spell string) combatsvc.SaveOptions {
		return combatsvc.SaveOptions{SpellID: spell, CasterSpecialist: true}
	}
	for _, c := range []saveCase{
		{
			"a mage against a specialist's Polymorph Other: -1, fails at the target", mage(), combatsvc.SavePetriPoly,
			flagged("polymorph_other"), 9,
			[]combatsvc.BreakdownModifier{casterSpecialistTerm},
			8, 9, false,
		},
		{
			"a mage against a specialist's Fireball: -1", mage(), combatsvc.SaveSpell,
			flagged("fireball"), 11,
			[]combatsvc.BreakdownModifier{casterSpecialistTerm},
			10, 8, true,
		},
		{
			"a transmuter against a transmuter's Polymorph Other: +1 and -1 cancel", transmuter(), combatsvc.SavePetriPoly,
			flagged("polymorph_other"), 9,
			[]combatsvc.BreakdownModifier{specialistTerm, casterSpecialistTerm},
			9, 9, true,
		},
		{
			"a transmuter against an invoker's Fireball: the -1 only", transmuter(), combatsvc.SaveSpell,
			flagged("fireball"), 8,
			[]combatsvc.BreakdownModifier{casterSpecialistTerm},
			7, 8, false,
		},
	} {
		t.Run(c.name, func(t *testing.T) { runSaveCase(t, c) })
	}
}

// ADR 0050 step 13: the flag without a wizard spell named (none, a priest
// spell, one the catalogue does not hold) is refused before anything is read,
// rolled or written: 400 caster_specialist_needs_spell (an invalid input).
func TestResolveSavingThrow_Step13_TheFlagNeedsAWizardSpell(t *testing.T) {
	for _, spell := range []string{"", "priest_alteration", "no_such_spell"} {
		t.Run("spell "+spell, func(t *testing.T) {
			f := newFixture(t)
			f.spells["priest_alteration"] = priestAlteration
			_, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell, combatsvc.SaveOptions{SpellID: spell, CasterSpecialist: true})
			require.ErrorIs(t, err, saves.ErrCasterSpecialistNeedsSpell)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Empty(t, f.entries)
		})
	}
}
