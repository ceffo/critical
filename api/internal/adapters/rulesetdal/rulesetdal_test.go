// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package rulesetdal_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/rulesetdal"
	"github.com/ceffo/critical/api/internal/adapters/rulesetdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/rulesetdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

func TestDefault_MapsTheRowAndItsChildren(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().RulesetByName(ctx, "default").Return(queries.Ruleset{
		ID: 3, Name: "default", CatalogueVersion: "v1", CritPlaceholder: true,
		MulticlassAnyRace: true, DualclassAnyRace: false, LevelLimits: "warn",
	}, nil).Once()
	q.EXPECT().ListRulesetCritRows(ctx, int64(3)).Return([]queries.ListRulesetCritRowsRow{
		{RollFrom: 1, RollTo: 50, Multiplier: 1.5}, {RollFrom: 51, RollTo: 100, Multiplier: 2.25},
	}, nil).Once()
	q.EXPECT().ListRulesetSuccessEffects(ctx, int64(3)).Return([]queries.ListRulesetSuccessEffectsRow{
		{ProficiencyID: "tumbling", Ac: -4, Rounds: 1},
	}, nil).Once()

	got, err := rulesetdal.New(q).Default(ctx)
	require.NoError(t, err)
	assert.Equal(t, ruleset.Ruleset{
		ID: 3, Name: "default", CatalogueVersion: "v1",
		Crit:           ruleset.CritTable{Placeholder: true, Rows: []ruleset.CritRow{{From: 1, To: 50, Multiplier: 1.5}, {From: 51, To: 100, Multiplier: 2.25}}},
		RacialLift:     ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: false},
		LevelLimits:    ruleset.LevelLimitsWarn,
		SuccessEffects: []ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -4, Rounds: 1}},
	}, got)
}

func TestDefault_NoRowIsNotFound(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().RulesetByName(ctx, "default").Return(queries.Ruleset{}, sql.ErrNoRows).Once()
	_, err := rulesetdal.New(q).Default(ctx)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ReplaceCritTable clears the rows first, inserts every row in order, then clears the placeholder.
func TestReplaceCritTable_SwapsTheWholeTable(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	mock.InOrder(
		q.EXPECT().DeleteRulesetCritRows(ctx, int64(3)).Return(nil).Once(),
		q.EXPECT().InsertRulesetCritRow(ctx, queries.InsertRulesetCritRowParams{RulesetID: 3, RollFrom: 1, RollTo: 40, Multiplier: 2}).Return(nil).Once(),
		q.EXPECT().InsertRulesetCritRow(ctx, queries.InsertRulesetCritRowParams{RulesetID: 3, RollFrom: 41, RollTo: 100, Multiplier: 1.5}).Return(nil).Once(),
		q.EXPECT().SetRulesetCritPlaceholder(ctx, queries.SetRulesetCritPlaceholderParams{ID: 3, CritPlaceholder: false}).Return(nil).Once(),
	)
	err := rulesetdal.New(q).ReplaceCritTable(ctx, 3, []ruleset.CritRow{{From: 1, To: 40, Multiplier: 2}, {From: 41, To: 100, Multiplier: 1.5}})
	require.NoError(t, err)
}

func TestReplaceCritTable_AnInsertFailureIsReturned(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().DeleteRulesetCritRows(ctx, int64(3)).Return(nil).Once()
	q.EXPECT().InsertRulesetCritRow(ctx, mock.Anything).Return(errors.New("boom")).Once()
	err := rulesetdal.New(q).ReplaceCritTable(ctx, 3, []ruleset.CritRow{{From: 1, To: 100, Multiplier: 2}})
	assert.ErrorContains(t, err, "boom")
}

func TestSetRacialLift_And_SuccessEffects_And_Version(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().SetRulesetRacialLift(ctx, queries.SetRulesetRacialLiftParams{ID: 3, MulticlassAnyRace: false, DualclassAnyRace: true}).Return(nil).Once()
	mock.InOrder(
		q.EXPECT().DeleteRulesetSuccessEffects(ctx, int64(3)).Return(nil).Once(),
		q.EXPECT().InsertRulesetSuccessEffect(ctx, queries.InsertRulesetSuccessEffectParams{RulesetID: 3, ProficiencyID: "tumbling", Ac: -4, Rounds: 1}).Return(nil).Once(),
	)
	q.EXPECT().SetRulesetCatalogueVersion(ctx, queries.SetRulesetCatalogueVersionParams{ID: 3, CatalogueVersion: "v2"}).Return(nil).Once()

	d := rulesetdal.New(q)
	require.NoError(t, d.SetRacialLift(ctx, 3, ruleset.RacialLift{DualclassAnyRace: true}))
	require.NoError(t, d.ReplaceSuccessEffects(ctx, 3, []ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -4, Rounds: 1}}))
	require.NoError(t, d.SetCatalogueVersion(ctx, 3, "v2"))
}

func TestChanges_AppendAndList(t *testing.T) {
	ctx := context.Background()
	by := uuid.New()
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	q := mocks.NewMockQuerier(t)
	q.EXPECT().InsertRulesetChange(ctx, queries.InsertRulesetChangeParams{
		RulesetID: 3, ChangedBy: by, Rule: "crit_table", BeforeValue: "[1]", AfterValue: "[2]",
	}).Return(nil).Once()
	q.EXPECT().ListRulesetChanges(ctx, int64(3)).Return([]queries.RulesetChange{
		{ID: 9, RulesetID: 3, ChangedAt: at, ChangedBy: by, Rule: "crit_table", BeforeValue: "[1]", AfterValue: "[2]"},
	}, nil).Once()

	d := rulesetdal.New(q)
	require.NoError(t, d.AddChange(ctx, ruleset.Change{RulesetID: 3, By: by, Rule: "crit_table", Before: "[1]", After: "[2]"}))
	got, err := d.ListChanges(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, []ruleset.Change{{ID: 9, RulesetID: 3, At: at, By: by, Rule: "crit_table", Before: "[1]", After: "[2]"}}, got)
}
