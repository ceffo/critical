package combatsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func rows(r ...[3]int) []combatsvc.CritRow {
	out := make([]combatsvc.CritRow, len(r))
	for i, x := range r {
		out[i] = combatsvc.CritRow{From: x[0], To: x[1], Multiplier: x[2]}
	}
	return out
}

func TestValidateCritRows(t *testing.T) {
	tests := []struct {
		name    string
		in      []combatsvc.CritRow
		want    []combatsvc.CritRow // sorted rows when valid
		wantErr string              // substring naming the bad row
	}{
		{name: "one row covers everything", in: rows([3]int{1, 100, 2}), want: rows([3]int{1, 100, 2})},
		{
			name: "three ranges", in: rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4}),
			want: rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4}),
		},
		{
			name: "rows in any order are sorted", in: rows([3]int{91, 100, 4}, [3]int{1, 90, 2}),
			want: rows([3]int{1, 90, 2}, [3]int{91, 100, 4}),
		},
		{name: "single-value ranges", in: rows([3]int{1, 1, 5}, [3]int{2, 100, 2}), want: rows([3]int{1, 1, 5}, [3]int{2, 100, 2})},
		{name: "no rows", in: nil, wantErr: "at least one row"},
		{name: "gap in the middle", in: rows([3]int{1, 49, 2}, [3]int{51, 100, 3}), wantErr: "gap before row 2 (51-100): 50-50"},
		{name: "gap at the start", in: rows([3]int{2, 100, 2}), wantErr: "gap before row 1 (2-100): 1-1"},
		{name: "gap at the end", in: rows([3]int{1, 99, 2}), wantErr: "gap after row 1 (1-99): 100-100"},
		{name: "overlap by one", in: rows([3]int{1, 50, 2}, [3]int{50, 100, 3}), wantErr: "row 2 (50-100) overlaps"},
		{name: "duplicate start", in: rows([3]int{1, 100, 2}, [3]int{1, 10, 3}), wantErr: "row 2 (1-10) overlaps"},
		{name: "from below 1", in: rows([3]int{0, 100, 2}), wantErr: "row 1 (0-100) is outside 1-100"},
		{name: "to above 100", in: rows([3]int{1, 50, 2}, [3]int{51, 101, 2}), wantErr: "row 2 (51-101) is outside 1-100"},
		{name: "backwards range", in: rows([3]int{1, 100, 2}, [3]int{60, 50, 2}), wantErr: "row 2 (60-50) ends before it starts"},
		{name: "multiplier 0", in: rows([3]int{1, 50, 2}, [3]int{51, 100, 0}), wantErr: "row 2 (51-100) has multiplier 0"},
		{name: "negative multiplier", in: rows([3]int{1, 100, -3}), wantErr: "row 1 (1-100) has multiplier -3"},
		{name: "multiplier 1 is allowed", in: rows([3]int{1, 100, 1}), want: rows([3]int{1, 100, 1})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := combatsvc.ValidateCritRows(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetCritTable(t *testing.T) {
	t.Run("a valid table is stored sorted", func(t *testing.T) {
		f := newFixture(t)
		want := rows([3]int{1, 40, 2}, [3]int{41, 100, 3})
		f.crit.EXPECT().Replace(mock.Anything, want).Return(nil).Once()
		got, err := f.svc.SetCritTable(context.Background(), rows([3]int{41, 100, 3}, [3]int{1, 40, 2}))
		require.NoError(t, err)
		assert.Equal(t, combatsvc.CritTable{Rows: want}, got)
	})
	t.Run("an invalid table never reaches the store", func(t *testing.T) {
		f := newFixture(t) // the mock fails the test on an unexpected Replace
		_, err := f.svc.SetCritTable(context.Background(), rows([3]int{1, 40, 2}, [3]int{50, 100, 3}))
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
	t.Run("a store failure is returned", func(t *testing.T) {
		f := newFixture(t)
		f.crit.EXPECT().Replace(mock.Anything, mock.Anything).Return(errors.New("disk full")).Once()
		_, err := f.svc.SetCritTable(context.Background(), rows([3]int{1, 100, 2}))
		require.ErrorContains(t, err, "disk full")
	})
}

func TestGetCritTable(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.GetCritTable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, combatsvc.CritTable{Rows: rows([3]int{1, 100, 2}), Placeholder: true}, got)

	f.critRows = rows([3]int{1, 100, 3})
	got, err = f.svc.GetCritTable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, combatsvc.CritTable{Rows: rows([3]int{1, 100, 3})}, got)
}

// The crit d100 is looked up in the stored table. Target AC 100 makes the
// roll needed so low that a d20 of 20 against any d100 is a crit; the long
// sword at STR 12 deals ((5 × 1) + 2) × multiplier.
func TestResolveAttack_CritLooksUpTheTable(t *testing.T) {
	table := rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4})
	tests := []struct {
		name        string
		table       []combatsvc.CritRow // nil: the unedited default
		d100        int
		mult        buffsvc.BuffModifiers
		wantMult    int
		wantFrom    int
		wantTo      int
		wantPlaceh  bool
		wantTotal   int
		wantTermSrc string
		wantTermDet string
		wantTermVal float64
	}{
		{name: "first value of the first range", table: table, d100: 1, wantMult: 2, wantFrom: 1, wantTo: 50, wantTotal: 14,
			wantTermSrc: "Critical hit (d100 1 in 1-50)", wantTermDet: "1-50", wantTermVal: 2},
		{name: "last value of the first range", table: table, d100: 50, wantMult: 2, wantFrom: 1, wantTo: 50, wantTotal: 14,
			wantTermSrc: "Critical hit (d100 50 in 1-50)", wantTermDet: "1-50", wantTermVal: 2},
		{name: "first value of the second range", table: table, d100: 51, wantMult: 3, wantFrom: 51, wantTo: 90, wantTotal: 21,
			wantTermSrc: "Critical hit (d100 51 in 51-90)", wantTermDet: "51-90", wantTermVal: 3},
		{name: "last value of the second range", table: table, d100: 90, wantMult: 3, wantFrom: 51, wantTo: 90, wantTotal: 21,
			wantTermSrc: "Critical hit (d100 90 in 51-90)", wantTermDet: "51-90", wantTermVal: 3},
		{name: "first value of the last range", table: table, d100: 91, wantMult: 4, wantFrom: 91, wantTo: 100, wantTotal: 28,
			wantTermSrc: "Critical hit (d100 91 in 91-100)", wantTermDet: "91-100", wantTermVal: 4},
		{name: "100 is in the last range", table: table, d100: 100, wantMult: 4, wantFrom: 91, wantTo: 100, wantTotal: 28,
			wantTermSrc: "Critical hit (d100 100 in 91-100)", wantTermDet: "91-100", wantTermVal: 4},
		{name: "the default table is a x2 placeholder", table: nil, d100: 77, wantMult: 2, wantFrom: 1, wantTo: 100, wantPlaceh: true,
			wantTotal: 14, wantTermSrc: "Critical hit (placeholder, d100 77)", wantTermDet: "placeholder", wantTermVal: 2},
		{
			// x3 from the table times x2 from a buff: ((5 × 1) + 2) × 6 = 42.
			name: "table multiplier stacks with a buff multiplier", table: table, d100: 60, wantMult: 3, wantFrom: 51, wantTo: 90, wantTotal: 42,
			mult: buffsvc.BuffModifiers{
				SpellsBuff: 1, Multipliers: 2, EffectiveStr: 12,
				MultipliersFrom: []buffsvc.Factor{{Buff: "Gem", Value: 2}},
			},
			wantTermSrc: "Critical hit (d100 60 in 51-90)", wantTermDet: "51-90", wantTermVal: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.critRows = tt.table
			mods := tt.mult
			if mods.EffectiveStr == 0 {
				mods = strMods(12)
			}
			f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(mods, nil)
			f.dice.EXPECT().RollD20().Return(20)
			f.dice.EXPECT().RollD100().Return(tt.d100)
			f.dice.EXPECT().RollDie(8).Return(5)
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 100})
			require.NoError(t, err)
			assert.True(t, res.IsCrit)
			assert.Equal(t, tt.d100, res.D100CritRoll)
			assert.Equal(t, tt.wantMult, res.CritMultiplier)
			assert.Equal(t, tt.wantFrom, res.CritFrom)
			assert.Equal(t, tt.wantTo, res.CritTo)
			assert.Equal(t, tt.wantPlaceh, res.CritPlaceholder)
			assert.Equal(t, tt.wantTotal, res.TotalDamage)
			var term *combatsvc.DamageTerm
			for i := range res.DamageTerms {
				if res.DamageTerms[i].Kind == combatsvc.ModCrit {
					term = &res.DamageTerms[i]
				}
			}
			require.NotNil(t, term)
			assert.Equal(t, combatsvc.DamageTerm{
				Term: combatsvc.TermMultiplier, Kind: combatsvc.ModCrit,
				Source: tt.wantTermSrc, Detail: tt.wantTermDet, Value: tt.wantTermVal,
			}, *term)

			// The log entry keeps the multiplier and range it was resolved with.
			require.Len(t, f.entries, 1)
			assert.Contains(t, string(f.entries[0].Payload), `"critMultiplier":`)
			assert.Contains(t, string(f.entries[0].Payload), `"critFrom":`)
		})
	}
}

// A hit that is not a crit never consults the table row (no lookup, x1).
func TestResolveAttack_NoCritIgnoresTable(t *testing.T) {
	f := newFixture(t)
	f.critRows = rows([3]int{1, 100, 9})
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
	f.dice.EXPECT().RollD20().Return(15)
	f.dice.EXPECT().RollD100().Return(10) // 15 - 10 = 5 < 6 needed
	f.dice.EXPECT().RollDie(8).Return(5)
	f.records()
	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
	require.NoError(t, err)
	assert.False(t, res.IsCrit)
	assert.Equal(t, 1, res.CritMultiplier)
	assert.Equal(t, 7, res.TotalDamage)
}
