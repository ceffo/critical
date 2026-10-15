package composition_test

// Two clients popping the same top of the action stack at once (two tabs): the
// transaction takes the write lock when it begins, so exactly one pop succeeds
// and the other is refused with the stack's own conflict error, never a
// database error. File-backed WAL database, real concurrent goroutines.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// race runs both pops behind a start barrier and returns their errors.
func race(pops ...func() error) []error {
	errs := make([]error, len(pops))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, pop := range pops {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = pop()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

// exactlyOne checks that one pop succeeded and the other was refused with one
// of the stack's conflict errors (all mapped to 409), not a database error.
func exactlyOne(t *testing.T, errs []error, want ...error) {
	t.Helper()
	var failed []error
	for _, err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	require.Len(t, failed, 1, "exactly one pop succeeds: %v", errs)
	for _, w := range want {
		if errors.Is(failed[0], w) {
			return
		}
	}
	require.Failf(t, "not a stack conflict", "%v", failed[0])
}

func voidedCount(t *testing.T, bs *composition.BackingServices) int {
	t.Helper()
	entries, err := bs.Log.List(context.Background(), bs.CharacterID, 100)
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		if e.Status() == actionlog.StatusVoided {
			n++
		}
	}
	return n
}

func TestConcurrentPops_OneWinsTheOtherConflicts(t *testing.T) {
	const rounds = 15
	for name, tc := range map[string]struct {
		pop  func(bs *composition.BackingServices, top int64) func() error
		want []error
	}{
		"undo": {
			pop: func(bs *composition.BackingServices, top int64) func() error {
				return func() error {
					_, err := bs.Log.Undo(context.Background(), bs.CharacterID, top)
					return err
				}
			},
			// The loser sees an empty stack or another entry on top.
			want: []error{actionlogsvc.ErrTopChanged, actionlogsvc.ErrNothingToUndo},
		},
	} {
		t.Run(name, func(t *testing.T) {
			bs := newBacking(t)
			ctx := context.Background()
			for range rounds {
				_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -1, "claw")
				require.NoError(t, err)
				top := topID(t, bs)
				before := voidedCount(t, bs)

				errs := race(tc.pop(bs, top), tc.pop(bs, top))

				exactlyOne(t, errs, tc.want...)
				assert.Equal(t, before+1, voidedCount(t, bs), "popped once")
			}
		})
	}
}

// Two rerolls of the round's initiative run one after the other: each is its
// own transaction replacing the newest initiative (ADR 0018 step 5), so the
// second replaces the first's roll. Neither fails, nothing is voided, and the
// newest initiative entry keeps the one before it as its "before".
func TestConcurrentPops_InitiativeReroll(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, bs.CharacterID)
	require.NoError(t, err)
	for range 10 {
		before := voidedCount(t, bs)
		errs := race(
			func() error { _, err := bs.Buffs.RerollInitiative(ctx, bs.CharacterID); return err },
			func() error { _, err := bs.Buffs.RerollInitiative(ctx, bs.CharacterID); return err },
		)
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		assert.Equal(t, before, voidedCount(t, bs), "a reroll voids nothing")
		current := initiativeOf(t, bs)
		require.NotNil(t, current)
		entries, err := bs.Log.List(ctx, bs.CharacterID, 2)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, current.LogEntryID, entries[0].ID, "the second reroll is on top")
		assert.Equal(t, entries[0].ID, entries[0].TransactionID, "its own transaction")
		require.Len(t, entries[0].Snapshots, 1)
		assert.JSONEq(t, fmt.Sprintf(`{"d10":10,"entryId":%d}`, entries[1].ID), string(entries[0].Snapshots[0].Before),
			"it replaced the first reroll's roll")
	}
}
