package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// frozen is a dual-class's old class as Derive gives it: its left-at level,
// dormant before the crossover (ADR 0042 step 13).
func frozen(class, group string, leftAt int, casts, dormant bool) charactersvc.EntryView {
	return charactersvc.EntryView{
		Class: class, Group: group, State: classes.EntryFrozen, RealizedLevel: leftAt, LeftAtLevel: leftAt, Casts: casts,
		NoArmour: group == "wizard", Dormant: dormant,
	}
}

// ADR 0042 step 13: "an action that uses a frozen entry's feature, spell
// progression or equipment allowance before the crossover is allowed and
// carries the warning `dual_class_old_ability` (with the class), shown on the
// action and in its journal entry"; after the crossover nothing is warned.
// Step 14: the frozen entry casts at its left-at level. A mage left at 9 who
// is now a cleric 5 casts stoneskin from the mage's slot at 9, warned, the
// warning and the class in the cast's journal payload; from the cleric's slot
// unwarned. A mage left at 9 now a fighter 3 casts from an untagged slot (the
// mage is his one spellcasting entry) at 9, warned. Past the crossover (cleric
// 10), the mage's slot is not warned.
func TestCast_Step13_AnOldClassSpellWarnsBeforeTheCrossover(t *testing.T) {
	tests := []struct {
		name      string
		c         charactersvc.View
		slotClass string
		wantLevel int
		wantClass string
		warnings  []string
	}{
		{
			name:      "mage 9 to cleric 5: the mage's slot",
			c:         multiOf(frozen("mage", "wizard", 9, true, true), active("cleric", "priest", 5, true)),
			slotClass: "mage", wantLevel: 9, wantClass: "mage", warnings: []string{classes.WarnOldAbility},
		},
		{
			name:      "mage 9 to cleric 5: the cleric's slot",
			c:         multiOf(frozen("mage", "wizard", 9, true, true), active("cleric", "priest", 5, true)),
			slotClass: "cleric", wantLevel: 5, wantClass: "cleric",
		},
		{
			name:      "mage 9 to fighter 3: an untagged slot is the mage's",
			c:         multiOf(frozen("mage", "wizard", 9, true, true), active("fighter", "warrior", 3, false)),
			wantLevel: 9, wantClass: "mage", warnings: []string{classes.WarnOldAbility},
		},
		{
			name:      "mage 9 to cleric 10: past the crossover",
			c:         multiOf(frozen("mage", "wizard", 9, true, false), active("cleric", "priest", 10, true)),
			slotClass: "mage", wantLevel: 9, wantClass: "mage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), tt.c, 4)
			s := slot("stoneskin", "Stoneskin", 4)
			s.Class = tt.slotClass
			f.prepared(s)

			r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err, "a warning never refuses the cast")
			assert.Equal(t, tt.wantLevel, r.CasterLevel)
			assert.Equal(t, tt.warnings, r.Warnings, "on the action")
			assert.Equal(t, tt.wantClass, r.Class)
			require.Len(t, f.added, 1, "the spell still applies")
			assert.Equal(t, buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", SpellID: "stoneskin", Charges: 4 + tt.wantLevel/2}, f.added[0])

			var cast *actionlog.NewEntry
			for i := range f.entries {
				if f.entries[i].Kind == actionlog.KindCast {
					cast = &f.entries[i]
				}
			}
			require.NotNil(t, cast, "the cast is logged")
			var logged castsvc.CastBreakdown
			require.NoError(t, json.Unmarshal(cast.Payload, &logged))
			assert.Equal(t, tt.warnings, logged.Warnings, "in its journal entry")
			assert.Equal(t, tt.wantClass, logged.Class)
		})
	}
}
