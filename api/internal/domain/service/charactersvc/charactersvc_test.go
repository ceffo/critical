package charactersvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
)

// tx runs the body in the caller's context and counts the transactions.
type tx struct{ n int }

func (t *tx) InTx(ctx context.Context, fn func(context.Context) error) error {
	t.n++
	return fn(ctx)
}

type env struct {
	svc      charactersvc.SVC
	reader   *mocks.MockReader
	owners   *mocks.MockOwners
	sessions *mocks.MockSessions
	tx       *tx
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		reader: mocks.NewMockReader(t), owners: mocks.NewMockOwners(t), sessions: mocks.NewMockSessions(t), tx: &tx{},
	}
	svc, err := charactersvc.NewWithConfig(charactersvc.DefaultConfig(), charactersvc.Deps{
		Reader: e.reader, Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

func TestNew_NilDependency(t *testing.T) {
	full := charactersvc.Deps{
		Reader: mocks.NewMockReader(t), Owners: mocks.NewMockOwners(t), Tx: &tx{}, Sessions: mocks.NewMockSessions(t),
	}
	for name, d := range map[string]charactersvc.Deps{
		"reader":   {Owners: full.Owners, Tx: full.Tx, Sessions: full.Sessions},
		"owners":   {Reader: full.Reader, Tx: full.Tx, Sessions: full.Sessions},
		"tx":       {Reader: full.Reader, Owners: full.Owners, Sessions: full.Sessions},
		"sessions": {Reader: full.Reader, Owners: full.Owners, Tx: full.Tx},
	} {
		_, err := charactersvc.New(d)
		require.Error(t, err, name)
	}
}

var owner = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// ADR 0029 steps 7.1 and 7.3: the list is the owner's port answer, as it is.
func TestList_IsTheOwnersCharacters(t *testing.T) {
	e := newEnv(t)
	want := []charactersvc.Summary{{ID: 1, Name: "Kelsen", Class: "bard", Level: 15}}
	e.owners.EXPECT().CharactersOf(mock.Anything, owner).Return(want, nil).Once()
	got, err := e.svc.List(context.Background(), owner)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// ADR 0029 step 7.2: setting the current character writes it in one
// transaction, then forgets the owner's cached sessions (after the commit, so
// a lookup cannot cache the old one).
func TestSetCurrent_Step7_2_WritesThenForgetsTheSessions(t *testing.T) {
	e := newEnv(t)
	set := e.owners.EXPECT().SetCurrentCharacter(mock.Anything, owner, int64(4)).Return(nil).Once()
	e.sessions.EXPECT().ForgetUser(owner).Once().NotBefore(set)
	require.NoError(t, e.svc.SetCurrent(context.Background(), owner, 4))
	assert.Equal(t, 1, e.tx.n)
}

// ADR 0029 step 7.1: another user's character (the port matches none) is
// ErrCharacterNotFound, and no session is forgotten.
func TestSetCurrent_Step7_1_AForeignCharacterIsNotFound(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, owner, int64(9)).Return(domain.ErrCharacterNotFound).Once()
	err := e.svc.SetCurrent(context.Background(), owner, 9)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)
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
			e := newEnv(t)
			e.reader.EXPECT().GetCharacter(context.Background(), int64(3)).Return(tt.ret, tt.retErr)

			got, err := e.svc.GetCharacter(context.Background(), 3)
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

// Score reads each ability's own field of the sheet (distinct values, so a
// crossed field fails) and refuses an unknown key.
func TestCharacter_Score(t *testing.T) {
	c := &charactersvc.Character{Str: 11, Dex: 12, Con: 13, Int: 14, Wis: 15, Cha: 16}
	want := map[charactersvc.Ability]int{"str": 11, "dex": 12, "con": 13, "int": 14, "wis": 15, "cha": 16}
	require.Len(t, charactersvc.Abilities, len(want))
	for _, a := range charactersvc.Abilities {
		got, ok := c.Score(a)
		assert.True(t, ok, a)
		assert.Equal(t, want[a], got, a)
	}
	_, ok := c.Score("luck")
	assert.False(t, ok)
}
