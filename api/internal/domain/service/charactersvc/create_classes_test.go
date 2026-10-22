package charactersvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// classesEnv is a service over the fixture catalogue's tables (the real rules, not a mock) and a
// resolver answering lift as the ruleset's racial lift.
func classesEnv(t *testing.T, lift ruleset.RacialLift) *env {
	t.Helper()
	e := newEnv(t)
	res := rulesetmocks.NewMockResolver(t)
	rs := ruleset.Default()
	rs.RacialLift = lift
	res.EXPECT().ForCharacter(mock.Anything, mock.Anything).Return(rs, nil).Maybe()
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: cataloguetest.Tables(t), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
		Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book, FreeSpells: e.free, Rulesets: res,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// entered is a draft of race with the class entries es and scores that meet every prime requisite.
func entered(race string, es ...charactersvc.NewClass) charactersvc.NewCharacter {
	return charactersvc.NewCharacter{
		Name: "Party", Race: race, Classes: es, Details: identity(),
		Str: 17, Dex: 16, Con: 15, Int: 17, Wis: 16, Cha: 10, HPMax: 60,
	}
}

// refused is the "field:code" of the draft's refusals.
func refused(t *testing.T, e *env, n charactersvc.NewCharacter) []string {
	t.Helper()
	d, err := e.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	out := []string{}
	for _, r := range d.Refusals {
		var fe *domain.FieldError
		require.ErrorAs(t, r, &fe)
		out = append(out, fe.Field+": "+fe.Message)
	}
	return out
}

// ADR 0042 step 2.1: a human fighter/thief is accepted with the lift and refused without it; a
// half-elf fighter/cleric is accepted either way.
func TestCreate_MultiClass_Step2_1_TheLiftDecidesTheRaceLimit(t *testing.T) {
	humanFT := entered("human", charactersvc.NewClass{Class: "fighter", XP: 9_000}, charactersvc.NewClass{Class: "thief", XP: 9_000})
	halfElfFC := entered("half_elf", charactersvc.NewClass{Class: "fighter", XP: 9_000}, charactersvc.NewClass{Class: "cleric", XP: 9_000})

	lifted := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true})
	assert.Empty(t, refused(t, lifted, humanFT))
	assert.Empty(t, refused(t, lifted, halfElfFC))

	book := classesEnv(t, ruleset.RacialLift{})
	assert.Equal(t, []string{"classes: " + charactersvc.CodeMultiClassNotAllowed}, refused(t, book, humanFT))
	assert.Empty(t, refused(t, book, halfElfFC))
	_, err := book.svc.Create(context.Background(), ownerID, humanFT)
	assert.ErrorContains(t, err, charactersvc.CodeMultiClassNotAllowed)
}

// ADR 0042 step 2.1 (amended 2026-10-22): a class the catalogue marks `multiclass: false` (the
// bard) is never one of a multi-class character's classes, refused at its own entry, lift or not;
// the same class stays allowed alone and as either class of a dual-class character.
func TestCreate_MultiClass_Step2_1_TheCatalogueBarsTheBard(t *testing.T) {
	thiefBard := entered("half_elf", charactersvc.NewClass{Class: "thief", XP: 9_000}, charactersvc.NewClass{Class: "bard", XP: 9_000})
	thiefBard.Cha = 15
	bardThiefMage := entered("half_elf", charactersvc.NewClass{Class: "bard", XP: 9_000},
		charactersvc.NewClass{Class: "fighter", XP: 9_000}, charactersvc.NewClass{Class: "mage", XP: 9_000})
	bardThiefMage.Cha = 15
	bardAlone := entered("half_elf", charactersvc.NewClass{Class: "bard", XP: 9_000})
	bardAlone.Cha = 15
	intoBard := entered("human", charactersvc.NewClass{Class: "fighter", LeftAtLevel: 9}, charactersvc.NewClass{Class: "bard"})
	intoBard.Dex, intoBard.Cha = 17, 17
	outOfBard := entered("human", charactersvc.NewClass{Class: "bard", LeftAtLevel: 9}, charactersvc.NewClass{Class: "fighter"})
	outOfBard.Dex, outOfBard.Cha, outOfBard.Str = 15, 15, 17

	for _, lift := range []ruleset.RacialLift{{MulticlassAnyRace: true, DualclassAnyRace: true}, {}} {
		e := classesEnv(t, lift)
		assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodeMultiClassClassNotAllowed}, refused(t, e, thiefBard))
		assert.Equal(t, []string{"classes.0.class: " + charactersvc.CodeMultiClassClassNotAllowed}, refused(t, e, bardThiefMage))
		assert.Empty(t, refused(t, e, bardAlone))
		_, err := e.svc.Create(context.Background(), ownerID, thiefBard)
		assert.ErrorContains(t, err, charactersvc.CodeMultiClassClassNotAllowed)
	}
	dual := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true})
	assert.Empty(t, refused(t, dual, intoBard), "a dual-class character may enter the bard")
	assert.Empty(t, refused(t, dual, outOfBard), "a dual-class character may leave the bard")
}

// ADR 0042 step 2.2: an elf fighter to mage is accepted with the lift; Strength 14 (under 15 in
// the fighter) and Intelligence 16 (under 17 in the mage) are each refused; the class left at
// level 1 is refused; so is going back to a class left.
func TestCreate_DualClass_Step2_2_PrimeRequisitesLevelAndNoReturn(t *testing.T) {
	e := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true})
	dual := func(left, str, in int) charactersvc.NewCharacter {
		n := entered("elf", charactersvc.NewClass{Class: "fighter", LeftAtLevel: left}, charactersvc.NewClass{Class: "mage", XP: 0})
		n.Str, n.Int = str, in
		return n
	}
	assert.Empty(t, refused(t, e, dual(9, 17, 17)))
	assert.Equal(t, []string{"classes.0.class: " + charactersvc.CodePrimeRequisiteTooLow}, refused(t, e, dual(9, 14, 17)))
	assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodePrimeRequisiteTooLow}, refused(t, e, dual(9, 17, 16)))
	assert.Equal(t, []string{"classes.0.leftAtLevel: " + charactersvc.CodeLevelTooLow}, refused(t, e, dual(1, 17, 17)))
	back := entered("elf", charactersvc.NewClass{Class: "fighter", LeftAtLevel: 9}, charactersvc.NewClass{Class: "fighter"})
	assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodeClassAlreadyLeft}, refused(t, e, back))

	noDual := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true})
	assert.Equal(t, []string{"classes: " + charactersvc.CodeDualClassNotAllowed}, refused(t, noDual, dual(9, 17, 17)))
}

// ADR 0042 step 4: the elf entered as its sheet has it is stored with each entry's level and XP,
// the class left at the XP that reaches its left-at level, and the max HP as entered; the entries
// are the shape the storage accepts.
func TestCreate_Step4_EntersLevelAndXPPerEntry(t *testing.T) {
	e := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true})
	var stored charactersvc.Character
	e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).
		Run(func(_ context.Context, _ uuid.UUID, c charactersvc.Character) { stored = c }).Return(5, nil).Once()
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(5)).Return(nil).Once()
	e.sessions.EXPECT().ForgetUser(ownerID).Once()

	_, err := e.svc.Create(context.Background(), ownerID, entered("elf",
		charactersvc.NewClass{Class: "fighter", LeftAtLevel: 9},
		charactersvc.NewClass{Class: "mage", XP: 135_000, Level: 8}))
	require.NoError(t, err)

	require.NoError(t, classes.ValidateEntries(stored.Entries()))
	want := []classes.Entry{
		{Class: "fighter", RealizedLevel: 9, StartingXP: 250_000, State: classes.EntryFrozen, LeftAtLevel: 9},
		{Class: "mage", RealizedLevel: 8, StartingXP: 135_000, State: classes.EntryActive},
	}
	assert.Equal(t, want, stored.Entries())
	assert.Equal(t, 60, stored.HPMax)
}

// The draft says what the form must offer, so the client computes no rule: the alignments every
// ACTIVE class admits (a class left does not limit), and the active class that carries the skill
// points and the one that carries the starting spellbook.
func TestDraft_NamesTheAlignmentsAndTheCarriers(t *testing.T) {
	e := classesEnv(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true})
	draft := func(es ...charactersvc.NewClass) *charactersvc.Draft {
		t.Helper()
		d, err := e.svc.Draft(context.Background(), entered("human", es...), nil)
		require.NoError(t, err)
		return d
	}
	nine := ruletablesvc.Alignments
	noLawfulGood := []string{"neutral_good", "chaotic_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "lawful_evil", "neutral_evil", "chaotic_evil"}

	// A thief admits eight alignments (no lawful good); left for the mage, it limits nothing.
	d := draft(charactersvc.NewClass{Class: "thief", LeftAtLevel: 9}, charactersvc.NewClass{Class: "mage"})
	assert.Equal(t, nine, d.Alignments, "the class left does not limit")
	assert.Equal(t, "", d.SkillsClass, "a frozen thief carries no skill points")
	assert.Equal(t, "mage", d.SpellbookClass)

	// Active together, the thief limits the fighter's nine.
	d = draft(charactersvc.NewClass{Class: "fighter", XP: 9_000}, charactersvc.NewClass{Class: "thief", XP: 9_000})
	assert.Equal(t, noLawfulGood, d.Alignments)
	assert.Equal(t, "thief", d.SkillsClass)
	assert.Equal(t, "", d.SpellbookClass)

	d = draft(charactersvc.NewClass{Class: "fighter", XP: 9_000}, charactersvc.NewClass{Class: "mage", XP: 9_000})
	assert.Equal(t, nine, d.Alignments)
	assert.Equal(t, "", d.SkillsClass)
	assert.Equal(t, "mage", d.SpellbookClass)
}

// ADR 0043 step 4.4 through Create: a fighter is stored with no PSP maximum; a maximum entered for
// a character with no psionics entry, or out of range, is refused. (TestCreate_Step4_4_PSPMaxEnteredIsKept
// proves a psionicist's maximum.)
func TestCreate_Step4_4_NoPsionicsNoPSPMax(t *testing.T) {
	fighter := entered("human")
	fighter.Class, fighter.Classes = "fighter", nil
	t.Run("stored with none", func(t *testing.T) {
		e := classesEnv(t, ruleset.RacialLift{})
		var stored charactersvc.Character
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).
			Run(func(_ context.Context, _ uuid.UUID, c charactersvc.Character) { stored = c }).Return(5, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(5)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, fighter)
		require.NoError(t, err)
		assert.Equal(t, 0, stored.PSPMax)
	})
	t.Run("entered, refused", func(t *testing.T) {
		e := classesEnv(t, ruleset.RacialLift{})
		n := fighter
		n.PSPMax = new(30)
		_, err := e.svc.Create(context.Background(), ownerID, n)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, charactersvc.FieldPSPMax+": "+charactersvc.CodeOutOfRange, fe.Field+": "+fe.Message)
	})
	t.Run("out of range, refused", func(t *testing.T) {
		e := classesEnv(t, ruleset.RacialLift{})
		n := fighter
		n.PSPMax = new(charactersvc.MaxPSPMax + 1)
		assert.Equal(t, []string{charactersvc.FieldPSPMax + ": " + charactersvc.CodeOutOfRange}, refused(t, e, n))
	})
	// An entered 0 is no "none": the maximum runs 1 to MaxPSPMax (ADR 0043 step 4.4).
	t.Run("zero, refused", func(t *testing.T) {
		e := classesEnv(t, ruleset.RacialLift{})
		n := fighter
		n.PSPMax = new(0)
		assert.Equal(t, []string{charactersvc.FieldPSPMax + ": " + charactersvc.CodeOutOfRange}, refused(t, e, n))
	})
}
