package spellsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
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

func TestSearch(t *testing.T) {
	svc, _, reader := newSvc(t)
	spells := []spellsvc.Spell{{Name: "Sleep", Class: "bard", Level: 1}}
	reader.EXPECT().Search(mock.Anything, "sle", "bard").Return(spells, nil)

	got, err := svc.Search(context.Background(), "sle", "bard")
	require.NoError(t, err)
	assert.Equal(t, spells, got)
}
