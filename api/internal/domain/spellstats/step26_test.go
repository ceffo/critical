package spellstats_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// ADR 0021 step 26.1: the importer types all six stats, each with the book's text as its raw,
// and the typed block survives the JSON the importer stores it as.
func TestSpellStats_Step26_1_AllSixFieldsAreTypedWithTheirRaw(t *testing.T) {
	raw := spellstats.Raw{
		CastingTime: "1 rd.", Range: "60 yds. + 10 yds./level", Duration: "3 rds./level",
		AreaOfEffect: "10-ft. radius", Components: "V, S, M", SavingThrow: "Neg.",
	}
	st := spellstats.Parse(raw)
	assert.Equal(t, raw.CastingTime, st.CastingTime.Raw)
	assert.Equal(t, raw.Range, st.Range.Raw)
	assert.Equal(t, raw.Duration, st.Duration.Raw)
	assert.Equal(t, raw.AreaOfEffect, st.Area.Raw)
	assert.Equal(t, raw.Components, st.Components.Raw)
	assert.Equal(t, raw.SavingThrow, st.Save.Raw)
	assert.Equal(t, "timed", st.CastingTime.Kind)
	assert.Equal(t, spellstats.RangeDistance, st.Range.Kind)
	assert.Equal(t, "timed", st.Duration.Kind)
	assert.Equal(t, spellstats.AreaArea, st.Area.Kind)
	assert.True(t, st.Components.V && st.Components.S && st.Components.M)
	assert.Equal(t, []string{spellstats.SaveNegates}, st.Save.Kinds)

	blob, err := json.Marshal(st)
	require.NoError(t, err)
	var back spellstats.Stats
	require.NoError(t, json.Unmarshal(blob, &back))
	assert.Equal(t, st, back)
}

// ADR 0021 step 26.2: a text the parser does not read keeps its raw as a special kind with no
// value at any caster level; a partial area keeps its reading and has none either.
func TestSpellStats_Step26_2_AnUnreadStatKeepsItsRawAndHasNoValue(t *testing.T) {
	st := spellstats.Parse(spellstats.Raw{
		CastingTime: "Some ritual", Range: "Within the temple", Duration: "Until the moon sets",
		AreaOfEffect: "Where the moon shines", Components: "V, S, a boon", SavingThrow: "At the DM's will",
	})
	assert.Equal(t, "special", st.CastingTime.Kind)
	assert.Equal(t, "Some ritual", st.CastingTime.Raw)
	assert.Equal(t, spellstats.KindSpecial, st.Range.Kind)
	assert.Equal(t, "Within the temple", st.Range.Raw)
	assert.Equal(t, "At the DM's will", st.Save.Raw)
	assert.Equal(t, []string{spellstats.SaveSpecial}, st.Save.Kinds)
	assert.True(t, st.Components.Special)
	assert.Empty(t, st.CastingTime.Label, "unread prose is never a label")
	assert.Empty(t, st.Range.Label)
	assert.Empty(t, st.Save.Label)

	assert.Equal(t, spellstats.KindSpecial, st.Area.Kind)
	assert.Equal(t, "Where the moon shines", st.Area.Raw)
	partial := spellstats.ParseArea("1 creature or 1 cu. ft. of substance/2 levels")
	assert.True(t, partial.Partial)
	assert.Equal(t, "1 creature or 1 cu. ft. of substance/2 levels", partial.Raw)

	for _, level := range []int{1, 9, 20} {
		v := st.At(level)
		assert.Zero(t, v.CastingTime, "casting time at level %d", level)
		assert.Zero(t, v.Range, "range at level %d", level)
		assert.Equal(t, spellstats.AreaValue{}, v.Area, "area at level %d", level)
		assert.Equal(t, spellstats.AreaValue{}, partial.At(level), "partial area at level %d", level)
	}
}

// ADR 0021 step 26.4: a common value of the book carries its label, from a closed list the
// parser owns, on the whole text only.
func TestParse_Step26_4_CommonValuesCarryTheirLabel(t *testing.T) {
	st := spellstats.Parse(spellstats.Raw{
		CastingTime: "Special", Range: "Special", Duration: "Special",
		AreaOfEffect: "Creature(s) touched", Components: "V", SavingThrow: "Special",
	})
	assert.Equal(t, "special", st.CastingTime.Label)
	assert.Equal(t, "special", st.Range.Label)
	assert.Equal(t, "special", st.Duration.Label)
	assert.Equal(t, "creature_s_touched", st.Area.Label)
	assert.Equal(t, "special", st.Save.Label)

	for raw, want := range map[string]string{
		"Line of sight":      "line_of_sight",
		" hearing  RANGE.":   "hearing_range",
		"The caster's hand":  "the_caster_s_hand",
		"Special":            "special",
		"Variable":           "variable",
		"10-ft. radius":      "",
		"Special (8 turns)":  "",
		"Line of sight, max": "",
	} {
		assert.Equal(t, want, spellstats.ParseArea(raw).Label, raw)
	}
	assert.Empty(t, spellstats.ParseDuration("Instantaneous").Label, "a kind needs no label")
	assert.Empty(t, spellstats.ParseSave("Neg.").Label)

	blob, err := json.Marshal(st)
	require.NoError(t, err)
	assert.Contains(t, string(blob), `"label":"special"`)
	blob, err = json.Marshal(spellstats.Parse(spellstats.Raw{Range: "Touch"}))
	require.NoError(t, err)
	assert.NotContains(t, string(blob), `"label"`)
}
