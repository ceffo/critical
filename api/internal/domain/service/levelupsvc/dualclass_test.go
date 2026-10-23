package levelupsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
)

// dualElf is the party's elf: a fighter frozen at 9, then a mage realized at
// mageLevel owing pending (as Derive sends them: one class, ADR 0042 step 11's
// NoHitPoints up to the fighter's 9). The primary entry is the active mage.
func dualElf(mageLevel int, pending ...charactersvc.PendingLevel) charactersvc.View {
	return multi(15, 40, owing{class: "fighter", level: 9, frozen: true}, owing{class: "mage", level: mageLevel, pending: pending})
}

// noHP is a dual-class's level that gains no hit points (ADR 0042 step 11).
func noHP(level int) charactersvc.PendingLevel {
	return charactersvc.PendingLevel{Level: level, NoHitPoints: true}
}

// ADR 0042 step 11: "While the active entry's level is not above every frozen
// entry's left_at_level, a level-up of it gains no hit points: the level is
// still taken [...], and the level_up entry records a gain of 0 with the
// reason dual_class_no_hit_points. From the first level above it, ADR 0030
// step 7.1 and 7.2 for the new class, the full Constitution adjustment
// included." The elf's mage 9 (the fighter's 9) gains 0 without a die; mage
// 10 rolls a d4 of 3 plus Constitution 15's +1, undivided.
func TestRoll_Step11_NoHitPointsUntilTheCrossover(t *testing.T) {
	t.Run("mage 9: the level taken, 0 hit points, no die", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().View(mock.Anything, charID).Return(dualElf(8, noHP(9)), nil).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "", 9, 40).Return(nil).Once()
		got := f.recorded()

		res, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "mage", Level: 9})
		require.NoError(t, err)
		assert.Equal(t, 0, res.Gain)
		assert.Equal(t, levelupsvc.CodeNoHitPoints, res.Reason)
		assert.Nil(t, res.Rolled)
		assert.Equal(t, 40, res.HPMaxAfter)
		require.Len(t, *got, 1)
		var payload map[string]any
		require.NoError(t, json.Unmarshal((*got)[0].Payload, &payload))
		assert.Equal(t, "dual_class_no_hit_points", payload["reason"])
		assert.InDelta(t, 0, payload["gain"], 0)
	})
	t.Run("mage 10: a d4 plus the whole Constitution adjustment", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().View(mock.Anything, charID).Return(dualElf(9, d4(10)), nil).Once()
		f.dice.EXPECT().RollDie(4).Return(3).Once()
		f.rules.EXPECT().ConstitutionFor(15).Return(con(1, 1, 0), nil).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "", 10, 44).Return(nil).Once()
		f.recorded()

		res, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "mage", Level: 10})
		require.NoError(t, err)
		assert.Equal(t, 4, res.Gain)
		assert.Empty(t, res.Reason)
	})
	t.Run("a die entered for a level without hit points is refused", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().View(mock.Anything, charID).Return(dualElf(8, noHP(9)), nil).Once()

		_, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "mage", Level: 9, Die: intp(3)})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "die", fe.Field)
		assert.Equal(t, levelupsvc.CodeNoHitPoints, fe.Message)
	})
}
