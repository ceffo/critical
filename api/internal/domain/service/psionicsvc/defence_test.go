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
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// defenceBook: thought shield (Wis -3, 1 PSP, not kept), mind blank (Wis
// -7, 0 PSPs, kept at 0: a standing defence), ego whip (an attack mode) and a
// devotion.
func defenceBook() psionics.Book {
	return psionics.NewBook([]catalogue.Discipline{{ID: "telepathy"}}, []catalogue.Power{
		{
			ID: "thought_shield", Kind: catalogue.KindDefenceMode, Name: map[string]string{"en": "Thought Shield"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -3}, InitialCost: catalogue.Cost{Amount: 1},
		},
		{
			ID: "mind_blank", Kind: catalogue.KindDefenceMode, Name: map[string]string{"en": "Mind Blank"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -7}, InitialCost: catalogue.Cost{Amount: 0},
			Maintenance: &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 0}, Period: 1},
		},
		{
			ID: "ego_whip", Kind: catalogue.KindAttackMode, Name: map[string]string{"en": "Ego Whip"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -3}, InitialCost: catalogue.Cost{Amount: 4},
		},
		{
			ID: "thrust", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			Score: &catalogue.PowerScore{Ability: "int", Modifier: -4}, InitialCost: catalogue.Cost{Amount: 7},
		},
	})
}

// defender is a psionicist with Wis 16 (thought shield at 13, mind blank at
// 9) who knows the book's powers, with current PSPs of a 30 maximum.
func defender(current int) *charactersvc.Character {
	c := psionicist(
		psionics.Pick{Kind: psionics.PickDiscipline, ID: "telepathy"},
		psionics.Pick{Kind: psionics.PickPower, ID: "thought_shield"},
		psionics.Pick{Kind: psionics.PickPower, ID: "mind_blank"},
		psionics.Pick{Kind: psionics.PickPower, ID: "ego_whip"},
		psionics.Pick{Kind: psionics.PickPower, ID: "thrust"},
	)
	c.Int, c.Wis = 18, 16
	c.Derived.PSP = &charactersvc.PSPPool{Current: current, Max: 30}
	return c
}

type defenceMocks struct {
	dice   *mocks.MockDice
	log    *mocks.MockLog
	buffs  *mocks.MockBuffs
	tables *mocks.MockTables
}

// newDefenceSvc is the service at round with the character c; its dice mock
// fails on any roll a test does not expect.
func newDefenceSvc(t *testing.T, c *charactersvc.Character, round int) (psionicsvc.SVC, defenceMocks) {
	t.Helper()
	chars := mocks.NewMockCharacters(t)
	m := defenceMocks{dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), buffs: mocks.NewMockBuffs(t), tables: mocks.NewMockTables(t)}
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil).Maybe()
	m.buffs.EXPECT().GetRound(mock.Anything, int64(7)).Return(round, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Tables: m.tables, Log: m.log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: m.dice, Buffs: m.buffs, Book: defenceBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, m
}

// expectAdd captures the row Add puts on.
func expectAdd(m defenceMocks, got *buffsvc.BuffEffect) {
	m.buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			*got = b
			b.ID = "row-1"
			return b, rowSnap, nil
		}).Once()
}

// ADR 0044 step 4: raising thought shield pays its 1 PSP with no roll (the
// dice mock is never called): a defence_raised entry with the PSP delta and
// the round's raised row in its snapshot.
func TestRaiseDefence_Step4_PaysTheCostNoRoll(t *testing.T) {
	svc, m := newDefenceSvc(t, defender(10), 3)
	var row buffsvc.BuffEffect
	expectAdd(m, &row)
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	res, err := svc.RaiseDefence(context.Background(), 7, "thought_shield")
	require.NoError(t, err)
	assert.Equal(t, buffsvc.DefenceRaised, row.Defence)
	assert.Equal(t, 1, row.Duration, "the round it is raised in")
	assert.Equal(t, "thought_shield", row.PowerID)
	assert.Zero(t, row.MaintenanceCost, "a raised mode is paid again, never kept")
	assert.Equal(t, actionlog.KindDefenceRaised, got.Kind)
	assert.Equal(t, -1, got.PSPDelta)
	assert.Equal(t, []actionlog.Snapshot{rowSnap}, got.Snapshots)
	var rec psionicsvc.RaiseRecord
	require.NoError(t, json.Unmarshal(got.Payload, &rec))
	assert.Equal(t, psionicsvc.RaiseRecord{Power: "thought_shield", Name: map[string]string{"en": "Thought Shield"}, Defence: buffsvc.DefenceRaised, Cost: 1}, rec)
	assert.Equal(t, rec, res.Raise)
	assert.Equal(t, actionlog.PSPState{Current: 9, Max: 30}, res.PSP)
	assert.Equal(t, "row-1", res.BuffID)
	m.dice.AssertNotCalled(t, "RollDie", mock.Anything)
}

// ADR 0044 step 4 (Q3): a raised mode is refused at round 0; a standing one
// (step 5, Q4) is raised in or out of combat, untimed.
func TestRaiseDefence_Step4_RefusedOutOfCombat(t *testing.T) {
	t.Run("a raised mode", func(t *testing.T) {
		svc, _ := newDefenceSvc(t, defender(10), 0)
		_, err := svc.RaiseDefence(context.Background(), 7, "thought_shield")
		require.ErrorIs(t, err, psionicsvc.ErrNotInCombat)
	})
	t.Run("a standing mode is not refused", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(0), 0)
		var row buffsvc.BuffEffect
		expectAdd(m, &row)
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.RaiseDefence(context.Background(), 7, "mind_blank")
		require.NoError(t, err)
		assert.Equal(t, buffsvc.DefenceStanding, row.Defence)
		assert.False(t, row.Timed(), "until dismissed")
		assert.False(t, row.Maintained(), "kept at no cost")
		assert.Equal(t, buffsvc.DefenceStanding, res.Raise.Defence)
		assert.Zero(t, got.PSPDelta)
	})
}

// ADR 0044 step 4: the other refusals, before anything is written.
func TestRaiseDefence_Step4_Refusals(t *testing.T) {
	for _, c := range []struct {
		name, power string
		psp         int
		err         error
	}{
		{"under the cost", "thought_shield", 0, psionicsvc.ErrNotEnoughPSP},
		{"not a defence mode", "thrust", 30, psionicsvc.ErrNotADefenceMode},
		{"an attack mode", "ego_whip", 30, psionicsvc.ErrNotADefenceMode},
		{"not known", "no_such_power", 30, psionicsvc.ErrPowerNotKnown},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newDefenceSvc(t, defender(c.psp), 2)
			_, err := svc.RaiseDefence(context.Background(), 7, c.power)
			require.ErrorIs(t, err, c.err)
		})
	}
}

// ADR 0044 step 4 amends ADR 0043 step 10 for the kind: a defence mode is
// raised, never used.
func TestUse_Step4_ADefenceModeIsRaised(t *testing.T) {
	svc, _ := newDefenceSvc(t, defender(30), 2)
	_, err := svc.Use(context.Background(), 7, "thought_shield", psionicsvc.UseRequest{})
	require.ErrorIs(t, err, psionicsvc.ErrDefenceModeIsRaised)
}

// ADR 0044 step 5: a standing row (a defence kept at no cost) does not
// refuse PSP recovery.
func TestRecover_Step5_AZeroCostRowAllowsRecovery(t *testing.T) {
	svc, m := newDefenceSvc(t, defender(10), 0)
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{
		{ID: "s", PowerID: "mind_blank", Defence: buffsvc.DefenceStanding},
	}, nil).Once()
	m.tables.EXPECT().PSPRecoveryRowFor("resting").Return(ruletablesvc.PSPRecoveryRow{Activity: "resting", PerHour: 3}, true, nil).Once()
	m.log.EXPECT().AppendPSP(mock.Anything, mock.Anything).Return(actionlog.PSPState{Current: 16, Max: 30}, nil).Once()
	_, err := svc.Recover(context.Background(), 7, 2, "resting")
	require.NoError(t, err)
}

// defendLog sets the log's reads of a contest: the round started at entry
// 5, the given defence_raised and psionic_defence entries, newest first.
func defendLog(m defenceMocks, raises, defences []actionlog.Entry) {
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), actionlog.KindRoundAdvance, 1).
		Return([]actionlog.Entry{{ID: 5, Round: 3, Kind: actionlog.KindRoundAdvance}}, nil).Maybe()
	for _, k := range []actionlog.Kind{actionlog.KindRoundSet, actionlog.KindRoundReset, actionlog.KindTimePassed} {
		m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), k, 1).Return(nil, nil).Maybe()
	}
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), actionlog.KindDefenceRaised, mock.Anything).Return(raises, nil).Maybe()
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), actionlog.KindPsionicDefence, mock.Anything).Return(defences, nil).Maybe()
}

func raised(id int64, power string) actionlog.Entry {
	return actionlog.Entry{ID: id, Round: 3, Kind: actionlog.KindDefenceRaised, Payload: []byte(`{"power":"` + power + `"}`)}
}

func defenceOf(t *testing.T, e actionlog.NewEntry) psionicsvc.DefenceRecord {
	t.Helper()
	var rec psionicsvc.DefenceRecord
	require.NoError(t, json.Unmarshal(e.Payload, &rec))
	return rec
}

var (
	shieldRaised = buffsvc.BuffEffect{ID: "r", PowerID: "thought_shield", Defence: buffsvc.DefenceRaised, Duration: 1}
	blankRow     = buffsvc.BuffEffect{ID: "s", PowerID: "mind_blank", Defence: buffsvc.DefenceStanding}
)

// ADR 0044 step 8: ego whip's two rolls against thought shield (13): a
// check each, two dice, each contest decided by step 6; one psionic_defence
// entry with no PSP delta; Table 14's cell beside the DM's roll.
func TestDefend_Step8_AContestPerIncomingRoll(t *testing.T) {
	svc, m := newDefenceSvc(t, defender(10), 3)
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{shieldRaised}, nil).Once()
	m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
	defendLog(m, []actionlog.Entry{raised(6, "thought_shield")}, nil)
	m.dice.EXPECT().RollDie(psionics.CheckDie).Return(10).Once()
	m.dice.EXPECT().RollDie(psionics.CheckDie).Return(13).Once()
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	res, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{
		Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 12, Success: true}, {Roll: 9, Success: true}},
	})
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindPsionicDefence, got.Kind)
	assert.Zero(t, got.PSPDelta, "the raise paid")
	rec := defenceOf(t, got)
	assert.Equal(t, rec, res.Defence)
	assert.Equal(t, "thought_shield", rec.Defence)
	assert.Equal(t, 13, rec.Score)
	assert.False(t, rec.Kept)
	require.NotNil(t, rec.Modifier)
	assert.Equal(t, -4, *rec.Modifier)
	require.Len(t, rec.Contests, 2)
	assert.Equal(t, psionics.SideAttacker, rec.Contests[0].Winner, "12 against 10")
	assert.Equal(t, psionics.SideDefender, rec.Contests[1].Winner, "9 against 13")
	assert.Equal(t, 13, rec.Contests[1].Check.Roll)
	assert.Nil(t, rec.KeptCheck)
}

// ADR 0044 step 8: a kept check is taken by the round's next contests of
// the same defence with no die: in the request that keeps it, and in the
// round's later requests (read from the round's earlier psionic_defence
// entry); a check kept before the round started does not count.
func TestDefend_Step8_AKeptRollForTheRound(t *testing.T) {
	kept := psionics.CheckContest(13, 4, false)
	prev := func(id int64, round int) actionlog.Entry {
		b, err := json.Marshal(psionicsvc.DefenceRecord{Attack: "ego_whip", Defence: "thought_shield", KeptCheck: &kept})
		require.NoError(t, err)
		return actionlog.Entry{ID: id, Round: round, Kind: actionlog.KindPsionicDefence, Payload: b}
	}
	whip := []psionics.ContestSide{{Roll: 6, Success: true}}

	t.Run("kept in the request: the second contest rolls no die", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(10), 3)
		m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{shieldRaised}, nil).Once()
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
		defendLog(m, []actionlog.Entry{raised(6, "thought_shield")}, nil)
		m.dice.EXPECT().RollDie(psionics.CheckDie).Return(4).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		keep := 0
		_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{
			Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 6, Success: true}, {Roll: 3, Success: true}}, Keep: &keep,
		})
		require.NoError(t, err)
		rec := defenceOf(t, got)
		require.NotNil(t, rec.KeptCheck)
		assert.Equal(t, kept, *rec.KeptCheck)
		assert.False(t, rec.Contests[0].FromKept)
		assert.True(t, rec.Contests[1].FromKept)
		assert.Equal(t, psionics.SideAttacker, rec.Contests[0].Winner, "6 against 4")
	})
	t.Run("the round's third contest", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(10), 3)
		m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{shieldRaised}, nil).Once()
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
		defendLog(m, []actionlog.Entry{raised(6, "thought_shield")}, []actionlog.Entry{prev(8, 3)})
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: whip})
		require.NoError(t, err)
		rec := defenceOf(t, got)
		require.Len(t, rec.Contests, 1)
		assert.True(t, rec.Contests[0].FromKept, "no die")
		assert.Equal(t, kept, *rec.Contests[0].Check)
		assert.Equal(t, &kept, rec.KeptCheck, "still kept for the round")
		m.dice.AssertNotCalled(t, "RollDie", mock.Anything)
	})
	t.Run("a check kept in the round before rolls again", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(10), 3)
		m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{shieldRaised}, nil).Once()
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
		defendLog(m, []actionlog.Entry{raised(6, "thought_shield")}, []actionlog.Entry{prev(4, 2)})
		m.dice.EXPECT().RollDie(psionics.CheckDie).Return(9).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: whip})
		require.NoError(t, err)
		assert.False(t, defenceOf(t, got).Contests[0].FromKept)
	})
	t.Run("keeping again in the round is refused", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(10), 3)
		m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{shieldRaised}, nil).Once()
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
		defendLog(m, []actionlog.Entry{raised(6, "thought_shield")}, []actionlog.Entry{prev(8, 3)})
		keep := 0
		_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: whip, Keep: &keep})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "keep", fe.Field)
	})
}

// ADR 0044 steps 5, 6 and 8: mind blank (9) raised in an earlier round is
// kept: +1, and a failed roll counts as a success of 1; raised this round it
// is not.
func TestDefend_Step8_TheStandingDefencePlusOne(t *testing.T) {
	for _, c := range []struct {
		name     string
		raisedAt int64
		kept     bool
		winner   string
	}{
		{"raised in an earlier round", 3, true, psionics.SideAttacker},
		{"raised this round", 6, false, psionics.SideAttacker},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, m := newDefenceSvc(t, defender(10), 3)
			m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{blankRow}, nil).Once()
			m.tables.EXPECT().AttackVersusDefence("ego_whip", "mind_blank").Return(-7, nil).Once()
			defendLog(m, []actionlog.Entry{raised(c.raisedAt, "mind_blank")}, nil)
			m.dice.EXPECT().RollDie(psionics.CheckDie).Return(15).Once()
			var got actionlog.NewEntry
			expectRecord(m.log, &got)
			_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{
				Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 2, Success: true}},
			})
			require.NoError(t, err)
			rec := defenceOf(t, got)
			assert.Equal(t, c.kept, rec.Kept)
			assert.Equal(t, 9, rec.Score)
			ch := rec.Contests[0].Check
			assert.Equal(t, psionics.CheckContest(9, 15, c.kept), *ch)
			assert.Equal(t, c.kept, ch.Side().Success, "a kept failure is a success of 1")
			assert.Equal(t, c.winner, rec.Contests[0].Winner, "2 beats 1 or a failure")
		})
	}
	t.Run("a kept failure of 1 beats a failed attack", func(t *testing.T) {
		svc, m := newDefenceSvc(t, defender(10), 3)
		m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{blankRow}, nil).Once()
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "mind_blank").Return(-7, nil).Once()
		defendLog(m, []actionlog.Entry{raised(3, "mind_blank")}, nil)
		m.dice.EXPECT().RollDie(psionics.CheckDie).Return(12).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{
			Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 1, Success: true}},
		})
		require.NoError(t, err)
		assert.Equal(t, psionics.SideDefender, defenceOf(t, got).Contests[0].Winner, "1 against 1: the defender")
	})
}

// ADR 0044 step 5: the current defence is the mode raised this round over
// the standing one.
func TestCurrentDefence_Step5_ARaisedModeOverTheStandingOne(t *testing.T) {
	svc, m := newDefenceSvc(t, defender(10), 3)
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return([]buffsvc.BuffEffect{blankRow, shieldRaised}, nil).Once()
	m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(-4, nil).Once()
	defendLog(m, []actionlog.Entry{raised(6, "thought_shield"), raised(2, "mind_blank")}, nil)
	m.dice.EXPECT().RollDie(psionics.CheckDie).Return(10).Once()
	var got actionlog.NewEntry
	expectRecord(m.log, &got)
	_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 5, Success: true}}})
	require.NoError(t, err)
	rec := defenceOf(t, got)
	assert.Equal(t, "thought_shield", rec.Defence)
	assert.Equal(t, buffsvc.DefenceRaised, rec.DefenceKind)
}

// ADR 0044 step 8: with no current defence nothing is rolled: the
// attacker's successes win, a failed attack loses; no Table 14 cell.
func TestDefend_Step8_NoDefenceLoses(t *testing.T) {
	svc, m := newDefenceSvc(t, defender(10), 3)
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return(nil, nil).Once()
	var got actionlog.NewEntry
	expectRecord(m.log, &got)
	_, err := svc.Defend(context.Background(), 7, psionicsvc.DefendRequest{
		Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 18, Success: true}, {Roll: 19, Success: false}},
	})
	require.NoError(t, err)
	rec := defenceOf(t, got)
	assert.Empty(t, rec.Defence)
	assert.Nil(t, rec.Modifier)
	assert.Equal(t, psionics.SideAttacker, rec.Contests[0].Winner)
	assert.Equal(t, psionics.SideDefender, rec.Contests[1].Winner)
	assert.Nil(t, rec.Contests[0].Check)
	m.dice.AssertNotCalled(t, "RollDie", mock.Anything)
}

// ADR 0044 step 8: a malformed contest is refused before anything is read.
func TestDefend_Step8_FieldErrors(t *testing.T) {
	one, three := 1, 3
	for _, c := range []struct {
		name, field string
		req         psionicsvc.DefendRequest
	}{
		{"a defence mode is not an attack", "attack", psionicsvc.DefendRequest{Attack: "thought_shield", Rolls: []psionics.ContestSide{{Roll: 5}}}},
		{"an unknown attack", "attack", psionicsvc.DefendRequest{Attack: "nope", Rolls: []psionics.ContestSide{{Roll: 5}}}},
		{"no roll", "rolls", psionicsvc.DefendRequest{Attack: "ego_whip"}},
		{"three rolls", "rolls", psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: make([]psionics.ContestSide, 3)}},
		{"a roll of 21", "rolls", psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 21}}}},
		{"keep past the rolls", "keep", psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 5}}, Keep: &one}},
		{"keep past the rolls, 3", "keep", psionicsvc.DefendRequest{Attack: "ego_whip", Rolls: []psionics.ContestSide{{Roll: 5}, {Roll: 6}}, Keep: &three}},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newDefenceSvc(t, defender(10), 3)
			_, err := svc.Defend(context.Background(), 7, c.req)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
		})
	}
}
