package itemsvc_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew_NilDAL(t *testing.T) {
	_, err := itemsvc.New(nil)
	require.Error(t, err)
}

func TestUse_ReturnsUpdatedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, err := itemsvc.NewWithConfig(itemsvc.DefaultConfig(), dal)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, "wand").Return(itemsvc.Item{ID: "wand", Count: 3}, nil)

	got, err := svc.Use(context.Background(), "wand")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Count)
}

func TestUse_PropagatesSentinels(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal)
		dal.EXPECT().Decrement(mock.Anything, "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.Use(context.Background(), "x")
		assert.True(t, errors.Is(err, target))
	}
}

func TestResetDaily_ResetsThenLists(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal)
	dal.EXPECT().ResetPerDay(mock.Anything).Return(nil)
	dal.EXPECT().List(mock.Anything).Return([]itemsvc.Item{{ID: "gem", Count: 1}}, nil)

	got, err := svc.ResetDaily(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Item{{ID: "gem", Count: 1}}, got)
}

func TestResetDaily_ResetErrorSkipsList(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal)
	dal.EXPECT().ResetPerDay(mock.Anything).Return(errors.New("boom"))
	_, err := svc.ResetDaily(context.Background())
	require.Error(t, err)
}
