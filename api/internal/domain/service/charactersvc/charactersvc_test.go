package charactersvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
)

func TestNew_NilReader(t *testing.T) {
	_, err := charactersvc.New(nil)
	require.Error(t, err)
}

func TestGetCharacter(t *testing.T) {
	want := &charactersvc.Character{ID: 3, Name: "Bard", Str: 18, Thac0Base: 16}

	tests := []struct {
		name    string
		ret     *charactersvc.Character
		retErr  error
		wantErr error
	}{
		{name: "found", ret: want},
		{name: "not found passes sentinel through", retErr: domain.ErrCharacterNotFound, wantErr: domain.ErrCharacterNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := mocks.NewMockReader(t)
			reader.EXPECT().GetCharacter(context.Background(), int64(3)).Return(tt.ret, tt.retErr)
			svc, err := charactersvc.NewWithConfig(charactersvc.DefaultConfig(), reader)
			require.NoError(t, err)

			got, err := svc.GetCharacter(context.Background(), 3)
			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr))
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}
