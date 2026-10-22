package psionicsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

var rowSnap = actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`[]`), After: json.RawMessage(`[{}]`)}

// ADR 0043 step 13: Lift (5 PSPs, 3 a round) succeeds: an untimed row with its
// power id and maintenance goes on in the check's transaction, its snapshot in
// the check's entry so the undo takes it back with the PSPs.
func TestActivate_Step13_AMaintainedPowerIsAnUntimedRow(t *testing.T) {
	svc, dice, log, buffs := newUseSvcWithBuffs(t, user(30, false))
	dice.EXPECT().RollDie(psionics.CheckDie).Return(3).Once()
	var row buffsvc.BuffEffect
	buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			row = b
			b.ID = "row-1"
			return b, rowSnap, nil
		}).Once()
	var got actionlog.NewEntry
	expectRecord(log, &got)

	res, err := svc.Use(context.Background(), 7, "lift", psionicsvc.UseRequest{})
	require.NoError(t, err)
	assert.Equal(t, buffsvc.BuffEffect{Name: "Lift", PowerID: "lift", MaintenanceCost: 3, MaintenancePeriod: 1}, row)
	assert.False(t, row.Timed(), "until stopped")
	assert.Equal(t, "row-1", res.BuffID)
	assert.Equal(t, -5, got.PSPDelta)
	assert.Equal(t, []actionlog.Snapshot{rowSnap}, got.Snapshots)
	rec := payloadOf(t, got)
	assert.Equal(t, 3, rec.MaintenanceCost)
	assert.Equal(t, 1, rec.MaintenancePeriod)
}

func TestActivate_Step13_NoMaintenanceNoRow(t *testing.T) {
	t.Run("a power without maintenance", func(t *testing.T) {
		svc, dice, log, _ := newUseSvcWithBuffs(t, user(30, false))
		dice.EXPECT().RollDie(psionics.CheckDie).Return(3).Once()
		var got actionlog.NewEntry
		expectRecord(log, &got)

		res, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{})
		require.NoError(t, err)
		assert.Empty(t, res.BuffID)
		assert.Empty(t, got.Snapshots)
	})
	t.Run("a failed check of a maintained power", func(t *testing.T) {
		svc, dice, log, _ := newUseSvcWithBuffs(t, user(30, false))
		dice.EXPECT().RollDie(psionics.CheckDie).Return(19).Once()
		var got actionlog.NewEntry
		expectRecord(log, &got)

		res, err := svc.Use(context.Background(), 7, "lift", psionicsvc.UseRequest{})
		require.NoError(t, err)
		assert.False(t, res.Check.Success)
		assert.Empty(t, res.BuffID, "the buffs mock refuses any Add")
		assert.Equal(t, -3, got.PSPDelta, "half of 5, rounded up")
	})
}

func TestActivate_Step13_AVariableMaintenanceIsEntered(t *testing.T) {
	t.Run("refused without it, before the roll", func(t *testing.T) {
		svc, _, _ := newUseSvc(t, user(30, false))
		_, err := svc.Use(context.Background(), 7, "push", psionicsvc.UseRequest{})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "maintenanceCost", fe.Field)
	})
	t.Run("refused for a power with a fixed one", func(t *testing.T) {
		svc, _, _ := newUseSvc(t, user(30, false))
		_, err := svc.Use(context.Background(), 7, "lift", psionicsvc.UseRequest{MaintenanceCost: intPtr(2)})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "maintenanceCost", fe.Field)
	})
	t.Run("the row carries it", func(t *testing.T) {
		svc, dice, log, buffs := newUseSvcWithBuffs(t, user(30, false))
		dice.EXPECT().RollDie(psionics.CheckDie).Return(2).Once()
		buffs.EXPECT().Add(mock.Anything, int64(7), mock.MatchedBy(func(b buffsvc.BuffEffect) bool {
			return b.PowerID == "push" && b.MaintenanceCost == 4
		})).Return(buffsvc.BuffEffect{ID: "r"}, rowSnap, nil).Once()
		var got actionlog.NewEntry
		expectRecord(log, &got)

		_, err := svc.Use(context.Background(), 7, "push", psionicsvc.UseRequest{MaintenanceCost: intPtr(4)})
		require.NoError(t, err)
	})
}

// ADR 0043 step 16: Stop dismisses the power's row (buffsvc logs it, free and
// undoable: TestDismiss_Step16_StoppingIsFreeAndUndoable).
func TestStop_Step16_DismissesThePowersRow(t *testing.T) {
	other := buffsvc.BuffEffect{ID: "x", Name: "Bless"}
	lift := buffsvc.BuffEffect{ID: "r", Name: "Lift", PowerID: "lift", MaintenanceCost: 3, MaintenancePeriod: 1}

	t.Run("the row is dismissed", func(t *testing.T) {
		svc, _, _, buffs := newUseSvcWithBuffs(t, user(30, false))
		buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{other, lift}, nil).Once()
		buffs.EXPECT().Dismiss(mock.Anything, int64(7), "r", mock.Anything).
			Return([]buffsvc.BuffEffect{other}, buffsvc.EndResult{}, nil).Once()

		got, err := svc.Stop(context.Background(), 7, "lift")
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.BuffEffect{other}, got)
	})
	t.Run("a power not maintained is refused", func(t *testing.T) {
		svc, _, _, buffs := newUseSvcWithBuffs(t, user(30, false))
		buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{other}, nil).Once()

		_, err := svc.Stop(context.Background(), 7, "lift")
		assert.ErrorIs(t, err, psionicsvc.ErrPowerNotMaintained)
	})
	t.Run("a character with no psionics is refused", func(t *testing.T) {
		c := user(30, false)
		c.Derived.Powers = nil
		svc, _, _ := newUseSvc(t, c)

		_, err := svc.Stop(context.Background(), 7, "lift")
		assert.ErrorIs(t, err, psionicsvc.ErrNotPsionicist)
	})
}

func intPtr(v int) *int { return &v }
