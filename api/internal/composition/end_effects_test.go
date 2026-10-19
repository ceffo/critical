package composition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// ADR 0004 step 21 through the real services and storage, on the fixture's
// Polymorph Self (heals 1d12 when ended by the player and when it runs
// out): a mage at 3 of 9 HP. Each way the row ends is applied, then undone:
// one undo gives back both the row and the HP (ADR 0018 steps 2 and 4).
// The heal is capped at max HP (ADR 0007 step 4); a dispel heals nothing.
func TestBuffs_Step21_EndEffectsThroughTheLedgerAndUndo(t *testing.T) {
	dice := &attackDice{t: t, calls: map[string]int{}}
	tb := mageWith(t, dice)
	hp := func() int {
		st, err := tb.bs.Log.GetHPState(tb.ctx, tb.id)
		require.NoError(t, err)
		return st.CurrentHp
	}
	rows := func() []buffsvc.BuffStatus {
		got, err := tb.bs.Buffs.ListStatus(tb.ctx, tb.id)
		require.NoError(t, err)
		return got
	}
	undo := func() {
		t.Helper()
		top, ok, err := tb.bs.Log.TopID(tb.ctx, tb.id)
		require.NoError(t, err)
		require.True(t, ok)
		_, err = tb.bs.Log.Undo(tb.ctx, tb.id, top)
		require.NoError(t, err)
	}

	_, err := tb.bs.Log.AppendHPEvent(tb.ctx, tb.id, -6, "goblin")
	require.NoError(t, err)
	_, err = tb.bs.Cast.Receive(tb.ctx, tb.id, castsvc.ReceiveRequest{SpellID: "polymorph_self", CasterLevel: 15})
	require.NoError(t, err)
	require.Len(t, rows(), 1)
	require.Equal(t, 3, hp())
	row := rows()[0]
	assert.True(t, row.AsksEndCause, "its stop asks how it ended")

	t.Run("no cause is refused, nothing written", func(t *testing.T) {
		_, _, err := tb.bs.Buffs.Dismiss(tb.ctx, tb.id, row.ID, "")
		require.ErrorIs(t, err, spelleffectsvc.ErrEndCauseRequired)
		assert.Len(t, rows(), 1)
		assert.Equal(t, 3, hp())
	})

	tests := []struct {
		name   string
		dies   []int
		act    func() (buffsvc.EndResult, error)
		heal   int
		hpWant int
	}{
		{"ended by me heals the d12", []int{4}, func() (buffsvc.EndResult, error) {
			_, end, err := tb.bs.Buffs.Dismiss(tb.ctx, tb.id, row.ID, spelleffectsvc.EndVoluntary)
			return end, err
		}, 4, 7},
		{"the heal is capped at max HP", []int{11}, func() (buffsvc.EndResult, error) {
			_, end, err := tb.bs.Buffs.Dismiss(tb.ctx, tb.id, row.ID, spelleffectsvc.EndVoluntary)
			return end, err
		}, 11, 9},
		{"dispelled or slain heals nothing", nil, func() (buffsvc.EndResult, error) {
			_, end, err := tb.bs.Buffs.Dismiss(tb.ctx, tb.id, row.ID, spelleffectsvc.EndDispelled)
			return end, err
		}, 0, 3},
		{"running out on Next Round heals the d12, before the initiative's d10", []int{5, 7}, func() (buffsvc.EndResult, error) {
			one := 1
			_, err := tb.bs.Buffs.Edit(tb.ctx, tb.id, row.ID, buffsvc.BuffEdit{Duration: &one})
			require.NoError(t, err)
			_, expired, _, ini, end, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
			require.Len(t, expired, 1)
			assert.Equal(t, 7, ini.D10)
			return end, err
		}, 5, 8},
		{"running out when time passes heals the d12", []int{6}, func() (buffsvc.EndResult, error) {
			_, _, _, _, end, err := tb.bs.Buffs.PassTime(tb.ctx, tb.id, 6)
			return end, err
		}, 6, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dice.dies = tt.dies
			end, err := tt.act()
			require.NoError(t, err)
			assert.Empty(t, dice.dies, "every die used")
			assert.Empty(t, rows(), "the row ended")
			assert.Equal(t, tt.hpWant, hp())
			if tt.heal == 0 {
				assert.Empty(t, end.Effects)
			} else {
				require.Len(t, end.Effects, 1)
				assert.Equal(t, tt.heal, end.Effects[0].Total)
				assert.Equal(t, 3, end.HPBefore)
				assert.Equal(t, tt.hpWant, end.HPAfter)
			}
			undo()
			if len(tt.dies) == 2 { // the Edit to 1 round is its own transaction
				undo()
			}
			got := rows()
			require.Len(t, got, 1, "one undo brings the row back")
			assert.Equal(t, row.Duration, got[0].Duration)
			assert.Equal(t, 3, hp(), "and the HP")
		})
	}
}
