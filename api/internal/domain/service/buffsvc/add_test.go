package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// Add writes the list and returns the buffs snapshot for the caller's entry;
// it records nothing itself, and a same-name buff is replaced (renewed).
func TestAdd(t *testing.T) {
	old := buffsvc.BuffEffect{ID: "s1", Name: "Stoneskin", Charges: 1}
	str1 := buffsvc.BuffEffect{ID: "st1", Name: "Strength", Duration: 540, Effects: buffsvc.BuffEffects{StrBonus: intPtr(2), StrMax: intPtr(18)}}
	str2 := buffsvc.BuffEffect{ID: "st2", Name: "Strength", Duration: 540, Effects: buffsvc.BuffEffects{StrBonus: intPtr(4), StrMax: intPtr(18)}}
	imp1 := buffsvc.BuffEffect{ID: "i1", Name: "Improved Strength", Duration: 9, Effects: buffsvc.BuffEffects{StrBonus: intPtr(5), StrMax: intPtr(23)}}
	imp2 := buffsvc.BuffEffect{ID: "i2", Name: "Improved Strength", Duration: 9, Effects: buffsvc.BuffEffects{StrBonus: intPtr(7), StrMax: intPtr(23)}}
	haste := buffsvc.BuffEffect{ID: "h1", Name: "Haste", Duration: 4}
	tests := []struct {
		name   string
		before []buffsvc.BuffEffect
		add    buffsvc.BuffEffect
		want   []string
	}{
		{"appends", []buffsvc.BuffEffect{haste}, buffsvc.BuffEffect{ID: "n", Name: "Stoneskin", Charges: 9}, []string{"h1", "n"}},
		{"replaces same name", []buffsvc.BuffEffect{old, haste}, buffsvc.BuffEffect{ID: "n", Name: "stoneskin", Charges: 9}, []string{"h1", "n"}},
		// ADR 0004 steps 6 and 7: Strength (L2) stacks with itself.
		{"a second Strength is a second buff", []buffsvc.BuffEffect{str1, haste}, str2, []string{"st1", "h1", "st2"}},
		{"a hand-made uncapped Strength stacks too", []buffsvc.BuffEffect{str1, haste}, buffsvc.BuffEffect{ID: "st3", Name: "Strength", Effects: buffsvc.BuffEffects{StrBonus: intPtr(3)}}, []string{"st1", "h1", "st3"}},
		{"Improved Strength is renewed", []buffsvc.BuffEffect{imp1, haste}, imp2, []string{"h1", "i2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.before, nil).Once()
			f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, nil).Once()
			var saved []buffsvc.BuffEffect
			f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
				func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()

			got, snap, err := f.svc.Add(context.Background(), charID, tt.add)
			require.NoError(t, err)
			want := tt.add
			want.ChargesTotal = want.Charges // ADR 0004 step 16
			assert.Equal(t, want, got)
			var ids []string
			for _, b := range saved {
				ids = append(ids, b.ID)
			}
			assert.Equal(t, tt.want, ids)
			assert.Equal(t, buffsvc.StateKeyBuffs, snap.Key)
			assert.Contains(t, string(snap.Before), `"id":"h1"`)
			assert.Contains(t, string(snap.After), `"id":"`+tt.add.ID+`"`)
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
