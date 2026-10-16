package spellsvc_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

func newSvc(t *testing.T) (spellsvc.SVC, *mocks.MockDAL, *mocks.MockReader) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	reader := mocks.NewMockReader(t)
	svc, err := spellsvc.NewWithConfig(spellsvc.DefaultConfig(), dal, reader, mocks.NewMockSpellbook(t))
	require.NoError(t, err)
	return svc, dal, reader
}

func slots() []spellsvc.SpellSlot {
	return []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 1, PreparedSpellName: "Charm Person", Used: true},
		{Level: 2, PreparedSpellName: "Invisibility"},
	}
}

func TestNew_NilDeps(t *testing.T) {
	_, err := spellsvc.New(nil, mocks.NewMockReader(t), mocks.NewMockSpellbook(t))
	require.Error(t, err)
	_, err = spellsvc.New(mocks.NewMockDAL(t), nil, mocks.NewMockSpellbook(t))
	require.Error(t, err)
	_, err = spellsvc.New(mocks.NewMockDAL(t), mocks.NewMockReader(t), nil)
	require.Error(t, err)
}

func TestGetSlots(t *testing.T) {
	svc, dal, _ := newSvc(t)
	dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return(slots(), nil)

	got, err := svc.GetSlots(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, slots(), got)
}

func TestResetSlots(t *testing.T) {
	svc, dal, _ := newSvc(t)
	dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return(slots(), nil)
	want := slots()
	want[1].Used = false
	dal.EXPECT().SaveSlots(mock.Anything, int64(1), want).Return(nil)

	got, err := svc.ResetSlots(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for _, s := range got {
		assert.False(t, s.Used)
	}
	assert.Equal(t, "Charm Person", got[1].PreparedSpellName)
}

func TestSearch_PassesTheFilterToTheReader(t *testing.T) {
	svc, _, reader := newSvc(t)
	lvl := 3
	f := spellsvc.Filter{Query: "fire", Class: "wizard", Levels: []int{lvl}, School: spellsvc.SchoolEvocation}
	spells := []spellsvc.Spell{{ID: "fireball", Name: localized.Of("Fireball"), Class: "wizard", Level: 3, School: localized.Of("Evocation")}}
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)
	expectDetails(reader, spells)

	got, err := svc.Search(context.Background(), f, spellsvc.PageRequest{})
	require.NoError(t, err)
	assert.Equal(t, spells, got.Items)
	assert.Equal(t, 1, got.Total)
}

func spell(name string, r spellstats.Raw) spellsvc.Spell {
	return spellsvc.Spell{ID: name, Name: localized.Of(name), Stats: spellstats.Parse(r)}
}

// named is a spell known by its English name only (its id too).
func named(name string) spellsvc.Spell {
	return spellsvc.Spell{ID: name, Name: localized.Of(name)}
}

// expectDetails answers Details from spells as the DAL does: the spells of
// ids, in that order.
func expectDetails(reader *mocks.MockReader, spells []spellsvc.Spell) {
	reader.EXPECT().Details(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ids []string) ([]spellsvc.Spell, error) {
			byID := map[string]spellsvc.Spell{}
			for _, s := range spells {
				byID[s.ID] = s
			}
			out := make([]spellsvc.Spell, 0, len(ids))
			for _, id := range ids {
				if s, ok := byID[id]; ok {
					out = append(out, s)
				}
			}
			return out, nil
		}).Maybe()
}

func searchNames(t *testing.T, f spellsvc.Filter, spells ...spellsvc.Spell) []string {
	t.Helper()
	svc, _, reader := newSvc(t)
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)
	expectDetails(reader, spells)
	got, err := svc.Search(context.Background(), f, spellsvc.PageRequest{PageSize: spellsvc.MaxPageSize})
	require.NoError(t, err)
	out := []string{}
	for _, s := range got.Items {
		out = append(out, s.Name.Default())
	}
	return out
}

func TestSearch_StatFilters(t *testing.T) {
	vs := spell("VS", spellstats.Raw{Components: "V, S", SavingThrow: "1/2", CastingTime: "3"})
	vsm := spell("VSM", spellstats.Raw{Components: "V, S, M", SavingThrow: "Neg.", CastingTime: "1 rd."})
	either := spell("Either", spellstats.Raw{Components: "V", SavingThrow: "Neg. or 1/2", CastingTime: "1 turn"})
	odd := spell("Odd", spellstats.Raw{Components: "V, S (M optional)", SavingThrow: "None", CastingTime: "Special"})
	all := []spellsvc.Spell{vs, vsm, either, odd}

	tests := []struct {
		name string
		f    spellsvc.Filter
		want []string
	}{
		{"no stat filter keeps all", spellsvc.Filter{}, []string{"VS", "VSM", "Either", "Odd"}},
		{"component needed", spellsvc.Filter{Components: []string{"M"}}, []string{"VSM", "Odd"}},
		{"all listed components needed", spellsvc.Filter{Components: []string{"V", "S", "M"}}, []string{"VSM", "Odd"}},
		{"V alone", spellsvc.Filter{Components: []string{"V"}}, []string{"VS", "VSM", "Either", "Odd"}},
		{"save half includes 'Neg. or 1/2'", spellsvc.Filter{Save: spellstats.SaveHalf}, []string{"VS", "Either"}},
		{"save negates includes 'Neg. or 1/2'", spellsvc.Filter{Save: spellstats.SaveNegates}, []string{"VSM", "Either"}},
		{"save none", spellsvc.Filter{Save: spellstats.SaveNone}, []string{"Odd"}},
		{"casting time in segments", spellsvc.Filter{CastingTime: "segment"}, []string{"VS"}},
		{"casting time in rounds", spellsvc.Filter{CastingTime: "round"}, []string{"VSM"}},
		{"casting time special", spellsvc.Filter{CastingTime: "special"}, []string{"Odd"}},
		{"filters combine", spellsvc.Filter{Components: []string{"S"}, Save: spellstats.SaveHalf}, []string{"VS"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, searchNames(t, tt.f, all...))
		})
	}
}

func numbered(n int, mk func(i int) spellsvc.Spell) []spellsvc.Spell {
	out := make([]spellsvc.Spell, n)
	for i := range out {
		out[i] = mk(i)
	}
	return out
}

func page(t *testing.T, f spellsvc.Filter, p spellsvc.PageRequest, spells []spellsvc.Spell) spellsvc.Page {
	t.Helper()
	svc, _, reader := newSvc(t)
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)
	expectDetails(reader, spells)
	got, err := svc.Search(context.Background(), f, p)
	require.NoError(t, err)
	return got
}

func pageNames(p spellsvc.Page) []string {
	out := []string{}
	for _, s := range p.Items {
		out = append(out, s.Name.Default())
	}
	return out
}

func TestSearch_Paging(t *testing.T) {
	all := numbered(25, func(i int) spellsvc.Spell { return named(fmt.Sprintf("S%02d", i)) })
	tests := []struct {
		name      string
		req       spellsvc.PageRequest
		first     string
		last      string
		count     int
		page, siz int
	}{
		{"first page", spellsvc.PageRequest{Page: 1, PageSize: 10}, "S00", "S09", 10, 1, 10},
		{"middle page", spellsvc.PageRequest{Page: 2, PageSize: 10}, "S10", "S19", 10, 2, 10},
		{"last partial page", spellsvc.PageRequest{Page: 3, PageSize: 10}, "S20", "S24", 5, 3, 10},
		{"past the end", spellsvc.PageRequest{Page: 4, PageSize: 10}, "", "", 0, 4, 10},
		{"defaults", spellsvc.PageRequest{}, "S00", "S24", 25, 1, spellsvc.DefaultPageSize},
		{"size clamped", spellsvc.PageRequest{Page: 1, PageSize: 9999}, "S00", "S24", 25, 1, spellsvc.MaxPageSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := page(t, spellsvc.Filter{}, tt.req, all)
			assert.Equal(t, 25, got.Total, "total counts every match")
			assert.Equal(t, tt.page, got.Page)
			assert.Equal(t, tt.siz, got.PageSize)
			require.Len(t, got.Items, tt.count)
			if tt.count > 0 {
				assert.Equal(t, tt.first, got.Items[0].Name.Default())
				assert.Equal(t, tt.last, got.Items[tt.count-1].Name.Default())
			}
		})
	}
}

func TestSearch_PagesCoverEveryMatchOnceInReaderOrder(t *testing.T) {
	all := numbered(900, func(i int) spellsvc.Spell { return named(fmt.Sprintf("S%03d", i)) })
	var seen []string
	for p := 1; ; p++ {
		got := page(t, spellsvc.Filter{}, spellsvc.PageRequest{Page: p, PageSize: spellsvc.MaxPageSize}, all)
		if len(got.Items) == 0 {
			break
		}
		seen = append(seen, pageNames(got)...)
	}
	require.Len(t, seen, 900)
	assert.True(t, slices.IsSorted(seen), "stable order across pages")
	assert.Len(t, slices.Compact(slices.Clone(seen)), 900, "no spell on two pages")
}

func TestSearch_TotalAndPagesCountOnlyStatMatches(t *testing.T) {
	// Every third spell needs M; the stat filter runs before paging.
	all := numbered(30, func(i int) spellsvc.Spell {
		c := "V, S"
		if i%3 == 0 {
			c = "V, S, M"
		}
		s := spell(fmt.Sprintf("S%02d", i), spellstats.Raw{Components: c})
		return s
	})
	f := spellsvc.Filter{Components: []string{"M"}}
	first := page(t, f, spellsvc.PageRequest{Page: 1, PageSize: 4}, all)
	assert.Equal(t, 10, first.Total)
	assert.Equal(t, []string{"S00", "S03", "S06", "S09"}, pageNames(first))
	last := page(t, f, spellsvc.PageRequest{Page: 3, PageSize: 4}, all)
	assert.Equal(t, []string{"S24", "S27"}, pageNames(last), "a full last page is not padded with non-matches")
	assert.Equal(t, 10, last.Total)
}

// A page's full texts are read by one Details call holding exactly the
// page's ids, in page order: never the whole match list.
func TestSearch_ReadsThePageTextsInOneBatch(t *testing.T) {
	svc, _, reader := newSvc(t)
	all := numbered(25, func(i int) spellsvc.Spell { return named(fmt.Sprintf("S%02d", i)) })
	reader.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(all, nil)
	full := []spellsvc.Spell{
		{ID: "S10", Name: localized.Of("S10"), Description: localized.Of("Ten.")},
		{ID: "S11", Name: localized.Of("S11"), Description: localized.Of("Eleven.")},
	}
	reader.EXPECT().Details(mock.Anything, []string{"S10", "S11"}).Return(full, nil).Once()

	got, err := svc.Search(context.Background(), spellsvc.Filter{}, spellsvc.PageRequest{Page: 6, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, full, got.Items, "the page carries the full texts Details read")
	assert.Equal(t, 25, got.Total)
}

// ADR 0008 step 9: a long rest ends the extra slots.
func TestResetSlots_DropsExtraSlots(t *testing.T) {
	svc, dal, _ := newSvc(t)
	dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true},
		{Level: 2, Extra: true},
	}, nil)
	dal.EXPECT().SaveSlots(mock.Anything, int64(1), []spellsvc.SpellSlot{{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm"}}).Return(nil)

	got, err := svc.ResetSlots(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
