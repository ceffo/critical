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
)

// ADR 0042 step 14: each spellcasting entry has "its own caster level, the
// entry's realized level". A fighter 13/mage 6 casts stoneskin at the
// mage's 6, not the character's first (fighter) level 13: the cast names
// caster level 6 and the charges are 1d4 + 6/2 (one die, 4: 7 charges, where
// level 13 would give 10).
func TestCast_Step14_CasterLevelIsTheEntrysLevel(t *testing.T) {
	caster := &charactersvc.Character{ID: charID, Class: "fighter", Str: 17, Derived: charactersvc.Derived{
		RealizedLevel: 13, Group: "warrior",
		Classes: []charactersvc.EntryDerived{
			{Class: "fighter", Group: "warrior", State: classes.EntryActive, RealizedLevel: 13},
			{Class: "mage", Group: "wizard", State: classes.EntryActive, RealizedLevel: 6, Casts: true},
		},
	}}
	f := newFixtureOf(t, catalog(t), caster, 4)
	f.prepared(slot("stoneskin", "Stoneskin", 4))

	r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals, "every queued die is rolled")
	assert.Equal(t, 6, r.CasterLevel)
	require.Len(t, f.added, 1)
	assert.Equal(t, buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", SpellID: "stoneskin", Charges: 4 + 6/2}, f.added[0])
}
