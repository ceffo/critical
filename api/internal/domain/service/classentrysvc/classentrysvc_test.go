package classentrysvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc"
	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc/mocks"
	prepmocks "github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
)

const charID = int64(1)

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type recorder struct{ got []actionlog.NewEntry }

func (r *recorder) Record(_ context.Context, e actionlog.NewEntry) (int64, error) {
	r.got = append(r.got, e)
	return int64(len(r.got)), nil
}

type fixture struct {
	dal   *mocks.MockDAL
	chars *mocks.MockCharacters
	slots *mocks.MockSlots
	book  *prepmocks.MockSpellbook
	cat   *mocks.MockCatalogue
	log   *recorder
	svc   classentrysvc.SVC
}

func newFixture(t *testing.T, catalogue ...classes.Class) *fixture {
	t.Helper()
	f := &fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), slots: mocks.NewMockSlots(t),
		book: prepmocks.NewMockSpellbook(t), cat: mocks.NewMockCatalogue(t), log: &recorder{},
	}
	for _, c := range catalogue {
		f.cat.EXPECT().Get(c.ID).Return(c, nil).Maybe()
	}
	f.cat.EXPECT().Get(mock.Anything).Return(classes.Class{}, domain.ErrNotFound).Maybe()
	svc, err := classentrysvc.New(classentrysvc.Deps{
		DAL: f.dal, Characters: f.chars, Slots: f.slots, Book: f.book, Catalogue: f.cat, Log: f.log, Tx: noTx{},
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

// The catalogue's classes of the tests: only what step 15.1 reads of them (group, spellbook,
// school specialization, minimums, alignments, multi-class limits).
var (
	spellbookFeature = classes.Feature{Kind: classes.KindSpellbook, Spellbook: &classes.Spellbook{MaxSpellLevel: 9}}
	school           = classes.Feature{Kind: classes.KindSchoolSpecialization, SchoolSpecialization: &classes.SchoolSpecialization{
		Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}, Races: []string{"human", "half_elf"},
	}}
	mage        = classes.Class{ID: "mage", Group: "wizard", Features: []classes.Feature{spellbookFeature}}
	transmuter  = classes.Class{ID: "transmuter", Group: "wizard", NoMultiClass: true, Features: []classes.Feature{spellbookFeature, school}}
	abjurer     = classes.Class{ID: "abjurer", Group: "wizard", NoMultiClass: true, Features: []classes.Feature{spellbookFeature, school}}
	illusionist = classes.Class{ID: "illusionist", Group: "wizard", Features: []classes.Feature{spellbookFeature, {
		Kind: classes.KindSchoolSpecialization, SchoolSpecialization: &classes.SchoolSpecialization{
			Schools: []string{"illusion"}, OppositionSchools: []string{"necromancy"}, Races: []string{"human", "gnome"}, MulticlassRaces: []string{"gnome"},
		},
	}}}
	fighter = classes.Class{ID: "fighter", Group: "warrior"}
	// bard has a spellbook but is of another group than the wizards.
	bard = classes.Class{ID: "bard", Group: "rogue", Features: []classes.Feature{spellbookFeature}}
	// ranger-like: same group as the mage, no spellbook.
	nonBook = classes.Class{ID: "scholar", Group: "wizard"}
	// sorcerer is a wizard-group spellbook class that is no specialist, like the mage.
	sorcerer = classes.Class{ID: "sorcerer", Group: "wizard", Features: []classes.Feature{spellbookFeature}}
)

// every is the alignment list of a class that admits them all (the catalogue's default).
var every = []string{"lawful_good", "neutral_good", "chaotic_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "lawful_evil", "neutral_evil", "chaotic_evil"}

// all is the catalogue of the tests, each class admitting every alignment.
var all = func() []classes.Class {
	out := []classes.Class{mage, transmuter, abjurer, illusionist, fighter, bard, nonBook, sorcerer}
	for i := range out {
		out[i].Alignments = every
	}
	return out
}()

func mageSheet() charactersvc.View {
	v := viewtest.Single("mage", 14, viewtest.WithIdentity(charID, "Ilsevel", "elf"), viewtest.WithScores(9, 16, 12, 18, 12, 10))
	v.Alignment = "neutral_good"
	return v
}

// Step 15.1: each refusal is a 409 with its own sentinel and nothing is written: the mocks fail
// the test on any ReplaceClassEntries, SaveSlots or log entry.
func TestSwitch_Step15_Refusals(t *testing.T) {
	strict := transmuter
	strict.Alignments = every
	strict.AbilityMinimums = []classes.AbilityMinimum{{Ability: "dex", Min: 17}}
	narrow := transmuter
	narrow.Alignments = []string{"lawful_good"}
	multi := viewtest.Multi(
		viewtest.WithEntry("mage", 5), viewtest.WithEntry("fighter", 5),
		viewtest.WithIdentity(charID, "Ilsevel", "gnome"), viewtest.WithScores(9, 16, 12, 18, 12, 10),
	)
	multi.Alignment = "neutral_good"

	for _, tc := range []struct {
		name  string
		sheet charactersvc.View
		entry int
		class string
		cat   []classes.Class
		want  error
	}{
		{"class_switch_not_allowed: another group", mageSheet(), 0, "fighter", all, classentrysvc.ErrNotAllowed},
		{"class_switch_not_allowed: a class with no spellbook", mageSheet(), 0, "scholar", all, classentrysvc.ErrNotAllowed},
		{"class_switch_not_allowed: a spellbook of another group", mageSheet(), 0, "bard", all, classentrysvc.ErrNotAllowed},
		{"class_switch_not_allowed: the entry's own class", mageSheet(), 0, "mage", all, classentrysvc.ErrNotAllowed},
		{"class_switch_not_allowed: no specialist on either side", func() charactersvc.View {
			v := mageSheet()
			v.Entries[0].Class = "sorcerer"
			return v
		}(), 0, "mage", all, classentrysvc.ErrNotAllowed},
		{"class_switch_not_allowed: another entry has the class", viewtest.Multi(
			viewtest.WithEntry("mage", 5), viewtest.WithFrozen("transmuter", 6),
			viewtest.WithIdentity(charID, "Ilsevel", "human"), viewtest.WithScores(9, 16, 12, 18, 12, 10)), 0, "transmuter", all, classentrysvc.ErrNotAllowed},
		{"class_switch_minimums_not_met", mageSheet(), 0, "transmuter", append([]classes.Class{strict}, mage), classentrysvc.ErrMinimumsNotMet},
		{"class_switch_alignment_not_allowed", mageSheet(), 0, "transmuter", append([]classes.Class{narrow}, mage), classentrysvc.ErrAlignmentNotAllowed},
		{"multiclass_class_not_allowed: a class never multi-class", multi, 0, "transmuter", all, classentrysvc.ErrMultiClassNotAllowed},
		{"multiclass_class_not_allowed: an illusionist who is no gnome", func() charactersvc.View {
			v := multi
			v.Race = "elf"
			return v
		}(), 0, "illusionist", all, classentrysvc.ErrMultiClassNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.cat...)
			f.chars.EXPECT().View(mock.Anything, charID).Return(tc.sheet, nil)
			_, err := f.svc.Switch(context.Background(), charID, tc.entry, tc.class)
			require.ErrorIs(t, err, tc.want)
			assert.ErrorIs(t, err, domain.ErrConflict)
			assert.Empty(t, f.log.got)
		})
	}
}

// Step 15.1 allowed cases: a mage to a specialist, a specialist back to a mage, a specialist to
// another (Q9), each in a single-class character and in a multi-class one for the classes the
// book lets be multi-class (a gnome fighter/illusionist's illusionist back to a mage). Only the
// entry's class is written (15.2); an elf is warned elsewhere, never refused here.
func TestSwitch_Step15_AllowedCases(t *testing.T) {
	gnomeMulti := viewtest.Multi(
		viewtest.WithEntry("fighter", 5), viewtest.WithEntry("illusionist", 5),
		viewtest.WithIdentity(charID, "Ilsevel", "gnome"), viewtest.WithScores(9, 16, 12, 18, 12, 10),
	)
	gnomeMulti.Alignment = "neutral_good"
	single := func(class string) charactersvc.View {
		v := mageSheet()
		v.Entries[0].Class = class
		return v
	}
	for _, tc := range []struct {
		name         string
		sheet        charactersvc.View
		entry        int
		class        string
		want         []classes.Entry
		wantFrom, to string
	}{
		{"a mage to a specialist", single("mage"), 0, "transmuter", []classes.Entry{{Class: "transmuter", RealizedLevel: 14, State: classes.EntryActive}}, "mage", "transmuter"},
		{"a specialist back to a mage", single("transmuter"), 0, "mage", []classes.Entry{{Class: "mage", RealizedLevel: 14, State: classes.EntryActive}}, "transmuter", "mage"},
		{"a specialist to another", single("transmuter"), 0, "abjurer", []classes.Entry{{Class: "abjurer", RealizedLevel: 14, State: classes.EntryActive}}, "transmuter", "abjurer"},
		{"a multi-class illusionist back to a mage", gnomeMulti, 1, "mage", []classes.Entry{
			{Class: "fighter", RealizedLevel: 5, State: classes.EntryActive}, {Class: "mage", RealizedLevel: 5, State: classes.EntryActive},
		}, "illusionist", "mage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, all...)
			f.chars.EXPECT().View(mock.Anything, charID).Return(tc.sheet, nil)
			f.dal.EXPECT().ReplaceClassEntries(mock.Anything, charID, tc.want).Return(nil)
			f.slots.EXPECT().GetSlots(mock.Anything, charID).Return(nil, nil)
			f.book.EXPECT().All(mock.Anything, charID).Return(nil, nil)

			got, err := f.svc.Switch(context.Background(), charID, tc.entry, tc.class)
			require.NoError(t, err)
			assert.Equal(t, &classentrysvc.Result{Entry: tc.entry, From: tc.wantFrom, To: tc.to}, got)
			require.Len(t, f.log.got, 1)
			assert.Equal(t, actionlog.KindClassEntrySwitch, f.log.got[0].Kind)
			require.Len(t, f.log.got[0].Snapshots, 2, "the class entries and the slot list")
			assert.Equal(t, []string{"class_entries", "spell_slots"}, []string{f.log.got[0].Snapshots[0].Key, f.log.got[0].Snapshots[1].Key})
		})
	}
}

// An entry the character does not have and a class the catalogue does not know are field errors
// (400), not the step's conflicts; nothing is written.
func TestSwitch_Step15_UnknownEntryAndClass(t *testing.T) {
	for _, tc := range []struct {
		name, field, class string
		entry              int
	}{
		{"entry past the last", "entry", "transmuter", 1},
		{"negative entry", "entry", "transmuter", -1},
		{"unknown class", "class", "no_such_class", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, all...)
			f.chars.EXPECT().View(mock.Anything, charID).Return(mageSheet(), nil)
			_, err := f.svc.Switch(context.Background(), charID, tc.entry, tc.class)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tc.field, fe.Field)
			assert.Empty(t, f.log.got)
		})
	}
}
