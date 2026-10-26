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

// contactBook: the catalogue's contact (Wis, its cost Table 29's, 1 PSP a
// round), inflict pain (Con -4, cost contact, 2 PSPs a round) and switch
// personality (Con -4, cost contact plus 30, no maintenance), all
// telepathy.
func contactBook() psionics.Book {
	return psionics.NewBook([]catalogue.Discipline{{ID: "telepathy"}}, []catalogue.Power{
		{
			ID: "contact", Discipline: "telepathy", Kind: catalogue.KindDevotion, Name: map[string]string{"en": "Contact"},
			Score:       &catalogue.PowerScore{Ability: "wis"},
			InitialCost: catalogue.Cost{Kind: catalogue.CostVariable, Table: psionicsvc.ContactCostTable},
			Maintenance: &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 1}, Period: 1},
		},
		{
			ID: "inflict_pain", Discipline: "telepathy", Kind: catalogue.KindDevotion, Name: map[string]string{"en": "Inflict Pain"},
			Score:       &catalogue.PowerScore{Ability: "con", Modifier: -4},
			InitialCost: catalogue.Cost{Kind: catalogue.CostContact},
			Maintenance: &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 2}, Period: 1},
		},
		{
			ID: "switch_personality", Discipline: "telepathy", Kind: catalogue.KindScience, Name: map[string]string{"en": "Switch Personality"},
			Score:       &catalogue.PowerScore{Ability: "con", Modifier: -4},
			InitialCost: catalogue.Cost{Kind: catalogue.CostContact, Plus: 30},
		},
	})
}

// telepath is a psionicist with Wis 15 (contact at 15) and Con 16 (inflict
// pain and switch personality at 12) who knows the three powers.
func telepath(current int) *charactersvc.Character {
	c := psionicist(
		psionics.Pick{Kind: psionics.PickDiscipline, ID: "telepathy"},
		psionics.Pick{Kind: psionics.PickPower, ID: "contact"},
		psionics.Pick{Kind: psionics.PickPower, ID: "inflict_pain"},
		psionics.Pick{Kind: psionics.PickPower, ID: "switch_personality"},
	)
	c.Wis, c.Con = 15, 16
	c.Derived.PSP = &charactersvc.PSPPool{Current: current, Max: 50}
	return c
}

// newContactSvc is newAttackSvc on contactBook.
func newContactSvc(t *testing.T, c *charactersvc.Character, rows []buffsvc.BuffEffect) (psionicsvc.SVC, attackMocks) {
	t.Helper()
	chars := mocks.NewMockCharacters(t)
	m := attackMocks{dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), buffs: mocks.NewMockBuffs(t), tables: mocks.NewMockTables(t)}
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil).Maybe()
	m.buffs.EXPECT().GetRound(mock.Anything, int64(7)).Return(3, nil).Maybe()
	m.buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return(rows, nil).Maybe()
	m.log.EXPECT().ActiveEntries(mock.Anything, int64(7), mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Tables: m.tables, Log: m.log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: m.dice, Buffs: m.buffs, Book: contactBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, m
}

// atSight expects the lines of a target in sight, a mammal, not resisting
// (0 and -1, the catalogue's).
func atSight(m attackMocks) {
	m.tables.EXPECT().ContactModifier(ruletablesvc.ContactDistance, "sight").Return(0, nil).Once()
	m.tables.EXPECT().ContactModifier(ruletablesvc.ContactLifeOrder, "mammal").Return(-1, nil).Once()
}

func openOrc(level int) psionics.Opponent {
	return psionics.Opponent{Target: "Orc", Mind: psionics.MindOpen, Level: level}
}

func contactReq(level int) psionicsvc.ContactRequest {
	return psionicsvc.ContactRequest{Power: "contact", Opponent: openOrc(level), Distance: "sight", LifeOrder: "mammal"}
}

var contactSnap = actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`[]`), After: json.RawMessage(`[{"target":"Orc"}]`)}

// expectContact expects one MakeContact on Orc, answering a new contact row
// c1 and its snapshot.
func expectContact(m attackMocks) {
	row := buffsvc.BuffEffect{ID: "c1", Name: "Orc", Target: "Orc", MaintenanceCost: buffsvc.LinkMaintenanceCost, MaintenancePeriod: 1}
	m.buffs.EXPECT().MakeContact(mock.Anything, int64(7), "Orc").
		Return(buffsvc.TangentsChange{Target: "Orc", Before: 0, After: buffsvc.MaxTangents + 1, Contact: true, Row: row}, contactSnap, nil).Once()
}

// ADR 0044 step 9: the initial cost is Table 29's row for the target's
// level (a level 7 target costs 8, 4 on a failure; a level 3 one 3, 2 on a
// failure), the level recorded beside the target.
func TestContact_Step9_CostByTheTargetsLevel(t *testing.T) {
	for _, c := range []struct {
		name          string
		level, cost   int
		roll, spent   int
		wantContactOn bool
	}{
		{"level 7, a success", 7, 8, 10, 8, true},
		{"level 7, a failure", 7, 8, 18, 4, false},
		{"level 3, a failure", 3, 3, 18, 2, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, m := newContactSvc(t, telepath(20), nil)
			m.tables.EXPECT().ContactCost(c.level).Return(c.cost, nil).Once()
			atSight(m)
			rolls(m, c.roll)
			if c.wantContactOn {
				expectContact(m)
			}
			var got actionlog.NewEntry
			expectRecord(m.log, &got)

			res, err := svc.Contact(context.Background(), 7, contactReq(c.level))
			require.NoError(t, err)
			rec := payloadOf(t, got)
			assert.Equal(t, actionlog.KindPowerCheck, got.Kind)
			assert.Equal(t, c.cost, rec.Cost)
			assert.Equal(t, c.spent, rec.Spent)
			assert.Equal(t, -c.spent, got.PSPDelta)
			assert.Equal(t, c.level, rec.Level)
			assert.Equal(t, "Orc", rec.Opponent)
			assert.Equal(t, actionlog.PSPState{Current: 20 - c.spent, Max: 50}, res.PSP)
		})
	}
}

// ADR 0044 step 9: the score is the contact power's (Wis 15) with the
// distance's line (Table 30), the life order's (Table 31) and, when the DM
// says so, resisting's, each recorded with its code.
func TestContact_Step9_DistanceLifeOrderAndResistance(t *testing.T) {
	svc, m := newContactSvc(t, telepath(20), nil)
	m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
	m.tables.EXPECT().ContactModifier(ruletablesvc.ContactDistance, "miles_10").Return(-3, nil).Once()
	m.tables.EXPECT().ContactModifier(ruletablesvc.ContactLifeOrder, "bird").Return(-3, nil).Once()
	m.tables.EXPECT().ContactModifier(ruletablesvc.ContactResistance, "resisting").Return(-2, nil).Once()
	rolls(m, 9)
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	req := psionicsvc.ContactRequest{
		Power: "contact", Opponent: openOrc(7), Distance: "miles_10", LifeOrder: "bird", Resisting: true,
		Modifiers: []psionicsvc.Modifier{{Value: 1, Reason: "rested"}},
	}
	_, err := svc.Contact(context.Background(), 7, req)
	require.NoError(t, err)
	rec := payloadOf(t, got)
	assert.Equal(t, []psionicsvc.ContactLine{
		{Kind: ruletablesvc.ContactDistance, Code: "miles_10", Modifier: -3},
		{Kind: ruletablesvc.ContactLifeOrder, Code: "bird", Modifier: -3},
		{Kind: ruletablesvc.ContactResistance, Code: "resisting", Modifier: -2},
	}, rec.ContactModifiers)
	assert.Equal(t, 15, rec.AbilityScore)
	assert.Equal(t, 8, rec.Target, "15 + 1 - 3 - 3 - 2: a bird at 10 miles, resisting")
	assert.False(t, rec.Success, "9 is over 8")
	assert.Equal(t, 4, rec.Spent)

	t.Run("not resisting: no line", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
		atSight(m)
		rolls(m, 18)
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		_, err := svc.Contact(context.Background(), 7, contactReq(7))
		require.NoError(t, err)
		rec := payloadOf(t, got)
		assert.Len(t, rec.ContactModifiers, 2)
		assert.Equal(t, 14, rec.Target, "15 + 0 - 1")
	})
}

// ADR 0044 step 9: the contact power never works on a closed mind: refused
// before the cost is read or a die rolled (the mocks fail on any call).
func TestContact_Step9_AClosedMindRefused(t *testing.T) {
	svc, _ := newContactSvc(t, telepath(20), nil)
	req := contactReq(7)
	req.Opponent.Mind = psionics.MindClosed
	_, err := svc.Contact(context.Background(), 7, req)
	require.ErrorIs(t, err, psionicsvc.ErrMindClosed)
}

// ADR 0044 step 9: a success makes the target's contact row in the
// power_check entry, its snapshot beside the PSPs (one undo takes both
// back); a failure makes none.
func TestContact_Step9_ASuccessPutsTheContactRow(t *testing.T) {
	svc, m := newContactSvc(t, telepath(20), nil)
	m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
	atSight(m)
	rolls(m, 5)
	expectContact(m)
	var got actionlog.NewEntry
	expectRecord(m.log, &got)

	res, err := svc.Contact(context.Background(), 7, contactReq(7))
	require.NoError(t, err)
	rec := payloadOf(t, got)
	assert.True(t, rec.Success)
	assert.True(t, rec.Contact)
	assert.Equal(t, "c1", res.BuffID)
	assert.Equal(t, []actionlog.Snapshot{contactSnap}, got.Snapshots)
	assert.Equal(t, -8, got.PSPDelta)
	assert.Equal(t, int64(41), res.LogEntryID)

	t.Run("a mind already contacted: renewed, no snapshot", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
		atSight(m)
		rolls(m, 5)
		row := buffsvc.BuffEffect{ID: "c0", Name: "Orc", Target: "Orc", RoundsKept: 4}
		m.buffs.EXPECT().MakeContact(mock.Anything, int64(7), "Orc").
			Return(buffsvc.TangentsChange{Target: "Orc", Before: 3, After: 3, Row: row}, actionlog.Snapshot{}, nil).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.Contact(context.Background(), 7, contactReq(7))
		require.NoError(t, err)
		assert.Empty(t, got.Snapshots)
		assert.Equal(t, "c0", res.BuffID)
		assert.True(t, payloadOf(t, got).Contact)
	})

	t.Run("a failure makes no row", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
		atSight(m)
		rolls(m, 19)
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.Contact(context.Background(), 7, contactReq(7))
		require.NoError(t, err)
		assert.Empty(t, got.Snapshots)
		assert.Empty(t, res.BuffID)
		assert.False(t, payloadOf(t, got).Contact)
	})
}

// ADR 0044 step 9 (Q15): the refusals, before any roll or write. A level
// past Table 29's last row is ErrLevelOutOfTable.
func TestContact_Step9_Refusals(t *testing.T) {
	for _, c := range []struct {
		name   string
		char   *charactersvc.Character
		req    func(*psionicsvc.ContactRequest)
		tables func(m attackMocks)
		err    error
		field  string
	}{
		{name: "not a psionicist", char: &charactersvc.Character{}, err: psionicsvc.ErrNotPsionicist},
		{name: "a power not known", req: func(r *psionicsvc.ContactRequest) { r.Power = "mindlink" }, err: psionicsvc.ErrPowerNotKnown},
		{name: "not the contact power", req: func(r *psionicsvc.ContactRequest) { r.Power = "inflict_pain" }, err: psionicsvc.ErrNotAContactPower},
		{
			name: "a level past the table", req: func(r *psionicsvc.ContactRequest) { r.Opponent.Level = 21 },
			tables: func(m attackMocks) {
				m.tables.EXPECT().ContactCost(21).Return(0, ruletablesvc.ErrLevelOutOfTable).Once()
			},
			err: psionicsvc.ErrLevelOutOfTable,
		},
		{
			name: "under the cost", char: telepath(7),
			tables: func(m attackMocks) {
				m.tables.EXPECT().ContactCost(7).Return(8, nil).Once()
				atSight(m)
			},
			err: psionicsvc.ErrNotEnoughPSP,
		},
		{name: "no level", req: func(r *psionicsvc.ContactRequest) { r.Opponent.Level = 0 }, field: "level"},
		{name: "a defence", req: func(r *psionicsvc.ContactRequest) { r.Opponent.Defence = "thought_shield" }, field: "defence"},
		{name: "no distance", req: func(r *psionicsvc.ContactRequest) { r.Distance = "" }, field: "distance"},
		{name: "an unknown life order", req: func(r *psionicsvc.ContactRequest) { r.LifeOrder = "dragon" }, field: "lifeOrder"},
		{name: "no mind", req: func(r *psionicsvc.ContactRequest) { r.Opponent.Mind = "" }, field: "mind"},
		{name: "a bad modifier", req: func(r *psionicsvc.ContactRequest) { r.Modifiers = []psionicsvc.Modifier{{Value: 99}} }, field: "modifiers"},
	} {
		t.Run(c.name, func(t *testing.T) {
			char := c.char
			if char == nil {
				char = telepath(20)
			}
			svc, m := newContactSvc(t, char, nil)
			if c.tables != nil {
				c.tables(m)
			}
			req := contactReq(7)
			if c.req != nil {
				c.req(&req)
			}
			_, err := svc.Contact(context.Background(), 7, req)
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

// contactRow is a contact row on target kept for kept rounds, contact made
// at from.
func contactRow(target string, kept, from int) buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: "c-" + target, Name: target, Target: target, MaintenanceCost: buffsvc.LinkMaintenanceCost, MaintenancePeriod: 1,
		RoundsKept: kept, ContactFrom: from,
	}
}

// ADR 0044 step 10: a power whose cost is contact needs a contact row on
// its target made in an earlier round (kept past the round contact was
// made): none, tangents only, or contact made this round are ErrNoContact,
// refused before the roll.
func TestContactPower_Step10_NeedsContactFromAnEarlierRound(t *testing.T) {
	for _, c := range []struct {
		name string
		rows []buffsvc.BuffEffect
	}{
		{"no row", nil},
		{"tangents only", []buffsvc.BuffEffect{{ID: "t", Name: "Orc", Target: "Orc", Tangents: 2, MaintenanceCost: 1, MaintenancePeriod: 1, RoundsKept: 3}}},
		{"contact made this round", []buffsvc.BuffEffect{contactRow("Orc", 2, 2)}},
		{"contact with another mind", []buffsvc.BuffEffect{contactRow("Goblin", 3, 1)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newContactSvc(t, telepath(40), c.rows)
			_, err := svc.Use(context.Background(), 7, "inflict_pain", psionicsvc.UseRequest{Target: "Orc"})
			require.ErrorIs(t, err, psionicsvc.ErrNoContact)
		})
	}

	t.Run("contact from an earlier round: the row carries its target", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(40), []buffsvc.BuffEffect{contactRow("Orc", 3, 2)})
		rolls(m, 4)
		var added buffsvc.BuffEffect
		m.buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).
			Run(func(_ context.Context, _ int64, b buffsvc.BuffEffect) { added = b }).
			Return(buffsvc.BuffEffect{ID: "p1"}, tangentSnap, nil).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.Use(context.Background(), 7, "inflict_pain", psionicsvc.UseRequest{Target: " orc "})
		require.NoError(t, err)
		assert.Equal(t, "Orc", added.Target, "the contact row's label")
		assert.Equal(t, "inflict_pain", added.PowerID)
		assert.Equal(t, 2, added.MaintenanceCost)
		assert.Equal(t, "Orc", payloadOf(t, got).Opponent)
		assert.Equal(t, "p1", res.BuffID)
	})

	t.Run("no target", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(40), nil)
		_, err := svc.Use(context.Background(), 7, "inflict_pain", psionicsvc.UseRequest{})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "target", fe.Field)
	})
}

// ADR 0044 step 10 (Q12): the contact paid for the rest: a power whose
// cost is contact costs only its plus amount (switch personality's 30,
// inflict pain's none), never an entered cost.
func TestContactPower_Step10_ThePlusAmount(t *testing.T) {
	rows := []buffsvc.BuffEffect{contactRow("Orc", 3, 2)}

	t.Run("switch personality: 30, 15 on a failure", func(t *testing.T) {
		for _, c := range []struct {
			roll, spent int
		}{{4, 30}, {18, 15}} {
			svc, m := newContactSvc(t, telepath(40), rows)
			rolls(m, c.roll)
			var got actionlog.NewEntry
			expectRecord(m.log, &got)
			res, err := svc.Use(context.Background(), 7, "switch_personality", psionicsvc.UseRequest{Target: "Orc"})
			require.NoError(t, err)
			assert.Equal(t, 30, res.Check.Cost)
			assert.Equal(t, c.spent, res.Check.Spent)
			assert.Equal(t, -c.spent, got.PSPDelta)
			assert.Empty(t, res.BuffID, "no maintenance: no row")
		}
	})

	t.Run("inflict pain: nothing", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(0), rows)
		rolls(m, 4)
		m.buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).Return(buffsvc.BuffEffect{ID: "p1"}, tangentSnap, nil).Once()
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		res, err := svc.Use(context.Background(), 7, "inflict_pain", psionicsvc.UseRequest{Target: "Orc"})
		require.NoError(t, err)
		assert.Zero(t, res.Check.Cost)
		assert.Zero(t, got.PSPDelta)
	})

	t.Run("under the plus amount", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(29), rows)
		_, err := svc.Use(context.Background(), 7, "switch_personality", psionicsvc.UseRequest{Target: "Orc"})
		require.ErrorIs(t, err, psionicsvc.ErrNotEnoughPSP)
	})

	t.Run("an entered cost", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(40), rows)
		cost := 5
		_, err := svc.Use(context.Background(), 7, "switch_personality", psionicsvc.UseRequest{Target: "Orc", Cost: &cost})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "cost", fe.Field)
	})
}

// ADR 0044 step 9 (Q15): for a level Table 29 has no row for, the DM gives
// the cost. The contact still makes its contact row, so a power whose cost
// is contact is accepted on that mind from the next round. A cost entered
// for a level the table has, or under 1, is refused.
func TestContact_Q15_TheDMsCostPastTheTable(t *testing.T) {
	cost := func(n int) *int { return &n }

	t.Run("level 21 with the DM's cost: the contact row", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(21).Return(0, ruletablesvc.ErrLevelOutOfTable).Once()
		atSight(m)
		rolls(m, 10)
		expectContact(m)
		var got actionlog.NewEntry
		expectRecord(m.log, &got)
		req := contactReq(21)
		req.Cost = cost(13)
		res, err := svc.Contact(context.Background(), 7, req)
		require.NoError(t, err)
		rec := payloadOf(t, got)
		assert.Equal(t, 13, rec.Cost)
		assert.Equal(t, 21, rec.Level)
		assert.True(t, rec.Contact)
		assert.Equal(t, "c1", res.BuffID)
		assert.Equal(t, []actionlog.Snapshot{contactSnap}, got.Snapshots)
	})

	t.Run("inflict pain on that mind the next round", func(t *testing.T) {
		// The row MakeContact made, after one Next Round.
		svc, m := newContactSvc(t, telepath(20), []buffsvc.BuffEffect{contactRow("Orc", 1, 0)})
		rolls(m, 4)
		m.buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).Return(buffsvc.BuffEffect{ID: "p1"}, tangentSnap, nil).Once()
		expectRecord(m.log, new(actionlog.NewEntry))
		res, err := svc.Use(context.Background(), 7, "inflict_pain", psionicsvc.UseRequest{Target: "Orc"})
		require.NoError(t, err)
		assert.True(t, res.Check.Success)
	})

	for _, c := range []struct {
		name   string
		level  int
		cost   int
		answer error
	}{
		{"a cost for a level the table has", 7, 5, nil},
		{"a cost under 1", 21, 0, ruletablesvc.ErrLevelOutOfTable},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, m := newContactSvc(t, telepath(20), nil)
			m.tables.EXPECT().ContactCost(c.level).Return(8, c.answer).Once()
			req := contactReq(c.level)
			req.Cost = cost(c.cost)
			_, err := svc.Contact(context.Background(), 7, req)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "cost", fe.Field)
		})
	}
}

// ADR 0044 step 9 (Q15): Use refuses the contact power, whose cost names
// a table, before the roll: it goes through Contact, which makes the
// contact row, even with the DM's cost.
func TestUse_Step9_TheContactPowerGoesThroughContact(t *testing.T) {
	svc, _ := newContactSvc(t, telepath(20), nil)
	cost := 13
	_, err := svc.Use(context.Background(), 7, "contact", psionicsvc.UseRequest{Cost: &cost})
	require.ErrorIs(t, err, psionicsvc.ErrUseContactRoute)
}

// The preview is Contact's own computation without the roll: Table 29's cost,
// the score with the lines (Wis 15, 0 and -1: 14), nothing written, and
// whether the pool covers the cost.
func TestContactPreview_ReportsTheCostAndScoreWithoutRolling(t *testing.T) {
	svc, m := newContactSvc(t, telepath(5), nil)
	m.tables.EXPECT().ContactCost(7).Return(8, nil).Twice()
	atSight(m)
	got, err := svc.ContactPreview(context.Background(), 7, contactReq(7))
	require.NoError(t, err)
	assert.Equal(t, &psionicsvc.ContactPreview{
		Cost: 8, FromTable: true, Target: 14, Current: 5, Enough: false,
		Lines: []psionicsvc.ContactLine{
			{Kind: ruletablesvc.ContactDistance, Code: "sight", Modifier: 0},
			{Kind: ruletablesvc.ContactLifeOrder, Code: "mammal", Modifier: -1},
		},
	}, got)
}

// Past the table the DM's cost is the preview's, flagged as not the table's;
// the refusals before the roll are Contact's.
func TestContactPreview_PastTheTableAndRefusals(t *testing.T) {
	t.Run("the DM's cost", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(21).Return(0, ruletablesvc.ErrLevelOutOfTable).Twice()
		atSight(m)
		req := contactReq(21)
		cost := 13
		req.Cost = &cost
		got, err := svc.ContactPreview(context.Background(), 7, req)
		require.NoError(t, err)
		assert.Equal(t, 13, got.Cost)
		assert.False(t, got.FromTable)
		assert.True(t, got.Enough)
	})
	t.Run("no row and no cost", func(t *testing.T) {
		svc, m := newContactSvc(t, telepath(20), nil)
		m.tables.EXPECT().ContactCost(21).Return(0, ruletablesvc.ErrLevelOutOfTable).Once()
		_, err := svc.ContactPreview(context.Background(), 7, contactReq(21))
		assert.ErrorIs(t, err, psionicsvc.ErrLevelOutOfTable)
	})
	t.Run("a closed mind", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(20), nil)
		req := contactReq(7)
		req.Opponent.Mind = psionics.MindClosed
		_, err := svc.ContactPreview(context.Background(), 7, req)
		assert.ErrorIs(t, err, psionicsvc.ErrMindClosed)
	})
	t.Run("a power whose cost names no table", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(20), nil)
		req := contactReq(7)
		req.Power = "inflict_pain"
		_, err := svc.ContactPreview(context.Background(), 7, req)
		assert.ErrorIs(t, err, psionicsvc.ErrNotAContactPower)
	})
	t.Run("an unknown distance", func(t *testing.T) {
		svc, _ := newContactSvc(t, telepath(20), nil)
		req := contactReq(7)
		req.Distance = "far"
		_, err := svc.ContactPreview(context.Background(), 7, req)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "distance", fe.Field)
	})
}

// The cost table is served from the rule tables' rows, whatever the web does
// with them; a name no power's cost carries is not found.
func TestCostTable_ServesTheBandsAndModifierRows(t *testing.T) {
	svc, m := newContactSvc(t, telepath(20), nil)
	m.tables.EXPECT().ContactCostRows().Return([]ruletablesvc.ContactCostRow{{MinLevel: 1, MaxLevel: 2, Cost: 2}, {MinLevel: 3, MaxLevel: 4, Cost: 3}}, nil).Once()
	m.tables.EXPECT().ContactModifierRows().Return([]ruletablesvc.ContactModifierRow{{Kind: "distance", Code: "sight", Modifier: 0}, {Kind: "resistance", Code: "resisting", Modifier: -2}}, nil).Once()
	got, err := svc.CostTable(context.Background(), psionicsvc.ContactCostTable)
	require.NoError(t, err)
	assert.Equal(t, psionicsvc.CostTable{
		Table: "contact_cost", MaxLevel: ruletablesvc.MaxContactLevel,
		Bands:     []psionicsvc.CostBand{{MinLevel: 1, MaxLevel: 2, Cost: 2}, {MinLevel: 3, MaxLevel: 4, Cost: 3}},
		Modifiers: []psionicsvc.ContactLine{{Kind: "distance", Code: "sight"}, {Kind: "resistance", Code: "resisting", Modifier: -2}},
	}, got)

	_, err = svc.CostTable(context.Background(), "nothing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
