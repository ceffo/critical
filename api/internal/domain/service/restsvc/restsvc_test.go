package restsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeTx runs fn directly, the way InTx does when it commits.
type fakeTx struct{ calls int }

func (f *fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func TestNew_NilDependency(t *testing.T) {
	_, err := restsvc.New(nil, mocks.NewMockItems(t), &fakeTx{})
	require.Error(t, err)
}

func TestLongRest_ResetsBothInsideOneTransaction(t *testing.T) {
	spells, items, tx := mocks.NewMockSpells(t), mocks.NewMockItems(t), &fakeTx{}
	svc, err := restsvc.New(spells, items, tx)
	require.NoError(t, err)
	spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{{Level: 1}}, nil)
	items.EXPECT().ResetDaily(mock.Anything).Return([]itemsvc.Item{{ID: "gem", Count: 1}}, nil)

	got, err := svc.LongRest(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, tx.calls)
	assert.Equal(t, []spellsvc.SpellSlot{{Level: 1}}, got.Slots)
	assert.Equal(t, []itemsvc.Item{{ID: "gem", Count: 1}}, got.Items)
}

func TestLongRest_ItemFailureFailsTheRest(t *testing.T) {
	spells, items := mocks.NewMockSpells(t), mocks.NewMockItems(t)
	svc, _ := restsvc.New(spells, items, &fakeTx{})
	spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, nil)
	boom := errors.New("boom")
	items.EXPECT().ResetDaily(mock.Anything).Return(nil, boom)

	_, err := svc.LongRest(context.Background(), 1)
	require.ErrorIs(t, err, boom)
}

func TestLongRest_SlotFailureSkipsItems(t *testing.T) {
	spells, items := mocks.NewMockSpells(t), mocks.NewMockItems(t)
	svc, _ := restsvc.New(spells, items, &fakeTx{})
	boom := errors.New("boom")
	spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, boom)

	_, err := svc.LongRest(context.Background(), 1)
	require.ErrorIs(t, err, boom)
}
