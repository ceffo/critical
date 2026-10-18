package turnsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/turnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/turnsvc/mocks"
)

const charID = int64(7)

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// table is Table 61 at a 4th-level cleric (PHB p.103) for the rows used.
var table = []ruletablesvc.TurningTarget{
	{Undead: "skeleton", HDMin: 1, HDMax: 1, Cell: ruletablesvc.TurningCell{Outcome: "turn"}},
	{Undead: "zombie", Cell: ruletablesvc.TurningCell{Roll: 4}},
	{Undead: "wight", HDMin: 5, HDMax: 5, Cell: ruletablesvc.TurningCell{Roll: 13}},
	{Undead: "vampire", HDMin: 9, HDMax: 9, Cell: ruletablesvc.TurningCell{Outcome: "cannot"}},
	{Undead: "ghoul", HDMin: 2, HDMax: 2, Cell: ruletablesvc.TurningCell{Outcome: "destroy"}},
	{Undead: "shadow", HDMin: 3, HDMax: 4, Cell: ruletablesvc.TurningCell{Outcome: "destroy_more"}},
}

type env struct {
	svc     turnsvc.SVC
	dice    *mocks.MockDice
	entries []actionlog.NewEntry
}

func newEnv(t *testing.T, turning []ruletablesvc.TurningTarget) *env {
	t.Helper()
	e := &env{dice: mocks.NewMockDice(t)}
	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{
		ID: charID, Class: "cleric", Derived: charactersvc.Derived{RealizedLevel: 4, Turning: turning},
	}, nil).Once()
	log := mocks.NewMockLog(t)
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n actionlog.NewEntry) (int64, error) {
		e.entries = append(e.entries, n)
		return 41, nil
	}).Maybe()
	svc, err := turnsvc.New(turnsvc.Deps{Characters: chars, Dice: e.dice, Log: log, Tx: fakeTx{}})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// rolls makes the dice answer values in order, sides checked, each once.
func (e *env) rolls(sidesValues ...int) {
	for i := 0; i < len(sidesValues); i += 2 {
		e.dice.EXPECT().RollDie(sidesValues[i]).Return(sidesValues[i+1]).Once()
	}
}

// ADR 0030 step 16: the cell of the cleric's realized level decides the
// dice. 16.1: a number rolls one d20, a success at or above it. 16.2: T, D
// and D* roll no d20. 16.4: a success rolls 2d6 affected, D* 2d4 more.
// 16.5: one turn_undead entry, no snapshot, with the type, the cell, the
// dice and the result.
func TestTurn_Step16_TheCellDecidesTheDice(t *testing.T) {
	tests := []struct {
		name   string
		undead string
		dice   []int
		want   turnsvc.Result
	}{
		{
			"a number, at it: turned", "wight",
			[]int{20, 13, 6, 2, 6, 5},
			turnsvc.Result{Undead: "wight", HDMin: 5, HDMax: 5, Cell: ruletablesvc.TurningCell{Roll: 13}, D20: 13, Success: true, Effect: turnsvc.EffectTurn, Affected: []int{2, 5}},
		},
		{
			"a number, under it: no 2d6", "wight",
			[]int{20, 12},
			turnsvc.Result{Undead: "wight", HDMin: 5, HDMax: 5, Cell: ruletablesvc.TurningCell{Roll: 13}, D20: 12},
		},
		{
			"T: turned with no d20", "skeleton",
			[]int{6, 3, 6, 4},
			turnsvc.Result{Undead: "skeleton", HDMin: 1, HDMax: 1, Cell: ruletablesvc.TurningCell{Outcome: "turn"}, Success: true, Effect: turnsvc.EffectTurn, Affected: []int{3, 4}},
		},
		{
			"D: destroyed with no d20", "ghoul",
			[]int{6, 1, 6, 6},
			turnsvc.Result{Undead: "ghoul", HDMin: 2, HDMax: 2, Cell: ruletablesvc.TurningCell{Outcome: "destroy"}, Success: true, Effect: turnsvc.EffectDestroy, Affected: []int{1, 6}},
		},
		{
			"D*: destroyed, 2d4 more", "shadow",
			[]int{6, 2, 6, 3, 4, 1, 4, 4},
			turnsvc.Result{Undead: "shadow", HDMin: 3, HDMax: 4, Cell: ruletablesvc.TurningCell{Outcome: "destroy_more"}, Success: true, Effect: turnsvc.EffectDestroy, Affected: []int{2, 3}, More: []int{1, 4}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, table)
			e.rolls(tc.dice...)
			got, err := e.svc.Turn(context.Background(), charID, tc.undead)
			require.NoError(t, err)
			want := tc.want
			want.EntryID, want.ClericLevel = 41, 4
			assert.Equal(t, want, *got)
			require.Len(t, e.entries, 1)
			assert.Equal(t, actionlog.KindTurnUndead, e.entries[0].Kind)
			assert.Empty(t, e.entries[0].Snapshots, "it changes no state")
			var payload map[string]any
			require.NoError(t, json.Unmarshal(e.entries[0].Payload, &payload))
			assert.Equal(t, tc.undead, payload["undead"])
			assert.InDelta(t, 4, payload["clericLevel"], 0)
			assert.Equal(t, want.Success, payload["success"])
		})
	}
	assert.True(t, actionlog.IsRoll(actionlog.KindTurnUndead), "a roll entry: it takes the result card")
}

// ADR 0030 step 16.3 and the request's refusals: a dash, an unknown type and
// a class that does not turn undead are refused before any die or entry.
func TestTurn_Step16_3_RefusalsRollNothing(t *testing.T) {
	e := newEnv(t, table)
	_, err := e.svc.Turn(context.Background(), charID, "vampire")
	require.ErrorIs(t, err, ruletablesvc.ErrCannotTurn)

	e = newEnv(t, table)
	_, err = e.svc.Turn(context.Background(), charID, "dracolich")
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	e = newEnv(t, nil)
	_, err = e.svc.Turn(context.Background(), charID, "skeleton")
	require.ErrorIs(t, err, turnsvc.ErrClassCannotTurn)
	assert.Empty(t, e.entries)
}
