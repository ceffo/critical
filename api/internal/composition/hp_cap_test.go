package composition_test

// HP cap on the real stack (critical-qki.38). User, 2026-10-13: "a player
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

// HP-4: voiding the damage a capped heal was measured against is refused
// (it would leave HP above max); voiding the heal first, then the damage,
// works and ends at max.
func TestHPCap_VoidDamageAfterCappedHeal(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()
	_, dmg := r.change(-10, "ogre club")
	hp, heal := r.change(15, "cure serious wounds")
	require.Equal(t, r.max, hp)
	require.Equal(t, 10, heal.HPDelta)

	_, err := r.bs.Log.Void(ctx, r.bs.CharacterID, dmg.ID)
	require.ErrorIs(t, err, actionlogsvc.ErrAboveMaxHP)
	assert.Contains(t, err.Error(), "put HP 10 above max")
	assert.Equal(t, r.max, r.current())
	e, err := r.bs.Log.List(ctx, r.bs.CharacterID, 5)
	require.NoError(t, err)
	assert.Equal(t, actionlog.StatusActive, e[1].Status(), "damage not voided")

	_, err = r.bs.Log.Void(ctx, r.bs.CharacterID, heal.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max-10, r.current())
	_, err = r.bs.Log.Void(ctx, r.bs.CharacterID, dmg.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max, r.current())
}

// HP-4: undo (void latest) of damage taken after a heal is always fine.
func TestHPCap_UndoLatestDamage(t *testing.T) {
	r := newHPRig(t)
	r.change(-3, "rat")
	r.change(2, "bandage")
	r.change(-6, "trap")
	st, err := r.bs.HP.UndoLast(context.Background(), r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, r.max-1, st.CurrentHp)
}

// HP-3: amending a heal up is capped against the HP without it; amending
// damage down when a later heal was capped against it is refused.
func TestHPCap_Amend(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()
	_, dmg := r.change(-11, "troll")
	_, heal := r.change(4, "potion")
	require.Equal(t, r.max-7, r.current())

	e, err := r.bs.Log.Amend(ctx, r.bs.CharacterID, heal.ID, actionlogsvc.Amendment{HPDelta: 20})
	require.NoError(t, err)
	assert.Equal(t, 11, e.EffectiveHPDelta())
	assert.Equal(t, actionlog.HPBreakdown{Requested: 20, Applied: 11}, breakdown(t, *e))
	assert.Equal(t, r.max, r.current())

	_, err = r.bs.Log.Amend(ctx, r.bs.CharacterID, dmg.ID, actionlogsvc.Amendment{HPDelta: -2})
	require.ErrorIs(t, err, actionlogsvc.ErrAboveMaxHP)
	assert.Equal(t, r.max, r.current())

	e, err = r.bs.Log.Amend(ctx, r.bs.CharacterID, heal.ID, actionlogsvc.Amendment{HPDelta: -1})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(e.EffectivePayload()), "no stale breakdown on a heal amended to damage")
	assert.Equal(t, r.max-12, r.current())
}
