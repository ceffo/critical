package rulesetsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/rulesetsvc"
	"github.com/ceffo/critical/api/internal/domain/service/rulesetsvc/mocks"
)

// noTx runs the body once, in the caller's context.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

var admin = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

type fixture struct {
	dal   *mocks.MockDAL
	profs *mocks.MockProficiencies
	svc   rulesetsvc.SVC
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dal: mocks.NewMockDAL(t), profs: mocks.NewMockProficiencies(t)}
	svc, err := rulesetsvc.New(rulesetsvc.Deps{DAL: f.dal, Proficiencies: f.profs, Tx: noTx{}})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func stored() ruleset.Ruleset {
	rs := ruleset.Default()
	rs.ID = 4
	return rs
}

func (f *fixture) loads() { f.dal.EXPECT().Default(mock.Anything).Return(stored(), nil) }

// ADR 0041 step 1: the default ruleset is named as the default and records the catalogue version
// the server loaded; recording the version it already holds writes nothing.
func TestRecordCatalogueVersion_Step1(t *testing.T) {
	f := newFixture(t)
	f.loads()
	f.dal.EXPECT().SetCatalogueVersion(mock.Anything, int64(4), "v9").Return(nil).Once()
	require.NoError(t, f.svc.RecordCatalogueVersion(context.Background(), "v9"))

	got, err := f.svc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ruleset.DefaultName, got.Name)

	g := newFixture(t)
	rs := stored()
	rs.CatalogueVersion = "v9"
	g.dal.EXPECT().Default(mock.Anything).Return(rs, nil)
	require.NoError(t, g.svc.RecordCatalogueVersion(context.Background(), "v9")) // SetCatalogueVersion: unexpected call fails
}

// ADR 0041 step 3: every character resolves the default ruleset, the one place that says so.
func TestForCharacter_Step3_ResolvesTheDefault(t *testing.T) {
	f := newFixture(t)
	f.loads()
	got, err := f.svc.ForCharacter(context.Background(), 12)
	require.NoError(t, err)
	assert.Equal(t, stored(), got)
}

// ADR 0041 step 7: an admin's crit table edit stores the sorted table and writes one change
// record with the table before and after.
func TestSetCritTable_Step7_WritesOneChange(t *testing.T) {
	f := newFixture(t)
	f.loads()
	sorted := []ruleset.CritRow{{From: 1, To: 40, Multiplier: 2}, {From: 41, To: 100, Multiplier: 1.5}}
	f.dal.EXPECT().ReplaceCritTable(mock.Anything, int64(4), sorted).Return(nil).Once()
	f.dal.EXPECT().AddChange(mock.Anything, ruleset.Change{
		RulesetID: 4, By: admin, Rule: ruleset.RuleCritTable,
		Before: `[{"from":1,"to":100,"multiplier":2}]`,
		After:  `[{"from":1,"to":40,"multiplier":2},{"from":41,"to":100,"multiplier":1.5}]`,
	}).Return(nil).Once()

	got, err := f.svc.SetCritTable(context.Background(), admin, []ruleset.CritRow{sorted[1], sorted[0]})
	require.NoError(t, err)
	assert.Equal(t, ruleset.CritTable{Rows: sorted}, got)
}

// ADR 0041 step 7: a failed edit writes no change record: an invalid table never reaches the
// store, and a store failure stops before the audit line.
func TestSetCritTable_Step7_FailedEditWritesNoChange(t *testing.T) {
	t.Run("invalid table", func(t *testing.T) {
		f := newFixture(t) // any DAL call fails the test
		_, err := f.svc.SetCritTable(context.Background(), admin, []ruleset.CritRow{{From: 1, To: 40, Multiplier: 2}, {From: 50, To: 100, Multiplier: 2}})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
	t.Run("store failure", func(t *testing.T) {
		f := newFixture(t)
		f.loads()
		f.dal.EXPECT().ReplaceCritTable(mock.Anything, int64(4), mock.Anything).Return(errors.New("disk full")).Once()
		_, err := f.svc.SetCritTable(context.Background(), admin, []ruleset.CritRow{{From: 1, To: 100, Multiplier: 3}})
		require.ErrorContains(t, err, "disk full")
	})
}

// ADR 0041 steps 2.2 and 7: the racial lift edit stores both switches and audits the change.
func TestSetRacialLift_Step7_WritesOneChange(t *testing.T) {
	f := newFixture(t)
	f.loads()
	lift := ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: false}
	f.dal.EXPECT().SetRacialLift(mock.Anything, int64(4), lift).Return(nil).Once()
	f.dal.EXPECT().AddChange(mock.Anything, ruleset.Change{
		RulesetID: 4, By: admin, Rule: ruleset.RuleRacialLift,
		Before: `{"multiclassAnyRace":true,"dualclassAnyRace":true}`,
		After:  `{"multiclassAnyRace":true,"dualclassAnyRace":false}`,
	}).Return(nil).Once()
	got, err := f.svc.SetRacialLift(context.Background(), admin, lift)
	require.NoError(t, err)
	assert.Equal(t, lift, got)
}

// ADR 0041 steps 2.4 and 7: the success effects are checked against the book's proficiencies; a
// valid edit audits the change, an unknown proficiency or zero rounds writes nothing.
func TestSetSuccessEffects_Steps2And7(t *testing.T) {
	t.Run("a valid edit", func(t *testing.T) {
		f := newFixture(t)
		f.profs.EXPECT().Has("tumbling").Return(true)
		f.profs.EXPECT().Has("tracking").Return(true)
		f.loads()
		want := []ruleset.SuccessEffect{{ProficiencyID: "tracking", AC: 0, Rounds: 2}, {ProficiencyID: "tumbling", AC: -4, Rounds: 1}}
		f.dal.EXPECT().ReplaceSuccessEffects(mock.Anything, int64(4), want).Return(nil).Once()
		f.dal.EXPECT().AddChange(mock.Anything, ruleset.Change{
			RulesetID: 4, By: admin, Rule: ruleset.RuleSuccessEffects,
			Before: `[{"proficiencyId":"tumbling","ac":-4,"rounds":1}]`,
			After:  `[{"proficiencyId":"tracking","ac":0,"rounds":2},{"proficiencyId":"tumbling","ac":-4,"rounds":1}]`,
		}).Return(nil).Once()
		got, err := f.svc.SetSuccessEffects(context.Background(), admin, []ruleset.SuccessEffect{want[1], want[0]})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("an unknown proficiency", func(t *testing.T) {
		f := newFixture(t)
		f.profs.EXPECT().Has("nope").Return(false)
		_, err := f.svc.SetSuccessEffects(context.Background(), admin, []ruleset.SuccessEffect{{ProficiencyID: "nope", Rounds: 1}})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
	t.Run("zero rounds", func(t *testing.T) {
		f := newFixture(t)
		f.profs.EXPECT().Has("tumbling").Return(true)
		_, err := f.svc.SetSuccessEffects(context.Background(), admin, []ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -4}})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}

func TestChanges_ListsTheDefaultsAudit(t *testing.T) {
	f := newFixture(t)
	f.loads()
	want := []ruleset.Change{{ID: 2, RulesetID: 4, Rule: ruleset.RuleCritTable}}
	f.dal.EXPECT().ListChanges(mock.Anything, int64(4)).Return(want, nil).Once()
	got, err := f.svc.Changes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
