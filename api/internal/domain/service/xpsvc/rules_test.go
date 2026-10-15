package xpsvc_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// table25 is the Thief/Bard column of PHB Table 25 (p.38), as printed.
type table25 struct{}

var phbTable25 = []int{
	0, 1250, 2500, 5000, 10000, 20000, 40000, 70000, 110000, 160000,
	220000, 440000, 660000, 880000, 1100000, 1320000, 1540000, 1760000, 1980000, 2200000,
}

func (table25) BardXPThreshold(level int) (int, error) {
	if level < 1 || level > len(phbTable25) {
		return 0, fmt.Errorf("level %d out of range", level)
	}
	return phbTable25[level-1], nil
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

// ADR 0015 step 2: the bonus applies by default when Dexterity and
// Charisma are both 16 or more.
func TestBonusEligible_DexAndChaBothAtLeast16(t *testing.T) {
	tests := []struct {
		dex, cha int
		want     bool
	}{
		{16, 16, true},
		{17, 16, true},
		{18, 18, true},
		{15, 18, false},
		{18, 15, false},
		{15, 15, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, xpsvc.BonusEligible(tt.dex, tt.cha), "dex %d cha %d", tt.dex, tt.cha)
	}
}

func cells(from int, statuses ...xpsvc.CellStatus) []xpsvc.LevelCell {
	out := make([]xpsvc.LevelCell, 0, len(statuses))
	for i, s := range statuses {
		out = append(out, xpsvc.LevelCell{Level: from + i, Threshold: phbTable25[from+i-1], Status: s})
	}
	return out
}

func ptr(n int) *int { return &n }

// ADR 0015 step 3: the floor is the Table 25 threshold of the sheet's level,
// the ceiling the next level's; what is left and the share of the way come
// from them, the cells are 8 levels from two below the sheet's.
func TestComputeProgress_FromTable25(t *testing.T) {
	const (
		past    = xpsvc.CellPast
		current = xpsvc.CellCurrent
		next    = xpsvc.CellNext
		ahead   = xpsvc.CellAhead
	)
	tests := []struct {
		name         string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xpsvc.ComputeProgress(tt.level, tt.total, table25{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := xpsvc.ComputeProgress(0, 0, table25{})
	require.Error(t, err, "a level below the table")
}
