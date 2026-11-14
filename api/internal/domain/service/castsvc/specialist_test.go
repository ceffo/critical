package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// wizardOf is a level 15 wizard whose spellbook has the specialization spec
// (the zero value: a mage).
func wizardOf(class string, spec spellbook.Specialization) charactersvc.View {
	v := casterOf(class)
	v.Entries[0].Group = "wizard"
	v.Spellbook = &charactersvc.SpellbookRules{Specialization: spec}
	return v
}

// ADR 0050 step 11: an opposed spell is not prepared, "so is picking one for a
// blank extra slot (ADR 0008 step 9)", with step 6's code and nothing written.
// A transmuter's opposition is abjuration and necromancy (PHB Table 22, p.31);
// Alarm is abjuration and evocation (opposed wins, Q4). A spell of another
// school is picked as before, and a mage picks Alarm.
func TestCast_Step11_AnOpposedSpellIsRefusedInAnExtraSlot(t *testing.T) {
	transmuter := spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}}
	extra := spellsvc.SpellSlot{Level: 1, Extra: true}
	schools := map[string][]string{"alarm": {"abjuration", "invocation_evocation"}, "identify": {"lesser_divination"}}

	t.Run("a transmuter picks Alarm: refused", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), wizardOf("transmuter", transmuter))
		f.schools = schools
		f.prepared(extra)
		_, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "alarm"})
		require.ErrorIs(t, err, spellbook.ErrSchoolOpposed)
		assert.Empty(t, f.saved)
		assert.Empty(t, f.entries)
	})
	t.Run("a transmuter picks Identify (other)", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), wizardOf("transmuter", transmuter))
		f.schools = schools
		f.prepared(extra)
		res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "identify"})
		require.NoError(t, err)
		assert.Equal(t, "identify", res.SpellID)
	})
	t.Run("a mage picks Alarm", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), wizardOf("mage", spellbook.Specialization{}))
		f.schools = schools
		f.prepared(extra)
		res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "alarm"})
		require.NoError(t, err)
		assert.Equal(t, "alarm", res.SpellID)
	})
}

// ADR 0050 step 13 (PHB p.32): a specialist's own cast of a wizard spell of
// his school that allows a save tells the targets' -1 for the DM, on the
// answer and in the logged payload's save. The invoker (PHB Table 22:
// invocation/evocation) casting Fireball (a save for half): -1. A mage, or a
// transmuter, casting it: none, and no key. The invoker's Magic Missile
// allows no save: none. A Fireball that were also enchantment/charm is of his
// opposition too, and opposed wins (ADR 0050 step 5, Q4): none.
func TestCast_Step13_ASchoolSpellTellsTheTargetsPenalty(t *testing.T) {
	invoker := spellbook.Specialization{Schools: []string{"invocation_evocation"}, OppositionSchools: []string{"enchantment_charm", "conjuration_summoning"}}
	transmuter := spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}}
	schools := map[string][]string{"fireball": {"invocation_evocation"}, "magic_missile": {"invocation_evocation"}}
	tenD6 := []int{6, 5, 4, 3, 2, 1, 6, 5, 4, 3}
	for _, c := range []struct {
		name     string
		caster   charactersvc.View
		slot     spellsvc.SpellSlot
		dice     []int
		schools  []string // the slot's spell's schools, when not the table's
		want     int
		wantSave map[string]any
	}{
		{
			"an invoker's Fireball: -1", wizardOf("invoker", invoker), slot("fireball", "Fireball", 3), tenD6, nil,
			-1,
			map[string]any{"type": "spell", "effect": "half", "targetSaveModifier": float64(-1)},
		},
		{
			"a mage's Fireball: none", wizardOf("mage", spellbook.Specialization{}), slot("fireball", "Fireball", 3), tenD6, nil,
			0,
			map[string]any{"type": "spell", "effect": "half"},
		},
		{
			"a transmuter's Fireball: none", wizardOf("transmuter", transmuter), slot("fireball", "Fireball", 3), tenD6, nil,
			0,
			map[string]any{"type": "spell", "effect": "half"},
		},
		{
			"an invoker's Magic Missile, no save: none", wizardOf("invoker", invoker), slot("magic_missile", "Magic Missile", 1),
			[]int{1, 2, 3, 4, 1}, nil,
			0, nil,
		},
		{
			"an invoker's Fireball also of his opposition: none", wizardOf("invoker", invoker), slot("fireball", "Fireball", 3), tenD6,
			[]string{"invocation_evocation", "enchantment_charm"},
			0,
			map[string]any{"type": "spell", "effect": "half"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), c.caster, c.dice...)
			f.schools = schools
			if c.schools != nil {
				f.schools = map[string][]string{"fireball": c.schools}
			}
			f.prepared(c.slot)
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			assert.Equal(t, c.want, res.TargetSaveModifier)
			require.Len(t, f.entries, 1)
			var p map[string]any
			require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
			if c.wantSave == nil {
				assert.NotContains(t, p, "save")
				return
			}
			assert.Equal(t, c.wantSave, p["save"])
		})
	}
}
