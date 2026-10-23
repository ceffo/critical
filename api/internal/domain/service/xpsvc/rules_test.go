package xpsvc_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// xpTables is the experience tables by class, as printed: the Thief/Bard
// column of PHB Table 25 (p.38) and Table 20 (p.30) for the mage.
type xpTables struct{}

var phbXP = map[string][]int{
	"bard": {
		0, 1250, 2500, 5000, 10000, 20000, 40000, 70000, 110000, 160000,
		220000, 440000, 660000, 880000, 1100000, 1320000, 1540000, 1760000, 1980000, 2200000,
	},
	"mage": {
		0, 2500, 5000, 10000, 20000, 40000, 60000, 90000, 135000, 250000,
		375000, 750000, 1125000, 1500000, 1875000, 2250000, 2625000, 3000000, 3375000, 3750000,
	},
}

// phbPrimes are the prime requisites of PHB chapter 3 (ADR 0040 step 6).
var phbPrimes = map[string][]string{
	"fighter": {"str"}, "mage": {"int"}, "cleric": {"wis"}, "thief": {"dex"}, "bard": {"dex", "cha"},
}

func (xpTables) PrimeRequisites(class string) ([]string, error) {
	p, ok := phbPrimes[class]
	if !ok {
		return nil, fmt.Errorf("unknown class %q", class)
	}
	return p, nil
}

func (xpTables) XPThreshold(class string, level int) (int, error) {
	col, ok := phbXP[class]
	if !ok {
		return 0, fmt.Errorf("unknown class %q", class)
	}
	if level < 1 || level > len(col) {
		return 0, fmt.Errorf("level %d out of range", level)
	}
	return col[level-1], nil
}

// ADR 0015 step 1: an entry adds its base, plus 10% of it rounded to the
// nearest point (halves up) when its bonus is on.
func TestEntryAmount_TenPercentBonusRoundedHalfUp(t *testing.T) {
	tests := []struct {
		base        int
		bonus       bool
		wantBonus   int
		wantTotal   int
		description string
	}{
		{1200, true, 120, 1320, "exact tenth"},
		{1200, false, 0, 1200, "no bonus adds the base"},
		{15, true, 2, 17, "1.5 rounds up"},
		{14, true, 1, 15, "1.4 rounds down"},
		{16, true, 2, 18, "1.6 rounds up"},
		{5, true, 1, 6, "0.5 rounds up"},
		{4, true, 0, 4, "0.4 rounds down to no bonus"},
		{76251, true, 7625, 83876, "7625.1 rounds down"},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			got := xpsvc.EntryAmount(tt.base, tt.bonus)
			assert.Equal(t, xpsvc.Amount{Base: tt.base, BonusAmount: tt.wantBonus, Total: tt.wantTotal}, got)
		})
	}
}

// ADR 0015 step 1: the total is the sheet's starting XP plus every entry's
// amount (base plus its own rounded bonus).
func TestLedgerTotal_OpeningPlusEveryEntry(t *testing.T) {
	entries := []xpsvc.Entry{
		{Base: 15, Bonus: true},    // 17
		{Base: 15, Bonus: true},    // 17, rounded per entry, not on the sum
		{Base: 1000, Bonus: false}, // 1000
	}
	assert.Equal(t, 1_293_418+17+17+1000, xpsvc.LedgerTotal(1_293_418, entries))
	assert.Equal(t, 500, xpsvc.LedgerTotal(500, nil), "no entries: the starting XP")
}

// ADR 0040 step 6: the bonus needs every prime requisite of the class (its
// definition's) at 16 or more; it replaces ADR 0015 step 2's Dexterity and
// Charisma for every class. On the old test a fighter of Dex 16 and Cha 16 got
// a bard's bonus and a fighter of Str 16 none.
func TestBonusEligible_Step6_EveryPrimeRequisiteAt16(t *testing.T) {
	tests := []struct {
		name  string
		class string
		c     charactersvc.View
		want  bool
	}{
		{"fighter Str 16", "fighter", charactersvc.View{Str: 16, Dex: 10, Cha: 10}, true},
		{"fighter 18/xx is 18", "fighter", charactersvc.View{Str: 18, StrExceptional: 50}, true},
		{"fighter Str 15, Dex 16, Cha 16", "fighter", charactersvc.View{Str: 15, Dex: 16, Cha: 16}, false},
		{"mage Int 16", "mage", charactersvc.View{Int: 16}, true},
		{"mage Int 15", "mage", charactersvc.View{Int: 15, Dex: 18, Cha: 18}, false},
		{"cleric Wis 17", "cleric", charactersvc.View{Wis: 17}, true},
		{"thief Dex 16", "thief", charactersvc.View{Dex: 16, Cha: 3}, true},
		{"bard Dex 16, Cha 16", "bard", charactersvc.View{Dex: 16, Cha: 16}, true},
		{"bard Dex 16, Cha 15", "bard", charactersvc.View{Dex: 16, Cha: 15}, false},
		{"bard Dex 15, Cha 18", "bard", charactersvc.View{Dex: 15, Cha: 18}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primes, err := xpTables{}.PrimeRequisites(tt.class)
			require.NoError(t, err)
			assert.Equal(t, tt.want, xpsvc.BonusEligible(primes, tt.c))
		})
	}
	assert.False(t, xpsvc.BonusEligible(nil, charactersvc.View{Str: 18}), "a class without prime requisites gets none")
}

func cells(from int, statuses ...xpsvc.CellStatus) []xpsvc.LevelCell {
	return classCells("bard", from, statuses...)
}

func mageCells(from int, statuses ...xpsvc.CellStatus) []xpsvc.LevelCell {
	return classCells("mage", from, statuses...)
}

func classCells(class string, from int, statuses ...xpsvc.CellStatus) []xpsvc.LevelCell {
	out := make([]xpsvc.LevelCell, 0, len(statuses))
	for i, s := range statuses {
		out = append(out, xpsvc.LevelCell{Level: from + i, Threshold: phbXP[class][from+i-1], Status: s})
	}
	return out
}

func ptr(n int) *int { return &n }

// ADR 0015 step 3, in the class's table (ADR 0030 step 3): the floor is the
// threshold of the level, the ceiling the next level's; what is left and the
// share of the way come from them, the cells are 8 levels from two below the
// level.
func TestComputeProgress_FromTheClassTable(t *testing.T) {
	const (
		past    = xpsvc.CellPast
		current = xpsvc.CellCurrent
		next    = xpsvc.CellNext
		ahead   = xpsvc.CellAhead
	)
	tests := []struct {
		name         string
		class        string
		level, total int
		want         xpsvc.Progress
	}{
		{
			name: "level 15 part way", level: 15, total: 1_293_418,
			want: xpsvc.Progress{
				Level: 15, CurrentFloor: 1_100_000, NextLevel: 16, NextThreshold: ptr(1_320_000),
				ToNext: 26_582, Progress: 193_418.0 / 220_000.0, ProgressPercent: 87,
				Levels: cells(13, past, past, current, next, ahead, ahead, ahead, ahead),
			},
		},
		{
			name: "ceiling reached: nothing left, full, no level-up", level: 15, total: 1_400_000,
			want: xpsvc.Progress{
				Level: 15, CurrentFloor: 1_100_000, NextLevel: 16, NextThreshold: ptr(1_320_000),
				ToNext: 0, Progress: 1, ProgressPercent: 100,
				Levels: cells(13, past, past, current, next, ahead, ahead, ahead, ahead),
			},
		},
		{
			name: "below the floor: empty", level: 15, total: 1_000_000,
			want: xpsvc.Progress{
				Level: 15, CurrentFloor: 1_100_000, NextLevel: 16, NextThreshold: ptr(1_320_000),
				ToNext: 320_000, Progress: 0, ProgressPercent: 0,
				Levels: cells(13, past, past, current, next, ahead, ahead, ahead, ahead),
			},
		},
		{
			name: "last level: no ceiling", level: 20, total: 2_300_000,
			want: xpsvc.Progress{
				Level: 20, CurrentFloor: 2_200_000, Progress: 1, ProgressPercent: 100,
				Levels: cells(13, past, past, past, past, past, past, past, current),
			},
		},
		{
			name: "level override above the table: the last level's state", level: 25, total: 2_300_000,
			want: xpsvc.Progress{
				Level: 25, CurrentFloor: 2_200_000, Progress: 1, ProgressPercent: 100,
				Levels: cells(13, past, past, past, past, past, past, past, past),
			},
		},
		{
			name: "first level: cells start at 1", level: 1, total: 625,
			want: xpsvc.Progress{
				Level: 1, CurrentFloor: 0, NextLevel: 2, NextThreshold: ptr(1250),
				ToNext: 625, Progress: 0.5, ProgressPercent: 50,
				Levels: cells(1, current, next, ahead, ahead, ahead, ahead, ahead, ahead),
			},
		},
		{
			name: "mage: Table 20, not the bard's", class: "mage", level: 2, total: 2_600,
			want: xpsvc.Progress{
				Level: 2, CurrentFloor: 2_500, NextLevel: 3, NextThreshold: ptr(5_000),
				ToNext: 2_400, Progress: 100.0 / 2_500.0, ProgressPercent: 4,
				Levels: mageCells(1, past, current, next, ahead, ahead, ahead, ahead, ahead),
			},
		},
		{
			name: "mage: last level", class: "mage", level: 20, total: 3_800_000,
			want: xpsvc.Progress{
				Level: 20, CurrentFloor: 3_750_000, Progress: 1, ProgressPercent: 100,
				Levels: mageCells(13, past, past, past, past, past, past, past, current),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class := tt.class
			if class == "" {
				class = "bard"
			}
			got, err := xpsvc.ComputeProgress(class, tt.level, tt.total, xpTables{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := xpsvc.ComputeProgress("bard", 0, 0, xpTables{})
	require.Error(t, err, "a level below the table")
	_, err = xpsvc.ComputeProgress("cleric", 1, 0, xpTables{})
	require.Error(t, err, "a class without a table")
}
