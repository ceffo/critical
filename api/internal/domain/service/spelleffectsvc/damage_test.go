package spelleffectsvc_test

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func damageEntry(d spelleffectsvc.Damage) spelleffectsvc.Entry {
	return spelleffectsvc.Entry{
		ID: "bolt", Name: "Bolt", Level: 3, Kind: spelleffectsvc.KindDamage,
		Confidence: spelleffectsvc.ConfidenceHigh, Target: spelleffectsvc.TargetArea,
		Duration: spelleffectsvc.Duration{Kind: spelleffectsvc.DurationInstant}, Damage: &d,
	}
}

func sum(c int, dice ...formula.Dice) formula.Sum { return formula.Sum{Dice: dice, Constant: c} }

func dice(count, sides int) formula.Dice { return formula.Dice{Count: count, Sides: sides} }

func TestEntry_DamageAt(t *testing.T) {
	p := func(src string) *formula.Formula { return must.Do(formula.Parse(src)) }
	tests := map[string]struct {
		damage spelleffectsvc.Damage
		level  int
		want   spelleffectsvc.DamageAt
	}{
		"capped dice at level 15": {
			spelleffectsvc.Damage{Dice: p("(min(level,10))d6"), Type: "fire"},
			15,
			spelleffectsvc.DamageAt{CasterLevel: 15, Dice: sum(0, dice(10, 6)), Instances: sum(1), Rounds: sum(1), Type: "fire"},
		},
		"level below the cap": {
			spelleffectsvc.Damage{Dice: p("(min(level,10))d6"), Type: "fire"},
			7,
			spelleffectsvc.DamageAt{CasterLevel: 7, Dice: sum(0, dice(7, 6)), Instances: sum(1), Rounds: sum(1), Type: "fire"},
		},
		"instances by level": {
			spelleffectsvc.Damage{Dice: p("1d4+1"), Instances: p("min(5,1+(level-1)/2)"), Type: "force"},
			15,
			spelleffectsvc.DamageAt{CasterLevel: 15, Dice: sum(1, dice(1, 4)), Instances: sum(5), Rounds: sum(1), Type: "force"},
		},
		"no instance below the first step": {
			spelleffectsvc.Damage{Dice: p("1d6+4d6"), Instances: p("level/5"), Type: "fire"},
			4,
			spelleffectsvc.DamageAt{CasterLevel: 4, Dice: sum(0, dice(1, 6), dice(4, 6)), Instances: sum(0), Rounds: sum(1), Type: "fire"},
		},
		"rounds by level": {
			spelleffectsvc.Damage{Dice: p("2d4"), Rounds: p("1+level/3"), Type: "acid"},
			15,
			spelleffectsvc.DamageAt{CasterLevel: 15, Dice: sum(0, dice(2, 4)), Instances: sum(1), Rounds: sum(6), Type: "acid"},
		},
		"rounds never below one": {
			spelleffectsvc.Damage{Dice: p("2d4"), Rounds: p("level-5"), Type: "acid"},
			2,
			spelleffectsvc.DamageAt{CasterLevel: 2, Dice: sum(0, dice(2, 4)), Instances: sum(1), Rounds: sum(1), Type: "acid"},
		},
		"by-level row that applies": {
			spelleffectsvc.Damage{ByLevel: []spelleffectsvc.LevelDice{
				{MinLevel: 1, Dice: p("1d6")}, {MinLevel: 10, Dice: p("3d6")}, {MinLevel: 20, Dice: p("5d6")},
			}},
			15,
			spelleffectsvc.DamageAt{CasterLevel: 15, Dice: sum(0, dice(3, 6)), Instances: sum(1), Rounds: sum(1)},
		},
		// Missiles by level with a floor and a cap: one at level 7, one more
		// every three levels, seven at most.
		"force missiles: one missile at level 7": {
			spelleffectsvc.Damage{Dice: p("2d4+level"), Instances: p("min(7,max(1,1+(level-7)/3))"), Type: "force"},
			7,
			spelleffectsvc.DamageAt{CasterLevel: 7, Dice: sum(7, dice(2, 4)), Instances: sum(1), Rounds: sum(1), Type: "force"},
		},
		"force missiles: three missiles at level 15": {
			spelleffectsvc.Damage{Dice: p("2d4+level"), Instances: p("min(7,max(1,1+(level-7)/3))"), Type: "force"},
			15,
			spelleffectsvc.DamageAt{CasterLevel: 15, Dice: sum(15, dice(2, 4)), Instances: sum(3), Rounds: sum(1), Type: "force"},
		},
		"force missiles: seven missiles at most": {
			spelleffectsvc.Damage{Dice: p("2d4+level"), Instances: p("min(7,max(1,1+(level-7)/3))"), Type: "force"},
			30,
			spelleffectsvc.DamageAt{CasterLevel: 30, Dice: sum(30, dice(2, 4)), Instances: sum(7), Rounds: sum(1), Type: "force"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := damageEntry(tt.damage).DamageAt(tt.level)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, *got)
		})
	}
}

// A cast rolls no damage for these entries, so none is served.
func TestEntry_DamageAt_None(t *testing.T) {
	low := damageEntry(spelleffectsvc.Damage{Dice: must.Do(formula.Parse("2d6"))})
	low.Confidence, low.Note = spelleffectsvc.ConfidenceLow, "unsure"
	tests := map[string]spelleffectsvc.Entry{
		"utility":        utility(),
		"low confidence": low,
		"no damage data": func() spelleffectsvc.Entry { e := damageEntry(spelleffectsvc.Damage{}); e.Damage = nil; return e }(),
	}
	for name, e := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := e.DamageAt(15)
			require.NoError(t, err)
			assert.Nil(t, got)
		})
	}
}
