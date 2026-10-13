package buffsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Add writes the list and returns the buffs snapshot for the caller's entry;
// it records nothing itself, and a same-name buff is replaced (renewed).
func TestAdd(t *testing.T) {
	old := buffsvc.BuffEffect{ID: "s1", Name: "Stoneskin", Charges: 1}
	haste := buffsvc.BuffEffect{ID: "h1", Name: "Haste", Duration: 4}
	tests := []struct {
		name   string
		before []buffsvc.BuffEffect
		add    buffsvc.BuffEffect
		want   []string
	}{
		{"appends", []buffsvc.BuffEffect{haste}, buffsvc.BuffEffect{ID: "n", Name: "Stoneskin", Charges: 9}, []string{"h1", "n"}},
		{"replaces same name", []buffsvc.BuffEffect{old, haste}, buffsvc.BuffEffect{ID: "n", Name: "stoneskin", Charges: 9}, []string{"h1", "n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.before, nil).Once()
			var saved []buffsvc.BuffEffect
			f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
				func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

			got, snap, err := f.svc.Add(context.Background(), charID, tt.add)
			require.NoError(t, err)
			assert.Equal(t, tt.add, got)
			var ids []string
			for _, b := range saved {
				ids = append(ids, b.ID)
			}
			assert.Equal(t, tt.want, ids)
			assert.Equal(t, buffsvc.StateKeyBuffs, snap.Key)
			assert.Contains(t, string(snap.Before), `"id":"h1"`)
			assert.Contains(t, string(snap.After), `"id":"n"`)
			assert.Equal(t, 0, f.tx.calls, "runs in the caller's transaction")
			f.log.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
		})
	}
}

func TestAdd_Invalid(t *testing.T) {
	f := newFixture(t)
	_, _, err := f.svc.Add(context.Background(), charID, buffsvc.BuffEffect{Name: " "})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
