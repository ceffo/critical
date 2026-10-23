package turnsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/turnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/turnsvc/mocks"
)

// ADR 0042 step 13: "an action that uses a frozen entry's feature [...]
// before the crossover is allowed and carries the warning
// dual_class_old_ability [...], shown on the action and in its journal entry.
// After the crossover nothing is warned." A cleric left at 7, now a fighter
// 3, turns at the cleric's 7, warned; as a fighter 8, not.
func TestTurn_Step13_AnOldClassTurnWarnsBeforeTheCrossover(t *testing.T) {
	for _, dormant := range []bool{true, false} {
		t.Run(map[bool]string{true: "fighter 3", false: "fighter 8"}[dormant], func(t *testing.T) {
			chars := mocks.NewMockCharacters(t)
			v := viewtest.Multi(viewtest.WithFrozen("cleric", 7), viewtest.WithEntry("fighter", 3))
			v.Turning = table
			v.Features = []charactersvc.FeatureGrant{{Kind: classes.KindTurnUndead, Class: "cleric", Level: 7, Dormant: dormant}}
			chars.EXPECT().View(mock.Anything, charID).Return(v, nil).Once()
			dice := mocks.NewMockDice(t)
			dice.EXPECT().RollDie(6).Return(3).Once()
			dice.EXPECT().RollDie(6).Return(4).Once()
			var entries []actionlog.NewEntry
			log := mocks.NewMockLog(t)
			log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n actionlog.NewEntry) (int64, error) {
				entries = append(entries, n)
				return 41, nil
			}).Once()
			svc, err := turnsvc.New(turnsvc.Deps{Characters: chars, Dice: dice, Log: log, Tx: fakeTx{}})
			require.NoError(t, err)

			res, err := svc.Turn(context.Background(), charID, "skeleton")
			require.NoError(t, err)
			assert.Equal(t, 7, res.ClericLevel, "the cleric entry's level")
			require.Len(t, entries, 1)
			if !dormant {
				assert.Empty(t, res.Warnings)
				assert.NotContains(t, string(entries[0].Payload), "warnings")
				return
			}
			assert.Equal(t, []string{classes.WarnOldAbility}, res.Warnings)
			assert.Contains(t, string(entries[0].Payload), `"warnings":["dual_class_old_ability"]`)
		})
	}
}
