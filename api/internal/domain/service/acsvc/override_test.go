package acsvc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

const charID = int64(3)

type countingTx struct{ n int }

func (c *countingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	c.n++
	return fn(ctx)
}

type fixture struct {
	svc       acsvc.SVC
	items     *mocks.MockItems
	effects   *mocks.MockEffects
	chars     *mocks.MockCharacters
	tables    *mocks.MockTables
	overrides *mocks.MockOverrides
	log       *mocks.MockLog
	tx        *countingTx
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		items: mocks.NewMockItems(t), effects: mocks.NewMockEffects(t), chars: mocks.NewMockCharacters(t),
		tables: mocks.NewMockTables(t), overrides: mocks.NewMockOverrides(t), log: mocks.NewMockLog(t),
		tx: &countingTx{},
	}
	svc, err := acsvc.New(acsvc.Deps{
		Items: f.items, Effects: f.effects, Characters: f.chars, Tables: f.tables,
		Overrides: f.overrides, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

// reads expects the reads of one Get: a Dexterity 17 character in chain
// mail with Protection from Evil on.
func (f *fixture) reads() {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Dex: 17}, nil).Once()
	f.tables.EXPECT().DexMods(17).Return(2, -3, nil).Once()
	f.items.EXPECT().List(mock.Anything, charID).Return([]itemsvc.Item{armour("chain", 5, itemsvc.StateWorn)}, nil).Once()
	f.effects.EXPECT().ListStatus(mock.Anything, charID).Return([]buffsvc.BuffStatus{acBuff("Protection from Evil", -2, true)}, nil).Once()
	f.effects.EXPECT().ListPowers(mock.Anything, charID).Return([]buffsvc.ItemPower{}, nil).Once()
}

// ADR 0033 step 10: Get derives the AC from the sheet's Dexterity (PHB Table
// 2 through the rule tables), the pieces, the buffs and the powers; nothing
// is written or logged (the AC is never stored).
func TestAC_Step10_GetReadsTheInputsAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.reads()
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(nil, nil).Once()

	ac, err := f.svc.Get(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, 0, ac.Value, "chain mail 5, Dexterity -3, Protection from Evil -2")
	assert.Nil(t, ac.Override)
	assert.Zero(t, f.tx.n, "a read opens no transaction")
}

// ADR 0033 step 13: setting the override is one ac_override entry with the
// override before and after (payload and snapshot), in one transaction; the
// AC answered carries the override and keeps the derived AC beside it.
func TestAC_Step13_SetTheOverrideIsOneLoggedMove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	set := acsvc.Override{Value: 4, Label: "Shield spell"}
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(nil, nil).Once()
	f.overrides.EXPECT().SetACOverride(mock.Anything, charID, &set).Return(nil).Once()
	var entry actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, e actionlog.NewEntry) (int64, error) {
		entry = e
		return 1, nil
	}).Once()
	f.reads()
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(&set, nil).Once()

	ac, err := f.svc.SetOverride(ctx, charID, acsvc.Override{Value: 4, Label: "  Shield spell "})
	require.NoError(t, err)
	assert.Equal(t, &set, ac.Override)
	assert.Equal(t, 0, ac.Value, "the derived AC stays beside it")
	assert.Equal(t, 1, f.tx.n)

	assert.Equal(t, actionlog.KindACOverride, entry.Kind)
	assert.Equal(t, charID, entry.CharacterID)
	assert.JSONEq(t, `{"before":null,"after":{"value":4,"label":"Shield spell"}}`, string(entry.Payload))
	require.Len(t, entry.Snapshots, 1)
	assert.Equal(t, acsvc.StateKey, entry.Snapshots[0].Key)
	assert.JSONEq(t, `{"override":null}`, snaptest.State(t, entry.Snapshots[0].Before))
	assert.JSONEq(t, `{"override":{"value":4,"label":"Shield spell"}}`, snaptest.State(t, entry.Snapshots[0].After))
}

// ADR 0033 step 13: clearing is logged the same way; the same override
// again, or clearing none, logs nothing.
func TestAC_Step13_ClearIsLoggedTheSameOverrideAgainIsNot(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cur := acsvc.Override{Value: 4}
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(&cur, nil).Once()
	f.overrides.EXPECT().SetACOverride(mock.Anything, charID, (*acsvc.Override)(nil)).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e actionlog.NewEntry) bool {
		return e.Kind == actionlog.KindACOverride && string(e.Payload) == `{"before":{"value":4},"after":null}`
	})).Return(2, nil).Once()
	f.reads()
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(nil, nil).Once()
	ac, err := f.svc.ClearOverride(ctx, charID)
	require.NoError(t, err)
	assert.Nil(t, ac.Override)

	// The same again: no write, no entry (the mocks refuse any).
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(&cur, nil).Twice()
	f.reads()
	_, err = f.svc.SetOverride(ctx, charID, acsvc.Override{Value: 4})
	require.NoError(t, err)
	f.overrides.EXPECT().GetACOverride(mock.Anything, charID).Return(nil, nil).Twice()
	f.reads()
	_, err = f.svc.ClearOverride(ctx, charID)
	require.NoError(t, err)
}

// ADR 0033 step 13: a number -10 to 10 and a short label; anything else is
// a field error and nothing is read, written or logged.
func TestAC_Step13_OverrideValidation(t *testing.T) {
	cases := []struct {
		name  string
		o     acsvc.Override
		field string
		code  string
	}{
		{"better than -10", acsvc.Override{Value: -11}, "value", acsvc.CodeOutOfRange},
		{"worse than 10", acsvc.Override{Value: 11}, "value", acsvc.CodeOutOfRange},
		{"a long label", acsvc.Override{Value: 4, Label: strings.Repeat("é", acsvc.MaxLabelLen+1)}, "label", acsvc.CodeTooLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.SetOverride(context.Background(), charID, c.o)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
			assert.Equal(t, c.code, fe.Message)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

// ADR 0033 step 13 with ADR 0018 and ADR 0019: the restorer writes the
// snapshot back, the override or none.
func TestAC_Step13_RestorerWritesTheSnapshotBack(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := f.svc.Restorers()[acsvc.StateKey]
	require.NotNil(t, r)
	f.overrides.EXPECT().SetACOverride(mock.Anything, charID, &acsvc.Override{Value: 4, Label: "Shield"}).Return(nil).Once()
	require.NoError(t, r.Restore(ctx, charID, snaptest.Env(`{"override":{"value":4,"label":"Shield"}}`)))
	f.overrides.EXPECT().SetACOverride(mock.Anything, charID, (*acsvc.Override)(nil)).Return(nil).Once()
	require.NoError(t, r.Restore(ctx, charID, snaptest.Env(`{"override":null}`)))

	f.overrides.EXPECT().SetACOverride(mock.Anything, charID, mock.Anything).Return(errors.New("boom")).Once()
	require.Error(t, r.Restore(ctx, charID, snaptest.Env(`{"override":null}`)))
}
