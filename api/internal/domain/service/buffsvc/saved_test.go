package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

func TestSaveBuff(t *testing.T) {
	existing := []buffsvc.SavedBuff{{ID: 1, Name: "Ward", Duration: 5}, {ID: 2, Name: "Haste", Duration: 4}}
	tests := []struct {
		name    string
		save    buffsvc.SavedBuff
		want    []buffsvc.SavedBuff // what is written
		wantErr error
	}{
		{
			"appends",
			buffsvc.SavedBuff{Name: " Form ", Note: " n ", Duration: 3},
			[]buffsvc.SavedBuff{existing[0], existing[1], {Name: "Form", Duration: 3, Note: "n"}},
			nil,
		},
		{
			"same name replaces",
			buffsvc.SavedBuff{Name: "ward", Duration: 9},
			[]buffsvc.SavedBuff{existing[1], {Name: "ward", Duration: 9}},
			nil,
		},
		{"blank name", buffsvc.SavedBuff{Name: " "}, nil, domain.ErrInvalidInput},
		{"negative rounds", buffsvc.SavedBuff{Name: "X", Duration: -1}, nil, domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.wantErr == nil {
				f.dal.EXPECT().ListSavedBuffs(mock.Anything, charID).Return(existing, nil).Twice()
				f.dal.EXPECT().ReplaceSavedBuffs(mock.Anything, charID, tt.want).Return(nil).Once()
				f.records()
			}
			_, err := f.svc.SaveBuff(context.Background(), charID, tt.save)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindBuffChange, f.entries[0].Kind)
			assert.Equal(t, []string{buffsvc.StateKeySavedBuffs}, snapKeys(f.entries[0]))
			assert.Equal(t, "save_template", payload(t, f.entries[0])["action"])
		})
	}
}

func TestDeleteSaved(t *testing.T) {
	existing := []buffsvc.SavedBuff{{ID: 1, Name: "Ward"}, {ID: 2, Name: "Haste"}}
	t.Run("removes and logs, active buffs not read", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().ListSavedBuffs(mock.Anything, charID).Return(existing, nil).Twice()
		f.dal.EXPECT().ReplaceSavedBuffs(mock.Anything, charID, []buffsvc.SavedBuff{existing[1]}).Return(nil).Once()
		f.records()
		_, err := f.svc.DeleteSaved(context.Background(), charID, 1)
		require.NoError(t, err)
		assert.JSONEq(t, `{"action":"delete_template","buff":"Ward"}`, string(f.entries[0].Payload))
		assert.Contains(t, string(f.entries[0].Snapshots[0].Before), `"name":"Ward"`)
	})
	t.Run("unknown id", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().ListSavedBuffs(mock.Anything, charID).Return(existing, nil).Once()
		_, err := f.svc.DeleteSaved(context.Background(), charID, 99)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestSavedRestorer_WritesTheSnapshotBack(t *testing.T) {
	f := newFixture(t)
	want := []buffsvc.SavedBuff{{ID: 7, Name: "Form", Duration: 2, Note: "n", Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5)}}}
	f.dal.EXPECT().ReplaceSavedBuffs(mock.Anything, charID, want).Return(nil).Once()
	r, ok := f.svc.Restorers()[buffsvc.StateKeySavedBuffs]
	require.True(t, ok)
	require.NoError(t, r.Restore(context.Background(), charID,
		[]byte(`[{"id":7,"name":"Form","duration":2,"charges":0,"effects":{"weapon_buff":5},"note":"n"}]`)))
}
