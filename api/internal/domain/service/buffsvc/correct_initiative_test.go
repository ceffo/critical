package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
)

// The corrector is built from the initiative store and the log alone: no
// buffs, no items, no weapons, no dice. That is what lets the root build it
// before both services (no late-bound dependency). It records the same
// initiative_correct entry as the service's own CorrectInitiative (ADR 0035
// item 6.2), and an out-of-combat call does nothing.
func TestInitiativeCorrector_ADR0035Item6_2_BuiltWithoutTheBuffsOrTheItems(t *testing.T) {
	ctx := context.Background()
	dal, log := mocks.NewMockDAL(t), mocks.NewMockLog(t)
	corrector, err := buffsvc.NewInitiativeCorrector(dal, log)
	require.NoError(t, err)
	wand := buffsvc.ItemInitiative("wand", "Wand of frost", "wand", 3, true)

	dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(2, nil).Once()
	dal.EXPECT().GetInitiative(mock.Anything, charID).Return(&buffsvc.Initiative{D10: 6, LogEntryID: 40}, nil).Once()
	var entry actionlog.NewEntry
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) { entry = e; return 41, nil }).Once()
	dal.EXPECT().SaveInitiative(mock.Anything, charID, &buffsvc.Initiative{D10: 6, LogEntryID: 41, Modifiers: []buffsvc.InitiativeModifier{wand}}).Return(nil).Once()

	got, err := corrector.CorrectInitiative(ctx, charID, wand)
	require.NoError(t, err)
	assert.Equal(t, 9, got.Total(), "the same d10 plus the item's +3")
	assert.Equal(t, actionlog.KindInitiativeCorrect, entry.Kind)
	assert.Equal(t, 2, entry.Round)

	dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, nil).Once()
	dal.EXPECT().GetInitiative(mock.Anything, charID).Return(nil, nil).Once()
	none, err := corrector.CorrectInitiative(ctx, charID, wand)
	require.NoError(t, err)
	assert.Nil(t, none, "out of combat: nothing recorded")
}

func TestNewInitiativeCorrector_RefusesNilPorts(t *testing.T) {
	_, err := buffsvc.NewInitiativeCorrector(nil, mocks.NewMockLog(t))
	assert.Error(t, err)
	_, err = buffsvc.NewInitiativeCorrector(mocks.NewMockDAL(t), nil)
	assert.Error(t, err)
}
