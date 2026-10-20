package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// tumblerBed is a thief of Dexterity 17 with Tumbling (target 17) on the fixture's tables,
// through the real services and the in-memory store; dice hands out the d20s (and the d10s of
// Next Round's initiative) in order.
type tumblerBed struct {
	ctx  context.Context
	bs   *composition.BackingServices
	id   int64
	dice *attackDice
}

func newTumblerBed(t *testing.T, dies ...int) *tumblerBed {
	t.Helper()
	ctx := context.Background()
	dice := &attackDice{t: t, dies: dies, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "human", Str: 12, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints:   map[string]int{"open_locks": 30, "pick_pockets": 30},
		Proficiencies: skills.Picks{Nonweapon: []skills.Pick{{ID: "tumbling", Slots: 1}}},
	})
	require.NoError(t, err)
	return &tumblerBed{ctx: ctx, bs: bs, id: id, dice: dice}
}

// tumbling is the active Tumbling buffs.
func (b *tumblerBed) tumbling(t *testing.T) []buffsvc.BuffEffect {
	t.Helper()
	active, err := b.bs.Buffs.ListActive(b.ctx, b.id)
	require.NoError(t, err)
	var out []buffsvc.BuffEffect
	for _, x := range active {
		if x.ProficiencyID == "tumbling" {
			out = append(out, x)
		}
	}
	return out
}

func (b *tumblerBed) ac(t *testing.T) int {
	t.Helper()
	a, err := b.bs.AC.Get(b.ctx, b.id)
	require.NoError(t, err)
	return a.Value
}

func (b *tumblerBed) roll(t *testing.T) *skillsvc.CheckResult {
	t.Helper()
	r, err := b.bs.Skills.Roll(b.ctx, b.id, "tumbling", skillsvc.Adjustment{})
	require.NoError(t, err)
	return r
}

// top is the newest entry not popped.
func (b *tumblerBed) top(t *testing.T) actionlog.Entry {
	t.Helper()
	es, err := b.bs.Log.List(b.ctx, b.id, 50)
	require.NoError(t, err)
	for _, e := range es {
		if !e.Voided() {
			return e
		}
	}
	require.FailNow(t, "no entry on the stack")
	return actionlog.Entry{}
}

func (b *tumblerBed) undo(t *testing.T) {
	t.Helper()
	_, err := b.bs.Log.Undo(b.ctx, b.id, b.top(t).ID)
	require.NoError(t, err)
}

func (b *tumblerBed) redo(t *testing.T) {
	t.Helper()
	target, err := b.bs.Log.RedoTarget(b.ctx, b.id)
	require.NoError(t, err)
	require.NotNil(t, target)
	_, err = b.bs.Log.Redo(b.ctx, b.id, target.ID)
	require.NoError(t, err)
}

func (b *tumblerBed) nextRound(t *testing.T) {
	t.Helper()
	_, _, _, _, _, err := b.bs.Buffs.AdvanceRound(b.ctx, b.id)
	require.NoError(t, err)
}

// ADR 0037 steps 20.3 and 20.4, ADR 0018 and 0019 through the real services: a success puts the
// buff on in the check's one entry (its buffs snapshot), a Next Round later a second success
// renews it (one row, 1 round, a new start round); undoing the renewal gives the first row back
// with its rounds, undoing the first check takes it off, and redo puts it back.
func TestTumbling_Step20_UndoRemovesTheBuffAndRedoPutsItBack(t *testing.T) {
	b := newTumblerBed(t, 12, 4, 9)
	base := b.ac(t)

	first := b.roll(t)
	require.True(t, first.Success)
	require.NotNil(t, first.Buff)
	e := b.top(t)
	assert.Equal(t, actionlog.KindSkillCheck, e.Kind)
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, buffsvc.StateKeyBuffs, e.Snapshots[0].Key)
	on := b.tumbling(t)
	require.Len(t, on, 1)
	assert.Equal(t, 1, on[0].Duration)
	assert.Equal(t, -4, *on[0].Effects.AC)
	assert.Equal(t, base-4, b.ac(t))

	// A d10 for the new round's initiative (ADR 0013): the buff of round 0 ends at round 1.
	b.nextRound(t)
	require.Empty(t, b.tumbling(t))
	b.undo(t)
	require.Len(t, b.tumbling(t), 1, "Next Round undone: the buff is back")

	second := b.roll(t)
	require.True(t, second.Success)
	assert.True(t, second.Buff.Renewed)
	renewed := b.tumbling(t)
	require.Len(t, renewed, 1, "renewed, never stacked")
	assert.NotEqual(t, on[0].ID, renewed[0].ID)
	assert.Equal(t, 1, renewed[0].Duration)
	assert.Equal(t, base-4, b.ac(t), "-4 once")

	b.undo(t)
	assert.Equal(t, on, b.tumbling(t), "the first row back, as it was")
	b.undo(t)
	assert.Empty(t, b.tumbling(t), "undoing the check takes the buff off")
	assert.Equal(t, base, b.ac(t))
	b.redo(t)
	require.Len(t, b.tumbling(t), 1, "redo puts it back")
	assert.Equal(t, base-4, b.ac(t))
	assert.Equal(t, 3, b.dice.calls["die"], "two d20s and the round's d10, each used once")
}

// ADR 0037 step 20.8 and ADR 0010 step 6 through the real services: an adjustment that turns
// the success into a failure takes the buff off in its correction step, which carries the buffs
// snapshot; undoing the correction gives the buff back and the check its success; one that turns
// a failure into a success puts it on, and undoing it takes it off.
func TestSkillCheck_Step20_UndoingTheAdjustmentRestoresTheBuff(t *testing.T) {
	t.Run("success to failure", func(t *testing.T) {
		b := newTumblerBed(t, 15)
		check := b.roll(t)
		require.True(t, check.Success)
		row := b.tumbling(t)
		require.Len(t, row, 1)

		adj, err := b.bs.Skills.Adjust(b.ctx, b.id, check.LogEntryID, skillsvc.Adjustment{Value: -3})
		require.NoError(t, err)
		assert.False(t, adj.Success)
		assert.Nil(t, adj.Buff)
		assert.Empty(t, b.tumbling(t))
		step := b.top(t)
		assert.Equal(t, actionlog.KindCorrection, step.Kind)
		require.Len(t, step.Snapshots, 1)
		assert.Equal(t, buffsvc.StateKeyBuffs, step.Snapshots[0].Key)

		b.undo(t)
		assert.Equal(t, row, b.tumbling(t), "the buff back as it was")
		list, err := b.bs.Skills.List(b.ctx, b.id)
		require.NoError(t, err)
		for _, p := range list.Proficiencies {
			if p.ID == "tumbling" {
				require.NotNil(t, p.Last)
				assert.True(t, p.Last.Success, "the check's success back")
			}
		}
		b.redo(t)
		assert.Empty(t, b.tumbling(t), "redo takes it off again")
	})
	t.Run("failure to success", func(t *testing.T) {
		b := newTumblerBed(t, 19)
		check := b.roll(t)
		require.False(t, check.Success)
		require.Empty(t, b.tumbling(t))

		adj, err := b.bs.Skills.Adjust(b.ctx, b.id, check.LogEntryID, skillsvc.Adjustment{Value: 2})
		require.NoError(t, err)
		assert.True(t, adj.Success)
		require.NotNil(t, adj.Buff)
		require.Len(t, b.tumbling(t), 1)
		b.undo(t)
		assert.Empty(t, b.tumbling(t))
	})
}

// ADR 0037 step 20.1, ADR 0004 step 2 and ADR 0033 step 10.6 through the real services: the
// buff counts in the AC derived block (its own row, -4, named from the proficiency) until the
// next Next Round ends it.
func TestBuffs_Step20_TumblingEndsAtNextRound(t *testing.T) {
	b := newTumblerBed(t, 3, 6)
	base := b.ac(t)
	b.roll(t)
	a, err := b.bs.AC.Get(b.ctx, b.id)
	require.NoError(t, err)
	assert.Equal(t, base-4, a.Value)
	var found bool
	for _, r := range a.Rows {
		if r.ProficiencyID == "tumbling" {
			found = true
			assert.Equal(t, -4, r.Value)
		}
	}
	assert.True(t, found, "a row of its own, named from the proficiency")
	b.nextRound(t)
	assert.Empty(t, b.tumbling(t), "1 round: gone at the next Next Round")
	assert.Equal(t, base, b.ac(t))
	assert.Equal(t, 2, b.dice.calls["die"], "the d20 and the round's d10")
}
