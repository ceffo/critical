package spelleffectsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
)

func buffEntry(target spelleffectsvc.Target, modifiers map[string]string) spelleffectsvc.Entry {
	e := spelleffectsvc.Entry{ID: "x", Kind: spelleffectsvc.KindBuff, Confidence: spelleffectsvc.ConfidenceHigh, Target: target}
	if modifiers != nil {
		e.Buff = &spelleffectsvc.Buff{Modifiers: map[string]*formula.Formula{}}
		for k, src := range modifiers {
			e.Buff.Modifiers[k] = must.Do(formula.Parse(src))
		}
	}
	return e
}

// rowEntry is a spell that adds no buff and lasts its level in rounds.
func rowEntry(kind spelleffectsvc.Kind, target spelleffectsvc.Target) spelleffectsvc.Entry {
	return spelleffectsvc.Entry{
		ID: "x", Kind: kind, Confidence: spelleffectsvc.ConfidenceLow, Target: target,
		Duration: spelleffectsvc.Duration{Kind: spelleffectsvc.DurationRounds, Rounds: must.Do(formula.Parse("level"))},
	}
}

// ADR 0027 step 10.1: a cast offers a choice of target only for a defensive
// buff it can put on the Bard that is not self-only: its default is where a
// cast naming no target lands, its alternate the other one. A hostile buff
// offers none.
func TestEntry_TargetChoiceAt(t *testing.T) {
	self := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}
	other := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}
	lowConfidence := buffEntry(spelleffectsvc.TargetSelfOrOther, nil)
	lowConfidence.Confidence = spelleffectsvc.ConfidenceLow
	abilitySet := buffEntry(spelleffectsvc.TargetSelfOrOther, nil)
	abilitySet.Buff = &spelleffectsvc.Buff{AbilitySet: map[string]string{"str": "18/00"}}
	tests := []struct {
		name  string
		entry spelleffectsvc.Entry
		want  *spelleffectsvc.TargetChoice
	}{
		{"self or other: on the Bard, else an ally", buffEntry(spelleffectsvc.TargetSelfOrOther, nil), self},
		{"other: on an ally, else the Bard", buffEntry(spelleffectsvc.TargetOther, map[string]string{"weapon_buff": "1"}), other},
		{"a friendly area: on the Bard, else not", buffEntry(spelleffectsvc.TargetArea, map[string]string{"move_pct": "200"}), self},
		{"other with a hostile buff (Blindness): no choice", buffEntry(spelleffectsvc.TargetOther, map[string]string{"to_hit": "-4", "attackers_to_hit": "4"}), nil},
		{"a hostile area (Malison): no choice", buffEntry(spelleffectsvc.TargetArea, map[string]string{"save": "-1"}), nil},
		{"self or other with a hostile buff: no choice", buffEntry(spelleffectsvc.TargetSelfOrOther, map[string]string{"ac": "2"}), nil},
		{"self-only: no choice", buffEntry(spelleffectsvc.TargetSelf, nil), nil},
		{"no target: no choice", buffEntry(spelleffectsvc.TargetNone, nil), nil},
		{"a low-confidence buff adds none: no choice", lowConfidence, nil},
		{"an ability set is applied by hand: no choice", abilitySet, nil},
		{"a damage spell: no choice", spelleffectsvc.Entry{Kind: spelleffectsvc.KindDamage, Confidence: spelleffectsvc.ConfidenceHigh, Target: spelleffectsvc.TargetOther}, nil},
		{"a utility spell that does not last: no choice", spelleffectsvc.Entry{Kind: spelleffectsvc.KindUtility, Confidence: spelleffectsvc.ConfidenceHigh, Target: spelleffectsvc.TargetArea}, nil},
		{"a damage spell that lasts: no choice", rowEntry(spelleffectsvc.KindDamage, spelleffectsvc.TargetOther), nil},
		{"a row-only spell on another: on an ally, else the Bard", rowEntry(spelleffectsvc.KindUtility, spelleffectsvc.TargetOther), other},
		{"a row-only spell on an area: not on the Bard, else him", rowEntry(spelleffectsvc.KindUtility, spelleffectsvc.TargetArea), other},
		{"a row-only spell, self or other: on the Bard, else an ally", rowEntry(spelleffectsvc.KindUtility, spelleffectsvc.TargetSelfOrOther), self},
		{"a row-only spell, self-only: no choice", rowEntry(spelleffectsvc.KindUtility, spelleffectsvc.TargetSelf), nil},
		{"a row-only spell with no target: no choice", rowEntry(spelleffectsvc.KindUtility, spelleffectsvc.TargetNone), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.entry.TargetChoiceAt(15)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Where a buff the Bard casts lands when he names no target.
func TestEntry_DefaultOnSelf(t *testing.T) {
	tests := []struct {
		name  string
		entry spelleffectsvc.Entry
		want  bool
	}{
		{"self", buffEntry(spelleffectsvc.TargetSelf, map[string]string{"ac": "4"}), true},
		{"self or other", buffEntry(spelleffectsvc.TargetSelfOrOther, nil), true},
		{"a friendly area", buffEntry(spelleffectsvc.TargetArea, nil), true},
		{"a hostile area", buffEntry(spelleffectsvc.TargetArea, map[string]string{"to_hit": "-4"}), false},
		{"other", buffEntry(spelleffectsvc.TargetOther, nil), false},
		{"no target", buffEntry(spelleffectsvc.TargetNone, nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.entry.DefaultOnSelf(15)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// A buff is hostile when a modifier hurts the bearer (Slow, Malison).
func TestEntry_Hostile(t *testing.T) {
	tests := []struct {
		name      string
		modifiers map[string]string
		want      bool
	}{
		{"no modifiers", nil, false},
		{"a to-hit penalty", map[string]string{"to_hit": "-4"}, true},
		{"a to-hit bonus", map[string]string{"to_hit": "1"}, false},
		{"a damage penalty", map[string]string{"damage": "-1"}, true},
		{"a save penalty (Malison)", map[string]string{"save": "-1"}, true},
		{"a save bonus", map[string]string{"save": "2"}, false},
		{"a worse AC", map[string]string{"ac": "4"}, true},
		{"a better AC", map[string]string{"ac": "-2"}, false},
		{"half movement", map[string]string{"move_pct": "50"}, true},
		{"double movement", map[string]string{"move_pct": "200"}, false},
		{"half attacks", map[string]string{"attacks_pct": "50"}, true},
		{"a better initiative is not hurting", map[string]string{"initiative": "-2"}, false},
		{"a penalty for attackers is not on the bearer", map[string]string{"attackers_to_hit": "-2"}, false},
		{"a penalty by level", map[string]string{"to_hit": "-(level/5)"}, true},
		{"a rolled penalty", map[string]string{"to_hit": "-1d4"}, true},
		{"haste", map[string]string{"move_pct": "200", "attacks_pct": "200", "initiative": "-2"}, false},
		{"slow", map[string]string{"move_pct": "50", "attacks_pct": "50", "ac": "4", "to_hit": "-4"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buffEntry(spelleffectsvc.TargetArea, tt.modifiers).Hostile(15)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
