package ruletablesadpt_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// parseWith parses the fixture's rule tables after change edits the decoded
// rule_tables object.
func parseWith(t *testing.T, change func(rt map[string]any)) (*ruletablesvc.Tables, error) {
	t.Helper()
	var rt map[string]any
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &rt))
	change(rt)
	raw, err := json.Marshal(rt)
	require.NoError(t, err)
	return ruletablesadpt.Parse(raw, fixtureEquipment(t), fixtureLanguages(t), fixtureDefinitions(t))
}

func rows(rt map[string]any, table string) []any { return rt[table].([]any) }

// ADR 0044 step 1.1: the 25 cells of Table 14 read as data; mind thrust meeting
// tower of iron will is -5. A missing cell, a pair twice and a cell naming a
// devotion where a mode belongs are each refused at start.
func TestRuleTables_Step1_AttackVersusDefence(t *testing.T) {
	tbl := phb(t)
	got, err := tbl.AttackVersusDefence("mind_thrust", "tower_of_iron_will")
	require.NoError(t, err)
	assert.Equal(t, -5, got)
	_, err = tbl.AttackVersusDefence("mind_thrust", "contact")
	require.Error(t, err, "a cell the table lacks")

	_, err = parseWith(t, func(rt map[string]any) { rt["attack_versus_defence"] = rows(rt, "attack_versus_defence")[1:] })
	require.ErrorContains(t, err, "attack_versus_defence table")
	_, err = parseWith(t, func(rt map[string]any) {
		r := rows(rt, "attack_versus_defence")
		rt["attack_versus_defence"] = append(r, r[0])
	})
	require.ErrorContains(t, err, "twice")

	// A cell naming a devotion: refused at start by the power-kind check.
	a := cataloguetest.Artifact(t)
	bad, err := parseWith(t, func(rt map[string]any) {
		for _, r := range rows(rt, "attack_versus_defence") {
			if r.(map[string]any)["attack"] == "psionic_blast" {
				r.(map[string]any)["attack"] = "contact"
			}
		}
	})
	require.NoError(t, err, "the grid alone cannot tell a devotion from a mode")
	require.ErrorContains(t, bad.CheckPowerKinds(a.Powers), `"contact" is not an attack mode`)
	require.NoError(t, phb(t).CheckPowerKinds(a.Powers))

	// A mode of the catalogue with no cells is refused too.
	extra := append([]catalogue.Power{}, a.Powers...)
	extra = append(extra, catalogue.Power{ID: "new_mode", Kind: catalogue.KindDefenceMode})
	require.ErrorContains(t, phb(t).CheckPowerKinds(extra), `no cells for the defence_mode "new_mode"`)
}

// ADR 0044 steps 1.2 and 1.3: Table 29's bands and Tables 30 and 31 read as data;
// a level above 20 is level_out_of_table (Q15); bands that skip a level and a
// code twice or unknown are refused at start.
func TestRuleTables_Step1_ContactCostAndModifiers(t *testing.T) {
	tbl := phb(t)
	for level, want := range map[int]int{1: 3, 5: 3, 6: 8, 10: 8, 11: 13, 16: 18, 20: 18} {
		got, err := tbl.ContactCost(level)
		require.NoError(t, err, "level %d", level)
		assert.Equal(t, want, got, "level %d", level)
	}
	for _, level := range []int{0, 21, 40} {
		_, err := tbl.ContactCost(level)
		require.ErrorIs(t, err, ruletablesvc.ErrLevelOutOfTable, "level %d", level)
	}
	for _, tc := range []struct {
		kind, code string
		want       int
	}{
		{ruletablesvc.ContactDistance, "miles_10", -3},
		{ruletablesvc.ContactLifeOrder, "bird", -3},
		{ruletablesvc.ContactResistance, "resisting", -2},
	} {
		got, err := tbl.ContactModifier(tc.kind, tc.code)
		require.NoError(t, err, tc.code)
		assert.Equal(t, tc.want, got, tc.code)
	}
	_, err := tbl.ContactModifier(ruletablesvc.ContactDistance, "bird")
	require.Error(t, err, "a code of another kind")

	for name, tc := range map[string]struct {
		change func(rt map[string]any)
		want   string
	}{
		"a band skipping a level": {func(rt map[string]any) {
			rows(rt, "contact_cost")[1].(map[string]any)["min_level"] = 4
		}, "contact_cost table"},
		"bands past level 20": {func(rt map[string]any) {
			rows(rt, "contact_cost")[3].(map[string]any)["max_level"] = 21
		}, "past level 20"},
		"a modifier code twice": {func(rt map[string]any) {
			r := rows(rt, "contact_modifiers")
			rt["contact_modifiers"] = append(r, r[0])
		}, "twice"},
		"a modifier code missing": {func(rt map[string]any) {
			rt["contact_modifiers"] = rows(rt, "contact_modifiers")[1:]
		}, "missing"},
		"a modifier above 0": {func(rt map[string]any) {
			rows(rt, "contact_modifiers")[0].(map[string]any)["modifier"] = 1
		}, "above 0"},
	} {
		_, err := parseWith(t, tc.change)
		require.ErrorContains(t, err, tc.want, name)
	}
}

// The contact tables' rows read as data for the cost-table read: Table 29's
// bands cover levels 1 to 20 in order, every modifier kind and code of
// ADR 0044 step 1.3 is a row, and a caller's edit of the copy leaves the
// table alone.
func TestRuleTables_Step1_ContactRowsAsData(t *testing.T) {
	tbl := phb(t)
	bands, err := tbl.ContactCostRows()
	require.NoError(t, err)
	require.NotEmpty(t, bands)
	assert.Equal(t, 1, bands[0].MinLevel)
	assert.Equal(t, ruletablesvc.MaxContactLevel, bands[len(bands)-1].MaxLevel)
	rows, err := tbl.ContactModifierRows()
	require.NoError(t, err)
	want := 0
	for _, codes := range ruletablesvc.ContactModifierCodes {
		want += len(codes)
	}
	assert.Len(t, rows, want)

	bands[0].Cost = 999
	again, err := tbl.ContactCostRows()
	require.NoError(t, err)
	assert.NotEqual(t, 999, again[0].Cost)
}
