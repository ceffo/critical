package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// healHPMax is the healer's max HP in these tests.
const healHPMax = 20

// healFixture is a caster of 20 max HP, wounded by wound HP (the ledger's
// first entry), over a catalogue of heals: Cure Light Wounds (1d8, self or
// other), Cure By Level (1d4+level) and Heal (full).
func healFixture(t *testing.T, wound int, dice ...int) *fixture {
	t.Helper()
	return healFixtureOf(t, casterOf("bard"), wound, dice...)
}

// healFixtureOf is healFixture for another caster.
func healFixtureOf(t *testing.T, v charactersvc.View, wound int, dice ...int) *fixture {
	t.Helper()
	c, err := spelleffectsadpt.Parse([]byte(`[
		{"id":"cure_light_wounds","name":"Cure Light Wounds","level":1,"kind":"heal","confidence":"high",
		 "target":"self_or_other","duration":{"kind":"permanent"},"heal":{"dice":"1d8"}},
		{"id":"cure_by_level","name":"Cure By Level","level":2,"kind":"heal","confidence":"high",
		 "target":"other","duration":{"kind":"instant"},"heal":{"dice":"1d4+level"}},
		{"id":"heal","name":"Heal","level":6,"kind":"heal","confidence":"high",
		 "target":"self_or_other","duration":{"kind":"permanent"},"heal":{"full":true}}]`), testGroups)
	require.NoError(t, err)
	v.HPMax = healHPMax
	f := newFixtureOf(t, c, v, dice...)
	if wound != 0 {
		f.entries = []actionlog.NewEntry{{Kind: actionlog.KindHPChange, HPDelta: -wound}}
	}
	return f
}

// ADR 0008 steps 17.4 and 17.5: cast on the caster, the dice roll once at
// his level and the cast entry, the one that spends the slot, carries the
// heal as its HP delta (the recording path caps it); the HP around it is the
// ledger's, capped at max HP, and at full HP nothing moves.
func TestCast_Heal_OnTheCasterThroughTheLedger(t *testing.T) {
	tests := map[string]struct {
		wound, die, before, after int
	}{
		"wounded":    {5, 3, 15, 18},
		"capped":     {2, 7, 18, 20},
		"at full HP": {0, 4, 20, 20},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := healFixture(t, tt.wound, tt.die)
			f.prepared(slot("cure_light_wounds", "Cure Light Wounds", 1))
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			assert.Equal(t, []int{8}, f.dice.sides, "one d8, nothing else")
			require.NotNil(t, res.Heal)
			assert.Equal(t, tt.die, res.Heal.Total)
			assert.False(t, res.Heal.OnOther)
			assert.Equal(t, tt.die, res.Heal.HPDelta, "asked for in full, the recording path caps")
			assert.Equal(t, intPtr(tt.before), res.Heal.HPBefore)
			assert.Equal(t, intPtr(tt.after), res.Heal.HPAfter)
			cast := f.entries[len(f.entries)-1]
			assert.Equal(t, actionlog.KindCast, cast.Kind)
			assert.Equal(t, tt.die, cast.HPDelta, "on the cast entry")
			assert.Equal(t, []string{"spell_slots"}, keys(cast.Snapshots), "no new snapshot key: one undo gives the slot back, the HP is the ledger's")
			assert.Equal(t, 1, f.tx.calls, "one transaction")
		})
	}
}

// ADR 0008 step 17.4: full rolls nothing and asks for the gap to max HP.
func TestCast_Heal_FullAsksTheGap(t *testing.T) {
	f := healFixture(t, 13)
	f.prepared(slot("heal", "Heal", 6))
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.sides, "nothing rolled")
	assert.Equal(t, &castsvc.HealResult{Full: true, Total: 13, HPDelta: 13, HPBefore: intPtr(7), HPAfter: intPtr(20)}, res.Heal)
	assert.Equal(t, 13, f.entries[len(f.entries)-1].HPDelta)
}

// ADR 0008 step 17.2: a heal received lands on the receiver and rolls its
// own dice at the caster level the player gives, even when the spell's
// default target is someone else.
func TestReceive_Heal_AtTheCastersLevel(t *testing.T) {
	f := healFixture(t, 15, 2)
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "cure_by_level", CasterLevel: 9})
	require.NoError(t, err)
	assert.Equal(t, []int{4}, f.dice.sides, "one d4")
	assert.Equal(t, 11, res.Heal.Total, "1d4 + level 9")
	assert.False(t, res.Heal.OnOther)
	assert.Equal(t, intPtr(16), res.Heal.HPAfter)
	assert.Equal(t, 11, f.entries[len(f.entries)-1].HPDelta)
}

// ADR 0008 step 17.6: cast on someone else (by choice or by the spell's
// default) the heal is rolled once and shown, the caster's entry moves no HP
// and the ledger is not read; full rolls nothing and has no amount.
func TestCast_Heal_OnSomeoneElseChangesNoHP(t *testing.T) {
	tests := map[string]struct {
		slot   string
		onSelf *bool
		dice   []int
		sides  []int
		total  int
	}{
		"chosen":            {"cure_light_wounds", boolPtr(false), []int{6}, []int{8}, 6},
		"the default other": {"cure_by_level", nil, []int{1}, []int{4}, 1 + bardLevel},
		"full":              {"heal", boolPtr(false), nil, nil, 0},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := healFixture(t, 5, tt.dice...)
			f.prepared(slot(tt.slot, tt.slot, 1))
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{OnSelf: tt.onSelf})
			require.NoError(t, err)
			assert.Equal(t, tt.sides, f.dice.sides)
			require.NotNil(t, res.Heal)
			assert.True(t, res.Heal.OnOther)
			assert.Equal(t, tt.total, res.Heal.Total)
			assert.Zero(t, res.Heal.HPDelta)
			assert.Nil(t, res.Heal.HPBefore)
			assert.Zero(t, f.entries[len(f.entries)-1].HPDelta, "the caster's HP does not move")
			assert.Contains(t, string(payloadField(t, f.entries[len(f.entries)-1].Payload, "heal")), `"onOther":true`)
		})
	}
}

// ADR 0008 step 17.2: a heal that fails (ADR 0030 step 14) heals nothing:
// the d100 and no heal die.
func TestCast_Heal_SpellFailureHealsNothing(t *testing.T) {
	f := healFixtureOf(t, clericCaster(), 5, 7)
	f.prepared(slot("cure_light_wounds", "Cure Light Wounds", 1))
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.True(t, res.Failure.Failed)
	assert.Nil(t, res.Heal)
	assert.Equal(t, []int{100}, f.dice.sides)
	assert.Zero(t, f.entries[len(f.entries)-1].HPDelta)
}

// ADR 0008 step 17.2: a heal is no effect to add: refused, nothing rolled
// or written.
func TestAddEffect_Heal_Refused(t *testing.T) {
	f := healFixture(t, 5)
	_, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: "cure_light_wounds"})
	require.ErrorIs(t, err, spelleffectsvc.ErrNotAnEffect)
	assert.Len(t, f.entries, 1, "only the wound")
	assert.Empty(t, f.dice.sides)
}

// ADR 0008 step 17.7: the entry's payload carries the heal: the dice, every
// die as rolls, the constant and the total, and the HP around it.
func TestCast_Heal_Payload(t *testing.T) {
	f := healFixture(t, 15, 3)
	_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "cure_by_level", CasterLevel: 4})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(payloadField(t, f.entries[len(f.entries)-1].Payload, "heal"), &got))
	assert.Equal(t, map[string]any{
		"dice": "1d4+level", "rolls": []any{map[string]any{"sides": 4.0, "value": 3.0}},
		"constant": 4.0, "total": 7.0, "hpBefore": 5.0, "hpAfter": 12.0,
	}, got)
}
