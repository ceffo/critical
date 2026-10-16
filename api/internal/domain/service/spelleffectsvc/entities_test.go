package spelleffectsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// ADR 0004 step 19: a spell adds a buff when its entry is a trusted buff that
// sets no ability; damage and utility entries never do.
func TestEntry_AddsBuff(t *testing.T) {
	high, low := spelleffectsvc.ConfidenceHigh, spelleffectsvc.ConfidenceLow
	tests := []struct {
		name  string
		entry spelleffectsvc.Entry
		want  bool
	}{
		{"a buff", spelleffectsvc.Entry{Kind: spelleffectsvc.KindBuff, Confidence: high}, true},
		{"a buff with modifiers", spelleffectsvc.Entry{Kind: spelleffectsvc.KindBuff, Confidence: high, Buff: &spelleffectsvc.Buff{}}, true},
		{"a low-confidence buff", spelleffectsvc.Entry{Kind: spelleffectsvc.KindBuff, Confidence: low}, false},
		{"an ability set", spelleffectsvc.Entry{Kind: spelleffectsvc.KindBuff, Confidence: high, Buff: &spelleffectsvc.Buff{AbilitySet: map[string]string{"str": "18/00"}}}, false},
		{"a damage spell", spelleffectsvc.Entry{Kind: spelleffectsvc.KindDamage, Confidence: high}, false},
		{"a utility spell", spelleffectsvc.Entry{Kind: spelleffectsvc.KindUtility, Confidence: high}, false},
		{"no entry", spelleffectsvc.Entry{}, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.entry.AddsBuff(), tt.name)
	}
}
