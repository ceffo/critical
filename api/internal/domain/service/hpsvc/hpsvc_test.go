package hpsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDAL is a hand-written test double of hpsvc.DAL. Tests configure
// canned return values and assert on call effects.
type fakeDAL struct {
	maxHP      int
	maxHPErr   error
	sum        int
	sumErr     error
	history    []hpsvc.HpEvent
	historyErr error
	last       struct {
		delta  int
		source string
		ok     bool
		err    error
	}
	appended  []appendCall
	appendErr error
}

type appendCall struct {
	charID int64
	delta  int
	source string
}

func (f *fakeDAL) AppendEvent(_ context.Context, charID int64, delta int, source string) error {
	f.appended = append(f.appended, appendCall{charID, delta, source})
	return f.appendErr
}
func (f *fakeDAL) GetCurrentHPSum(_ context.Context, _ int64) (int, error) { return f.sum, f.sumErr }
func (f *fakeDAL) GetHistory(_ context.Context, _ int64, _ int) ([]hpsvc.HpEvent, error) {
	return f.history, f.historyErr
}
func (f *fakeDAL) GetMaxHP(_ context.Context, _ int64) (int, error) { return f.maxHP, f.maxHPErr }
func (f *fakeDAL) LastEvent(_ context.Context, _ int64) (int, string, bool, error) {
	return f.last.delta, f.last.source, f.last.ok, f.last.err
}

func newSvc(t *testing.T, dal hpsvc.DAL) hpsvc.SVC {
	t.Helper()
	svc, err := hpsvc.NewWithConfig(hpsvc.DefaultConfig(), dal)
	require.NoError(t, err)
	return svc
}

func TestGetState(t *testing.T) {
	dal := &fakeDAL{maxHP: 20, sum: -5}
	svc := newSvc(t, dal)

	state, err := svc.GetState(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 15, state.CurrentHp)
	assert.Equal(t, 20, state.MaxHp)
}

func TestAppendEvent_RequiresSource(t *testing.T) {
	svc := newSvc(t, &fakeDAL{maxHP: 20})
	_, err := svc.AppendEvent(context.Background(), 1, -5, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidInput))
}

func TestAppendEvent_PersistsAndReturnsState(t *testing.T) {
	dal := &fakeDAL{maxHP: 20, sum: -5}
	svc := newSvc(t, dal)

	state, err := svc.AppendEvent(context.Background(), 7, -5, "sword")
	require.NoError(t, err)
	require.Len(t, dal.appended, 1)
	assert.Equal(t, appendCall{7, -5, "sword"}, dal.appended[0])
	assert.Equal(t, 15, state.CurrentHp)
}

func TestUndoLast_NoEventsIsNoop(t *testing.T) {
	dal := &fakeDAL{maxHP: 20}
	svc := newSvc(t, dal)
	_, err := svc.UndoLast(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, dal.appended, "no event existed, no append should occur")
}

func TestUndoLast_AppendsNegation(t *testing.T) {
	dal := &fakeDAL{maxHP: 20}
	dal.last.delta = -10
	dal.last.source = "giant"
	dal.last.ok = true
	svc := newSvc(t, dal)

	_, err := svc.UndoLast(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, dal.appended, 1)
	assert.Equal(t, appendCall{3, 10, "undo:giant"}, dal.appended[0])
}
