package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// everyEffect is a row marked as on someone else that holds every effect a
// buff can have: no aggregate may count any of it (ADR 0004 step 20.5).
var everyEffect = buffsvc.BuffEffect{
	ID: "o", Name: "Elsewhere", SpellID: "haste", Duration: 9, OnOther: true,
	Effects: buffsvc.BuffEffects{
		SpellsBuff: floatPtr(2), WeaponBuff: intPtr(5), Multiplier: floatPtr(2), SaveBonus: intPtr(3),
		StrBonus: intPtr(9), StrMax: intPtr(23), AC: intPtr(-4), ToHit: intPtr(2), Damage: intPtr(3),
		BaseAC: intPtr(2), Initiative: intPtr(-2),
	},
}

// ADR 0004 step 20.5: a row on someone else or on an area counts in no
// aggregate (to-hit, damage, saves, AC, STR, initiative) and shows as not
// applying; the rows on the character beside it count as before.
func TestBuffs_Step20_OnOtherCountsInNoAggregate(t *testing.T) {
	bless := buffsvc.BuffEffect{ID: "b", Name: "Bless", Duration: 6, Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}

	t.Run("aggregate", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{everyEffect, bless}, nil)
		f.powers()
		got, err := f.svc.Aggregate(context.Background(), charID, buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 12}}, buffsvc.WeaponMelee)
		require.NoError(t, err)
		assert.Equal(t, buffsvc.BuffModifiers{
			SpellsBuff: 1, Multipliers: 1, ToHit: 1, EffectiveStr: 12,
			ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}},
		}, got)
	})

	t.Run("effective STR", func(t *testing.T) {
		f := newFixture(t)
		f.base(12)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{everyEffect}, nil)
		got, err := f.svc.GetEffectiveStr(context.Background(), charID)
		require.NoError(t, err)
		assert.Equal(t, buffsvc.Strength{Base: 12, Effective: 12}, got)
	})

	t.Run("the list shows it as not applying", func(t *testing.T) {
		f := newFixture(t)
		f.base(12)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{everyEffect, bless}, nil)
		got, err := f.svc.ListStatus(context.Background(), charID)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.False(t, got[0].Applies)
		assert.True(t, got[1].Applies)
	})

	t.Run("initiative", func(t *testing.T) {
		f := newFixture(t)
		advanceWith(f, []buffsvc.BuffEffect{everyEffect}, nil, nil, 4)
		_, _, _, got, _, err := f.svc.AdvanceRound(context.Background(), charID)
		require.NoError(t, err)
		assert.Nil(t, got.Modifiers, "no initiative effect")
		assert.Equal(t, 4, got.Total())
	})
}

// ADR 0004 step 20.5: a row on someone else never renews a same-name row and
// is never renewed: two Webs on two areas are two rows, and a Haste on the
// Bard and one on an ally stay side by side.
func TestBuffs_Step20_OnOtherIsNeverRenewed(t *testing.T) {
	web := buffsvc.BuffEffect{ID: "w1", Name: "Web", Duration: 30, OnOther: true}
	haste := buffsvc.BuffEffect{ID: "h1", Name: "Haste", Duration: 18}
	hasteOther := buffsvc.BuffEffect{ID: "h2", Name: "Haste", Duration: 18, OnOther: true}
	tests := []struct {
		name   string
		before []buffsvc.BuffEffect
		add    buffsvc.BuffEffect
		want   []string
	}{
		{"a second web on another area", []buffsvc.BuffEffect{web}, buffsvc.BuffEffect{ID: "w2", Name: "Web", Duration: 30, OnOther: true}, []string{"w1", "w2"}},
		{"haste on an ally beside his own", []buffsvc.BuffEffect{haste}, hasteOther, []string{"h1", "h2"}},
		{"haste on him beside an ally's", []buffsvc.BuffEffect{hasteOther}, buffsvc.BuffEffect{ID: "h3", Name: "Haste", Duration: 18}, []string{"h2", "h3"}},
		{"his own haste is still renewed", []buffsvc.BuffEffect{hasteOther, haste}, buffsvc.BuffEffect{ID: "h3", Name: "Haste", Duration: 18}, []string{"h2", "h3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.before, nil).Once()
			f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, nil).Once()
			var saved []buffsvc.BuffEffect
			f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
				func(_ context.Context, _ int64, b []buffsvc.BuffEffect) error { saved = b; return nil }).Once()
			_, _, err := f.svc.Add(context.Background(), charID, tt.add)
			require.NoError(t, err)
			var ids []string
			for _, b := range saved {
				ids = append(ids, b.ID)
			}
			assert.Equal(t, tt.want, ids)
		})
	}
}

// ADR 0004 step 20.5: the mark is kept in the buffs snapshot, so a void
// restores it with the list; a snapshot from before it reads false; and a
// buff activated by hand never carries it (the client cannot set it).
func TestBuffs_Step20_OnOtherRoundTrips(t *testing.T) {
	t.Run("the snapshot keeps it", func(t *testing.T) {
		f := newFixture(t)
		mine := buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 4}
		before := []buffsvc.BuffEffect{{ID: "w", Name: "Web", Duration: 30, OnOther: true}, mine}
		f.state(before, 3)
		f.records()
		f.saves([]buffsvc.BuffEffect{before[0]})
		_, _, err := f.svc.Dismiss(context.Background(), charID, "h", "")
		require.NoError(t, err)
		assert.Equal(t, true, decodeSnap(t, f.entries[0].Snapshots[0].Before)[0]["on_other"])
		assert.NotContains(t, decodeSnap(t, f.entries[0].Snapshots[0].Before)[1], "on_other", "absent for a buff on him")

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyBuffs).Restore(context.Background(), charID, f.entries[0].Snapshots[0].Before))
	})

	t.Run("activate drops a caller's mark", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 0)
		f.records()
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()
		got, err := f.svc.Activate(context.Background(), charID, buffsvc.BuffEffect{Name: "Web", Duration: 5, OnOther: true})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.False(t, got[0].OnOther)
	})
}
