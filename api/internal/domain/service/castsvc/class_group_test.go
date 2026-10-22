package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// casterOf is a level 15 character of class.
func casterOf(class string) *charactersvc.Character {
	return &charactersvc.Character{ID: charID, Class: class, Str: 12, Derived: charactersvc.Derived{RealizedLevel: bardLevel}}
}

// ADR 0030 step 2: the class group of the character picks a by-group effect.
// Strength (the seed: warrior 1d8, priest and rogue 1d6, wizard 1d4, max 18)
// cast on oneself rolls the die of one's own group, once.
func TestCast_Step2_StrengthRollsTheCastersGroupDie(t *testing.T) {
	tests := []struct {
		class     string
		die       int
		sides     int
		wantBonus int
	}{
		{class: "fighter", die: 7, sides: 8, wantBonus: 7},
		{class: "cleric", die: 5, sides: 6, wantBonus: 5},
		{class: "mage", die: 3, sides: 4, wantBonus: 3},
		{class: "thief", die: 4, sides: 6, wantBonus: 4},
		{class: "bard", die: 2, sides: 6, wantBonus: 2},
	}
	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), casterOf(tt.class), tt.die)
			f.prepared(slot("strength", "Strength", 2))

			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{OnSelf: boolPtr(true)})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, []int{tt.sides}, f.dice.sides, "one die of the group's size")
			require.NotNil(t, res.Buff)
			require.NotNil(t, res.Buff.StrGain)
			assert.Equal(t, tt.wantBonus, res.Buff.StrGain.Total)
			require.Len(t, f.added, 1)
			assert.Equal(t, intPtr(tt.wantBonus), f.added[0].Effects.StrBonus)
			assert.Equal(t, intPtr(18), f.added[0].Effects.StrMax)
		})
	}
}

// groupMight is a test-only Strength-type spell whose die and cap differ for
// every group, so a wrong group always shows.
func groupMight(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.Parse([]byte(`[{"id":"group_might","name":"Group Might","level":2,
	  "kind":"buff","confidence":"high","target":"self_or_other",
	  "duration":{"kind":"rounds","rounds":"10"},
	  "ability_gain":{"str":{
	    "by_group":{"warrior":"1d8","priest":"1d10","rogue":"1d6","wizard":"1d4"},
	    "max":18,
	    "max_by_group":{"warrior":21,"priest":20,"rogue":19,"wizard":22}}}}]`))
	require.NoError(t, err)
	return c
}

// ADR 0030 step 2: the group is the class's for every way an effect reaches
// the character: cast on oneself, received from another caster, added from
// the lookup. Each group gets its own die and its own cap.
func TestEffects_Step2_ByGroupFollowsTheCharactersClass(t *testing.T) {
	classes := []struct {
		class    string
		die      int
		sides    int
		wantCap  int
		wantGain int
	}{
		{class: "fighter", die: 8, sides: 8, wantCap: 21, wantGain: 8},
		{class: "cleric", die: 9, sides: 10, wantCap: 20, wantGain: 9},
		{class: "mage", die: 3, sides: 4, wantCap: 22, wantGain: 3},
		{class: "thief", die: 5, sides: 6, wantCap: 19, wantGain: 5},
	}
	ways := []struct {
		name string
		run  func(*fixture) (castsvc.CastResult, error)
	}{
		{name: "cast", run: func(f *fixture) (castsvc.CastResult, error) {
			f.prepared(slot("group_might", "Group Might", 2))
			return f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{OnSelf: boolPtr(true)})
		}},
		{name: "received", run: func(f *fixture) (castsvc.CastResult, error) {
			return f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "group_might", CasterLevel: 9})
		}},
		{name: "added as an effect", run: func(f *fixture) (castsvc.CastResult, error) {
			return f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: "group_might"})
		}},
	}
	for _, way := range ways {
		for _, tt := range classes {
			t.Run(way.name+"/"+tt.class, func(t *testing.T) {
				f := newFixtureOf(t, groupMight(t), casterOf(tt.class), tt.die)

				res, err := way.run(f)
				require.NoError(t, err)
				assert.Empty(t, f.dice.vals, "every queued die is rolled")
				assert.Equal(t, []int{tt.sides}, f.dice.sides, "one die of the group's size")
				require.NotNil(t, res.Buff)
				require.NotNil(t, res.Buff.StrGain)
				assert.Equal(t, tt.wantGain, res.Buff.StrGain.Total)
				require.Len(t, f.added, 1)
				assert.Equal(t, intPtr(tt.wantGain), f.added[0].Effects.StrBonus)
				assert.Equal(t, intPtr(tt.wantCap), f.added[0].Effects.StrMax)
			})
		}
	}
}

// A by-group effect on a character whose class has no group is refused
// before anything is rolled or written; an effect that is the same for
// everybody needs no group.
func TestEffects_Step2_UnknownClassRefusesAByGroupEffect(t *testing.T) {
	f := newFixtureOf(t, groupMight(t), casterOf("paladin"))
	_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "group_might", CasterLevel: 9})
	require.ErrorContains(t, err, `unknown class "paladin"`)
	assert.Empty(t, f.dice.sides)
	assert.Empty(t, f.added)
	assert.Empty(t, f.entries)

	f = newFixtureOf(t, catalog(t), casterOf("paladin"), 2)
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "stoneskin", CasterLevel: 10})
	require.NoError(t, err)
	assert.Equal(t, 7, res.Buff.Buff.Charges)
}
