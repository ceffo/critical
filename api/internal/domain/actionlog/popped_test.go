package actionlog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

func corr(id, actionID int64, kind actionlog.CorrectionKind) actionlog.Correction {
	return actionlog.Correction{ID: id, ActionID: actionID, Kind: kind}
}

// ADR 0019 step 1: an entry is popped when its newest void or redo is a void;
// the map gives the id of that standing void. Amends never pop.
func TestStandingVoids_NewestVoidOrRedoDecides(t *testing.T) {
	const (
		void  = actionlog.CorrectionVoid
		redo  = actionlog.CorrectionRedo
		amend = actionlog.CorrectionAmend
	)
	cases := []struct {
		name string
		cs   []actionlog.Correction
		want map[int64]int64
	}{
		{"none", nil, map[int64]int64{}},
		{"an amend does not pop", []actionlog.Correction{corr(1, 5, amend)}, map[int64]int64{}},
		{"void pops", []actionlog.Correction{corr(1, 5, void)}, map[int64]int64{5: 1}},
		{"redo after the void un-pops", []actionlog.Correction{corr(1, 5, void), corr(2, 5, redo)}, map[int64]int64{}},
		{"popped again: the newest void stands", []actionlog.Correction{corr(1, 5, void), corr(2, 5, redo), corr(3, 5, void)}, map[int64]int64{5: 3}},
		{"any order: the newest id decides", []actionlog.Correction{corr(3, 5, void), corr(1, 5, void), corr(2, 5, redo)}, map[int64]int64{5: 3}},
		{"any order: a newer redo wins", []actionlog.Correction{corr(4, 5, redo), corr(3, 5, void)}, map[int64]int64{}},
		{"amend after the void keeps it popped", []actionlog.Correction{corr(1, 5, void), corr(2, 5, amend)}, map[int64]int64{5: 1}},
		{"per entry", []actionlog.Correction{corr(1, 5, void), corr(2, 6, void), corr(3, 6, redo), corr(4, 7, void)}, map[int64]int64{5: 1, 7: 4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.StandingVoids(c.cs))
		})
	}
}

// ADR 0019 step 2: the redo target is the entry whose standing void is the
// newest, whatever the entries' own ids.
func TestLatestPopped_NewestStandingVoid(t *testing.T) {
	_, ok := actionlog.LatestPopped(map[int64]int64{})
	assert.False(t, ok, "nothing popped")

	id, ok := actionlog.LatestPopped(map[int64]int64{9: 15, 7: 20, 3: 4})
	assert.True(t, ok)
	assert.Equal(t, int64(7), id, "entry 7 was popped last, after the newer entry 9")
}
