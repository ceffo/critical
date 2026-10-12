package buffsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func newSvc(t *testing.T) (buffsvc.SVC, *mocks.MockDAL) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	svc, err := buffsvc.NewWithConfig(buffsvc.DefaultConfig(), dal)
	require.NoError(t, err)
	return svc, dal
}

func TestNew_NilDAL(t *testing.T) {
	_, err := buffsvc.New(nil)
	require.Error(t, err)
}

func TestListActive(t *testing.T) {
	svc, dal := newSvc(t)
	buffs := []buffsvc.BuffEffect{{ID: "a", Name: "Enlarge"}}
	dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(buffs, nil)

	got, err := svc.ListActive(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, buffs, got)
}

func TestActivate(t *testing.T) {
	existing := buffsvc.BuffEffect{ID: "old", Name: "Old", Type: "duration", Duration: 3}

	t.Run("assigns a UUID when no ID given", func(t *testing.T) {
		svc, dal := newSvc(t)
		dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return([]buffsvc.BuffEffect{existing}, nil)
		dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), mock.Anything).Return(nil)

		got, err := svc.Activate(context.Background(), 1, buffsvc.BuffEffect{Name: "New", Type: "charge", Charges: 2})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, existing, got[0])
		assert.Len(t, got[1].ID, 36, "uuid string expected")
		assert.Equal(t, "New", got[1].Name)

		saved := dal.Calls[len(dal.Calls)-1].Arguments.Get(2).([]buffsvc.BuffEffect)
		assert.Equal(t, got, saved)
	})

	t.Run("keeps an explicit ID", func(t *testing.T) {
		svc, dal := newSvc(t)
		dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), mock.Anything).Return(nil)

		got, err := svc.Activate(context.Background(), 1, buffsvc.BuffEffect{ID: "mine", Name: "X"})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "mine", got[0].ID)
	})

	t.Run("get error is wrapped and nothing is saved", func(t *testing.T) {
		svc, dal := newSvc(t)
		boom := errors.New("boom")
		dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(nil, boom)

		_, err := svc.Activate(context.Background(), 1, buffsvc.BuffEffect{Name: "X"})
		assert.True(t, errors.Is(err, boom))
	})

	t.Run("save error is wrapped", func(t *testing.T) {
		svc, dal := newSvc(t)
		boom := errors.New("boom")
		dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), mock.Anything).Return(boom)

		_, err := svc.Activate(context.Background(), 1, buffsvc.BuffEffect{Name: "X"})
		assert.True(t, errors.Is(err, boom))
	})
}

func TestDismiss(t *testing.T) {
	a := buffsvc.BuffEffect{ID: "a"}
	b := buffsvc.BuffEffect{ID: "b"}

	tests := []struct {
		name string
		id   string
		want []buffsvc.BuffEffect
	}{
		{name: "removes the matching buff", id: "a", want: []buffsvc.BuffEffect{b}},
		{name: "unknown id is a no-op", id: "zzz", want: []buffsvc.BuffEffect{a, b}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal := newSvc(t)
			dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return([]buffsvc.BuffEffect{a, b}, nil)
			dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), tt.want).Return(nil)

			got, err := svc.Dismiss(context.Background(), 1, tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTrigger(t *testing.T) {
	tests := []struct {
		name  string
		start []buffsvc.BuffEffect
		id    string
		want  []buffsvc.BuffEffect
	}{
		{
			name:  "decrements charges",
			start: []buffsvc.BuffEffect{{ID: "c", Type: "charge", Charges: 3}},
			id:    "c",
			want:  []buffsvc.BuffEffect{{ID: "c", Type: "charge", Charges: 2}},
		},
		{
			name:  "removed when charges reach zero",
			start: []buffsvc.BuffEffect{{ID: "c", Type: "charge", Charges: 1}, {ID: "d", Type: "duration", Duration: 2}},
			id:    "c",
			want:  []buffsvc.BuffEffect{{ID: "d", Type: "duration", Duration: 2}},
		},
		{
			name:  "duration buff is untouched",
			start: []buffsvc.BuffEffect{{ID: "d", Type: "duration", Duration: 2}},
			id:    "d",
			want:  []buffsvc.BuffEffect{{ID: "d", Type: "duration", Duration: 2}},
		},
		{
			name:  "unknown id is a no-op",
			start: []buffsvc.BuffEffect{{ID: "c", Type: "charge", Charges: 3}},
			id:    "nope",
			want:  []buffsvc.BuffEffect{{ID: "c", Type: "charge", Charges: 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal := newSvc(t)
			dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(tt.start, nil)
			dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), tt.want).Return(nil)

			got, err := svc.Trigger(context.Background(), 1, tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRound(t *testing.T) {
	svc, dal := newSvc(t)
	dal.EXPECT().GetRoundNumber(mock.Anything, int64(1)).Return(4, nil)

	got, err := svc.GetRound(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 4, got)
}

func TestAdvanceRound(t *testing.T) {
	t.Run("increments round and ages duration buffs inside one tx", func(t *testing.T) {
		svc, dal := newSvc(t)
		start := []buffsvc.BuffEffect{
			{ID: "d2", Type: "duration", Duration: 2},
			{ID: "d1", Type: "duration", Duration: 1},
			{ID: "c", Type: "charge", Charges: 2},
		}
		wantActive := []buffsvc.BuffEffect{
			{ID: "d2", Type: "duration", Duration: 1},
			{ID: "c", Type: "charge", Charges: 2},
		}
		wantExpired := []buffsvc.BuffEffect{{ID: "d1", Type: "duration", Duration: 0}}

		// The mock tx-bound DAL is the same mock: the body runs against it.
		dal.EXPECT().WrapTx(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, body func(buffsvc.DAL) error) error { return body(dal) })
		dal.EXPECT().GetRoundNumber(mock.Anything, int64(1)).Return(5, nil)
		dal.EXPECT().SaveRoundNumber(mock.Anything, int64(1), 6).Return(nil)
		dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(start, nil)
		dal.EXPECT().SaveActiveBuffs(mock.Anything, int64(1), wantActive).Return(nil)

		active, expired, round, err := svc.AdvanceRound(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, 6, round)
		assert.Equal(t, wantActive, active)
		assert.Equal(t, wantExpired, expired)
	})

	t.Run("tx error is returned and results are zeroed", func(t *testing.T) {
		svc, dal := newSvc(t)
		boom := errors.New("boom")
		dal.EXPECT().WrapTx(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, body func(buffsvc.DAL) error) error { return body(dal) })
		dal.EXPECT().GetRoundNumber(mock.Anything, int64(1)).Return(0, boom)

		active, expired, round, err := svc.AdvanceRound(context.Background(), 1)
		assert.True(t, errors.Is(err, boom))
		assert.Nil(t, active)
		assert.Nil(t, expired)
		assert.Zero(t, round)
	})
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name  string
		buffs []buffsvc.BuffEffect
		str   int
		want  buffsvc.BuffModifiers
	}{
		{
			name: "no buffs gives neutral modifiers",
			str:  10,
			want: buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0},
		},
		{
			name: "multipliers stack multiplicatively, flat bonuses additively",
			buffs: []buffsvc.BuffEffect{
				{Effects: buffsvc.BuffEffects{Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5), WeaponBuff: intPtr(5), SaveBonus: intPtr(1)}},
				{Effects: buffsvc.BuffEffects{Multiplier: floatPtr(1.5), SpellsBuff: floatPtr(2.0), WeaponBuff: intPtr(2), SaveBonus: intPtr(2)}},
			},
			str:  12,
			want: buffsvc.BuffModifiers{SpellsBuff: 3.0, WeaponBuff: 7, Multipliers: 3.0, SaveBonus: 3},
		},
		{
			name: "MinStr gates the buff when STR is too low",
			buffs: []buffsvc.BuffEffect{
				{Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(10), MinStr: intPtr(19)}},
			},
			str:  18,
			want: buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0},
		},
		{
			name: "MinStr met exactly applies",
			buffs: []buffsvc.BuffEffect{
				{Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(10), MinStr: intPtr(18)}},
			},
			str:  18,
			want: buffsvc.BuffModifiers{SpellsBuff: 1.0, WeaponBuff: 10, Multipliers: 1.0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal := newSvc(t)
			dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(tt.buffs, nil)

			got, err := svc.Aggregate(context.Background(), 1, tt.str)
			require.NoError(t, err)
			assert.InDelta(t, tt.want.SpellsBuff, got.SpellsBuff, 1e-9)
			assert.InDelta(t, tt.want.Multipliers, got.Multipliers, 1e-9)
			assert.Equal(t, tt.want.WeaponBuff, got.WeaponBuff)
			assert.Equal(t, tt.want.SaveBonus, got.SaveBonus)
		})
	}
}

func TestAggregate_DALError(t *testing.T) {
	svc, dal := newSvc(t)
	boom := errors.New("boom")
	dal.EXPECT().GetActiveBuffs(mock.Anything, int64(1)).Return(nil, boom)

	_, err := svc.Aggregate(context.Background(), 1, 10)
	assert.True(t, errors.Is(err, boom))
}
