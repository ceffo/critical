package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// ADR 0004 step 20: casting Polymorph Self (utility, 20 rounds a level) at
// level 15 puts a 300-round row on the list, with its spell id and no
// effects, in the cast's one entry beside the slot, so voiding the cast
// removes it. Its rounds are a formula with no dice: nothing is rolled.
func TestBuffs_Step20_UtilityCastGetsARow(t *testing.T) {
	f := newFixture(t)
	f.prepared(slot("polymorph_self", "Polymorph Self", 4))
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.sides, "nothing rolled")
	require.Len(t, f.added, 1)
	assert.Equal(t, buffsvc.BuffEffect{ID: buffsAdded, Name: "Polymorph Self", SpellID: "polymorph_self", Duration: 300}, f.added[0])
	assert.Equal(t, castsvc.TextOnlyUtility, res.TextOnly, "still nothing numeric to track")
	require.Len(t, f.entries, 1, "one entry")
	assert.Equal(t, []string{prepsvc.StateKey, buffsvc.StateKeyBuffs}, keys(f.entries[0].Snapshots))
	assert.JSONEq(t, `{"name":"Polymorph Self","rounds":300,"applied":true,"id":"added-buff",
		"durationRoll":{"formula":"20*level","dice":[],"total":300},"effects":{}}`,
		string(payloadField(t, f.entries[0].Payload, "buff")))
}

// ADR 0004 step 20.5: a spell the Bard casts on someone else or on an area
// puts a row marked as not on him, with no effects, whatever its kind; a
// buff's rolls stay shown on the result for the one it landed on, and they
// are rolled once. A spell with no entry lands on him unless he says not.
func TestBuffs_Step20_NotOnTheBard(t *testing.T) {
	tests := []struct {
		name    string
		slot    string
		onSelf  *bool
		stats   string // the catalogue duration (a spell with no entry)
		dice    []int
		want    buffsvc.BuffEffect
		onOther bool
	}{
		{
			name: "sleep: an area utility spell", slot: "sleep", onOther: true,
			want: buffsvc.BuffEffect{Name: "Sleep", SpellID: "sleep", Duration: 75},
		},
		{
			name: "haste cast on another: the buff's 3 + 15 rounds, its effects not on the row", slot: "haste", onSelf: boolPtr(false), onOther: true,
			want: buffsvc.BuffEffect{Name: "Haste", SpellID: "haste", Duration: 18},
		},
		{
			name: "death fog: an area damage spell, its 1d4 + 15 rounds rolled once", slot: "death_fog", dice: ones(40), onOther: true,
			want: buffsvc.BuffEffect{Name: "Death Fog", SpellID: "death_fog", Duration: 16},
		},
		{
			name: "identify named on another", slot: "identify", onSelf: boolPtr(false), onOther: true,
			want: buffsvc.BuffEffect{Name: "Identify", SpellID: "identify", Duration: 15},
		},
		{
			name: "polymorph self has no target: on the Bard", slot: "polymorph_self",
			want: buffsvc.BuffEffect{Name: "Polymorph Self", SpellID: "polymorph_self", Duration: 300},
		},
		{
			name: "mirror image is self only: on-self false is ignored", slot: "mirror_image", onSelf: boolPtr(false), dice: []int{1},
			want: buffsvc.BuffEffect{Name: "Mirror Image", SpellID: "mirror_image", Duration: 45, Charges: 6},
		},
		{
			name: "a spell with no entry lands on the Bard", slot: "bless", stats: "6 rds.",
			want: buffsvc.BuffEffect{Name: "Text of bless", SpellID: "bless", Duration: 6},
		},
		{
			name: "a spell with no entry, named on another", slot: "bless", stats: "6 rds.", onSelf: boolPtr(false), onOther: true,
			want: buffsvc.BuffEffect{Name: "Text of bless", SpellID: "bless", Duration: 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			if tt.stats != "" {
				f.stats = map[string]spellstats.Stats{tt.slot: {Duration: spellstats.ParseDuration(tt.stats)}}
			}
			f.prepared(slot(tt.slot, tt.slot, 1))
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{OnSelf: tt.onSelf})
			require.NoError(t, err)
			require.Len(t, f.added, 1, "one row")
			want := tt.want
			want.ID, want.OnOther = buffsAdded, tt.onOther
			assert.Equal(t, want, f.added[0])
			require.NotNil(t, res.Buff)
			assert.Equal(t, tt.onOther, res.Buff.OnOther)
			assert.Equal(t, !tt.onOther, res.Buff.Applied)
			if tt.onOther {
				assert.JSONEq(t, `true`, string(payloadField(t, payloadField(t, f.entries[0].Payload, "buff"), "onOther")))
			}
			if tt.slot == "haste" {
				assert.Equal(t, buffsvc.BuffEffects{Initiative: intPtr(-2)}, res.Buff.Buff.Effects, "the rolls shown for the one it landed on")
			}
		})
	}
}

// ADR 0004 step 20.1: a spell with no effect entry (a Priest spell) takes
// its rounds from the catalogue's duration, rolled at the caster's level
// with exactly the dice it names; the row is on the Bard.
func TestBuffs_Step20_PriestDurationFromTheCatalogue(t *testing.T) {
	tests := []struct {
		name      string
		duration  string
		level     int
		dice      []int
		wantSides []int
		want      int
	}{
		{name: "bless: 6 rounds", duration: "6 rds.", level: 1, want: 6},
		{name: "a round a level", duration: "1 rd./level", level: 7, want: 7},
		{name: "a turn and a round a level", duration: "1 turn + 1 rd./level", level: 9, want: 19},
		{name: "2d10 rounds, both dice", duration: "2d10 rounds", level: 3, dice: []int{4, 9}, wantSides: []int{10, 10}, want: 13},
		{name: "1d4 hours", duration: "1d4 hours", level: 3, dice: []int{3}, wantSides: []int{4}, want: 180},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			f.stats = map[string]spellstats.Stats{"bless": {Duration: spellstats.ParseDuration(tt.duration)}}
			res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "bless", CasterLevel: tt.level})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die rolled")
			assert.Equal(t, tt.wantSides, f.dice.sides)
			require.Len(t, f.added, 1)
			assert.Equal(t, buffsvc.BuffEffect{ID: buffsAdded, Name: "Text of bless", SpellID: "bless", Duration: tt.want}, f.added[0])
			assert.Equal(t, castsvc.TextOnlyNoEffect, res.TextOnly)
			assert.True(t, res.Buff.Applied)
			require.Len(t, f.entries, 1)
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, keys(f.entries[0].Snapshots))
		})
	}
}

// ADR 0004 step 20.2: a spell that does not last a number of rounds puts no
// row; an "until discharged" buff keeps its charges and nothing more; a
// duration that comes to 0 rounds puts none.
func TestBuffs_Step20_NoRow(t *testing.T) {
	tests := []struct {
		name     string
		spellID  string
		duration string // the catalogue's, for a spell with no entry
		level    int
		dice     []int
		charges  int // the buff an until-discharged spell puts on
	}{
		{name: "permanent", spellID: "cure_light_wounds", duration: "Permanent"},
		{name: "instantaneous", spellID: "flame_strike", duration: "Instantaneous"},
		{name: "concentration", spellID: "thought_capture", duration: "Concentration"},
		{name: "special", spellID: "animal_friendship", duration: "Special"},
		{name: "until", spellID: "create_water", duration: "Until used"},
		{name: "time of chanting", spellID: "chant", duration: "Time of chanting"},
		{name: "0 rounds at this level", spellID: "command", duration: "1 turn/3 levels", level: 2},
		{name: "an instant entry", spellID: "dispel_magic"},
		{name: "a special entry", spellID: "alarm"},
		{name: "stoneskin: until discharged, its layers as today", spellID: "stoneskin", dice: []int{2}, charges: 2 + 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			if tt.duration != "" {
				f.stats = map[string]spellstats.Stats{tt.spellID: {Duration: spellstats.ParseDuration(tt.duration)}}
			}
			level := tt.level
			if level == 0 {
				level = 7
			}
			_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: tt.spellID, CasterLevel: level})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals)
			if tt.charges == 0 {
				assert.Empty(t, f.added, "no row")
				return
			}
			require.Len(t, f.added, 1)
			assert.Equal(t, 0, f.added[0].Duration)
			assert.Equal(t, tt.charges, f.added[0].Charges)
		})
	}
}
