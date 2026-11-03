package psionicsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc/mocks"
)

// effectBook: displacement as the catalogue has it (Con -3, 6 PSPs, 3 a round,
// AC -2); a test power without maintenance whose effect lasts 4 rounds (no
// book power is one, ADR 0043 step 17); a maintained power without an effect.
func effectBook() psionics.Book {
	return psionics.NewBook([]catalogue.Discipline{{ID: "psychometabolism"}}, []catalogue.Power{
		{
			ID: "displacement", Discipline: "psychometabolism", Kind: catalogue.KindDevotion, Name: localized.Text{"en": "Displacement"},
			Score: &catalogue.PowerScore{Ability: "con", Modifier: -3}, InitialCost: catalogue.Cost{Amount: 6},
			Maintenance: &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 3}, Period: 1},
			Effect:      &catalogue.PowerEffect{AC: -2},
		},
		{
			ID: "steady_hand", Discipline: "psychometabolism", Kind: catalogue.KindDevotion, Name: localized.Text{"en": "Steady Hand"},
			Score: &catalogue.PowerScore{Ability: "con"}, InitialCost: catalogue.Cost{Amount: 2},
			Effect: &catalogue.PowerEffect{ToHit: 1, Damage: 2, WeaponKind: catalogue.WeaponKindMissile, Rounds: 4},
		},
		{
			ID: "hold", Discipline: "psychometabolism", Kind: catalogue.KindDevotion, Name: localized.Text{"en": "Hold"},
			Score: &catalogue.PowerScore{Ability: "con"}, InitialCost: catalogue.Cost{Amount: 1},
			Maintenance: &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 1}, Period: 1},
		},
	})
}

func newEffectSvc(t *testing.T) (psionicsvc.SVC, *mocks.MockDice, *mocks.MockLog, *mocks.MockBuffs) {
	t.Helper()
	c := psionicist(
		psionics.Pick{Kind: psionics.PickDiscipline, ID: "psychometabolism"},
		psionics.Pick{Kind: psionics.PickPower, ID: "displacement"},
		psionics.Pick{Kind: psionics.PickPower, ID: "steady_hand"},
		psionics.Pick{Kind: psionics.PickPower, ID: "hold"},
	)
	c.Con = 16
	c.Derived.PSP = &charactersvc.PSPPool{Current: 30, Max: 30}
	chars, dice, log, buffs := mocks.NewMockCharacters(t), mocks.NewMockDice(t), mocks.NewMockLog(t), mocks.NewMockBuffs(t)
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil).Maybe()
	buffs.EXPECT().GetRound(mock.Anything, int64(7)).Return(0, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Tables: mocks.NewMockTables(t), Log: log, Characters: chars, Picks: mocks.NewMockPicks(t),
		Dice: dice, Buffs: buffs, Book: effectBook(), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, dice, log, buffs
}

// expectRowAdd captures the row Use hands to buffsvc.
func expectRowAdd(buffs *mocks.MockBuffs, row *buffsvc.BuffEffect) {
	buffs.EXPECT().Add(mock.Anything, int64(7), mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			*row = b
			b.ID = "row-1"
			return b, rowSnap, nil
		}).Once()
}

// ADR 0043 step 17: a successful check puts the power's effect from the
// catalogue on its row, through one generic mapping (no power ids); the
// derived AC it moves is TestMaintenance_Step17_TheEffectMovesTheDerivedAC.
func TestActivate_Step17_AnEffectFromTheData(t *testing.T) {
	t.Run("a maintained power's untimed row carries it", func(t *testing.T) {
		svc, dice, log, buffs := newEffectSvc(t)
		dice.EXPECT().RollDie(psionics.CheckDie).Return(3).Once()
		var row buffsvc.BuffEffect
		expectRowAdd(buffs, &row)
		var got actionlog.NewEntry
		expectRecord(log, &got)

		res, err := svc.Use(context.Background(), 7, "displacement", psionicsvc.UseRequest{})
		require.NoError(t, err)
		assert.Equal(t, buffsvc.BuffEffect{
			Name: "Displacement", PowerID: "displacement", PowerName: localized.Text{"en": "Displacement"},
			MaintenanceCost: 3, MaintenancePeriod: 1, Effects: buffsvc.BuffEffects{AC: intPtr(-2)},
		}, row)
		assert.False(t, row.Timed(), "until stopped")
		assert.Equal(t, "row-1", res.BuffID)
		assert.Equal(t, []actionlog.Snapshot{rowSnap}, got.Snapshots)
	})
	t.Run("a power without maintenance lasts its rounds", func(t *testing.T) {
		svc, dice, log, buffs := newEffectSvc(t)
		dice.EXPECT().RollDie(psionics.CheckDie).Return(3).Once()
		var row buffsvc.BuffEffect
		expectRowAdd(buffs, &row)
		var got actionlog.NewEntry
		expectRecord(log, &got)

		res, err := svc.Use(context.Background(), 7, "steady_hand", psionicsvc.UseRequest{})
		require.NoError(t, err)
		assert.Equal(t, buffsvc.BuffEffect{
			Name: "Steady Hand", PowerID: "steady_hand", PowerName: localized.Text{"en": "Steady Hand"}, Duration: 4,
			Effects: buffsvc.BuffEffects{ToHit: intPtr(1), Damage: intPtr(2), WeaponKind: buffsvc.WeaponMissile},
		}, row)
		assert.True(t, row.Timed())
		assert.Zero(t, row.MaintenanceCost, "nothing to pay at Next Round")
		assert.Equal(t, "row-1", res.BuffID)
		assert.Equal(t, -2, got.PSPDelta)
	})
	t.Run("a failed check puts no row", func(t *testing.T) {
		for _, id := range []string{"displacement", "steady_hand"} {
			svc, dice, log, _ := newEffectSvc(t)
			dice.EXPECT().RollDie(psionics.CheckDie).Return(19).Once()
			var got actionlog.NewEntry
			expectRecord(log, &got)

			res, err := svc.Use(context.Background(), 7, id, psionicsvc.UseRequest{})
			require.NoError(t, err, id)
			assert.False(t, res.Check.Success, id)
			assert.Empty(t, res.BuffID, "%s: the buffs mock refuses any Add", id)
			assert.Empty(t, got.Snapshots, id)
		}
	})
	t.Run("a power without an effect keeps empty effects", func(t *testing.T) {
		svc, dice, log, buffs := newEffectSvc(t)
		dice.EXPECT().RollDie(psionics.CheckDie).Return(3).Once()
		var row buffsvc.BuffEffect
		expectRowAdd(buffs, &row)
		var got actionlog.NewEntry
		expectRecord(log, &got)

		_, err := svc.Use(context.Background(), 7, "hold", psionicsvc.UseRequest{})
		require.NoError(t, err)
		assert.Equal(t, buffsvc.BuffEffects{}, row.Effects)
		assert.Equal(t, 1, row.MaintenanceCost)
	})
}
