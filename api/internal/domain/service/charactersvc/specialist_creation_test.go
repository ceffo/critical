package charactersvc_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// The creation of a specialist wizard (ADR 0050 steps 6, 9 and 14), on the fixture catalogue's
// real tables, spells (with their school codes) and starting spellbook seed
// (api/testdata/seeds/starting_spellbook.json), as the server reads them.

// specialistEnv is a service over the fixture's tables, spells and starting spellbook seed, with a
// resolver answering lift as the ruleset's racial lift.
func specialistEnv(t *testing.T, lift ruleset.RacialLift) *env {
	t.Helper()
	e := newEnv(t)
	res := rulesetmocks.NewMockResolver(t)
	rs := ruleset.Default()
	rs.RacialLift = lift
	res.EXPECT().ForCharacter(mock.Anything, mock.Anything).Return(rs, nil).Maybe()
	tables := cataloguetest.Tables(t)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	cat := cataloguetest.Artifact(t).Catalogue
	seed, err := spellbookadpt.ReadStartingSeed(filepath.Join(filepath.Dir(cataloguetest.Path()), "seeds"), cat, defs)
	require.NoError(t, err)
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: tables, Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
		Armour: e.armour, HP: e.hp, Spells: spellbookadpt.NewCreation(cat, seed), Book: e.book, FreeSpells: e.free, Rulesets: res,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// wizardOf is a valid new single-class character of class and race at level 1, with scores that
// meet every specialist's PHB Table 22 minimum (Wis, Con, Dex, Cha 16) and Int 17 (Table 4: up
// to 8th-level spells, so only the class's slots limit a level).
func wizardOf(class, race string, spells ...string) charactersvc.NewCharacter {
	return charactersvc.NewCharacter{
		Name: "Tenser", Class: class, Race: race, Details: identity(), Spellbook: spells,
		Str: 10, Dex: 16, Con: 16, Int: 17, Wis: 16, Cha: 16, HPMax: 4,
	}
}

// codes are the "field: code" of errs, the FieldErrors of a draft or a creation.
func codes(t *testing.T, errs []error) []string {
	t.Helper()
	out := []string{}
	for _, r := range errs {
		out = append(out, fieldCode(t, r))
	}
	return out
}

// fieldCode is the "field: code" of err, a FieldError.
func fieldCode(t *testing.T, err error) string {
	t.Helper()
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	return fe.Field + ": " + fe.Message
}

// warningsOf are the "code class race" of the warnings with code.
func warningsOf(ws []charactersvc.Warning, code string) []string {
	out := []string{}
	for _, w := range ws {
		if w.Code == code {
			out = append(out, w.Field+" "+w.Class+" "+w.Race)
		}
	}
	return out
}

// ADR 0050 step 6 on the starting book (amends ADR 0030 step 20.7): a transmuter's spell of an
// opposition school (PHB Table 22: abjuration and necromancy) is refused at spellbook.<id> with
// spell_school_opposed, in the draft and at Create, before the level checks (Dispel Magic, an
// abjuration of the 3rd level, at level 1 reads opposed, not above the class); a mage enters Alarm
// (abjuration, invocation/evocation) and Dispel Magic is above his class at level 1 (PHB Table 21).
func TestDraft_Step6_OpposedStartingSpellRefused(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	d, err := e.svc.Draft(context.Background(), wizardOf("transmuter", "human", "detect_magic", "alarm", "enlarge", "dispel_magic", "mirror_image"), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"spellbook.alarm: " + charactersvc.CodeSpellSchoolOpposed,
		"spellbook.dispel_magic: " + charactersvc.CodeSpellSchoolOpposed,
		"spellbook.mirror_image: " + charactersvc.CodeSpellLevelAboveClass,
	}, codes(t, d.Refusals))
	require.NotNil(t, d.Spellbook)
	assert.Equal(t, []charactersvc.SpellLevelCount{{Level: 1, Count: 2}}, d.Spellbook.Levels, "Detect Magic and Enlarge are kept")

	_, err = e.svc.Create(context.Background(), ownerID, wizardOf("transmuter", "human", "detect_magic", "alarm"))
	assert.Equal(t, "spellbook.alarm: "+charactersvc.CodeSpellSchoolOpposed, fieldCode(t, err))
	assert.Empty(t, e.book.entries, "nothing written")

	mage, err := e.svc.Draft(context.Background(), wizardOf("mage", "human", "detect_magic", "alarm", "enlarge", "dispel_magic"), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"spellbook.dispel_magic: " + charactersvc.CodeSpellLevelAboveClass}, codes(t, mage.Refusals))
}

// ADR 0050 step 9: the specialist's starting book is pre-ticked like the mage's (the seed is keyed
// by class and gives the eight specialists the mage's entry, data: the fixture's Detect Magic),
// with no learn roll and the mage's limits.
func TestOptions_Step9_ASpecialistIsPreTickedLikeAMage(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	books := map[string][]string{}
	for _, c := range got.Classes {
		if c.Spellbook != nil {
			books[c.Class] = c.Spellbook.StartingSpells
		}
	}
	want := []string{"detect_magic"}
	assert.Equal(t, want, books["mage"])
	for _, class := range []string{"abjurer", "conjurer", "diviner", "enchanter", "illusionist", "invoker", "necromancer", "transmuter"} {
		assert.Equal(t, want, books[class], class)
	}

	// A specialist created with his pre-ticked book has it written as a mage's: origin starting.
	expectCreated(e)
	id, err := e.svc.Create(context.Background(), ownerID, wizardOf("illusionist", "human", books["illusionist"]...))
	require.NoError(t, err)
	assert.Equal(t, starting("detect_magic"), e.book.entries[id])
}

// ADR 0050 step 14.1: a race outside PHB Table 22's for the class (p.31: the abjurer is human
// only) warns specialist_race {class, race} on the race field, never a refusal: an elf abjurer
// is created (DMG Table 7's Mage row admits the elf) and the draft's sheet carries the warning; a
// human abjurer and an elf diviner (Table 22: human, half-elf, elf) are not warned.
func TestCreate_Step14_TableTwentyTwoRaceWarns(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	d, err := e.svc.Draft(context.Background(), wizardOf("abjurer", "elf"), nil)
	require.NoError(t, err)
	assert.Empty(t, d.Refusals)
	require.NotNil(t, d.Character)
	assert.Equal(t, []string{"race abjurer elf"}, warningsOf(d.Character.Warnings, charactersvc.CodeSpecialistRace))
	expectCreated(e)
	_, err = e.svc.Create(context.Background(), ownerID, wizardOf("abjurer", "elf"))
	require.NoError(t, err, "a warning, never a refusal")

	for _, n := range []charactersvc.NewCharacter{wizardOf("abjurer", "human"), wizardOf("diviner", "elf"), wizardOf("mage", "elf")} {
		d, err := e.svc.Draft(context.Background(), n, nil)
		require.NoError(t, err)
		require.NotNil(t, d.Character)
		assert.Empty(t, warningsOf(d.Character.Warnings, charactersvc.CodeSpecialistRace), "%s %s", n.Race, n.Class)
	}
}

// ADR 0050 step 14.1 on a stored character: every read carries the warning, for the entry whose
// class has the kind (a half-elf necromancer: Table 22 lists the human only).
func TestGetCharacter_Step14_TheRaceWarningOnEveryRead(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	c := &charactersvc.Character{
		ID: 9, Name: "Test", Class: "necromancer", Race: "half_elf", Str: 10, Dex: 12, Con: 11, Int: 17, Wis: 16, Cha: 10,
		HPMax: 4, RealizedLevel: 1,
	}
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(9)).Return(c, nil).Once()
	e.xp.EXPECT().Awards(mock.Anything, int64(9)).Return(nil, nil).Once()
	got, err := e.svc.GetCharacter(context.Background(), 9)
	require.NoError(t, err)
	assert.Equal(t, []string{"race necromancer half_elf"}, warningsOf(got.Warnings, charactersvc.CodeSpecialistRace))
}

// ADR 0050 step 14.2 (Q8): DMG Table 7's dash still refuses a single-class character, as for any
// class: a gnome transmuter is refused at race (the Mage row: a dash for the gnome), as a gnome
// mage is; a gnome illusionist (the Illusionist row: gnome 15) is created, unwarned (Table 22:
// human, gnome).
func TestCreate_Step14_TheDashStillRefuses(t *testing.T) {
	for _, lift := range []ruleset.RacialLift{{}, {MulticlassAnyRace: true, DualclassAnyRace: true}} {
		e := specialistEnv(t, lift)
		for _, class := range []string{"transmuter", "mage"} {
			assert.Equal(t, []string{"race: " + charactersvc.CodeRaceNotAllowed}, refused(t, e, wizardOf(class, "gnome")), class)
		}
		_, err := e.svc.Create(context.Background(), ownerID, wizardOf("transmuter", "gnome"))
		assert.Equal(t, "race: "+charactersvc.CodeRaceNotAllowed, fieldCode(t, err))

		d, err := e.svc.Draft(context.Background(), wizardOf("illusionist", "gnome"), nil)
		require.NoError(t, err)
		assert.Empty(t, d.Refusals)
		assert.Empty(t, warningsOf(d.Character.Warnings, charactersvc.CodeSpecialistRace))
		expectCreated(e)
		_, err = e.svc.Create(context.Background(), ownerID, wizardOf("illusionist", "gnome"))
		require.NoError(t, err)
	}
}

// ADR 0050 step 14.3 (PHB p.31: multi-class characters cannot be specialists, except gnome
// illusionists): a gnome fighter/illusionist is accepted; a half-elf fighter/transmuter (the
// transmuter is `multiclass: false`) and an elf fighter/illusionist (multiclass_races: gnome) are
// refused at the specialist's entry with multiclass_class_not_allowed, with the racial lift on
// and off (the lift does not waive it). A race not chosen yet is not judged.
func TestCreate_Step14_MultiClassSpecialistOnlyAGnomeIllusionist(t *testing.T) {
	fighterAnd := func(race, class string) charactersvc.NewCharacter {
		n := entered(race, charactersvc.NewClass{Class: "fighter", XP: 4_000}, charactersvc.NewClass{Class: class, XP: 4_000})
		n.Wis, n.Con, n.Cha = 16, 16, 16
		return n
	}
	for _, lift := range []ruleset.RacialLift{{}, {MulticlassAnyRace: true, DualclassAnyRace: true}} {
		e := specialistEnv(t, lift)
		assert.Empty(t, refused(t, e, fighterAnd("gnome", "illusionist")), "%+v", lift)
		assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodeMultiClassClassNotAllowed},
			refused(t, e, fighterAnd("half_elf", "transmuter")), "%+v", lift)
		assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodeMultiClassClassNotAllowed},
			refused(t, e, fighterAnd("elf", "illusionist")), "%+v", lift)
		_, err := e.svc.Create(context.Background(), ownerID, fighterAnd("elf", "illusionist"))
		assert.Equal(t, "classes.1.class: "+charactersvc.CodeMultiClassClassNotAllowed, fieldCode(t, err))

		d, err := e.svc.Draft(context.Background(), fighterAnd("", "illusionist"), nil)
		require.NoError(t, err)
		assert.Empty(t, d.Refusals, "the race is still to choose")
		assert.Contains(t, d.Missing, "race")

		expectCreated(e)
		_, err = e.svc.Create(context.Background(), ownerID, fighterAnd("gnome", "illusionist"))
		require.NoError(t, err)
	}
}

// ADR 0050 step 14.4 (ADR 0042 step 2.2 unchanged; PHB p.31: dual-class humans can become
// specialists): a human fighter to transmuter needs 15 in Strength and 17 in each of the
// transmuter's prime requisites, Intelligence and Dexterity (Q1): accepted with Int 17 and Dex 17,
// refused at the new class's entry with Dex 16.
func TestCreate_Step14_DualClassToASpecialist(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	dual := func(dex int) charactersvc.NewCharacter {
		n := entered("human", charactersvc.NewClass{Class: "fighter", LeftAtLevel: 9}, charactersvc.NewClass{Class: "transmuter"})
		n.Str, n.Int, n.Dex = 17, 17, dex
		return n
	}
	assert.Empty(t, refused(t, e, dual(17)))
	assert.Equal(t, []string{"classes.1.class: " + charactersvc.CodePrimeRequisiteTooLow}, refused(t, e, dual(16)))
}

// ADR 0050 step 10 for the one multi-class specialist step 14.3 allows (ADR 0042 step 14): a gnome
// fighter 5/illusionist 6's school slots follow the illusionist entry, one per spell level its
// PHB Table 21 allotment opens at 6 (4, 2 and 2: levels 1 to 3); the fighter entry has none, and
// the character's are the illusionist's, beside his slots; his spellbook block is the
// illusionist's school (Table 22: illusion, against necromancy, invocation/evocation, abjuration).
func TestDerive_Step14_TheGnomeFighterIllusionistsSchoolSlots(t *testing.T) {
	c := multi("gnome", entry("fighter", 5, 16_000), entry("illusionist", 6, 40_000))
	d := derived(t, c)
	ill := entryOf(t, d, "illusionist")
	assert.Equal(t, map[int]int{1: 4, 2: 2, 3: 2}, ill.SpellSlots)
	assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 1}, ill.SchoolSlots)
	assert.Nil(t, entryOf(t, d, "fighter").SchoolSlots)
	assert.Equal(t, ill.SpellSlots, d.SpellSlots)
	assert.Equal(t, ill.SchoolSlots, d.SchoolSlots)
	require.NotNil(t, d.Spellbook)
	assert.Equal(t, []string{"illusion"}, d.Spellbook.Specialization.Schools)
	assert.Equal(t, []string{"necromancy", "invocation_evocation", "abjuration"}, d.Spellbook.Specialization.OppositionSchools)
}

// The creation options name each specialist class's school (ADR 0050 step 2, PHB Table 22), so the
// web words it without a class id; any other class carries none. Compared against an explicit table.
func TestOptions_Step2_ASpecialistClassCarriesItsSchool(t *testing.T) {
	e := specialistEnv(t, ruleset.RacialLift{})
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	schools := map[string][]string{}
	for _, c := range got.Classes {
		require.NotNil(t, c.Schools, c.Class)
		schools[c.Class] = c.Schools
	}
	assert.Equal(t, map[string][]string{
		"abjurer": {"abjuration"}, "conjurer": {"conjuration_summoning"}, "diviner": {"greater_divination", "lesser_divination"},
		"enchanter": {"enchantment_charm"}, "illusionist": {"illusion"}, "invoker": {"invocation_evocation"},
		"necromancer": {"necromancy"}, "transmuter": {"alteration"},
	}, onlyNonEmpty(schools))
}

func onlyNonEmpty(m map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range m {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}
