package actionlogsvc_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
)

// stubState stubs the GetHPState reads with fixed max HP and log total and
// the given history.
func stubState(dal *mocks.MockDAL, maxHP *mocks.MockMaxHP, max, total int, history []actionlog.Entry) {
	maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(max, nil)
	dal.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(total, nil)
	dal.EXPECT().ListActiveEntries(mock.Anything, mock.Anything, actionlog.KindHPChange, mock.Anything).Return(history, nil)
}

func TestGetState(t *testing.T) {
	at := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)
	minus12 := -12
	svc, dal, _, _, maxHP := newSvcMaxHP(t)
	stubState(dal, maxHP, 20, -5, []actionlog.Entry{
		{ID: 2, CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 3, CreatedAt: at},
		{
			ID: 1, CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, CreatedAt: at,
			Corrections: []actionlog.Correction{{Kind: actionlog.CorrectionAmend, HPDelta: &minus12, Summary: "orc axe"}},
		},
	})

	state, err := svc.GetHPState(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 15, state.CurrentHp)
	assert.Equal(t, 20, state.MaxHp)
	assert.Equal(t, []actionlog.HPEvent{
		{ID: 2, CharacterID: 1, Delta: 3, Source: "potion", CreatedAt: at},
		{ID: 1, CharacterID: 1, Delta: -12, Source: "orc axe", CreatedAt: at},
	}, state.Events)
}

func TestGetState_Errors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(dal *mocks.MockDAL, maxHP *mocks.MockMaxHP)
		want  error
	}{
		{"character missing", func(_ *mocks.MockDAL, maxHP *mocks.MockMaxHP) {
			maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(0, domain.ErrCharacterNotFound)
		}, domain.ErrCharacterNotFound},
		{"total fails", func(dal *mocks.MockDAL, maxHP *mocks.MockMaxHP) {
			maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(20, nil)
			dal.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, errBoom)
		}, errBoom},
		{"history fails", func(dal *mocks.MockDAL, maxHP *mocks.MockMaxHP) {
			maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(20, nil)
			dal.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, nil)
			dal.EXPECT().ListActiveEntries(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errBoom)
		}, errBoom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, _, maxHP := newSvcMaxHP(t)
			c.setup(dal, maxHP)
			_, err := svc.GetHPState(context.Background(), 1)
			require.ErrorIs(t, err, c.want)
		})
	}
}

// A total above 0 (rows logged before the cap) is never shown above max
// (ADR 0007 step 6).
func TestGetState_NeverAboveMax(t *testing.T) {
	for _, c := range []struct{ total, want int }{{-1, 19}, {0, 20}, {7, 20}} {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		stubState(dal, maxHP, 20, c.total, nil)
		state, err := svc.GetHPState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.CurrentHp, "total %d", c.total)
		assert.Equal(t, 20, state.MaxHp)
	}
}

// ADR 0011 step 1 (display guard): rows logged before the floor existed
// never show HP below -10; the state carries the floor.
func TestGetState_NeverBelowFloor(t *testing.T) {
	for _, c := range []struct{ total, want int }{{-29, -9}, {-30, -10}, {-45, -10}} {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		stubState(dal, maxHP, 20, c.total, nil)
		state, err := svc.GetHPState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.CurrentHp, "total %d", c.total)
		assert.Equal(t, -10, state.FloorHp)
	}
}

// ADR 0011 step 2: GetHPState evaluates the condition on the HP it returns
// (after the floor and max guards).
func TestGetState_Condition(t *testing.T) {
	for _, c := range []struct {
		total int
		want  actionlog.Condition
	}{{0, actionlog.ConditionOK}, {-15, actionlog.ConditionBadlyWounded}, {-20, actionlog.ConditionUnconscious}, {-30, actionlog.ConditionDead}, {-41, actionlog.ConditionDead}} {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		stubState(dal, maxHP, 20, c.total, nil)
		state, err := svc.GetHPState(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, c.want, state.Condition, "total %d", c.total)
	}
}

func TestAppendEvent(t *testing.T) {
	cases := []struct {
		name        string
		delta       int
		source      string
		wantSummary string
		wantPayload string // JSON; "" = none stored
	}{
		{"no source", -5, "", "", `{}`},
		{"blank source", 4, "   ", "", `{"hpRequested":4,"hpApplied":4}`},
		{"damage", -5, " sword ", "sword", `{}`},
		{"heal", 4, "potion", "potion", `{"hpRequested":4,"hpApplied":4}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, rounds, _, maxHP := newSvcMaxHP(t)
			// From 10/20: the log total is -10 before the entry, delta-10 after.
			maxHP.EXPECT().GetMaxHP(mock.Anything, int64(7)).Return(20, nil)
			rounds.EXPECT().GetRound(mock.Anything, int64(7)).Return(1, nil)
			dal.EXPECT().HPTotal(mock.Anything, int64(7)).Return(-10, nil).Once()
			var stored actionlog.NewEntry
			dal.EXPECT().InsertEntry(mock.Anything, mock.Anything, int64(0)).
				Run(func(_ context.Context, e actionlog.NewEntry, _ int64) { stored = e }).Return(1, nil).Once()
			dal.EXPECT().HPTotal(mock.Anything, int64(7)).Return(c.delta-10, nil).Once()
			dal.EXPECT().ListActiveEntries(mock.Anything, int64(7), actionlog.KindHPChange, mock.Anything).Return(nil, nil)

			state, err := svc.AppendHPEvent(context.Background(), 7, c.delta, c.source)
			require.NoError(t, err)
			assert.Equal(t, 10+c.delta, state.CurrentHp)
			assert.Equal(t, actionlog.KindHPChange, stored.Kind)
			assert.Equal(t, c.delta, stored.HPDelta)
			assert.Equal(t, c.wantSummary, stored.Summary)
			assert.JSONEq(t, c.wantPayload, string(stored.Payload))
		})
	}
}

func TestAppendEvent_RecordFails(t *testing.T) {
	svc, dal, rounds, _, maxHP := newSvcMaxHP(t)
	rounds.EXPECT().GetRound(mock.Anything, mock.Anything).Return(1, nil)
	maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(20, nil)
	dal.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, nil)
	dal.EXPECT().InsertEntry(mock.Anything, mock.Anything, int64(0)).Return(0, errBoom)
	_, err := svc.AppendHPEvent(context.Background(), 7, -5, "sword")
	require.ErrorIs(t, err, errBoom)
}

// historyLimitSeen builds the service via New (env-driven) and returns the
// limit it passes to the DAL's ListActiveEntries.
func historyLimitSeen(t *testing.T) int {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	maxHP := mocks.NewMockMaxHP(t)
	maxHP.EXPECT().GetMaxHP(mock.Anything, mock.Anything).Return(10, nil)
	dal.EXPECT().HPTotal(mock.Anything, mock.Anything).Return(0, nil)
	var got int
	dal.EXPECT().ListActiveEntries(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ int64, _ actionlog.Kind, limit int) { got = limit }).Return(nil, nil)
	svc, err := actionlogsvc.New(dal, &passTx{}, mocks.NewMockRounds(t), maxHP)
	require.NoError(t, err)
	_, err = svc.GetHPState(context.Background(), 1)
	require.NoError(t, err)
	return got
}

func TestNew_HistoryLimitEnv(t *testing.T) {
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
		_, err := actionlogsvc.New(mocks.NewMockDAL(t), &passTx{}, mocks.NewMockRounds(t), mocks.NewMockMaxHP(t))
		require.ErrorContains(t, err, "HistoryLimit")
	})
}
