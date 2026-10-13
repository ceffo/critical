package spellsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newSvc(t *testing.T) (spellsvc.SVC, *mocks.MockDAL, *mocks.MockReader) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	reader := mocks.NewMockReader(t)
	svc, err := spellsvc.NewWithConfig(spellsvc.DefaultConfig(), dal, reader)
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
	_, err := spellsvc.New(nil, mocks.NewMockReader(t))
	require.Error(t, err)
	_, err = spellsvc.New(mocks.NewMockDAL(t), nil)
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
	f := spellsvc.Filter{Query: "fire", Class: "wizard", Level: &lvl, School: "Evo"}
	spells := []spellsvc.Spell{{Name: "Fireball", Class: "wizard", Level: 3}}
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)

	got, err := svc.Search(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, spells, got)
}

func spell(name string, r spellstats.Raw) spellsvc.Spell {
	return spellsvc.Spell{Name: name, Stats: spellstats.Parse(r)}
}

func searchNames(t *testing.T, f spellsvc.Filter, spells ...spellsvc.Spell) []string {
	t.Helper()
	svc, _, reader := newSvc(t)
	reader.EXPECT().Search(mock.Anything, f).Return(spells, nil)
	got, err := svc.Search(context.Background(), f)
	require.NoError(t, err)
	out := []string{}
	for _, s := range got {
		out = append(out, s.Name)
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

func TestSearch_ReturnsEveryMatchWithoutACap(t *testing.T) {
	many := make([]spellsvc.Spell, 900)
	assert.Len(t, searchNames(t, spellsvc.Filter{}, many...), len(many))
}
