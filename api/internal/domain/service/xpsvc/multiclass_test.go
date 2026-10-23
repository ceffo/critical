package xpsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
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
			f.chars.EXPECT().View(mock.Anything, int64(1)).Return(viewOf(tc.entries, viewtest.WithScores(tc.str, 0, 0, tc.int, 0, 0)), nil).Once()
			f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil).Once()
			got, err := f.svc.Get(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.BonusEligible)
		})
	}
}

// ADR 0042 step 5.1: a multi-class character's award goes in equal shares to
// every active entry, floored; a preview and a ledger entry carry the split so
// the web words it without dividing.
func TestEntrySplit_Step5_1_ThePreviewAndTheLedgerCarryTheShares(t *testing.T) {
	f := newFixture(t)
	two := []classes.Entry{classes.SingleEntry("mage", 1, 0), classes.SingleEntry("cleric", 1, 0)}
	f.chars.EXPECT().ClassEntries(mock.Anything, int64(1)).Return(two, nil).Once()
	f.chars.EXPECT().View(mock.Anything, int64(1)).Return(viewOf(two), nil)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{{ID: 3, Base: 1001}}, nil)
	want := []xpsvc.ClassAmount{{Class: "mage", Amount: 500}, {Class: "cleric", Amount: 500}}

	got, err := f.svc.Preview(context.Background(), 1, 1001, false)
	require.NoError(t, err)
	assert.Equal(t, want, got.Split)

	l, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, want, l.Entries[0].Split)
}

// Step 5.1: three active entries share 1001 as 333 each; the remainder is lost.
func TestEntrySplit_Step5_1_ThreeEntriesLoseTheRemainder(t *testing.T) {
	f := newFixture(t)
	f.entries(classes.SingleEntry("fighter", 1, 0), classes.SingleEntry("mage", 1, 0), classes.SingleEntry("thief", 1, 0))
	got, err := f.svc.Preview(context.Background(), 1, 1001, false)
	require.NoError(t, err)
	assert.Equal(t, []xpsvc.ClassAmount{{Class: "fighter", Amount: 333}, {Class: "mage", Amount: 333}, {Class: "thief", Amount: 333}}, got.Split)
}

// Step 5.1 with step 5: a frozen entry earns nothing, so a dual-class
// character's one active entry takes the whole award: one share, no split.
func TestEntrySplit_Step5_1_ADualClassCharacterHasOneShare(t *testing.T) {
	f := newFixture(t)
	f.entries(classes.Entry{Class: "fighter", RealizedLevel: 4, State: classes.EntryFrozen, LeftAtLevel: 4}, classes.SingleEntry("mage", 1, 0))
	got, err := f.svc.Preview(context.Background(), 1, 1001, false)
	require.NoError(t, err)
	assert.Nil(t, got.Split)
}

// The logged payload carries the split on the entry, and on `before` for an
// edit; a delete carries it on the removed entry. Read with the narrow entries
// read, not a full derive.
func TestLoggedPayload_Step5_1_CarriesTheSplitOnEntryAndBefore(t *testing.T) {
	two := []classes.Entry{classes.SingleEntry("mage", 1, 0), classes.SingleEntry("cleric", 1, 0)}
	stored := xpsvc.Entry{ID: 3, Date: "2026-10-10", Reason: "ogre", Base: 1001}
	list := func(f fixture) {
		f.chars.EXPECT().View(mock.Anything, int64(1)).Return(viewOf(two), nil)
		f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	}

	t.Run("add", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(7, nil).Once()
		rec := f.recorded()
		f.entries(two...)
		list(f)
		_, err := f.svc.Add(context.Background(), 1, xpsvc.EntryInput{Base: 1001})
		require.NoError(t, err)
		assert.Contains(t, string(rec.Payload), `"split":[{"class":"mage","amount":500},{"class":"cleric","amount":500}]`)
	})
	t.Run("edit", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
		f.dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
		rec := f.recorded()
		f.entries(two...)
		list(f)
		base := 2001
		_, err := f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &base})
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"entry":{"id":3,"date":"2026-10-10","session":"","reason":"ogre","base":2001,"bonus":false,"bonusAmount":0,"total":2001,"split":[{"class":"mage","amount":1000},{"class":"cleric","amount":1000}]},
			"before":{"id":3,"date":"2026-10-10","session":"","reason":"ogre","base":1001,"bonus":false,"bonusAmount":0,"total":1001,"split":[{"class":"mage","amount":500},{"class":"cleric","amount":500}]}
		}`, string(rec.Payload))
	})
	t.Run("delete", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
		f.dal.EXPECT().Delete(mock.Anything, int64(1), int64(3)).Return(nil).Once()
		rec := f.recorded()
		f.entries(two...)
		list(f)
		_, err := f.svc.Delete(context.Background(), 1, 3)
		require.NoError(t, err)
		assert.Contains(t, string(rec.Payload), `"split":[{"class":"mage","amount":500},{"class":"cleric","amount":500}]`)
		assert.NotContains(t, string(rec.Payload), `"before"`)
	})
}

// A failing entries read inside the transaction aborts the write: the error
// leaves the unit of work (the real runner rolls back) and nothing is logged.
func TestAdd_AFailingEntriesReadFailsTheWriteAndLogsNothing(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(7, nil).Once()
	boom := errors.New("read failed")
	f.chars.EXPECT().ClassEntries(mock.Anything, int64(1)).Return(nil, boom).Once()
	_, err := f.svc.Add(context.Background(), 1, xpsvc.EntryInput{Base: 100})
	require.ErrorIs(t, err, boom)
	assert.Equal(t, 1, f.tx.n) // the failure happened inside the one unit of work; no Record (the mock would fail)
}

// viewOf is a view with the stored entries es, each at its realized level and
// its target level 3 (the first one's, the primary's, is the level read).
func viewOf(es []classes.Entry, opts ...viewtest.Option) charactersvc.View {
	var all []viewtest.Option
	for _, e := range es {
		if e.State == classes.EntryFrozen {
			all = append(all, viewtest.WithFrozen(e.Class, e.LeftAtLevel))
		} else {
			all = append(all, viewtest.WithEntry(e.Class, e.RealizedLevel))
		}
	}
	v := viewtest.Multi(append(all, opts...)...)
	return v
}

// ADR 0042 step 5: a dual-class that crossed over in play ignores the awards up
// to its cut-off. The new mage starts at 0 XP, so the ledger total, the
// progress and the per-entry split count only the award after the cut-off.
func TestGet_Step5_ADualClassCrossedInPlayIgnoresTheEarlierAwards(t *testing.T) {
	f := newFixture(t)
	v := viewtest.Multi(
		viewtest.WithFrozen("fighter", 4), viewtest.WithEntry("mage", 2), viewtest.WithIgnoredAwards("mage", 5),
	)
	f.chars.EXPECT().View(mock.Anything, int64(1)).Return(v, nil)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{
		{ID: 7, Base: 3000}, {ID: 3, Base: 9000},
	}, nil)

	got, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 3000, got.Total, "only the award after the cut-off counts")
	assert.Equal(t, 2500, got.CurrentFloor)
	assert.Equal(t, 3000, got.Entries[0].Total)
	assert.Nil(t, got.Entries[0].Split, "one active entry: no split")
}

// The same cut-off keeps an ignoring entry out of a multi-class split.
func TestGet_Step5_1_AnEntryIgnoringAnAwardIsOutOfItsSplit(t *testing.T) {
	f := newFixture(t)
	v := viewtest.Multi(
		viewtest.WithEntry("mage", 1), viewtest.WithEntry("cleric", 1), viewtest.WithIgnoredAwards("cleric", 5),
	)
	f.chars.EXPECT().View(mock.Anything, int64(1)).Return(v, nil)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{{ID: 7, Base: 1000}, {ID: 3, Base: 1000}}, nil)

	got, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []xpsvc.ClassAmount{{Class: "mage", Amount: 500}, {Class: "cleric", Amount: 500}}, got.Entries[0].Split)
	assert.Equal(t, []xpsvc.ClassAmount{{Class: "mage", Amount: 500}}, got.Entries[1].Split)
}

// Opening is the primary entry's starting XP, for a single, a multi and a
// dual-class character (what the stored XP total was).
func TestGet_Opening_IsThePrimaryEntrysStartingXP(t *testing.T) {
	withXP := func(v charactersvc.View, xp ...int) charactersvc.View {
		for i, x := range xp {
			v.Entries[i].StartingXP = x
		}
		return v
	}
	for name, v := range map[string]charactersvc.View{
		"single": withXP(viewtest.Single("mage", 2), 2600),
		"multi":  withXP(viewtest.Multi(viewtest.WithEntry("mage", 2), viewtest.WithEntry("cleric", 2)), 2600, 2600),
		"dual":   withXP(viewtest.Multi(viewtest.WithFrozen("fighter", 4), viewtest.WithEntry("mage", 2), viewtest.WithDormant()), 9000, 2600),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().View(mock.Anything, int64(1)).Return(v, nil)
			f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
			got, err := f.svc.Get(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, 2600, got.Opening)
		})
	}
}
