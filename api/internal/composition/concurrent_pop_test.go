package composition_test

// Two clients popping the same top of the action stack at once (two tabs): the
// transaction takes the write lock when it begins, so exactly one pop succeeds
// and the other is refused with the stack's own conflict error, never a
// database error. File-backed WAL database, real concurrent goroutines.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"void": {
			pop: func(bs *composition.BackingServices, top int64) func() error {
				return func() error {
					_, err := bs.Log.Void(context.Background(), bs.CharacterID, top)
					return err
				}
			},
			want: []error{actionlogsvc.ErrAlreadyVoided, actionlogsvc.ErrNotLatest},
		},
		"void latest": {
			pop: func(bs *composition.BackingServices, _ int64) func() error {
				return func() error {
					_, err := bs.Log.VoidLatest(context.Background(), bs.CharacterID, actionlog.KindHPChange)
					return err
				}
			},
			// Both succeed when two entries are on the stack; see below.
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

				if tc.want != nil {
					exactlyOne(t, errs, tc.want...)
					assert.Equal(t, before+1, voidedCount(t, bs), "popped once")
					continue
				}
				// VoidLatest pops whatever HP entry is on top: with one entry the
				// loser finds nothing to pop (no error); it must never fail.
				assert.NoError(t, errs[0])
				assert.NoError(t, errs[1])
				assert.Equal(t, before+1, voidedCount(t, bs), "popped once")
			}
		})
	}
}

// Two rerolls of the round's initiative run one after the other: the second
// pops the first's roll. Neither fails, the first roll is voided once and
// exactly one initiative is left active.
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
		assert.Equal(t, before+2, voidedCount(t, bs), "each reroll popped the roll it replaced")
		assert.NotNil(t, initiativeOf(t, bs))
	}
}
