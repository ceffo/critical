package spelleffectsvc_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

func damageWire(d *spelleffectsvc.DamageAt) *spelleffectsvc.DamageAtWire { return d.Wire() }

// ADR 0008 step 13: the served damage carries each part's dice and save
// effect key; a damage without parts serves none.
func TestDamageAtWire_Parts(t *testing.T) {
	d2d4 := formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}}
	got := damageWire(&spelleffectsvc.DamageAt{
		CasterLevel: 15, Dice: formula.Sum{Dice: d2d4.Dice, Constant: 15},
		Instances: formula.Sum{Constant: 3}, Rounds: formula.Sum{Constant: 1}, Type: "force",
		Parts: []spelleffectsvc.DamagePartAt{
			{Dice: d2d4, Save: spelleffectsvc.SaveNone},
			{Dice: formula.Sum{Constant: 15}, Save: spelleffectsvc.SaveNegates},
		},
	})
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"caster_level":15,"type":"force",
		"dice":{"groups":[{"count":2,"sides":4}],"modifier":15},
		"instances":{"groups":[],"modifier":3},"rounds":{"groups":[],"modifier":1},
		"parts":[{"dice":{"groups":[{"count":2,"sides":4}],"modifier":0},"save":"none"},
		         {"dice":{"groups":[],"modifier":15},"save":"negates"}]}`, string(raw))

	raw, err = json.Marshal(damageWire(&spelleffectsvc.DamageAt{CasterLevel: 1}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "parts")
}

// ADR 0008 step 15: the served damage carries the steps' unit and each
// step's number and dice (none: no groups, 0); a damage without steps serves
// neither.
func TestDamageAtWire_Steps(t *testing.T) {
	got := damageWire(&spelleffectsvc.DamageAt{
		CasterLevel: 12, Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 3}, Type: "fire",
		StepsPer: spelleffectsvc.StepPerRound, Steps: []spelleffectsvc.DamageStepAt{
			{Step: 1},
			{Step: 2, Dice: formula.Sum{Dice: []formula.Dice{{Count: 12, Sides: 2}}}},
			{Step: 3, Dice: formula.Sum{Dice: []formula.Dice{{Count: 12, Sides: 4}}, Constant: 1}},
		},
	})
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"caster_level":12,"type":"fire",
		"dice":{"groups":[],"modifier":0},
		"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":3},
		"steps_per":"round","steps":[
		  {"step":1,"dice":{"groups":[],"modifier":0}},
		  {"step":2,"dice":{"groups":[{"count":12,"sides":2}],"modifier":0}},
		  {"step":3,"dice":{"groups":[{"count":12,"sides":4}],"modifier":1}}]}`, string(raw))

	raw, err = json.Marshal(damageWire(&spelleffectsvc.DamageAt{CasterLevel: 1}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "steps")
}
