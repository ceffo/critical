package strength_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/strength"
)

// table1 is PHB Table 1 as committed rows (hit, damage), the values the
// service read before the owner existed.
var table1 = []strength.Row{
	{StrMin: 1, StrMax: 1, Hit: -5, Dmg: -4},
	{StrMin: 2, StrMax: 2, Hit: -3, Dmg: -2},
	{StrMin: 3, StrMax: 3, Hit: -3, Dmg: -1},
	{StrMin: 4, StrMax: 5, Hit: -2, Dmg: -1},
	{StrMin: 6, StrMax: 7, Hit: -1, Dmg: 0},
	{StrMin: 8, StrMax: 15, Hit: 0, Dmg: 0},
	{StrMin: 16, StrMax: 16, Hit: 0, Dmg: 1},
	{StrMin: 17, StrMax: 17, Hit: 1, Dmg: 1},
	{StrMin: 18, StrMax: 18, ExcMin: 0, ExcMax: 0, Hit: 1, Dmg: 2},
	{StrMin: 18, StrMax: 18, ExcMin: 1, ExcMax: 50, Hit: 1, Dmg: 3},
	{StrMin: 18, StrMax: 18, ExcMin: 51, ExcMax: 75, Hit: 2, Dmg: 3},
	{StrMin: 18, StrMax: 18, ExcMin: 76, ExcMax: 90, Hit: 2, Dmg: 4},
	{StrMin: 18, StrMax: 18, ExcMin: 91, ExcMax: 99, Hit: 2, Dmg: 5},
	{StrMin: 18, StrMax: 18, ExcMin: 100, ExcMax: 100, Hit: 3, Dmg: 6},
	{StrMin: 19, StrMax: 19, Hit: 3, Dmg: 7},
	{StrMin: 20, StrMax: 20, Hit: 3, Dmg: 8},
	{StrMin: 21, StrMax: 21, Hit: 4, Dmg: 9},
	{StrMin: 22, StrMax: 22, Hit: 4, Dmg: 10},
	{StrMin: 23, StrMax: 23, Hit: 5, Dmg: 11},
	{StrMin: 24, StrMax: 24, Hit: 6, Dmg: 12},
	{StrMin: 25, StrMax: 25, Hit: 7, Dmg: 14},
}

func newTable(t *testing.T) strength.Table {
	t.Helper()
	tbl, err := strength.NewTable(table1)
	require.NoError(t, err)
	return tbl
}

// The owner alone gives a score's hit and damage adjustments, with no service
// around it.
func TestStrength_TheScoresAdjustments(t *testing.T) {
	tbl := newTable(t)
	for _, tc := range []struct {
		score, exc int
		want       strength.Mods
	}{
		{1, 0, strength.Mods{Hit: -5, Dmg: -4}},
		{3, 0, strength.Mods{Hit: -3, Dmg: -1}},
		{12, 0, strength.Mods{}},
		{16, 0, strength.Mods{Dmg: 1}},
		{17, 0, strength.Mods{Hit: 1, Dmg: 1}},
		{18, 0, strength.Mods{Hit: 1, Dmg: 2}},
		{25, 0, strength.Mods{Hit: 7, Dmg: 14}},
	} {
		t.Run(fmt.Sprintf("%d/%d", tc.score, tc.exc), func(t *testing.T) {
			got, err := tbl.Of(tc.score, tc.exc)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// The exceptional percentile picks the 18 row; every other score ignores it.
func TestStrength_ExceptionalOnlyAtEighteen(t *testing.T) {
	tbl := newTable(t)
	for _, tc := range []struct {
		score, exc int
		want       strength.Mods
	}{
		{18, 1, strength.Mods{Hit: 1, Dmg: 3}},
		{18, 50, strength.Mods{Hit: 1, Dmg: 3}},
		{18, 51, strength.Mods{Hit: 2, Dmg: 3}},
		{18, 76, strength.Mods{Hit: 2, Dmg: 4}},
		{18, 99, strength.Mods{Hit: 2, Dmg: 5}},
		{18, 100, strength.Mods{Hit: 3, Dmg: 6}},
		{17, 99, strength.Mods{Hit: 1, Dmg: 1}},
		{19, 100, strength.Mods{Hit: 3, Dmg: 7}},
	} {
		t.Run(fmt.Sprintf("%d/%d", tc.score, tc.exc), func(t *testing.T) {
			got, err := tbl.Of(tc.score, tc.exc)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A score or an exceptional outside the table is refused, as is any score by
// the zero table.
func TestStrength_OutOfRangeRefused(t *testing.T) {
	tbl := newTable(t)
	for _, tc := range []struct{ score, exc int }{{0, 0}, {26, 0}, {-1, 0}, {18, 101}, {18, -1}, {12, 101}} {
		_, err := tbl.Of(tc.score, tc.exc)
		assert.Error(t, err, "%d/%d", tc.score, tc.exc)
	}
	_, err := strength.Table{}.Of(12, 0)
	assert.Error(t, err, "the zero table has no rows")
	assert.False(t, strength.Table{}.Valid())
	assert.True(t, tbl.Valid())
}

// NewTable refuses rows that leave a gap, overlap or run past 25.
func TestStrength_NewTableValidates(t *testing.T) {
	gap := append([]strength.Row(nil), table1[:3]...)
	gap = append(gap, table1[4:]...)
	short := table1[:len(table1)-1]
	noPercentile := []strength.Row{{StrMin: 1, StrMax: 17}, {StrMin: 18, StrMax: 18, ExcMin: 1, ExcMax: 100}}
	for name, rows := range map[string][]strength.Row{"gap": gap, "short": short, "exceptional not from 0": noPercentile, "none": nil} {
		_, err := strength.NewTable(rows)
		assert.Error(t, err, name)
	}
}
