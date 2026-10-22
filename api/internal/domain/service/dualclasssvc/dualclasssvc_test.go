package dualclasssvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rsmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc/mocks"
)

const charID = int64(5)

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// recorder keeps the entries the service logs.
type recorder struct{ got []actionlog.NewEntry }

func (r *recorder) Record(_ context.Context, e actionlog.NewEntry) (int64, error) {
	r.got = append(r.got, e)
	return int64(len(r.got)), nil
}

type fixture struct {
	dal    *mocks.MockDAL
	chars  *mocks.MockCharacters
	ledger *mocks.MockLedger
	cat    *mocks.MockCatalogue
	rules  *rsmocks.MockResolver
	log    *recorder
	svc    dualclasssvc.SVC
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), ledger: mocks.NewMockLedger(t),
		cat: mocks.NewMockCatalogue(t), rules: rsmocks.NewMockResolver(t), log: &recorder{},
	}
	svc, err := dualclasssvc.New(dualclasssvc.Deps{
		DAL: f.dal, Characters: f.chars, Ledger: f.ledger, Catalogue: f.cat, Rulesets: f.rules, Log: f.log, Tx: noTx{},
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

var (
	fighter = classes.Class{ID: "fighter", PrimeRequisites: []string{"str"}}
	mage    = classes.Class{
		ID: "mage", PrimeRequisites: []string{"int"},
		AbilityMinimums: []classes.AbilityMinimum{{Ability: "int", Min: 9}},
		Alignments:      []string{"true_neutral", "lawful_good"},
	}
)

// sheet is a human level-6 fighter with 21,000 XP.
func sheet(str, intel int, alignment string) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Race: "human", Class: "fighter", RealizedLevel: 6, Str: str, Int: intel, Dex: 12, Con: 14, Wis: 10, Cha: 10,
		Details: charactersvc.Details{Alignment: alignment},
		Derived: charactersvc.Derived{Classes: []charactersvc.EntryDerived{{Class: "fighter", XP: 21_000}}},
	}
}

func (f *fixture) expectRead(c *charactersvc.Character) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(c, nil)
	f.rules.EXPECT().ForCharacter(mock.Anything, charID).Return(ruleset.Default(), nil)
	f.cat.EXPECT().Race("human").Return(classes.Race{}, false).Maybe()
	f.cat.EXPECT().Get("fighter").Return(fighter, nil).Maybe()
	f.cat.EXPECT().Get("mage").Return(mage, nil).Maybe()
}

func TestSwitch_Step2_2_TheNewClassMinimumsAndAlignmentAreRefusedWithNothingWritten(t *testing.T) {
	strictMage := mage
	strictMage.AbilityMinimums = []classes.AbilityMinimum{{Ability: "dex", Min: 15}}
	for _, tc := range []struct {
		name  string
		class classes.Class
		sheet *charactersvc.Character
		want  error
	}{
		{"ability minimums of the new class", strictMage, sheet(17, 17, "true_neutral"), dualclasssvc.ErrMinimumsNotMet},
		{"alignment the new class does not admit", mage, sheet(17, 17, "chaotic_evil"), dualclasssvc.ErrAlignmentNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.sheet, nil)
			f.rules.EXPECT().ForCharacter(mock.Anything, charID).Return(ruleset.Default(), nil)
			f.cat.EXPECT().Race("human").Return(classes.Race{}, false)
			f.cat.EXPECT().Get("fighter").Return(fighter, nil)
			f.cat.EXPECT().Get("mage").Return(tc.class, nil)

			_, err := f.svc.Switch(context.Background(), charID, "mage")
			require.ErrorIs(t, err, tc.want)
			assert.ErrorIs(t, err, domain.ErrConflict)
			assert.Empty(t, f.log.got, "nothing logged")
			// the mocks fail the test on an unexpected ReplaceClassEntries or Awards call
		})
	}
}

func TestSwitch_Step3_FreezesTheOldAndTheNewIgnoresEveryAwardUpToTheLastOne(t *testing.T) {
	f := newFixture(t)
	f.expectRead(sheet(17, 17, "true_neutral"))
	f.ledger.EXPECT().Awards(mock.Anything, charID).Return([]charactersvc.Award{{ID: 3, Amount: 1_000}, {ID: 9, Amount: 500}, {ID: 4, Amount: 1}}, nil)
	f.dal.EXPECT().ReplaceClassEntries(mock.Anything, charID, []classes.Entry{
		{Class: "fighter", RealizedLevel: 6, StartingXP: 21_000, State: classes.EntryFrozen, LeftAtLevel: 6},
		{Class: "mage", RealizedLevel: 1, State: classes.EntryActive, IgnoredAwardsThrough: 9},
	}).Return(nil)

	got, err := f.svc.Switch(context.Background(), charID, "mage")
	require.NoError(t, err)
	assert.Equal(t, &dualclasssvc.Result{From: "fighter", To: "mage", LeftAtLevel: 6, XP: 21_000}, got)
	require.Len(t, f.log.got, 1)
	assert.Equal(t, actionlog.KindClassSwitch, f.log.got[0].Kind)
	assert.JSONEq(t, `{"from":"fighter","to":"mage","leftAtLevel":6,"xp":21000}`, string(f.log.got[0].Payload))
}

func TestSwitch_AWriteFailureIsReturnedAndNothingIsLogged(t *testing.T) {
	f := newFixture(t)
	f.expectRead(sheet(17, 17, "true_neutral"))
	f.ledger.EXPECT().Awards(mock.Anything, charID).Return(nil, nil)
	boom := errors.New("boom")
	f.dal.EXPECT().ReplaceClassEntries(mock.Anything, charID, mock.Anything).Return(boom)

	_, err := f.svc.Switch(context.Background(), charID, "mage")
	require.ErrorIs(t, err, boom)
	assert.Empty(t, f.log.got)
}
