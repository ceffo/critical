package composition_test

// HP cap on the real stack. User, 2026-10-13: "a player
// cannot go above its max hp. period. max hp grows when leveling up only".

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hpRig struct {
	t   *testing.T
	bs  *composition.BackingServices
	max int
}

func newHPRig(t *testing.T) *hpRig {
	t.Helper()
	bs := newBacking(t)
	st, err := bs.HP.GetState(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	require.Equal(t, st.MaxHp, st.CurrentHp, "starts at full HP")
	require.Greater(t, st.MaxHp, 20)
	return &hpRig{t: t, bs: bs, max: st.MaxHp}
}

// change logs an HP change and returns the new current HP and the entry.
func (r *hpRig) change(delta int, source string) (int, actionlog.Entry) {
	r.t.Helper()
	ctx := context.Background()
	st, err := r.bs.HP.AppendEvent(ctx, r.bs.CharacterID, delta, source)
	require.NoError(r.t, err)
	entries, err := r.bs.Log.List(ctx, r.bs.CharacterID, 1)
	require.NoError(r.t, err)
	require.Len(r.t, entries, 1)
	require.Equal(r.t, source, entries[0].Summary)
	return st.CurrentHp, entries[0]
}

func (r *hpRig) current() int {
	r.t.Helper()
	st, err := r.bs.HP.GetState(context.Background(), r.bs.CharacterID)
	require.NoError(r.t, err)
	return st.CurrentHp
}

func breakdown(t *testing.T, e actionlog.Entry) actionlog.HPBreakdown {
	t.Helper()
	var b actionlog.HPBreakdown
	require.NoError(t, json.Unmarshal(e.EffectivePayload(), &b))
	return b
}

// HP-1: at max-1 a heal of 6 logs +1 and lands on max.
func TestHPCap_HealAtMaxMinusOne(t *testing.T) {
	r := newHPRig(t)
	hp, _ := r.change(-1, "scratch")
	require.Equal(t, r.max-1, hp)
	hp, e := r.change(6, "cure light wounds")
	assert.Equal(t, r.max, hp)
	assert.Equal(t, 1, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: 6, Applied: 1}, breakdown(t, e))
}

// HP-1: a heal exactly the size of the gap is logged in full and lands on max.
func TestHPCap_HealExactlyToMax(t *testing.T) {
	r := newHPRig(t)
	r.change(-7, "orc")
	hp, e := r.change(7, "potion")
	assert.Equal(t, r.max, hp)
	assert.Equal(t, 7, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: 7, Applied: 7}, breakdown(t, e))
}

// HP-1: the bug report's case, 80/85 + 12 -> 85: the excess is dropped when logged.
func TestHPCap_HealBiggerThanGap(t *testing.T) {
	r := newHPRig(t)
	r.change(-5, "goblin arrow")
	hp, e := r.change(12, "cure serious wounds")
	assert.Equal(t, r.max, hp)
	assert.Equal(t, 5, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: 12, Applied: 5}, breakdown(t, e))
	total, err := r.bs.Log.HPTotal(context.Background(), r.bs.CharacterID)
	require.NoError(t, err)
	assert.Zero(t, total, "the ledger itself holds no surplus")
}

// HP-2: at full HP the heal is still logged, as 0, with what was requested.
func TestHPCap_HealAtFullHPLogsZero(t *testing.T) {
	r := newHPRig(t)
	hp, e := r.change(9, "potion of healing")
	assert.Equal(t, r.max, hp)
	assert.Equal(t, actionlog.KindHPChange, e.Kind)
	assert.Equal(t, 0, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: 9, Applied: 0}, breakdown(t, e))
	// Damage is unchanged afterwards: no hidden surplus absorbs it.
	hp, _ = r.change(-4, "kobold")
	assert.Equal(t, r.max-4, hp)
}

// Checklist 2: at full HP a heal is logged as 0; undo-last pops that entry
// and HP stays at max.
func TestHPCap_UndoHealLoggedAtFullHP(t *testing.T) {
	r := newHPRig(t)
	_, e := r.change(8, "e2e potion at full")
	require.Equal(t, 0, e.HPDelta)
	st, err := r.bs.HP.UndoLast(context.Background(), r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, r.max, st.CurrentHp)
	got, err := r.bs.Log.List(context.Background(), r.bs.CharacterID, 1)
	require.NoError(t, err)
	assert.Equal(t, e.ID, got[0].ID)
	assert.Equal(t, actionlog.StatusVoided, got[0].Status())
}

// Checklist 3: damage 10, heal 4, pop the heal, pop the damage: HP is right
// after each step and back to the start.
func TestHPCap_PopHealThenDamage(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()
	hp, dmg := r.change(-10, "ogre club")
	require.Equal(t, r.max-10, hp)
	hp, heal := r.change(4, "cure light wounds")
	require.Equal(t, r.max-6, hp)
	require.Equal(t, 4, heal.HPDelta)

	_, err := r.bs.Log.Void(ctx, r.bs.CharacterID, dmg.ID)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "the damage is not the top of the stack")
	assert.Equal(t, r.max-6, r.current())

	_, err = r.bs.Log.Void(ctx, r.bs.CharacterID, heal.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max-10, r.current())
	_, err = r.bs.Log.Void(ctx, r.bs.CharacterID, dmg.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max, r.current())
}
