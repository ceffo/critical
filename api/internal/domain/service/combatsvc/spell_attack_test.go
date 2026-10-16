package combatsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// a spell's to-hit is one d20 against THAC0 - AC with the
// Dex missile adjustment, no crit (no d100 is rolled); a natural 1 or 20 is plain arithmetic, as in combat.
func TestRollSpellToHit(t *testing.T) {
	tests := []struct {
		name     string
		d20, ac  int
		wantHit  bool
		wantNeed int
	}{
		{"exactly the number needed hits", 6, 5, true, 6},
		{"one below misses", 5, 5, false, 6},
		{"a natural 1 is plain arithmetic: it hits when the sum reaches the number needed (AC 10 needs 1)", 1, 10, true, 1},
		{"a natural 20 is plain arithmetic: it misses when the sum falls short", 20, -15, false, 26},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(bard()) // THAC0 13, STR 12 (no adjustment), DEX 17 (+2 missile)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12, buffsvc.WeaponNone).Return(strMods(12), nil)
			f.dice.EXPECT().RollD20().Return(tt.d20).Once() // the only roll: no d100, no damage dice

			res, err := f.svc.RollSpellToHit(context.Background(), charID, tt.ac)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHit, res.Hit)
			assert.Equal(t, 11, res.EffectiveThac0)
			assert.Equal(t, tt.wantNeed, res.TargetACNeeded)
			assert.Equal(t, []combatsvc.Modifier{
				{Kind: combatsvc.ModStrength, Detail: "12", Value: 0},
				{Kind: combatsvc.ModDexterity, Detail: "17", Value: 2},
			}, res.Modifiers)
		})
	}
}
