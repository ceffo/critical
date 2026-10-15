package composition_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
)

// payloadOf is the payload of entry id, from the log.
func payloadOf(t *testing.T, ctx context.Context, bs interface {
	List(context.Context, int64, int) ([]actionlog.Entry, error)
}, charID, id int64,
) json.RawMessage {
	t.Helper()
	entries, err := bs.List(ctx, charID, 50)
	require.NoError(t, err)
	for _, e := range entries {
		if e.ID == id {
			return e.Payload
		}
	}
	t.Fatalf("entry %d not in the log", id)
	return nil
}

// ADR 0008 step 16: the cast and exposure entries record the exposure served
// in the spell lookup's wire shape (the UI reads both as one type), for every
// seed exposure (one damage, modes, dice steps), at two caster levels; the
// exposure endpoint serves the same.
func TestExposureRecord_MatchesTheLookupsWireShape(t *testing.T) {
	cat, err := spelleffectsadpt.New(filepath.Join("..", "..", "testdata", "seeds"))
	require.NoError(t, err)
	bs := bootMaxDice(t)
	ctx, id := context.Background(), bs.CharacterID
	n := 0
	for _, e := range cat.All() {
		if e.Exposure == nil {
			continue
		}
		for _, level := range []int{9, 15} {
			served, err := bs.Cast.ExposureAt(ctx, e.ID, level)
			require.NoError(t, err)
			require.NotNil(t, served, e.ID)
			want, err := json.Marshal(spellhdl.ExposureToDTO(served))
			require.NoError(t, err)

			cast, err := bs.Cast.Receive(ctx, id, castsvc.ReceiveRequest{SpellID: e.ID, CasterLevel: level})
			require.NoError(t, err)
			var p struct {
				Exposure json.RawMessage `json:"exposure"`
			}
			require.NoError(t, json.Unmarshal(payloadOf(t, ctx, bs.Log, id, cast.EntryID), &p))
			assert.JSONEq(t, string(want), string(p.Exposure), "%s cast entry at %d", e.ID, level)

			x, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: e.ID, CasterLevel: level})
			require.NoError(t, err)
			p.Exposure = nil
			require.NoError(t, json.Unmarshal(payloadOf(t, ctx, bs.Log, id, x.EntryID), &p))
			assert.JSONEq(t, string(want), string(p.Exposure), "%s exposure entry at %d", e.ID, level)
			dto, err := json.Marshal(casthdl.ExposureToDTO(x).Exposure)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(dto), "%s exposure result at %d", e.ID, level)
			n++
		}
	}
	assert.Equal(t, 2*3, n, "cloudkill, wall of fire, death fog")
}

// ADR 0008 step 16 with ADR 0011 step 1: an exposure the Bard takes at -8 HP
// (Cloudkill at max, 10) is cut by the floor to -2; the result and the entry
// say what the ledger applied.
func TestExposureOnTheBard_FloorCutsTheAppliedHP(t *testing.T) {
	bs := bootMaxDice(t)
	ctx, id := context.Background(), bs.CharacterID
	st, err := bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	_, err = bs.HP.AppendEvent(ctx, id, -(st.CurrentHp + 8), "troll")
	require.NoError(t, err)

	x, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 15, OnSelf: true})
	require.NoError(t, err)
	assert.Equal(t, 10, x.Damage.Total)
	assert.Equal(t, -2, x.HPDelta, "down to the floor")
	st, err = bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -10, st.CurrentHp)
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, -2, entries[0].EffectiveHPDelta())
	assert.Contains(t, string(entries[0].Payload), `"hpApplied":-2`)
	assert.Contains(t, string(entries[0].Payload), `"hpRequested":-10`)
}

// An exposure is a roll: after it, the older save can no longer be adjusted,
// and popping the exposure does not reopen it (a voided roll still ends the
// window, as for any roll kind, ADR 0006 step 6); the exposure rolls again.
func TestExposure_ClosesTheSaveAdjustmentUntilPopped(t *testing.T) {
	bs := bootMaxDice(t)
	ctx, id := context.Background(), bs.CharacterID
	save, err := bs.Combat.ResolveSavingThrow(ctx, id, combatsvc.SaveSpell)
	require.NoError(t, err)
	x, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 12})
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 2})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "the exposure is the latest roll")

	_, err = bs.Log.Undo(ctx, id, x.EntryID)
	require.NoError(t, err)
	_, err = bs.Combat.AdjustSave(ctx, id, save.LogEntryID, combatsvc.SaveAdjustment{Value: 2})
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "a voided roll still ends the window")
	again, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 12})
	require.NoError(t, err)
	assert.Equal(t, 1, again.Damage.Total, "round 1: 1 point")
}
