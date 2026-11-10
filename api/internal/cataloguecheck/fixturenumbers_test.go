package cataloguecheck_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// realCataloguePath is where `just catalogue` puts the built artifact; CRITICAL_REAL_CATALOGUE
// points the test at another copy.
func realCataloguePath() string {
	if p := os.Getenv("CRITICAL_REAL_CATALOGUE"); p != "" {
		return p
	}
	return "../../../.catalogue/catalogue.json"
}

func readJSON(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // fixed test paths
	require.NoError(t, err)
	var v any
	require.NoError(t, json.Unmarshal(raw, &v))
	return v
}

// entryKey is what names a list entry in the catalogue: its id, its code (equipment) or its level.
func entryKey(m map[string]any) (string, bool) {
	for _, k := range []string{"id", "code"} {
		if s, ok := m[k].(string); ok {
			return s, true
		}
	}
	if l, ok := m["level"].(float64); ok { // a class's experience and spell-slot rows
		return fmt.Sprint(l), true
	}
	return "", false
}

// numberDiffs walks the fixture next to the real catalogue and reports every number the fixture
// holds that differs from the real one, as "path: fixture X, catalogue Y". Only what both hold is
// compared: entries are matched by id or code (a fixture-only entry is invented and skipped), keys
// the other side lacks are skipped, and text is not compared (the fixture's text is placeholder).
// Table rows are matched by their ranges (see rows).
func numberDiffs(path string, fix, real any, out *[]string) {
	switch f := fix.(type) {
	case float64:
		if r, ok := real.(float64); ok && r != f {
			*out = append(*out, fmt.Sprintf("%s: fixture %v, catalogue %v", path, f, r))
		}
	case map[string]any:
		r, ok := real.(map[string]any)
		if !ok {
			return
		}
		keys := make([]string, 0, len(f))
		for k := range f {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if rv, ok := r[k]; ok {
				numberDiffs(path+"."+k, f[k], rv, out)
			}
		}
	case []any:
		r, ok := real.([]any)
		if !ok {
			return
		}
		byKey := map[string]any{}
		for _, e := range r {
			if m, ok := e.(map[string]any); ok {
				if k, ok := entryKey(m); ok {
					byKey[k] = e
				}
			}
		}
		for i, e := range f {
			if m, ok := e.(map[string]any); ok {
				if k, ok := entryKey(m); ok {
					if rv, ok := byKey[k]; ok {
						numberDiffs(fmt.Sprintf("%s[%s]", path, k), e, rv, out)
					}
					continue
				}
			}
			if _, isNum := e.(float64); !isNum {
				if _, isMap := e.(map[string]any); !isMap {
					continue
				}
			}
			rows(fmt.Sprintf("%s[%d]", path, i), e, r, out)
		}
	}
}

// TestFixture_NumbersMatchTheBuiltCatalogue: the fixture carries the book's numbers (ADR 0021), not
// invented ones. Every number the fixture shares with the real catalogue (spells, tables, equipment,
// classes, races, psionic powers, ...) is equal; a difference names the entry and field. The real
// catalogue is private: the test is skipped, with the way to get it, when `just catalogue` has not
// fetched it. `just check-fixture-numbers` runs it and fails instead of skipping.
func TestFixture_NumbersMatchTheBuiltCatalogue(t *testing.T) {
	path := realCataloguePath()
	if _, err := os.Stat(path); err != nil {
		if os.Getenv("CRITICAL_REQUIRE_REAL_CATALOGUE") != "" {
			t.Fatalf("the real catalogue is required and missing at %s: run `just catalogue`", path)
		}
		t.Skipf("SKIPPED: the real catalogue is not at %s (private; `just catalogue` fetches it): fixture numbers not compared", path)
	}
	fix := readJSON(t, "../../testdata/catalogue.json").(map[string]any)
	real := readJSON(t, path).(map[string]any)
	require.Equal(t, fix["format"], real["format"], "the fixture and the catalogue are in different formats: fetch the matching catalogue before comparing")
	delete(fix, "format")

	var diffs []string
	numberDiffs("", fix, real, &diffs)
	require.Empty(t, diffs, "the fixture holds numbers that are not the book's")
}

// TestNumberDiffs_ReportsTheEntryAndField: the comparison names the entry and field of a difference,
// matches entries by id or code, and skips what only one side holds.
func TestNumberDiffs_ReportsTheEntryAndField(t *testing.T) {
	fix := map[string]any{
		"powers": []any{
			map[string]any{"id": "b", "initial_cost": 5.0, "text": "x"},
			map[string]any{"id": "only_fixture", "initial_cost": 1.0},
		},
		"rule_tables": map[string]any{"t": []any{map[string]any{"hit": 1.0}, map[string]any{"hit": 2.0}}},
	}
	real := map[string]any{
		"powers": []any{
			map[string]any{"id": "a", "initial_cost": 9.0},
			map[string]any{"id": "b", "initial_cost": 6.0, "text": "y"},
		},
		"rule_tables": map[string]any{"t": []any{map[string]any{"hit": 1.0}, map[string]any{"hit": 3.0}}},
	}
	var diffs []string
	numberDiffs("", fix, real, &diffs)
	require.Equal(t, []string{
		".powers[b].initial_cost: fixture 5, catalogue 6",
		".rule_tables.t[1].hit: fixture 2, catalogue 3",
	}, diffs)
}

// rows compares one fixture table row with the catalogue's rows. A row's range is its pairs of
// X_min / X_max fields (a Strength row is str_min..str_max and exc_min..exc_max). The fixture may merge
// consecutive catalogue rows that hold the same values into one wider row, so the fixture row is
// compared with every catalogue row its range covers: each of those must hold the same values in the
// other fields. A fixture row that covers no catalogue row is reported, and so is a row with no ranges
// that has no counterpart at the same position.
func rows(path string, fix any, real []any, out *[]string) {
	f, ok := fix.(map[string]any)
	if !ok {
		return
	}
	var pairs []string
	for k := range f {
		if base, ok := strings.CutSuffix(k, "_min"); ok {
			if _, has := f[base+"_max"]; has {
				pairs = append(pairs, base)
			}
		}
	}
	sort.Strings(pairs)
	if len(pairs) == 0 { // no ranges: the row at the same position
		if i, err := strconv.Atoi(path[strings.LastIndex(path, "[")+1 : len(path)-1]); err == nil && i < len(real) {
			numberDiffs(path, fix, real[i], out)
		} else {
			*out = append(*out, path+": the fixture has a row the catalogue lacks")
		}
		return
	}
	covered := 0
	for _, e := range real {
		r, ok := e.(map[string]any)
		if !ok || !covers(f, r, pairs) {
			continue
		}
		covered++
		rest := map[string]any{}
		for k, v := range f {
			if !isRangeField(k, pairs) {
				rest[k] = v
			}
		}
		numberDiffs(path, rest, r, out)
	}
	if covered == 0 && len(pairs) > 0 {
		*out = append(*out, path+": the fixture row covers no catalogue row")
	}
}

func isRangeField(k string, pairs []string) bool {
	for _, p := range pairs {
		if k == p+"_min" || k == p+"_max" {
			return true
		}
	}
	return false
}

// covers reports whether the catalogue row r lies inside the fixture row f on every range and has
// the same text fields (a row's text, such as its group, says which table section it belongs to).
func covers(f, r map[string]any, pairs []string) bool {
	for k, v := range f {
		if s, ok := v.(string); ok {
			if rs, ok := r[k].(string); ok && rs != s {
				return false
			}
		}
	}
	for _, p := range pairs {
		fmin, _ := f[p+"_min"].(float64)
		fmax, _ := f[p+"_max"].(float64)
		rmin, ok1 := r[p+"_min"].(float64)
		rmax, ok2 := r[p+"_max"].(float64)
		if !ok1 || !ok2 || rmin < fmin || rmax > fmax {
			return false
		}
	}
	return true
}

// TestNumberDiffs_AMergedRowIsComparedWithEveryRowItCovers: the fixture may fold consecutive rows of
// equal values into one; it is wrong when one of the folded rows differs, or when it covers none.
func TestNumberDiffs_AMergedRowIsComparedWithEveryRowItCovers(t *testing.T) {
	row := func(lo, hi, v float64) any {
		return map[string]any{"group": "g", "x_min": lo, "x_max": hi, "v": v}
	}
	real := map[string]any{"t": []any{row(1, 1, 0), row(2, 2, 0), row(3, 3, 1)}}
	var diffs []string
	numberDiffs("", map[string]any{"t": []any{row(1, 2, 0), row(3, 3, 1)}}, real, &diffs)
	require.Empty(t, diffs, "a folded row of equal values is the book's")

	diffs = nil
	numberDiffs("", map[string]any{"t": []any{row(1, 3, 0)}}, real, &diffs)
	require.Equal(t, []string{".t[0].v: fixture 0, catalogue 1"}, diffs)

	diffs = nil
	numberDiffs("", map[string]any{"t": []any{row(7, 8, 0)}}, real, &diffs)
	require.Equal(t, []string{".t[0]: the fixture row covers no catalogue row"}, diffs)
}
