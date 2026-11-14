package castsvc_test

import (
	"context"
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
