package composition_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// A note is logged on activation and on edit, and voiding the edit brings the
// previous note back (the note is part of the buffs snapshot).
func TestBuffNoteIsLoggedEditableAndVoidRestoresIt(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	buffs, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Ward", Duration: 5, Note: "  +4 saves vs fire  "})
	require.NoError(t, err)
	assert.Equal(t, "+4 saves vs fire", buffs[0].Note)

	buffs, err = bs.Buffs.Edit(ctx, id, buffs[0].ID, buffsvc.BuffEdit{Note: ptr("fire −50%")})
	require.NoError(t, err)
	assert.Equal(t, "fire −50%", buffs[0].Note)
	assert.Equal(t, 5, buffs[0].Duration)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Contains(t, string(entries[0].Payload), `"noteChanged":true`)
	assert.Contains(t, string(entries[0].Payload), `"fromNote":"+4 saves vs fire"`)
	assert.Contains(t, string(entries[1].Payload), `"note":"+4 saves vs fire"`)

	_, err = bs.Log.Void(ctx, id, entries[0].ID)
	require.NoError(t, err)
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "+4 saves vs fire", buffs[0].Note, "void restores the previous note")
}

func ptr[T any](v T) *T { return &v }

func hammer() buffsvc.SavedBuff {
	return buffsvc.SavedBuff{
		Name: "Hammer form", Duration: 10, Charges: 0, Note: "stuns",
		Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), MinStr: intPtr(18), SpellsBuff: floatPtr(2)},
	}
}

func TestSavedBuffs_StartEmptyAndRoundTrip(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	list, err := bs.Buffs.ListSaved(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, list, "nothing is seeded")

	list, err = bs.Buffs.SaveBuff(ctx, id, hammer())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotZero(t, list[0].ID)
	assert.Equal(t, hammer().Effects, list[0].Effects)
	assert.Equal(t, "stuns", list[0].Note)
	assert.Equal(t, 10, list[0].Duration)

	// Saving the same name again updates it instead of duplicating.
	again := hammer()
	again.Duration = 20
	list, err = bs.Buffs.SaveBuff(ctx, id, again)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 20, list[0].Duration)
}

// Deleting a template is logged, never touches active buffs, and voiding it
// brings the template back with the same id.
func TestSavedBuffs_DeleteIsLoggedAndVoidable(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	list, err := bs.Buffs.SaveBuff(ctx, id, hammer())
	require.NoError(t, err)
	savedID := list[0].ID
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Hammer form", Duration: 3})
	require.NoError(t, err)

	list, err = bs.Buffs.DeleteSaved(ctx, id, savedID)
	require.NoError(t, err)
	assert.Empty(t, list)
	active, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hammer form"}, names(active), "active buffs are untouched")
	_, err = bs.Buffs.DeleteSaved(ctx, id, savedID)
	assert.Error(t, err, "already gone")

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, actionlog.KindBuffChange, entries[0].Kind)
	assert.JSONEq(t, `{"action":"delete_template","buff":"Hammer form"}`, string(entries[0].Payload))

	_, err = bs.Log.Void(ctx, id, entries[0].ID)
	require.NoError(t, err)
	list, err = bs.Buffs.ListSaved(ctx, id)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, savedID, list[0].ID)
	assert.Equal(t, hammer().Effects, list[0].Effects)
	active, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hammer form"}, names(active))

	// Pop-only: the save sits under the activation, so it
	// is refused until the activation is popped; then the template goes.
	_, err = bs.Log.Void(ctx, id, entries[2].ID)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Log.Void(ctx, id, entries[1].ID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, id, entries[2].ID)
	require.NoError(t, err)
	list, err = bs.Buffs.ListSaved(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, list)
}

// A saved Power Gem form (weapon +5 at STR 18, dice ×2) activated from its
// template feeds the attack resolver: doubled dice, the crit multiplier on
// top.
func TestSavedBuffs_PowerGemFormDoublesDiceThenCrit(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.DB.ExecContext(ctx, `UPDATE characters SET str = 18 WHERE id = ?`, id)
	require.NoError(t, err)
	_, err = bs.Combat.SetCritTable(ctx, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 1.5}})
	require.NoError(t, err)

	list, err := bs.Buffs.SaveBuff(ctx, id, hammer())
	require.NoError(t, err)
	s := list[0]
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: s.Name, Duration: s.Duration, Charges: s.Charges, Effects: s.Effects, Note: s.Note})
	require.NoError(t, err)

	res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(1000)})
	require.NoError(t, err)
	require.True(t, res.IsCrit)
	assert.InDelta(t, 2, res.SpellsBuff, 0)
	assert.Equal(t, 5, res.WeaponBuff)
	assert.InDelta(t, 1.5, res.Multipliers, 0)
	want := int(math.Floor((float64(res.BaseDamage)*2+5)*1.5)) + res.StrengthBonus + res.BonusDamage
	assert.Equal(t, want, res.TotalDamage, "(dice ×2 + 5) × crit, then the strength bonus")
}

// Saving the same name again keeps one template; voiding the second save
// brings the first back with its old id and values.
func TestSavedBuffs_VoidingAReplacingSaveRestoresTheOldTemplate(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	first, err := bs.Buffs.SaveBuff(ctx, id, hammer())
	require.NoError(t, err)
	again := hammer()
	again.Duration = 20
	list, err := bs.Buffs.SaveBuff(ctx, id, again)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 20, list[0].Duration)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, id, entries[0].ID)
	require.NoError(t, err)
	list, err = bs.Buffs.ListSaved(ctx, id)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, first[0].ID, list[0].ID)
	assert.Equal(t, 10, list[0].Duration)
}

// While a delete is live, voiding the earlier save is refused and the error
// names the later entry.
func TestSavedBuffs_VoidingAnEarlierSaveIsBlockedByALiveDelete(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	list, err := bs.Buffs.SaveBuff(ctx, id, hammer())
	require.NoError(t, err)
	_, err = bs.Buffs.DeleteSaved(ctx, id, list[0].ID)
	require.NoError(t, err)
	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	_, err = bs.Log.Void(ctx, id, entries[1].ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("undo #%d (buff_change", entries[0].ID))

	// Void the delete: the chip is back, same id.
	_, err = bs.Log.Void(ctx, id, entries[0].ID)
	require.NoError(t, err)
	back, err := bs.Buffs.ListSaved(ctx, id)
	require.NoError(t, err)
	require.Len(t, back, 1)
	assert.Equal(t, list[0].ID, back[0].ID)
}

// A note-only edit on a charges-only buff (rounds 0) keeps the buff and logs
// a note change; clearing the note logs a cleared note.
func TestBuffNote_EditOnChargesOnlyBuffKeepsIt(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	buffs, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Stoneskin", Charges: 4})
	require.NoError(t, err)
	buffs, err = bs.Buffs.Edit(ctx, id, buffs[0].ID, buffsvc.BuffEdit{Note: ptr("vs arrows")})
	require.NoError(t, err)
	require.Len(t, buffs, 1)
	assert.Equal(t, 4, buffs[0].Charges)
	buffs, err = bs.Buffs.Edit(ctx, id, buffs[0].ID, buffsvc.BuffEdit{Note: ptr("")})
	require.NoError(t, err)
	require.Len(t, buffs, 1)
	assert.Empty(t, buffs[0].Note)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	assert.Contains(t, string(entries[1].Payload), `"note":"vs arrows"`)
	assert.Contains(t, string(entries[1].Payload), `"noteChanged":true`)
	assert.Contains(t, string(entries[0].Payload), `"fromNote":"vs arrows"`)
	assert.NotContains(t, string(entries[0].Payload), `"note":`+`"vs`)
}
