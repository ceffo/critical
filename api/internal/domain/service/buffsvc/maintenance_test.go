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

// maintained is a row a power's check put on (ADR 0043 step 13), kept for
// kept rounds already (0: its activation round).
func maintained(id, power string, cost, period, kept int) buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: id, Name: power, PowerID: power, MaintenanceCost: cost, MaintenancePeriod: period, RoundsKept: kept,
	}
}

// keptMore is b after n more rounds kept.
func keptMore(b buffsvc.BuffEffect, n int) buffsvc.BuffEffect {
	b.RoundsKept += n
	return b
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
	a, b := maintained("a", "ballistic_attack", 1, 1, 2), maintained("b", "levitation", 3, 1, 1)
	f.pspIs(20)

	active, expired, e := f.advances(t, []buffsvc.BuffEffect{a, b}, 4)

	assert.Equal(t, []buffsvc.BuffEffect{keptMore(a, 1), keptMore(b, 1)}, active, "both paid: both stay, one round more kept")
	assert.Empty(t, expired)
	assert.Equal(t, -4, e.PSPDelta, "1 + 3")
	assert.JSONEq(t, `{"action":"next_round","fromRound":4,"toRound":5,
		"maintenance":[{"powerId":"ballistic_attack","cost":1},{"powerId":"levitation","cost":3}]}`, string(e.Payload))
}

func TestAdvanceRound_Step14_TheStartRoundChargesNothing(t *testing.T) {
	f := newFixture(t)
	a := maintained("a", "levitation", 3, 1, 0)

	active, _, e := f.advances(t, []buffsvc.BuffEffect{a}, 4)

	assert.Equal(t, []buffsvc.BuffEffect{keptMore(a, 1)}, active)
	assert.Zero(t, e.PSPDelta, "Q10: the activation round's own Next Round charges nothing, and reads no PSP")
}

func TestAdvanceRound_Step14_UnpaidMaintenanceEnds(t *testing.T) {
	f := newFixture(t)
	a := maintained("a", "levitation", 3, 1, 2)
	b := maintained("b", "ballistic_attack", 1, 1, 2)
	f.pspIs(2)

	active, expired, e := f.advances(t, []buffsvc.BuffEffect{a, b}, 3)

	assert.Equal(t, []buffsvc.BuffEffect{keptMore(b, 1)}, active, "the 1-PSP power after it still pays")
	assert.Equal(t, []buffsvc.BuffEffect{a}, expired)
	assert.Equal(t, -1, e.PSPDelta, "0 spent on the unpaid power")
	assert.JSONEq(t, `{"action":"next_round","fromRound":3,"toRound":4,"expired":["levitation"],
		"maintenance":[{"powerId":"ballistic_attack","cost":1}],"maintenanceUnpaid":["levitation"]}`, string(e.Payload))
}

func TestAdvanceRound_Step14_PeriodOfTenChargesEveryTenthRound(t *testing.T) {
	for kept, due := range map[int]bool{0: false, 1: false, 9: false, 10: true, 11: false, 19: false, 20: true} {
		f := newFixture(t)
		if due {
			f.pspIs(40)
		}
		_, _, e := f.advances(t, []buffsvc.BuffEffect{maintained("a", "clairvoyance", 5, 10, kept)}, 3)
		want := 0
		if due {
			want = -5
		}
		assert.Equal(t, want, e.PSPDelta, "Next Round after %d rounds kept", kept)
	}
}

func TestAdd_Step13_APowerActivatedAgainRenewsItsRow(t *testing.T) {
	f := newFixture(t)
	old := maintained("old", "levitation", 3, 1, 7)
	other := buffsvc.BuffEffect{ID: "x", Name: "Bless"}
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{old, other}, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(6, nil).Once()
	renewed := buffsvc.BuffEffect{ID: "new", Name: "Lévitation", PowerID: "levitation", MaintenanceCost: 3, MaintenancePeriod: 1, RoundsKept: 4}
	want := renewed
	want.StartRound = 6
	want.RoundsKept = 0 // its count starts again: the next charge is a full period away
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

// passes runs one PassTime of hours from round with buffs and returns its
// entry.
func (f *fixture) passes(t *testing.T, buffs []buffsvc.BuffEffect, round int, hours float64) (active, expired []buffsvc.BuffEffect, e actionlog.NewEntry) {
	t.Helper()
	f.state(buffs, round)
	f.records()
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()
	f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
	f.initiativeIs(nil)
	active, expired, _, _, _, err := f.svc.PassTime(context.Background(), charID, hours)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	return active, expired, f.entries[0]
}

// ADR 0043 step 15: 1 h (60 rounds, the Next Rounds ending rounds 0 to 59)
// of a 1-PSP power started in round 0 with 40 PSPs pays rounds 1 to 40 and
// ends at the 41st charge.
func TestPassTime_Step15_ChargesMaintenanceForTheRounds(t *testing.T) {
	f := newFixture(t)
	a := maintained("a", "ballistic_attack", 1, 1, 0)
	f.pspIs(40)

	active, expired, e := f.passes(t, []buffsvc.BuffEffect{a}, 0, 1)

	assert.Empty(t, active)
	assert.Equal(t, []buffsvc.BuffEffect{a}, expired)
	assert.Equal(t, -40, e.PSPDelta)
	assert.JSONEq(t, `{"action":"pass_time","hours":1,"rounds":60,"fromRound":0,"toRound":0,
		"expired":["ballistic_attack"],
		"maintenance":[{"powerId":"ballistic_attack","cost":40}],"maintenanceUnpaid":["ballistic_attack"]}`, string(e.Payload))
}

// The charges go round by round, not row by row: a 5-PSP power every 10
// rounds listed first is due only in round 10, after nine 1-PSP charges of
// the power after it; with 12 PSPs it cannot pay then and ends, the 1-PSP
// power pays rounds 1 to 12 and ends at round 13.
func TestPassTime_Step15_InRoundOrder(t *testing.T) {
	f := newFixture(t)
	b, a := maintained("b", "levitation", 5, 10, 0), maintained("a", "ballistic_attack", 1, 1, 0)
	f.pspIs(12)

	active, expired, e := f.passes(t, []buffsvc.BuffEffect{b, a}, 0, 20.0/60)

	assert.Empty(t, active)
	assert.Equal(t, []buffsvc.BuffEffect{b, a}, expired)
	assert.Equal(t, -12, e.PSPDelta)
	assert.Contains(t, string(e.Payload), `"maintenance":[{"powerId":"ballistic_attack","cost":12}],"maintenanceUnpaid":["levitation","ballistic_attack"]`)
}

// Enough PSPs: every period due is paid and the rows stay; the activation
// round's own Next Round charges nothing (Q10).
func TestPassTime_Step15_PaidPowersStay(t *testing.T) {
	f := newFixture(t)
	b, a := maintained("b", "levitation", 5, 10, 0), maintained("a", "ballistic_attack", 1, 1, 0)
	f.pspIs(100)

	// 30 rounds from the activation round: a is due after 1 to 29 rounds
	// kept (29), b after 10 and 20 (10).
	active, expired, e := f.passes(t, []buffsvc.BuffEffect{b, a}, 3, 0.5)

	assert.Equal(t, []buffsvc.BuffEffect{keptMore(b, 30), keptMore(a, 30)}, active)
	assert.Empty(t, expired)
	assert.Equal(t, -39, e.PSPDelta)
	assert.Contains(t, string(e.Payload), `"maintenance":[{"powerId":"levitation","cost":10},{"powerId":"ballistic_attack","cost":29}]`)
}

// The preview runs the same charges and writes nothing: the PSPs spent, and
// the unpaid power among the buffs that would expire.
func TestPassTime_Step15_PreviewShowsThePSP(t *testing.T) {
	f := newFixture(t) // mocks fail on any Record or Save
	a := maintained("a", "ballistic_attack", 1, 1, 0)
	timed := buffsvc.BuffEffect{ID: "t", Name: "Bless", Duration: 6}
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{timed, a}, nil).Once()
	f.pspIs(40)

	rounds, expiring, spent, err := f.svc.PreviewPassTime(context.Background(), charID, 1)
	require.NoError(t, err)
	assert.Equal(t, 60, rounds)
	assert.Equal(t, 40, spent)
	assert.Equal(t, []string{"t", "a"}, ids(expiring))
	assert.Zero(t, f.tx.calls)
}

// ADR 0043 step 14: End combat (or a Set round to another round) closes a
// maintained row's activation round, charging nothing, so the next Next
// Round charges it; a row already past its activation round is untouched and
// the buff list is not written then. The count lives on the row: the round
// counter going back to 0 changes nothing else.
func TestMoveRound_Step14_ClosesTheActivationRound(t *testing.T) {
	ctx := context.Background()
	fresh, older := maintained("a", "aura_sight", 9, 1, 0), maintained("b", "levitation", 3, 1, 4)

	t.Run("End combat: the fresh row counts one round kept", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{fresh, older}, 5)
		f.records()
		f.saves([]buffsvc.BuffEffect{keptMore(fresh, 1), older})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)

		_, round, err := f.svc.ResetRound(ctx, charID)
		require.NoError(t, err)
		assert.Zero(t, round)
		require.Len(t, f.entries, 1)
		assert.Zero(t, f.entries[0].PSPDelta, "closing the activation round is free")
		assert.ElementsMatch(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound}, snapKeys(f.entries[0]), "undo reopens it")
	})

	t.Run("no fresh row: round only", func(t *testing.T) {
		f := newFixture(t) // SaveActiveBuffs unexpected
		f.state([]buffsvc.BuffEffect{older}, 5)
		f.records()
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)

		_, _, err := f.svc.ResetRound(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, []string{buffsvc.StateKeyRound}, snapKeys(f.entries[0]))
	})

	t.Run("Set round to the same round closes nothing", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{fresh}, 5)
		f.records()
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 5).Return(nil).Once()

		_, _, err := f.svc.SetRound(ctx, charID, 5)
		require.NoError(t, err)
		assert.Equal(t, []string{buffsvc.StateKeyRound}, snapKeys(f.entries[0]))
	})
}
