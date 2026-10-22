package xpsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// ADR 0042 step 6: "Eligible when every prime requisite of every active
// entry's class is 16 or more; a frozen entry's class does not count (it
// earns nothing)." The progress reads the mage's table (the stub has no
// fighter's), so the mage is the first entry.
func TestBonusEligible_Step6_EveryActiveClassesPrimeRequisites(t *testing.T) {
	active := func(class string) classes.Entry { return classes.SingleEntry(class, 3, 0) }
	frozen := classes.Entry{Class: "fighter", RealizedLevel: 4, State: classes.EntryFrozen, LeftAtLevel: 4}
	for _, tc := range []struct {
		name     string
		str, int int
		entries  []classes.Entry
		want     bool
	}{
		{"a mage/fighter of Str 16, Int 15", 16, 15, []classes.Entry{active("mage"), active("fighter")}, false},
		{"a mage/fighter of Str 15, Int 16", 15, 16, []classes.Entry{active("mage"), active("fighter")}, false},
		{"a mage/fighter of Str 16, Int 16", 16, 16, []classes.Entry{active("mage"), active("fighter")}, true},
		{"a dual-class mage of Int 16 whose frozen fighter's Str is 15", 15, 16, []classes.Entry{frozen, active("mage")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(&charactersvc.Character{
				ID: 1, Class: "mage", Str: tc.str, Int: tc.int, Classes: tc.entries,
				Derived: charactersvc.Derived{TargetLevel: 3},
			}, nil).Once()
			f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil).Once()
			got, err := f.svc.Get(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.BonusEligible)
		})
	}
}

// ADR 0042 step 5.1: a multi-class character's award goes in equal shares to
// every active entry, floored; a preview, a ledger entry and the logged
// payload all carry the split so the web words it without dividing.
func TestEntrySplit_Step5_1_ThePreviewTheLedgerAndTheLogCarryTheShares(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(&charactersvc.Character{
		ID: 1, Class: "mage", Derived: charactersvc.Derived{TargetLevel: 1},
		Classes: []classes.Entry{classes.SingleEntry("mage", 1, 0), classes.SingleEntry("cleric", 1, 0)},
	}, nil)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{{ID: 3, Base: 1001}}, nil)
	want := []xpsvc.ClassAmount{{Class: "mage", Amount: 500}, {Class: "cleric", Amount: 500}}

	got, err := f.svc.Preview(context.Background(), 1, 1001, false)
	require.NoError(t, err)
	assert.Equal(t, want, got.Split)

	l, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, want, l.Entries[0].Split)
}
