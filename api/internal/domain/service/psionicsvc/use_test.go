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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc/mocks"
)

// useBook: a 7-PSP devotion at Int -4, a power with a variable cost and its
// score in its text, and a power paid on contact.
func useBook() psionics.Book {
	return psionics.NewBook([]catalogue.Discipline{{ID: "telepathy"}}, []catalogue.Power{
		{
			ID: "thrust", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			Score: &catalogue.PowerScore{Ability: "int", Modifier: -4}, InitialCost: catalogue.Cost{Amount: 7},
		},
		{
			ID: "leap", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			InitialCost: catalogue.Cost{Kind: catalogue.CostVariable},
		},
		{
			ID: "touch", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			Score: &catalogue.PowerScore{Ability: "wis", Modifier: 0}, InitialCost: catalogue.Cost{Kind: catalogue.CostContact},
		},
		{
			ID: "unknown", Discipline: "telepathy", Kind: catalogue.KindDevotion,
			Score: &catalogue.PowerScore{Ability: "int"}, InitialCost: catalogue.Cost{Amount: 1},
		},
	})
}

// user is a psionicist with Int 18 who knows thrust (relearned once when
// relearn), leap and touch, with current PSPs of a 30 maximum.
func user(current int, relearn bool) *charactersvc.Character {
	picks := []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"},
		{Kind: psionics.PickPower, ID: "thrust"},
		{Kind: psionics.PickPower, ID: "leap"},
		{Kind: psionics.PickPower, ID: "touch"},
	}
	if relearn {
		picks = append(picks, psionics.Pick{Kind: psionics.PickRelearn, ID: "thrust"})
	}
	c := psionicist(picks...)
	c.Int, c.Wis = 18, 12
	c.Derived.PSP = &charactersvc.PSPPool{Current: current, Max: 30}
	return c
}

func newUseSvc(t *testing.T, c *charactersvc.Character) (psionicsvc.SVC, *mocks.MockDice, *mocks.MockLog) {
	t.Helper()
	chars, dice, log := mocks.NewMockCharacters(t), mocks.NewMockDice(t), mocks.NewMockLog(t)
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Rates: mocks.NewMockRecoveryRates(t), Log: log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: dice, Book: useBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, dice, log
}

// expectRecord captures the one entry Use logs.
func expectRecord(log *mocks.MockLog, got *actionlog.NewEntry) {
	log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { *got = e }).Return(int64(41), nil).Once()
}

func payloadOf(t *testing.T, e actionlog.NewEntry) psionicsvc.CheckRecord {
	t.Helper()
	var rec psionicsvc.CheckRecord
	require.NoError(t, json.Unmarshal(e.Payload, &rec))
	return rec
}

// ADR 0043 step 10, through the service with a dice mock: one d20 each, the
// outcome by the step's order. Thrust's target is 14 (Int 18 - 4) before the
// modifiers.
func TestPowerCheck_Step10_TheFourOutcomes(t *testing.T) {
	for _, c := range []struct {
		name    string
		mod     int
		roll    int
		target  int
		outcome string
		success bool
	}{
		{"20 at a target of 25 fails", 11, 20, 25, psionics.OutcomeNatural20, false},
		{"1 at a target of -2 succeeds", -16, 1, -2, psionics.OutcomeSuccess, true},
		{"a roll equal to 14 is power_score", 0, 14, 14, psionics.OutcomePowerScore, true},
		{"15 against 14 fails", 0, 15, 14, psionics.OutcomeFailure, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, dice, log := newUseSvc(t, user(30, false))
			dice.EXPECT().RollDie(psionics.CheckDie).Return(c.roll).Once()
			var got actionlog.NewEntry
			expectRecord(log, &got)

			res, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{
				Modifiers: []psionicsvc.Modifier{{Value: c.mod, Reason: "the DM"}},
			})
			require.NoError(t, err)
			rec := payloadOf(t, got)
			assert.Equal(t, c.target, rec.Target)
			assert.Equal(t, c.roll, rec.Roll)
			assert.Equal(t, c.outcome, rec.Outcome)
			assert.Equal(t, c.success, rec.Success)
			assert.Equal(t, rec, res.Check)
			assert.Equal(t, int64(41), res.LogEntryID)
		})
	}
}

// ADR 0043 step 10: the entry's payload is the check's breakdown: the
// ability and its score, the power's modifier, the relearns, each labelled
// modifier (a 0 dropped, reasons trimmed), the target.
func TestPowerCheck_Step10_TheBreakdown(t *testing.T) {
	svc, dice, log := newUseSvc(t, user(30, true))
	dice.EXPECT().RollDie(psionics.CheckDie).Return(9).Once()
	var got actionlog.NewEntry
	expectRecord(log, &got)

	_, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{Modifiers: []psionicsvc.Modifier{
		{Value: 2, Reason: "  the power's text  "}, {Value: 0, Reason: "nothing"}, {Value: -3, Reason: "the DM"},
	}})
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.CharacterID)
	assert.Equal(t, actionlog.KindPowerCheck, got.Kind)
	assert.Empty(t, got.Snapshots, "no maintenance row yet")
	var raw map[string]any
	require.NoError(t, json.Unmarshal(got.Payload, &raw))
	assert.Equal(t, map[string]any{
		"power": "thrust", "ability": "int", "abilityScore": float64(18), "powerModifier": float64(-4),
		"relearns": float64(1),
		"modifiers": []any{
			map[string]any{"value": float64(2), "reason": "the power's text"},
			map[string]any{"value": float64(-3), "reason": "the DM"},
		},
		"target": float64(14), "d20": float64(9), "outcome": "success", "success": true,
		"cost": float64(7), "spent": float64(7),
	}, raw)
}

// ADR 0043 step 11: a 7-PSP power spends 7 on a success and 4 on a failure,
// as the entry's PSP delta; the pool after it is answered.
func TestActivate_Step11_SuccessSpendsTheCostFailureHalfRoundedUp(t *testing.T) {
	for _, c := range []struct {
		name  string
		roll  int
		delta int
	}{
		{"a success spends 7", 3, -7},
		{"a failure spends 4", 17, -4},
		{"a natural 20 spends 4", 20, -4},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, dice, log := newUseSvc(t, user(10, false))
			dice.EXPECT().RollDie(psionics.CheckDie).Return(c.roll).Once()
			var got actionlog.NewEntry
			expectRecord(log, &got)

			res, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{})
			require.NoError(t, err)
			assert.Equal(t, c.delta, got.PSPDelta)
			assert.Equal(t, -c.delta, payloadOf(t, got).Spent)
			assert.Equal(t, actionlog.PSPState{Current: 10 + c.delta, Max: 30}, res.PSP)
		})
	}
}

// ADR 0043 step 11 (Q6): under the full initial cost the check is refused
// before the roll; exactly the cost is enough.
func TestActivate_Step11_RefusedUnderTheCost(t *testing.T) {
	svc, _, _ := newUseSvc(t, user(6, false))
	_, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{})
	require.ErrorIs(t, err, psionicsvc.ErrNotEnoughPSP)

	svc, dice, log := newUseSvc(t, user(7, false))
	dice.EXPECT().RollDie(psionics.CheckDie).Return(2).Once()
	var got actionlog.NewEntry
	expectRecord(log, &got)
	res, err := svc.Use(context.Background(), 7, "thrust", psionicsvc.UseRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.PSP.Current)
}

// ADR 0043 steps 8 and 11: a variable cost is the player's, at least 1; a
// score in the power's text is the player's too, the relearns and
// modifiers on top.
func TestActivate_Step11_AVariableCostAndAnEnteredScore(t *testing.T) {
	svc, dice, log := newUseSvc(t, user(30, false))
	dice.EXPECT().RollDie(psionics.CheckDie).Return(13).Once()
	var got actionlog.NewEntry
	expectRecord(log, &got)
	cost, score := 12, 11
	res, err := svc.Use(context.Background(), 7, "leap", psionicsvc.UseRequest{
		Cost: &cost, Score: &score, Modifiers: []psionicsvc.Modifier{{Value: 1}},
	})
	require.NoError(t, err)
	rec := payloadOf(t, got)
	assert.Equal(t, 12, rec.Target)
	require.NotNil(t, rec.EnteredScore)
	assert.Equal(t, 11, *rec.EnteredScore)
	assert.Empty(t, rec.Ability)
	assert.Equal(t, psionics.OutcomeFailure, rec.Outcome)
	assert.Equal(t, 12, rec.Cost)
	assert.Equal(t, -6, got.PSPDelta)
	assert.Equal(t, 24, res.PSP.Current)
}

// Every refusal comes before the roll and writes nothing (the dice and log
// mocks fail on any call).
func TestUse_RefusalsRollAndWriteNothing(t *testing.T) {
	zero, one, big, twelve := 0, 1, 26, 12
	for _, c := range []struct {
		name  string
		char  *charactersvc.Character
		power string
		req   psionicsvc.UseRequest
		err   error
		field string
	}{
		{"not a psionicist", &charactersvc.Character{ID: 7}, "thrust", psionicsvc.UseRequest{}, psionicsvc.ErrNotPsionicist, ""},
		{"a power of the catalogue not known", user(30, false), "unknown", psionicsvc.UseRequest{}, psionicsvc.ErrPowerNotKnown, ""},
		{"no such power", user(30, false), "nope", psionicsvc.UseRequest{}, psionicsvc.ErrPowerNotKnown, ""},
		{"a contact cost", user(30, false), "touch", psionicsvc.UseRequest{}, psionicsvc.ErrNeedsContact, ""},
		{"a variable cost not given", user(30, false), "leap", psionicsvc.UseRequest{Score: &twelve}, nil, "cost"},
		{"a variable cost of 0", user(30, false), "leap", psionicsvc.UseRequest{Cost: &zero, Score: &twelve}, nil, "cost"},
		{"a cost given for a fixed cost", user(30, false), "thrust", psionicsvc.UseRequest{Cost: &one}, nil, "cost"},
		{"a score not given for a power with none", user(30, false), "leap", psionicsvc.UseRequest{Cost: &one}, nil, "score"},
		{"a score over 25", user(30, false), "leap", psionicsvc.UseRequest{Cost: &one, Score: &big}, nil, "score"},
		{"a score given for a power with one", user(30, false), "thrust", psionicsvc.UseRequest{Score: &twelve}, nil, "score"},
		{"a modifier over 20", user(30, false), "thrust", psionicsvc.UseRequest{Modifiers: []psionicsvc.Modifier{{Value: 21}}}, nil, "modifiers"},
		{"a modifier under -20", user(30, false), "thrust", psionicsvc.UseRequest{Modifiers: []psionicsvc.Modifier{{Value: -21}}}, nil, "modifiers"},
		{"a variable cost over the PSPs", user(5, false), "leap", psionicsvc.UseRequest{Cost: &twelve, Score: &twelve}, psionicsvc.ErrNotEnoughPSP, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _, _ := newUseSvc(t, c.char)
			_, err := svc.Use(context.Background(), 7, c.power, c.req)
			if c.field != "" {
				var fe *domain.FieldError
				require.ErrorAs(t, err, &fe)
				assert.Equal(t, c.field, fe.Field)
				return
			}
			require.ErrorIs(t, err, c.err)
		})
	}
}
