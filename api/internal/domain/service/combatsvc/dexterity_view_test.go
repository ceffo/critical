package combatsvc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// strengthOnlyRules is the rule tables combat needs once Dexterity comes from
// the view (ADR 0046 step 2.2): Strength and the attack rate, no Dexterity
// table. It compiles only while combatsvc.Rules has no DexMods.
type strengthOnlyRules struct{ t *ruletablesvc.Tables }

func (r strengthOnlyRules) StrengthMods(str, exceptional int) (int, int, error) {
	return r.t.StrengthMods(str, exceptional)
}

func (r strengthOnlyRules) AttackRateFor(class string, level int, kind, weaponType string, specialized bool) (ruletablesvc.AttackRate, bool, error) {
	return r.t.AttackRateFor(class, level, kind, weaponType, specialized)
}

var _ combatsvc.Rules = strengthOnlyRules{}

// dexRows are PHB Table 2's missile column read at the sheet's score, plus one
// row that disagrees with the table (Dexterity 17 with +4) to show the view
// is what is read, not a table by the score.
var dexRows = []struct{ dex, missile int }{{3, -3}, {6, 0}, {14, 0}, {16, 1}, {17, 2}, {19, 3}, {25, 5}, {17, 4}}

// ADR 0046 step 2.2: an attack's Dexterity modifier is the missile adjustment
// the view carries; combat reads no Dexterity table (the rules stub has none).
// Each roll has its own value and is rolled once.
func TestAttack_Step2_MissileAdjustmentFromTheView(t *testing.T) {
	for _, row := range dexRows {
		t.Run(fmt.Sprintf("dex %d missile %+d", row.dex, row.missile), func(t *testing.T) {
			f := newFixtureWithRules(t, strengthOnlyRules{phb(t)})
			c := bard()
			c.Dex, c.DexMissile = row.dex, row.missile
			f.withChar(c)
			f.weapons.EXPECT().Get(mock.Anything, charID, "shortbow").Return(shortbow, nil).Once()
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
			f.items.EXPECT().Use(mock.Anything, charID, "arrows_plus3").Return(arrows3Left1, nil).Once()
			f.dice.EXPECT().RollD20().Return(14).Once()
			f.d100s(t, 50, 77)
			f.dice.EXPECT().RollDie(6).Return(4).Once()
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: new(2),
			})
			require.NoError(t, err)
			assert.Equal(t, modDex(row.dex, row.missile), res.Modifiers[0])
			assert.Equal(t, 13-(row.missile+2+3), res.EffectiveThac0, "dexterity, bow +2 and arrow +3")
		})
	}
}

// ADR 0046 step 2.2: a thrown spell's Dexterity modifier is the view's missile
// adjustment as well; the one d20 is rolled once.
func TestSpellAttack_Step2_MissileAdjustmentFromTheView(t *testing.T) {
	for _, row := range dexRows {
		t.Run(fmt.Sprintf("dex %d missile %+d", row.dex, row.missile), func(t *testing.T) {
			f := newFixtureWithRules(t, strengthOnlyRules{phb(t)})
			c := bard()
			c.Dex, c.DexMissile = row.dex, row.missile
			f.withChar(c)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, buffsvc.WeaponNone).Return(strMods(12), nil).Once()
			f.dice.EXPECT().RollD20().Return(9).Once()

			res, err := f.svc.RollSpellToHit(context.Background(), charID, 5)
			require.NoError(t, err)
			assert.Equal(t, modDex(row.dex, row.missile), res.Modifiers[1])
			assert.Equal(t, 13-row.missile, res.EffectiveThac0)
		})
	}
}
