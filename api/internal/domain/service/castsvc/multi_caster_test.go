package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// multiOf is a multi-class character whose entries are given in order; the
// character's Group is the first entry's, as Derive sets it.
func multiOf(entries ...charactersvc.EntryDerived) *charactersvc.Character {
	return &charactersvc.Character{ID: charID, Class: entries[0].Class, Str: 12, Derived: charactersvc.Derived{
		RealizedLevel: entries[0].RealizedLevel, Group: entries[0].Group, Classes: entries,
	}}
}

// active is an active entry as Derive gives it; a wizard wears no body armour
// (the fixture's mage equipment, EntryDerived.NoArmour).
func active(class, group string, level int, casts bool) charactersvc.EntryDerived {
	return charactersvc.EntryDerived{
		Class: class, Group: group, State: classes.EntryActive, RealizedLevel: level, Casts: casts, NoArmour: group == "wizard",
	}
}

// ADR 0042 step 14 (by-group effects): a by-group gain reads every active
// class's group, and a multi-class character "use[s] the best die" (PHB,
// Strength). Group Might's line differs per group (warrior 1d8 cap 21,
// priest 1d10 cap 20, rogue 1d6 cap 19, wizard 1d4 cap 22): a mage/fighter
// rolls the warrior's d8 with its cap, a thief/cleric the priest's d10, by
// every way an effect reaches the character, though the first entry is the
// mage's and the thief's.
func TestEffects_Step14_ByGroupGainTakesTheBestDie(t *testing.T) {
	chars := []struct {
		name     string
		c        *charactersvc.Character
		die      int
		sides    int
		wantCap  int
		wantGain int
	}{
		{
			name: "mage/fighter", c: multiOf(active("mage", "wizard", 6, true), active("fighter", "warrior", 7, false)),
			die: 7, sides: 8, wantCap: 21, wantGain: 7,
		},
		{
			name: "thief/cleric", c: multiOf(active("thief", "rogue", 8, false), active("cleric", "priest", 7, true)),
			die: 9, sides: 10, wantCap: 20, wantGain: 9,
		},
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
		for _, tt := range chars {
			t.Run(way.name+"/"+tt.name, func(t *testing.T) {
				f := newFixtureOf(t, groupMight(t), tt.c, tt.die)

				res, err := way.run(f)
				require.NoError(t, err)
				assert.Empty(t, f.dice.vals, "every queued die is rolled")
				assert.Equal(t, []int{tt.sides}, f.dice.sides, "one die, the best group's")
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

// ADR 0042 step 14: "a preparation and a cast name the entry whose slot they
// use", with "its own caster level". A cleric 5/mage 9 casts stoneskin from
// the mage's slot at the mage's 9 (1d4 + 9/2 charges) and, in chain mail,
// is warned spell_in_armour (step 16.2: the mage's spells); from an untagged
// slot (the first spellcasting entry's, the cleric's) at 5 and unwarned.
func TestCast_Step14_TheSlotNamesItsEntry(t *testing.T) {
	clericMage := func() *charactersvc.Character {
		return multiOf(active("cleric", "priest", 5, true), active("mage", "wizard", 9, true))
	}
	chain := []itemsvc.Item{{ID: "a", Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: "metal"}}
	tests := []struct {
		name      string
		class     string
		wantLevel int
		warnings  []string
	}{
		{"the mage's slot", "mage", 9, []string{itemsvc.WarnSpellInArmour}},
		{"an untagged slot is the cleric's", "", 5, nil},
		{"the cleric's slot", "cleric", 5, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), clericMage(), 4)
			f.worn = chain
			s := slot("stoneskin", "Stoneskin", 4)
			s.Class = tt.class
			f.prepared(s)

			r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantLevel, r.CasterLevel)
			assert.Equal(t, tt.warnings, r.Warnings)
			require.Len(t, f.added, 1)
			assert.Equal(t, buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", SpellID: "stoneskin", Charges: 4 + tt.wantLevel/2}, f.added[0])
		})
	}
}

// ADR 0042 step 14 on ADR 0030 step 14: spell failure is the casting entry's.
// A cleric/mage at Wisdom 9 (Table 5: 20%, the cleric's spell_failure) rolls
// the d100 on a cast from the cleric's slot, and none from the mage's: the
// character's one chance (the sheet's, Derived.SpellFailure) is not read.
func TestCast_Step14_SpellFailureIsTheCastingEntrys(t *testing.T) {
	for _, tt := range []struct {
		name  string
		class string
		dice  []int
		sides []int
		want  *castsvc.SpellFailure
	}{
		{"the cleric's slot rolls the d100", "cleric", []int{21, 4}, []int{100, 4}, &castsvc.SpellFailure{Roll: 21, Chance: 20}},
		{"the mage's slot rolls none", "mage", []int{4}, []int{4}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cleric := active("cleric", "priest", 5, true)
			cleric.SpellFailure = 20
			c := multiOf(cleric, active("mage", "wizard", 9, true))
			c.Wis, c.Derived.SpellFailure = 9, 20
			f := newFixtureOf(t, catalog(t), c, tt.dice...)
			s := slot("stoneskin", "Stoneskin", 4)
			s.Class = tt.class
			f.prepared(s)

			r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, r.Failure)
			assert.Equal(t, tt.sides, f.dice.sides)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
		})
	}
}
