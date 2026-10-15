package actionlogdal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
)

// /api/hp/undo with nothing left to undo is a no-op that returns the current
// state, on real SQLite through hpsvc and the action log.
func TestHPUndo_NothingToUndoIsNoop(t *testing.T) {
	cases := []struct {
		name   string
		deltas []int
		undos  int // undos before the checked one
		want   int
	}{
		{"empty log", nil, 0, 30},
		{"every hp entry voided", []int{-8, 3}, 2, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := openDB(t)
			charID := seedCharacter(t, db) // hp_max 30
			log := newLog(t, db, actionlogdal.New(db))
			hp, err := hpsvc.NewWithConfig(hpsvc.DefaultConfig(), hpdal.New(db), log)
			require.NoError(t, err)
			ctx := context.Background()
			for _, d := range c.deltas {
				_, err := hp.AppendEvent(ctx, charID, d, "x")
				require.NoError(t, err)
			}
			for range c.undos {
				_, err := hp.UndoLast(ctx, charID)
				require.NoError(t, err)
			}
			entriesBefore, err := log.List(ctx, charID, 0)
			require.NoError(t, err)

			state, err := hp.UndoLast(ctx, charID)
			require.NoError(t, err)
			assert.Equal(t, c.want, state.CurrentHp)
			assert.Equal(t, 30, state.MaxHp)
			assert.Empty(t, state.Events)

			entriesAfter, err := log.List(ctx, charID, 0)
			require.NoError(t, err)
			assert.Equal(t, entriesBefore, entriesAfter, "no new corrections")
		})
	}
}
