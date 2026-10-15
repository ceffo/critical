package hpsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc/mocks"
)

var errBoom = errors.New("boom")

func newSvc(t *testing.T) (hpsvc.SVC, *mocks.MockDAL, *mocks.MockLog) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	log := mocks.NewMockLog(t)
	svc, err := hpsvc.NewWithConfig(hpsvc.DefaultConfig(), dal, log)
	require.NoError(t, err)
	return svc, dal, log
}

// stubState stubs the GetState calls with fixed maxHP and log total and the
// given history.
func stubState(dal *mocks.MockDAL, log *mocks.MockLog, maxHP, total int, history []actionlog.Entry) {
	dal.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(maxHP, nil)
	log.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(total, nil)
	log.EXPECT().ActiveEntries(mock.Anything, mock.Anything, actionlog.KindHPChange, mock.Anything).Return(history, nil)
}

func TestGetState(t *testing.T) {
	at := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)
	minus12 := -12
	svc, dal, log := newSvc(t)
	stubState(dal, log, 20, -5, []actionlog.Entry{
		{ID: 2, CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 3, CreatedAt: at},
		{
			ID: 1, CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, CreatedAt: at,
			Corrections: []actionlog.Correction{{Kind: actionlog.CorrectionAmend, HPDelta: &minus12, Summary: "orc axe"}},
		},
	})

	state, err := svc.GetState(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 15, state.CurrentHp)
	assert.Equal(t, 20, state.MaxHp)
	assert.Equal(t, []hpsvc.HpEvent{
		{ID: 2, CharacterID: 1, Delta: 3, Source: "potion", CreatedAt: at},
		{ID: 1, CharacterID: 1, Delta: -12, Source: "orc axe", CreatedAt: at},
	}, state.Events)
}

func TestGetState_Errors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(dal *mocks.MockDAL, log *mocks.MockLog)
		want  error
	}{
		{"character missing", func(dal *mocks.MockDAL, _ *mocks.MockLog) {
			dal.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(0, domain.ErrCharacterNotFound)
		}, domain.ErrCharacterNotFound},
		{"total fails", func(dal *mocks.MockDAL, log *mocks.MockLog) {
			dal.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(20, nil)
			log.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, errBoom)
		}, errBoom},
		{"history fails", func(dal *mocks.MockDAL, log *mocks.MockLog) {
			dal.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(20, nil)
			log.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, nil)
			log.EXPECT().ActiveEntries(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errBoom)
		}, errBoom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, log := newSvc(t)
			c.setup(dal, log)
			_, err := svc.GetState(context.Background(), 1)
			require.ErrorIs(t, err, c.want)
		})
	}
}

func TestAppendEvent(t *testing.T) {
	cases := []struct {
		name    string
		delta   int
		source  string
		wantErr error
		want    *actionlog.NewEntry
	}{
		{"no source", -5, "", nil, &actionlog.NewEntry{CharacterID: 7, Kind: actionlog.KindHPChange, HPDelta: -5, Payload: json.RawMessage(`{}`)}},
		{"blank source", 4, "   ", nil, &actionlog.NewEntry{CharacterID: 7, Kind: actionlog.KindHPChange, HPDelta: 4, Payload: json.RawMessage(`{}`)}},
		{"damage", -5, " sword ", nil, &actionlog.NewEntry{CharacterID: 7, Kind: actionlog.KindHPChange, Summary: "sword", HPDelta: -5}},
		{"heal", 4, "potion", nil, &actionlog.NewEntry{CharacterID: 7, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, log := newSvc(t)
			if c.want != nil {
				log.EXPECT().Record(mock.Anything, *c.want).Return(1, nil)
				stubState(dal, log, 20, c.delta-10, nil) // from 10/20
			}
			state, err := svc.AppendEvent(context.Background(), 7, c.delta, c.source)
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 10+c.delta, state.CurrentHp)
		})
	}
}

// HP-5: a total above 0 (rows logged before the cap) is
// never shown above max.
func TestGetState_NeverAboveMax(t *testing.T) {
	for _, c := range []struct{ total, want int }{{-1, 19}, {0, 20}, {7, 20}} {
		svc, dal, log := newSvc(t)
		stubState(dal, log, 20, c.total, nil)
		state, err := svc.GetState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.CurrentHp, "total %d", c.total)
		assert.Equal(t, 20, state.MaxHp)
	}
}

// ADR 0011 step 1 (display guard): rows logged before the floor existed
// never show HP below -10; the state carries the floor.
func TestGetState_NeverBelowFloor(t *testing.T) {
	for _, c := range []struct{ total, want int }{{-29, -9}, {-30, -10}, {-45, -10}} {
		svc, dal, log := newSvc(t)
		stubState(dal, log, 20, c.total, nil)
		state, err := svc.GetState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.CurrentHp, "total %d", c.total)
		assert.Equal(t, -10, state.FloorHp)
	}
}

// ADR 0011 step 2: the condition thresholds, at each boundary. Max 40: a
// quarter is 10; max 41: a quarter is 10.25.
func TestConditionOf(t *testing.T) {
	cases := []struct {
		name         string
		current, max int
		want         hpsvc.Condition
	}{
		{"full HP", 40, 40, hpsvc.ConditionOK},
		{"one above a quarter", 11, 40, hpsvc.ConditionOK},
		{"exactly a quarter", 10, 40, hpsvc.ConditionBadlyWounded},
		{"just above a fractional quarter", 11, 41, hpsvc.ConditionOK},
		{"just below a fractional quarter", 10, 41, hpsvc.ConditionBadlyWounded},
		{"1 HP", 1, 40, hpsvc.ConditionBadlyWounded},
		{"1 HP of 3 is above a quarter", 1, 3, hpsvc.ConditionOK},
		{"exactly 0", 0, 40, hpsvc.ConditionUnconscious},
		{"-1", -1, 40, hpsvc.ConditionUnconscious},
		{"-9, one above the floor", -9, 40, hpsvc.ConditionUnconscious},
		{"-10, the floor", -10, 40, hpsvc.ConditionDead},
		{"below the floor (legacy)", -12, 40, hpsvc.ConditionDead},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, hpsvc.ConditionOf(c.current, c.max))
		})
	}
}

// ADR 0011 step 2: GetState evaluates the condition on the HP it returns
// (after the floor and max guards).
func TestGetState_Condition(t *testing.T) {
	for _, c := range []struct {
		total int
		want  hpsvc.Condition
	}{{0, hpsvc.ConditionOK}, {-15, hpsvc.ConditionBadlyWounded}, {-20, hpsvc.ConditionUnconscious}, {-30, hpsvc.ConditionDead}, {-41, hpsvc.ConditionDead}} {
		svc, dal, log := newSvc(t)
		stubState(dal, log, 20, c.total, nil)
		state, err := svc.GetState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.Condition, "total %d", c.total)
	}
}

func TestAppendEvent_RecordFails(t *testing.T) {
	svc, _, log := newSvc(t)
	log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, errBoom)
	_, err := svc.AppendEvent(context.Background(), 7, -5, "sword")
	require.ErrorIs(t, err, errBoom)
}

func TestUndoLast(t *testing.T) {
	for _, voided := range []bool{true, false} {
		svc, dal, log := newSvc(t)
		log.EXPECT().VoidLatest(mock.Anything, int64(3), actionlog.KindHPChange).Return(voided, nil)
		stubState(dal, log, 20, 0, nil)
		state, err := svc.UndoLast(context.Background(), 3)
		require.NoError(t, err)
		assert.Equal(t, 20, state.CurrentHp)
	}
}

func TestUndoLast_VoidFails(t *testing.T) {
	svc, _, log := newSvc(t)
	log.EXPECT().VoidLatest(mock.Anything, int64(3), actionlog.KindHPChange).Return(false, errBoom)
	_, err := svc.UndoLast(context.Background(), 3)
	require.ErrorIs(t, err, errBoom)
}

// historyLimitSeen builds the service via New (env-driven) and returns the
// limit it passes to the log's ActiveEntries.
func historyLimitSeen(t *testing.T) int {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	log := mocks.NewMockLog(t)
	dal.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(10, nil)
	log.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, nil)
	var got int
	log.EXPECT().ActiveEntries(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ int64, _ actionlog.Kind, limit int) { got = limit }).Return(nil, nil)
	svc, err := hpsvc.New(dal, log)
	require.NoError(t, err)
	_, err = svc.GetState(context.Background(), 1)
	require.NoError(t, err)
	return got
}

func TestNew_EnvConfig(t *testing.T) {
	t.Run("unset uses default", func(t *testing.T) {
		t.Setenv("HP_HISTORY_LIMIT", "")
		require.NoError(t, os.Unsetenv("HP_HISTORY_LIMIT"))
		assert.Equal(t, 5, historyLimitSeen(t))
	})
	t.Run("override", func(t *testing.T) {
		t.Setenv("HP_HISTORY_LIMIT", "12")
		assert.Equal(t, 12, historyLimitSeen(t))
	})
	t.Run("zero rejected", func(t *testing.T) {
		t.Setenv("HP_HISTORY_LIMIT", "0")
		_, err := hpsvc.New(mocks.NewMockDAL(t), mocks.NewMockLog(t))
		require.ErrorContains(t, err, "HistoryLimit")
	})
	t.Run("not a number", func(t *testing.T) {
		t.Setenv("HP_HISTORY_LIMIT", "abc")
		_, err := hpsvc.New(mocks.NewMockDAL(t), mocks.NewMockLog(t))
		require.ErrorContains(t, err, "parse env")
	})
	t.Run("nil log", func(t *testing.T) {
		_, err := hpsvc.NewWithConfig(hpsvc.DefaultConfig(), mocks.NewMockDAL(t), nil)
		require.ErrorContains(t, err, "nil log")
	})
}
