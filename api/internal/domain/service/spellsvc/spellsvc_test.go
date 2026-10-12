package spellsvc_test

import (
	"context"
	"errors"
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

func TestCast(t *testing.T) {
	tests := []struct {
		name      string
		index     int
		wantErr   error
		wantSaved bool
	}{
		{name: "unused slot is consumed", index: 0, wantSaved: true},
		{name: "already used", index: 1, wantErr: spellsvc.ErrSlotAlreadyUsed},
		{name: "negative index", index: -1, wantErr: spellsvc.ErrInvalidSlotIndex},
		{name: "index past end", index: 3, wantErr: spellsvc.ErrInvalidSlotIndex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal, _ := newSvc(t)
			dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return(slots(), nil)
			if tt.wantSaved {
				want := slots()
				want[tt.index].Used = true
				dal.EXPECT().SaveSlots(mock.Anything, int64(1), want).Return(nil)
			}

			got, err := svc.Cast(context.Background(), 1, tt.index)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
				assert.Nil(t, got)
				dal.AssertNotCalled(t, "SaveSlots", mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			assert.True(t, got[tt.index].Used)
			// Only the cast slot changes.
			assert.True(t, got[1].Used)
			assert.False(t, got[2].Used)
		})
	}
}

func TestCast_DoesNotMutateDALSlice(t *testing.T) {
	svc, dal, _ := newSvc(t)
	orig := slots()
	dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return(orig, nil)
	dal.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)

	_, err := svc.Cast(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.False(t, orig[0].Used)
}

func TestCast_SaveError(t *testing.T) {
	svc, dal, _ := newSvc(t)
	boom := errors.New("boom")
	dal.EXPECT().GetSlots(mock.Anything, int64(1)).Return(slots(), nil)
	dal.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(boom)

	_, err := svc.Cast(context.Background(), 1, 0)
	assert.True(t, errors.Is(err, boom))
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
