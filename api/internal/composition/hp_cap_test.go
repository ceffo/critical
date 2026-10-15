package composition_test

// HP cap on the real stack. User, 2026-10-13: "a player
// cannot go above its max hp. period. max hp grows when leveling up only".

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
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

func (r *hpRig) condition() hpsvc.Condition {
	r.t.Helper()
	st, err := r.bs.HP.GetState(context.Background(), r.bs.CharacterID)
	require.NoError(r.t, err)
	return st.Condition
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
	st := journalUndo(t, r.bs)
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

	_, err := r.bs.Log.Undo(ctx, r.bs.CharacterID, dmg.ID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged, "the damage is not the top of the stack")
	assert.Equal(t, r.max-6, r.current())

	_, err = r.bs.Log.Undo(ctx, r.bs.CharacterID, heal.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max-10, r.current())
	_, err = r.bs.Log.Undo(ctx, r.bs.CharacterID, dmg.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max, r.current())
}

// ADR 0011 step 1 on the real stack: damage to below 0 is logged as typed,
// damage past -10 is logged clamped, at the floor further damage logs 0, and
// the ledger SUM is the shown HP throughout.
func TestHPFloor_DamagePastTheFloorIsClamped(t *testing.T) {
	r := newHPRig(t)
	hp, e := r.change(-(r.max + 4), "dragon breath")
	require.Equal(t, -4, hp)
	assert.Equal(t, -(r.max + 4), e.HPDelta, "below 0 but above the floor: as typed")

	hp, e = r.change(-9, "bleeding")
	assert.Equal(t, -10, hp)
	assert.Equal(t, -6, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: -9, Applied: -6}, breakdown(t, e))

	hp, e = r.change(-3, "kick")
	assert.Equal(t, -10, hp)
	assert.Equal(t, 0, e.HPDelta)
	assert.Equal(t, actionlog.HPBreakdown{Requested: -3, Applied: 0}, breakdown(t, e))

	total, err := r.bs.Log.HPTotal(context.Background(), r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, -10-r.max, total, "the ledger holds no damage past the floor")
	st, err := r.bs.HP.GetState(context.Background(), r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, hpsvc.ConditionDead, st.Condition)

	// A heal from the floor counts from -10, not from a hidden deficit.
	hp, _ = r.change(5, "cure light wounds")
	assert.Equal(t, -5, hp)
	assert.Equal(t, hpsvc.ConditionUnconscious, r.condition())

	// Healed to exactly a quarter of max: still badly wounded; one more: ok.
	quarter := r.max / 4
	hp, _ = r.change(quarter+5, "cure serious wounds")
	require.Equal(t, quarter, hp)
	assert.Equal(t, hpsvc.ConditionBadlyWounded, r.condition())
	_, _ = r.change(1, "potion")
	assert.Equal(t, hpsvc.ConditionOK, r.condition(), "the label clears above a quarter of max")
}

// ADR 0007 step 8 applies to clamped entries too: the row Void of a clamped
// entry that is not the top is refused and changes nothing; once the entries
// above it are popped, voiding it puts HP back exactly to the SUM before it.
func TestHPFloor_VoidClampedEntryBelowTheTop(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()
	before, _ := r.change(-(r.max - 2), "fireball")
	require.Equal(t, 2, before)
	hp, clamped := r.change(-30, "ogre club")
	require.Equal(t, -10, hp)
	require.Equal(t, -12, clamped.HPDelta)
	hp, heal := r.change(4, "cure light wounds")
	require.Equal(t, -6, hp)

	_, err := r.bs.Log.Undo(ctx, r.bs.CharacterID, clamped.ID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	assert.Equal(t, -6, r.current(), "a refused void changes nothing")

	_, err = r.bs.Log.Undo(ctx, r.bs.CharacterID, heal.ID)
	require.NoError(t, err)
	assert.Equal(t, -10, r.current())
	_, err = r.bs.Log.Undo(ctx, r.bs.CharacterID, clamped.ID)
	require.NoError(t, err)
	assert.Equal(t, before, r.current())
	total, err := r.bs.Log.HPTotal(ctx, r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, before-r.max, total, "HP is the ledger SUM")
}

// Undo of a clamped damage (the global undo) restores the exact HP
// before it, and undoing in order walks back each step.
func TestHPFloor_UndoClampedDamageRestoresExactHP(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()
	before, _ := r.change(-(r.max - 3), "fireball")
	require.Equal(t, 3, before)
	hp, e := r.change(-40, "ogre club")
	require.Equal(t, -10, hp)
	require.Equal(t, -13, e.HPDelta)

	// The global undo (ADR 0010) pops the clamped entry.
	popped, err := r.bs.Log.Undo(ctx, r.bs.CharacterID, e.ID)
	require.NoError(t, err)
	require.Equal(t, e.ID, popped.ID)
	st, err := r.bs.HP.GetState(ctx, r.bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, before, st.CurrentHp)
	assert.Equal(t, hpsvc.ConditionBadlyWounded, st.Condition)

	// Clamped again, then popped by the HP block's undo.
	hp, _ = r.change(-40, "ogre club again")
	require.Equal(t, -10, hp)
	st = journalUndo(t, r.bs)
	assert.Equal(t, before, st.CurrentHp)

	st = journalUndo(t, r.bs)
	assert.Equal(t, r.max, st.CurrentHp)
	assert.Equal(t, hpsvc.ConditionOK, st.Condition)
}

// A damage or healing entry without a source is stored with no player text,
// still counts against the HP ledger and can be undone.
func TestHP_NoSourceOnTheRealStack(t *testing.T) {
	r := newHPRig(t)
	ctx := context.Background()

	cur, e := r.change(-7, "")
	assert.Equal(t, r.max-7, cur)
	assert.Equal(t, actionlog.KindHPChange, e.Kind)
	assert.Empty(t, e.Summary)
	assert.Equal(t, -7, e.HPDelta)

	st, err := r.bs.HP.GetState(ctx, r.bs.CharacterID)
	require.NoError(t, err)
	require.NotEmpty(t, st.Events)
	assert.Empty(t, st.Events[0].Source)

	st = journalUndo(t, r.bs)
	assert.Equal(t, r.max, st.CurrentHp)
}

// A source-less HP change can be given a note, then voided: the journal keeps
// the note beside an empty source, the voided entry stops counting and HP is back at max.
func TestHP_AmendThenVoidSourceLessChange(t *testing.T) {
	r := newHPRig(t)
	ctx, id := context.Background(), r.bs.CharacterID

	_, e := r.change(-7, "")
	amended, err := r.bs.Log.Amend(ctx, id, e.ID, actionlogsvc.Amendment{Note: "trap"})
	require.NoError(t, err)
	assert.Equal(t, "trap", amended.Note())
	assert.Empty(t, amended.EffectiveSummary(), "an amend sets the note, never the source")
	assert.Equal(t, r.max-7, r.current(), "an amend never changes the delta")
	st, err := r.bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, st.Events[0].Source)

	_, err = r.bs.Log.Undo(ctx, id, e.ID)
	require.NoError(t, err)
	assert.Equal(t, r.max, r.current())
	entries, err := r.bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, actionlog.StatusVoided, entries[0].Status())
}
