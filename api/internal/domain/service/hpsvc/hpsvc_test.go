package hpsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newSvc(t *testing.T, dal hpsvc.DAL) hpsvc.SVC {
	t.Helper()
	svc, err := hpsvc.NewWithConfig(hpsvc.DefaultConfig(), dal)
	require.NoError(t, err)
	return svc
}

// stubState stubs the three GetState DAL calls on a mock with fixed maxHP and
// sumHP values and empty history. Call after wiring any other expectations.
func stubState(dal *mocks.MockDAL, maxHP, sumHP int) {
	dal.On("GetMaxHP", mock.Anything, mock.Anything).Return(maxHP, nil)
	dal.On("GetCurrentHPSum", mock.Anything, mock.Anything).Return(sumHP, nil)
	dal.On("GetHistory", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
}

func TestGetState(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	stubState(dal, 20, -5)
	svc := newSvc(t, dal)

	state, err := svc.GetState(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 15, state.CurrentHp)
	assert.Equal(t, 20, state.MaxHp)
}

func TestAppendEvent_RequiresSource(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc := newSvc(t, dal)

	_, err := svc.AppendEvent(context.Background(), 1, -5, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidInput))
}

func TestAppendEvent_PersistsAndReturnsState(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	dal.On("AppendEvent", mock.Anything, int64(7), -5, "sword").Return(nil)
	stubState(dal, 20, -5)
	svc := newSvc(t, dal)

	state, err := svc.AppendEvent(context.Background(), 7, -5, "sword")
	require.NoError(t, err)
	assert.Equal(t, 15, state.CurrentHp)
	dal.AssertCalled(t, "AppendEvent", mock.Anything, int64(7), -5, "sword")
}

func TestUndoLast_NoEventsIsNoop(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	dal.On("LastEvent", mock.Anything, mock.Anything).Return(0, "", false, nil)
	stubState(dal, 20, 0)
	svc := newSvc(t, dal)

	_, err := svc.UndoLast(context.Background(), 1)
	require.NoError(t, err)
	dal.AssertNotCalled(t, "AppendEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUndoLast_AppendsNegation(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	dal.On("LastEvent", mock.Anything, int64(3)).Return(-10, "giant", true, nil)
	dal.On("AppendEvent", mock.Anything, int64(3), 10, "undo:giant").Return(nil)
	stubState(dal, 20, 0)
	svc := newSvc(t, dal)

	_, err := svc.UndoLast(context.Background(), 3)
	require.NoError(t, err)
	dal.AssertCalled(t, "AppendEvent", mock.Anything, int64(3), 10, "undo:giant")
}
