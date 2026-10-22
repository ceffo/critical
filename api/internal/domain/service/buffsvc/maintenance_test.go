package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// maintained is a row a power's check put on (ADR 0043 step 13).
func maintained(id, power string, cost, period, start int) buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: id, Name: power, PowerID: power, MaintenanceCost: cost, MaintenancePeriod: period, StartRound: start,
	}
}

// pspIs answers the character read of Next Round's maintenance with current
// PSPs (before the initiative's own read).
func (f *fixture) pspIs(current int) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{
		ID: charID, Str: 10, Derived: charactersvc.Derived{PSP: &charactersvc.PSPPool{Current: current, Max: 50}},
	}, nil).Once()
}

// advances runs one Next Round from round with buffs and returns its entry.
func (f *fixture) advances(t *testing.T, buffs []buffsvc.BuffEffect, round int) (active, expired []buffsvc.BuffEffect, e actionlog.NewEntry) {
	t.Helper()
	f.state(buffs, round)
	f.records()
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()
	f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, round+1).Return(nil).Once()
	f.initiativeIs(nil)
	f.rollsInitiative(5)
	active, expired, _, _, _, err := f.svc.AdvanceRound(context.Background(), charID)
	require.NoError(t, err)
	require.Len(t, f.entries, 2)
	return active, expired, f.entries[0]
}

func TestAdvanceRound_Step14_ChargesEachMaintainedPower(t *testing.T) {
	f := newFixture(t)
	a, b := maintained("a", "ballistic_attack", 1, 1, 2), maintained("b", "levitation", 3, 1, 3)
	f.pspIs(20)

	active, expired, e := f.advances(t, []buffsvc.BuffEffect{a, b}, 4)

	assert.Equal(t, []buffsvc.BuffEffect{a, b}, active, "both paid: both stay")
	assert.Empty(t, expired)
	assert.Equal(t, -4, e.PSPDelta, "1 + 3")
	assert.JSONEq(t, `{"action":"next_round","fromRound":4,"toRound":5,
		"maintenance":[{"powerId":"ballistic_attack","cost":1},{"powerId":"levitation","cost":3}]}`, string(e.Payload))
}

func TestAdvanceRound_Step14_TheStartRoundChargesNothing(t *testing.T) {
	f := newFixture(t)
	a := maintained("a", "levitation", 3, 1, 4)

	active, _, e := f.advances(t, []buffsvc.BuffEffect{a}, 4)

	assert.Equal(t, []buffsvc.BuffEffect{a}, active)
	assert.Zero(t, e.PSPDelta, "Q10: the activation round's own Next Round charges nothing, and reads no PSP")
}

func TestAdvanceRound_Step14_UnpaidMaintenanceEnds(t *testing.T) {
	f := newFixture(t)
	a := maintained("a", "levitation", 3, 1, 1)
	b := maintained("b", "ballistic_attack", 1, 1, 1)
	f.pspIs(2)

	active, expired, e := f.advances(t, []buffsvc.BuffEffect{a, b}, 3)

	assert.Equal(t, []buffsvc.BuffEffect{b}, active, "the 1-PSP power after it still pays")
	assert.Equal(t, []buffsvc.BuffEffect{a}, expired)
	assert.Equal(t, -1, e.PSPDelta, "0 spent on the unpaid power")
	assert.JSONEq(t, `{"action":"next_round","fromRound":3,"toRound":4,"expired":["levitation"],
		"maintenance":[{"powerId":"ballistic_attack","cost":1}],"maintenanceUnpaid":["levitation"]}`, string(e.Payload))
}

func TestAdvanceRound_Step14_PeriodOfTenChargesEveryTenthRound(t *testing.T) {
	row := maintained("a", "clairvoyance", 5, 10, 2)
	for from, due := range map[int]bool{3: false, 11: false, 12: true, 13: false, 21: false, 22: true} {
		f := newFixture(t)
		if due {
			f.pspIs(40)
		}
		_, _, e := f.advances(t, []buffsvc.BuffEffect{row}, from)
		want := 0
		if due {
			want = -5
		}
		assert.Equal(t, want, e.PSPDelta, "Next Round from round %d", from)
	}
}

func TestAdd_Step13_APowerActivatedAgainRenewsItsRow(t *testing.T) {
	f := newFixture(t)
	old := maintained("old", "levitation", 3, 1, 1)
	other := buffsvc.BuffEffect{ID: "x", Name: "Bless"}
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{old, other}, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(6, nil).Once()
	renewed := buffsvc.BuffEffect{ID: "new", Name: "Lévitation", PowerID: "levitation", MaintenanceCost: 3, MaintenancePeriod: 1}
	want := renewed
	want.StartRound = 6
	f.saves([]buffsvc.BuffEffect{other, want})

	got, _, err := f.svc.Add(context.Background(), charID, renewed)
	require.NoError(t, err)
	assert.Equal(t, want, got, "the renewed row starts again")
}

func TestActivate_Step13_ByHandKeepsNoPower(t *testing.T) {
	f := newFixture(t)
	f.state(nil, 0)
	f.records()
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()

	got, err := f.svc.Activate(context.Background(), charID, maintained("", "levitation", 3, 1, 0))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].Maintained(), "only a power's check keeps a power up")
}

func TestDismiss_Step16_StoppingIsFreeAndUndoable(t *testing.T) {
	f := newFixture(t)
	row := maintained("a", "levitation", 3, 1, 1)
	f.state([]buffsvc.BuffEffect{row}, 5)
	f.records()
	f.saves([]buffsvc.BuffEffect{})

	got, _, err := f.svc.Dismiss(context.Background(), charID, "a", "")
	require.NoError(t, err)
	assert.Empty(t, got)
	require.Len(t, f.entries, 1, "one logged entry")
	e := f.entries[0]
	assert.Zero(t, e.PSPDelta, "stopping is free")
	assert.JSONEq(t, `{"action":"dismiss","buff":"levitation","powerId":"levitation"}`, string(e.Payload))
	require.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(e), "undo restores the row")
	before := decodeSnap(t, e.Snapshots[0].Before)
	require.Len(t, before, 1)
	assert.Equal(t, "levitation", before[0]["power_id"], "the row comes back with its power")
	assert.InDelta(t, 3, before[0]["maintenance_cost"], 0, "and its maintenance")
	assert.InDelta(t, 1, before[0]["maintenance_period"], 0)
}
