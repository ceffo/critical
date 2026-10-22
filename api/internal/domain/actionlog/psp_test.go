package actionlog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0043 step 5: current PSPs are psp_max plus the sum of every entry's
// effective PSP delta, whatever its kind; a popped entry counts 0.
func TestPSPTotal_Step5_EveryKindCounts(t *testing.T) {
	entries := []actionlog.Entry{
		{ID: 1, Kind: actionlog.Kind("power_check"), PSPDelta: -7},
		{ID: 2, Kind: actionlog.Kind("long_rest"), PSPDelta: 5},
		{ID: 3, Kind: actionlog.Kind("hp_change"), HPDelta: -4, PSPDelta: -2},
		{ID: 4, Kind: actionlog.Kind("power_check"), PSPDelta: -11, Corrections: []actionlog.Correction{{Kind: actionlog.CorrectionVoid}}},
	}
	assert.Equal(t, -4, actionlog.PSPTotal(entries), "-7 + 5 - 2; the popped -11 counts 0")
	assert.Equal(t, -4, actionlog.HPTotal(entries), "the HP ledger reads its own deltas only")
	assert.Equal(t, 0, actionlog.PSPTotal(nil))
}

// ADR 0043 step 5: a gain is capped at the PSPs spent, a loss at the PSPs
// left; current PSPs are shown within 0..max.
func TestCapPSP_Step5_GainCappedAtTheMaxNeverUnderZero(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		maxPSP, total, delta int
		want                 int
	}{
		{"a gain within the PSPs spent", 50, -20, 12, 12},
		{"a gain over the max is capped", 50, -20, 96, 20},
		{"a gain at full PSPs is 0", 50, 0, 6, 0},
		{"a loss within the PSPs left", 50, -20, -7, -7},
		{"a loss past 0 is floored", 50, -45, -11, -5},
		{"a loss at 0 is 0", 50, -50, -3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, actionlog.CapPSP(tc.maxPSP, tc.total, tc.delta))
		})
	}
	assert.Equal(t, 30, actionlog.CurrentPSP(50, -20))
	assert.Equal(t, 50, actionlog.CurrentPSP(50, 4), "never above the max")
	assert.Equal(t, 0, actionlog.CurrentPSP(20, -25), "never under 0")
}

// An entry that moves PSPs names the psp family (ADR 0043 step 5), when it is
// recorded and when it is popped or replayed, so the web reads the pool again;
// one that moves none doesn't.
func TestStateKeys_Step5_APSPDeltaNamesThePool(t *testing.T) {
	assert.Equal(t, []string{actionlog.FamilyLog, actionlog.FamilyPSP},
		actionlog.NewEntryKeys(actionlog.NewEntry{Kind: actionlog.KindPSPChange, PSPDelta: -7}))
	assert.Equal(t, []string{actionlog.FamilyLog, actionlog.FamilyPSP},
		actionlog.TouchedKeys(actionlog.Entry{Kind: actionlog.KindPSPRecovery, PSPDelta: 18}))
	assert.Equal(t, []string{actionlog.FamilyLog},
		actionlog.TouchedKeys(actionlog.Entry{Kind: actionlog.KindPSPRecovery}))
	assert.True(t, actionlog.IsLogFamily(actionlog.FamilyPSP))
}
