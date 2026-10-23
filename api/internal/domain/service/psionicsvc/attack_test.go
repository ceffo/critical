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
)

// attackBook: the catalogue's ego whip (Wis -3, 4 PSPs) and mind thrust
// (Wis -2, 2 PSPs), thought shield and tower of iron will (defence modes)
// and a devotion.
func attackBook() psionics.Book {
	return psionics.NewBook([]catalogue.Discipline{{ID: "telepathy"}}, []catalogue.Power{
		{
			ID: "ego_whip", Kind: catalogue.KindAttackMode, Name: map[string]string{"en": "Ego Whip"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -3}, InitialCost: catalogue.Cost{Amount: 4},
		},
		{
			ID: "mind_thrust", Kind: catalogue.KindAttackMode, Name: map[string]string{"en": "Mind Thrust"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -2}, InitialCost: catalogue.Cost{Amount: 2},
		},
		{
			ID: "thought_shield", Kind: catalogue.KindDefenceMode, Name: map[string]string{"en": "Thought Shield"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -3}, InitialCost: catalogue.Cost{Amount: 1},
		},
		{
			ID: "tower_of_iron_will", Kind: catalogue.KindDefenceMode, Name: map[string]string{"en": "Tower of Iron Will"},
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: -2}, InitialCost: catalogue.Cost{Amount: 6},
		},
		{
			ID: "thrust", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			Score: &catalogue.PowerScore{Ability: "int", Modifier: -4}, InitialCost: catalogue.Cost{Amount: 7},
		},
	})
}

// attacker is a psionicist with Wis 16 (ego whip at 13, mind thrust at 14)
// who knows ego whip, mind thrust, thought shield and the devotion.
func attacker(current int) *charactersvc.Character {
	c := psionicist(
		psionics.Pick{Kind: psionics.PickDiscipline, ID: "telepathy"},
		psionics.Pick{Kind: psionics.PickPower, ID: "ego_whip"},
		psionics.Pick{Kind: psionics.PickPower, ID: "mind_thrust"},
		psionics.Pick{Kind: psionics.PickPower, ID: "thought_shield"},
		psionics.Pick{Kind: psionics.PickPower, ID: "thrust"},
	)
	c.Int, c.Wis = 18, 16
	c.Derived.PSP = &charactersvc.PSPPool{Current: current, Max: 30}
	return c
}

type attackMocks struct {
	dice   *mocks.MockDice
	log    *mocks.MockLog
	buffs  *mocks.MockBuffs
	tables *mocks.MockTables
}

var tangentSnap = actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`[]`), After: json.RawMessage(`[{"target":"Orc"}]`)}

// newAttackSvc is the service at round 3 (out of step 12's way: no entry
// in the round) with the character c and the active rows; its mocks fail on
// any call a test does not expect.
func newAttackSvc(t *testing.T, c *charactersvc.Character, rows []buffsvc.BuffEffect) (psionicsvc.SVC, attackMocks) {
	t.Helper()
	chars := mocks.NewMockCharacters(t)
	m := attackMocks{dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), buffs: mocks.NewMockBuffs(t), tables: mocks.NewMockTables(t)}
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil).Maybe()
	m.buffs.EXPECT().GetRound(mock.Anything, int64(7)).Return(3, nil).Maybe()
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return(rows, nil).Maybe()
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Tables: m.tables, Log: m.log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: m.dice, Buffs: m.buffs, Book: attackBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, m
}

// expectTangents expects one AddTangents of won on target, answering ch
// and a snapshot when it changed something.
func expectTangents(m attackMocks, target string, won int, ch buffsvc.TangentsChange) {
	snap := actionlog.Snapshot{}
	if ch.Broken != nil || ch.Before != ch.After {
		snap = tangentSnap
	}
	ch.Target = target
	m.buffs.EXPECT().AddTangents(mock.Anything, int64(7), target, won).Return(ch, snap, nil).Once()
}

// rolls makes the dice mock answer the d20s in order, each once.
func rolls(m attackMocks, d20s ...int) {
	for _, r := range d20s {
		m.dice.EXPECT().RollDie(psionics.CheckDie).Return(r).Once()
	}
}

func attackOf(t *testing.T, e actionlog.NewEntry) psionicsvc.AttackRecord {
	t.Helper()
	require.Equal(t, actionlog.KindPsionicAttack, e.Kind)
	var rec psionicsvc.AttackRecord
	require.NoError(t, json.Unmarshal(e.Payload, &rec))
	return rec
}

func winners(rec psionicsvc.AttackRecord) []string {
	var out []string
	for _, c := range rec.Checks {
		out = append(out, c.Winner)
	}
	return out
}

func closedOrc() psionics.Opponent {
	return psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed}
}

// ADR 0044 step 7.1: ego whip's one cost (4 PSPs) buys two d20 through the
// dice port, in one psionic_attack entry with the PSP delta and the
// tangents' snapshot.
func TestAttack_Step7_TwoChecksOneCost(t *testing.T) {
	svc, m := newAttackSvc(t, attacker(10), nil)
	rolls(m, 5, 6)
	expectTangents(m, "Orc", 2, buffsvc.TangentsChange{Before: 0, After: 2})
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	res, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
	require.NoError(t, err)
	rec := attackOf(t, got)
	assert.Equal(t, -4, got.PSPDelta)
	assert.Equal(t, []actionlog.Snapshot{tangentSnap}, got.Snapshots)
	assert.Equal(t, 4, rec.Cost)
	assert.Equal(t, 4, rec.Spent)
	assert.Equal(t, 13, rec.Score)
	assert.Equal(t, "wis", rec.Ability)
	require.Len(t, rec.Checks, 2)
	assert.Equal(t, 13, rec.Checks[0].Check.Target)
	assert.Equal(t, 5, rec.Checks[0].Check.Roll)
	assert.Equal(t, 6, rec.Checks[1].Check.Roll)
	assert.Equal(t, rec, res.Attack)
	assert.Equal(t, actionlog.PSPState{Current: 6, Max: 30}, res.PSP)
	assert.Equal(t, int64(41), res.LogEntryID)
}

// ADR 0044 step 7.1: mind thrust (14) against tower of iron will: Table
// 14's -5 on both checks' targets, beside the DM's rolls.
func TestAttack_Step7_TableFourteenModifiesTheScore(t *testing.T) {
	svc, m := newAttackSvc(t, attacker(10), nil)
	m.tables.EXPECT().AttackVersusDefence("mind_thrust", "tower_of_iron_will").Return(-5, nil).Once()
	rolls(m, 9, 10)
	expectTangents(m, "Orc", 1, buffsvc.TangentsChange{Before: 0, After: 1})
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	opp := closedOrc()
	opp.Defence = "tower_of_iron_will"
	opp.Rolls = []psionics.ContestSide{{Roll: 3, Success: true}, {Roll: 4, Success: true}}
	_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "mind_thrust", Opponent: opp})
	require.NoError(t, err)
	rec := attackOf(t, got)
	require.NotNil(t, rec.AttackVersusDefence)
	assert.Equal(t, -5, *rec.AttackVersusDefence)
	assert.Equal(t, "tower_of_iron_will", rec.Defence)
	assert.Equal(t, 14, rec.Score)
	require.Len(t, rec.Checks, 2)
	for i, c := range rec.Checks {
		assert.Equal(t, 9, c.Check.Target, "check %d: 14 - 5", i)
		assert.Equal(t, &opp.Rolls[i], c.Defence)
	}
	assert.Equal(t, []string{psionics.SideAttacker, psionics.SideDefender}, winners(rec), "9 beats 3; 10 fails at 9")
}

// ADR 0044 step 7.1 (Q5): half the cost, rounded up, when both checks fail;
// the whole cost when one succeeds, even if its contest is lost.
func TestAttack_Step7_HalfTheCostWhenBothFail(t *testing.T) {
	t.Run("both fail", func(t *testing.T) {
		svc, m := newAttackSvc(t, attacker(10), nil)
		rolls(m, 19, 20)
		expectTangents(m, "Orc", 0, buffsvc.TangentsChange{})
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
		require.NoError(t, err)
		assert.Equal(t, -2, got.PSPDelta)
		assert.Equal(t, 2, res.Attack.Spent)
		assert.Empty(t, got.Snapshots, "nothing changed on the rows")
	})
	t.Run("one succeeds and loses its contest", func(t *testing.T) {
		svc, m := newAttackSvc(t, attacker(10), nil)
		m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(0, nil).Once()
		rolls(m, 4, 19)
		expectTangents(m, "Orc", 0, buffsvc.TangentsChange{})
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		opp := closedOrc()
		opp.Defence = "thought_shield"
		opp.Rolls = []psionics.ContestSide{{Roll: 8, Success: true}, {Roll: 2, Success: true}}
		_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: opp})
		require.NoError(t, err)
		assert.Equal(t, -4, got.PSPDelta)
		assert.Equal(t, []string{psionics.SideDefender, psionics.SideDefender}, winners(attackOf(t, got)))
	})
}

// ADR 0044 step 7.3: against a closed mind each won contest is a tangent,
// the won count AddTangents takes; the entry carries the tangents before
// and after.
func TestAttack_Step7_ATangentPerWonContest(t *testing.T) {
	rows := []buffsvc.BuffEffect{{ID: "t", Name: "Orc", Target: "Orc", Tangents: 1, MaintenanceCost: 1, MaintenancePeriod: 1}}
	svc, m := newAttackSvc(t, attacker(10), rows)
	m.tables.EXPECT().AttackVersusDefence("ego_whip", "thought_shield").Return(0, nil).Once()
	rolls(m, 12, 2)
	expectTangents(m, "Orc", 1, buffsvc.TangentsChange{Before: 1, After: 2})
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	opp := closedOrc()
	opp.Target = "  Orc "
	opp.Defence = "thought_shield"
	opp.Rolls = []psionics.ContestSide{{Roll: 10, Success: true}, {Roll: 3, Success: true}}
	_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: opp})
	require.NoError(t, err)
	rec := attackOf(t, got)
	assert.Equal(t, "Orc", rec.Target, "trimmed")
	assert.Equal(t, []string{psionics.SideAttacker, psionics.SideDefender}, winners(rec), "12 beats 10; 2 under 3")
	assert.Equal(t, 1, rec.TangentsBefore)
	assert.Equal(t, 2, rec.TangentsAfter)
	assert.False(t, rec.Contact)
	for _, c := range rec.Checks {
		assert.False(t, c.Effect, "a closed mind: tangents, not the effect")
	}
}

// ADR 0044 step 7.3 (Q11): two tangents held, the first check wins: full
// contact, the second check is not rolled (one die) and the cost is whole.
func TestAttack_Step7_TheThirdTangentIsContactNoMoreAttacks(t *testing.T) {
	rows := []buffsvc.BuffEffect{{ID: "t", Name: "Orc", Target: "Orc", Tangents: 2, MaintenanceCost: 1, MaintenancePeriod: 1}}
	svc, m := newAttackSvc(t, attacker(10), rows)
	rolls(m, 7)
	expectTangents(m, "Orc", 1, buffsvc.TangentsChange{Before: 2, After: 3, Contact: true})
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	res, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
	require.NoError(t, err)
	rec := attackOf(t, got)
	require.Len(t, rec.Checks, 1)
	assert.True(t, rec.Contact)
	assert.Equal(t, 3, rec.TangentsAfter)
	assert.Equal(t, 4, res.Attack.Spent)
	m.dice.AssertNumberOfCalls(t, "RollDie", 1)
}

// ADR 0044 step 7.2 (Q14): with no defence raised a successful check wins,
// a failed one loses; no Table 14 read.
func TestAttack_Step7_NoDefenceASuccessWins(t *testing.T) {
	svc, m := newAttackSvc(t, attacker(10), nil)
	rolls(m, 13, 14)
	expectTangents(m, "Orc", 1, buffsvc.TangentsChange{Before: 0, After: 1})
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
	require.NoError(t, err)
	rec := attackOf(t, got)
	assert.Equal(t, []string{psionics.SideAttacker, psionics.SideDefender}, winners(rec), "13 at 13 succeeds; 14 fails")
	assert.Nil(t, rec.AttackVersusDefence)
	assert.Nil(t, rec.Checks[0].Defence)
	assert.Equal(t, psionics.OutcomePowerScore, rec.Checks[0].Check.Outcome)
	m.tables.AssertNotCalled(t, "AttackVersusDefence", mock.Anything, mock.Anything)
}

// ADR 0044 step 7.4 (Q8, Q9): against an open mind, or a contacted one, a
// won contest is the mode's effect, no tangent (AddTangents takes 0, still
// breaking another mind's tangent row).
func TestAttack_Step7_AnOpenMindShowsTheEffect(t *testing.T) {
	t.Run("an open mind", func(t *testing.T) {
		svc, m := newAttackSvc(t, attacker(10), nil)
		rolls(m, 5, 18)
		expectTangents(m, "Peasant", 0, buffsvc.TangentsChange{})
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{
			Mode: "ego_whip", Opponent: psionics.Opponent{Target: "Peasant", Mind: psionics.MindOpen},
		})
		require.NoError(t, err)
		rec := attackOf(t, got)
		require.Len(t, rec.Checks, 2)
		assert.True(t, rec.Checks[0].Effect)
		assert.False(t, rec.Checks[1].Effect, "a lost contest has no effect")
		assert.Equal(t, psionics.MindOpen, rec.Mind)
	})
	t.Run("a contacted mind", func(t *testing.T) {
		rows := []buffsvc.BuffEffect{{ID: "c", Name: "Orc", Target: "Orc", MaintenanceCost: 1, MaintenancePeriod: 1}}
		svc, m := newAttackSvc(t, attacker(10), rows)
		rolls(m, 5, 6)
		expectTangents(m, "Orc", 0, buffsvc.TangentsChange{Before: 3, After: 3})
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
		require.NoError(t, err)
		rec := attackOf(t, got)
		require.Len(t, rec.Checks, 2, "no contact made: both are rolled")
		assert.True(t, rec.Checks[0].Effect)
		assert.True(t, rec.Checks[1].Effect)
	})
}

// ADR 0044 step 2: the opponent is the request's. Two attacks naming two
// targets read no opponent store (the service has none): each names its
// target to AddTangents, the second breaking the first's tangent row.
func TestAttack_Step2_TheOpponentIsTheRequests(t *testing.T) {
	svc, m := newAttackSvc(t, attacker(20), nil)
	rolls(m, 5, 18, 5, 18)
	expectTangents(m, "Orc", 1, buffsvc.TangentsChange{Before: 0, After: 1})
	orcRow := buffsvc.BuffEffect{ID: "t", Name: "Orc", Target: "Orc", Tangents: 1}
	expectTangents(m, "Goblin", 1, buffsvc.TangentsChange{Before: 0, After: 1, Broken: &orcRow})
	m.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(41), nil).Twice()

	first, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
	require.NoError(t, err)
	assert.Equal(t, "Orc", first.Attack.Target)
	assert.Empty(t, first.Attack.Broken)
	second, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{
		Mode: "ego_whip", Opponent: psionics.Opponent{Target: "Goblin", Mind: psionics.MindClosed},
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin", second.Attack.Target)
	assert.Equal(t, "Orc", second.Attack.Broken, "another target breaks the tangent row")
}

// ADR 0044 step 7 and ADR 0043 step 12: an attack counts as the round's
// power: a second attack in the round, or a power check after it, warns.
func TestAttack_Step7_CountsTowardTheRoundsPowers(t *testing.T) {
	chars := mocks.NewMockCharacters(t)
	m := attackMocks{dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), buffs: mocks.NewMockBuffs(t), tables: mocks.NewMockTables(t)}
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(attacker(20), nil)
	m.buffs.EXPECT().GetRound(mock.Anything, int64(7)).Return(3, nil)
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return(nil, nil).Maybe()
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), actionlog.KindRoundAdvance, 1).
		Return([]actionlog.Entry{{ID: 5, Round: 3, Kind: actionlog.KindRoundAdvance}}, nil)
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), actionlog.KindPsionicAttack, 1).
		Return([]actionlog.Entry{{ID: 9, Round: 3, Kind: actionlog.KindPsionicAttack}}, nil)
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Tables: m.tables, Log: m.log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: m.dice, Buffs: m.buffs, Book: attackBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	rolls(m, 18, 18, 18)
	expectTangents(m, "Orc", 0, buffsvc.TangentsChange{})
	m.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(41), nil).Twice()

	res, err := svc.Attack(context.Background(), 7, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closedOrc()})
	require.NoError(t, err)
	assert.Equal(t, []string{psionicsvc.WarnSecondPowerThisRound}, res.Attack.Warnings)
	use, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{psionicsvc.WarnSecondPowerThisRound}, use.Check.Warnings)
}

// ADR 0044 step 7: the refusals, before any roll or write (the mocks fail
// on any call).
func TestAttack_Step7_RefusalsRollAndWriteNothing(t *testing.T) {
	closed := closedOrc()
	for _, c := range []struct {
		name  string
		char  *charactersvc.Character
		req   psionicsvc.AttackRequest
		err   error
		field string
	}{
		{"not a psionicist", &charactersvc.Character{}, psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closed}, psionicsvc.ErrNotPsionicist, ""},
		{"a mode not known", attacker(10), psionicsvc.AttackRequest{Mode: "psionic_blast", Opponent: closed}, psionicsvc.ErrPowerNotKnown, ""},
		{"a defence mode", attacker(10), psionicsvc.AttackRequest{Mode: "thought_shield", Opponent: closed}, psionicsvc.ErrNotAnAttackMode, ""},
		{"a devotion", attacker(10), psionicsvc.AttackRequest{Mode: "thrust", Opponent: closed}, psionicsvc.ErrNotAnAttackMode, ""},
		{"under the cost", attacker(3), psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closed}, psionicsvc.ErrNotEnoughPSP, ""},
		{"no target", attacker(10), psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: psionics.Opponent{Target: " ", Mind: psionics.MindClosed}}, nil, "target"},
		{"no mind", attacker(10), psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: psionics.Opponent{Target: "Orc"}}, nil, "mind"},
		{
			"a defence with one roll", attacker(10),
			psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: psionics.Opponent{
				Target: "Orc", Mind: psionics.MindClosed, Defence: "thought_shield", Rolls: []psionics.ContestSide{{Roll: 3}},
			}},
			nil, "rolls",
		},
		{"a bad modifier", attacker(10), psionicsvc.AttackRequest{Mode: "ego_whip", Opponent: closed, Modifiers: []psionicsvc.Modifier{{Value: 99}}}, nil, "modifiers"},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newAttackSvc(t, c.char, nil)
			_, err := svc.Attack(context.Background(), 7, c.req)
			require.Error(t, err)
			if c.err != nil {
				require.ErrorIs(t, err, c.err)
				return
			}
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
		})
	}
}
