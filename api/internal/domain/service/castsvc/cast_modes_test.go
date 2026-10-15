package castsvc_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

var (
	wallOfIce    = slot("wall_of_ice", "Wall of Ice", 4)
	iceStorm     = slot("ice_storm", "Ice Storm", 4)
	meteorSwarm  = slot("meteor_swarm", "Meteor Swarm", 9)
	fireballSlot = slot("fireball", "Fireball", 3)
)

func ones(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = 1
	}
	return out
}

// ADR 0008 step 14: a cast in a mode rolls exactly that mode's damage (the
// falling ice sheet: 3d10), spends the slot, and the result and the entry
// record the mode's key.
func TestCast_Mode_RollsTheChosenModesDamage(t *testing.T) {
	f := newFixture(t, 7, 3, 9)
	f.prepared(wallOfIce)
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{Mode: "ice_sheet"})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals, "every queued die is rolled")
	assert.Equal(t, []int{10, 10, 10}, f.dice.sides, "three d10, no other die")
	assert.Equal(t, "ice_sheet", res.Mode)
	require.NotNil(t, res.Damage)
	assert.Equal(t, "3d10", res.Damage.Dice)
	assert.Equal(t, "cold", res.Damage.Type)
	assert.Equal(t, 19, res.Damage.Total)
	assert.Equal(t, castsvc.TextOnlyNone, res.TextOnly)
	require.Len(t, f.saved, 1)
	assert.True(t, f.saved[0][0].Used, "the slot is spent")
	require.Len(t, f.entries, 1)
	assert.JSONEq(t, `"ice_sheet"`, string(payloadField(t, f.entries[0].Payload, "mode")))
	assert.JSONEq(t, `19`, string(payloadField(t, payloadField(t, f.entries[0].Payload, "damage"), "total")))
}

// ADR 0008 step 14: eight small spheres of 5d4 when picked, four large ones
// of 10d4 by default (the first mode listed).
func TestCast_Mode_DefaultIsTheFirst(t *testing.T) {
	tests := []struct {
		name               string
		slot               spellsvc.SpellSlot
		mode, wantMode     string
		instances, perRoll int
		sides              int
	}{
		{"meteor swarm small spheres", meteorSwarm, "small_spheres", "small_spheres", 8, 5, 4},
		{"meteor swarm, no mode: large spheres", meteorSwarm, "", "large_spheres", 4, 10, 4},
		{"ice storm, no mode: hail", iceStorm, "", "hail", 1, 3, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, ones(tt.instances*tt.perRoll)...)
			f.prepared(tt.slot)
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{Mode: tt.mode})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, slices.Repeat([]int{tt.sides}, tt.instances*tt.perRoll), f.dice.sides)
			assert.Equal(t, tt.wantMode, res.Mode)
			require.NotNil(t, res.Damage)
			assert.Equal(t, tt.instances, res.Damage.Instances)
			assert.Len(t, res.Damage.Rolls, tt.instances)
			assert.Equal(t, tt.instances*tt.perRoll, res.Damage.Total)
			assert.JSONEq(t, `"`+tt.wantMode+`"`, string(payloadField(t, f.entries[0].Payload, "mode")))
		})
	}
}

// ADR 0008 step 14: a mode without damage rolls nothing and still spends the
// slot (the ice plane is Wall of Ice's default; sleet is picked).
func TestCast_Mode_WithoutDamageSpendsTheSlot(t *testing.T) {
	tests := []struct {
		name     string
		slot     spellsvc.SpellSlot
		mode     string
		wantMode string
	}{
		{"wall of ice, no mode: ice plane", wallOfIce, "", "ice_plane"},
		{"wall of ice hemisphere", wallOfIce, "hemisphere", "hemisphere"},
		{"ice storm sleet", iceStorm, "sleet", "sleet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t) // no die may be rolled
			f.prepared(tt.slot)
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{Mode: tt.mode})
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, res.Mode)
			assert.Nil(t, res.Damage)
			assert.Equal(t, castsvc.TextOnlyNone, res.TextOnly, "the mode is the effect, not a text-only spell")
			require.Len(t, f.saved, 1)
			assert.True(t, f.saved[0][0].Used, "the slot is spent")
			require.Len(t, f.entries, 1)
			assert.JSONEq(t, `"`+tt.wantMode+`"`, string(payloadField(t, f.entries[0].Payload, "mode")))
		})
	}
}

// ADR 0008 step 14: a mode the spell does not list, or any mode for a spell
// without modes, is refused with nothing written, rolled or logged.
func TestCast_Mode_UnknownIsRefused(t *testing.T) {
	tests := []struct {
		name string
		slot spellsvc.SpellSlot
		mode string
	}{
		{"unknown key", wallOfIce, "fog"},
		{"another spell's key", iceStorm, "ice_sheet"},
		{"spell without modes", fireballSlot, "hail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.prepared(tt.slot)
			_, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{Mode: tt.mode})
			require.ErrorIs(t, err, spelleffectsvc.ErrUnknownMode)
			assert.Empty(t, f.saved, "no slot written")
			assert.Empty(t, f.entries, "nothing logged")
			assert.Empty(t, f.dice.sides, "nothing rolled")
		})
	}

	f := newFixture(t)
	_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "ice_storm", CasterLevel: 9, Mode: "fog"})
	require.ErrorIs(t, err, spelleffectsvc.ErrUnknownMode)
	assert.Empty(t, f.entries, "nothing logged")
	assert.Zero(t, f.tx.calls, "refused before the transaction")
}

// ADR 0008 step 14: a spell received from someone else picks its mode too,
// and rolls it at the caster's level.
func TestReceive_Mode_AtTheCastersLevel(t *testing.T) {
	f := newFixture(t, ones(8*5)...)
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "meteor_swarm", CasterLevel: 18, Mode: "small_spheres"})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals)
	assert.Equal(t, "small_spheres", res.Mode)
	assert.Equal(t, 18, res.CasterLevel)
	require.NotNil(t, res.Damage)
	assert.Equal(t, "5d4", res.Damage.Dice)
	assert.Equal(t, 8, res.Damage.Instances)
	assert.Equal(t, 40, res.Damage.Total)
	assert.JSONEq(t, `"small_spheres"`, string(payloadField(t, f.entries[0].Payload, "mode")))

	f = newFixture(t)
	res, err = f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "ice_storm", CasterLevel: 9, Mode: "sleet"})
	require.NoError(t, err)
	assert.Equal(t, "sleet", res.Mode)
	assert.Nil(t, res.Damage, "sleet rolls nothing")
}
