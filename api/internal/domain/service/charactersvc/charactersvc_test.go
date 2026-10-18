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
	xp       *mocks.MockXPLedger
	rules    *mocks.MockRules
	owners   *mocks.MockOwners
	sessions *mocks.MockSessions
	tx       *tx
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		reader: mocks.NewMockReader(t), xp: mocks.NewMockXPLedger(t), rules: newRules(t),
		owners: mocks.NewMockOwners(t), sessions: mocks.NewMockSessions(t), tx: &tx{},
	}
	svc, err := charactersvc.NewWithConfig(charactersvc.DefaultConfig(), charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

func TestNew_NilDependency(t *testing.T) {
	full := charactersvc.Deps{
		Reader: mocks.NewMockReader(t), XP: mocks.NewMockXPLedger(t), Rules: newRules(t),
		Owners: mocks.NewMockOwners(t), Tx: &tx{}, Sessions: mocks.NewMockSessions(t),
	}
	without := map[string]func(d *charactersvc.Deps){
		"reader":   func(d *charactersvc.Deps) { d.Reader = nil },
		"xp":       func(d *charactersvc.Deps) { d.XP = nil },
		"rules":    func(d *charactersvc.Deps) { d.Rules = nil },
		"owners":   func(d *charactersvc.Deps) { d.Owners = nil },
		"tx":       func(d *charactersvc.Deps) { d.Tx = nil },
		"sessions": func(d *charactersvc.Deps) { d.Sessions = nil },
	}
	for name, drop := range without {
		d := full
		drop(&d)
		_, err := charactersvc.New(d)
		require.Error(t, err, name)
	}
}

var owner = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// ADR 0029 steps 7.1 and 7.3: the list is the owner's port answer, each
// character at its realized level (ADR 0030 step 7): Kelsen's XP gives 16,
// level 16 is not taken yet, the list says 15.
func TestList_IsTheOwnersCharactersAtTheirRealizedLevel(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().CharactersOf(mock.Anything, owner).
		Return([]charactersvc.Summary{{ID: 1, Name: "Kelsen", Class: "bard"}}, nil).Once()
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(kelsen(), nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(1)).Return(26_582, nil).Once()
	e.rules.EXPECT().LevelFor("bard", 1_320_000).Return(16, nil).Once()
	e.rules.EXPECT().XPThreshold("bard", 17).Return(1_540_000, nil).Once()
	e.rules.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
	e.rules.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	e.rules.EXPECT().BardSpellsPerDay(15).Return(level15Slots, nil).Once()
	e.rules.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	e.rules.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Once()
	got, err := e.svc.List(context.Background(), owner)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.Summary{{ID: 1, Name: "Kelsen", Class: "bard", Level: 15}}, got)
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

// GetCharacter derives the character's block from what the reader returns
// and what the XP ledger's entries add (ADR 0030 steps 3 to 8), and passes a
// reader's error through.
func TestGetCharacter(t *testing.T) {
	t.Run("derived from the reader's character and the ledger", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenRules(e.rules, 1_293_418, 15)

		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		want := kelsen()
		want.Derived = kelsenDerived()
		assert.Equal(t, want, got)
	})
	t.Run("not found passes sentinel through", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(nil, domain.ErrCharacterNotFound).Once()
		got, err := e.svc.GetCharacter(context.Background(), 3)
		assert.True(t, errors.Is(err, domain.ErrCharacterNotFound))
		assert.Nil(t, got)
	})
	t.Run("a ledger error is the read's", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		boom := errors.New("boom")
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, boom).Once()
		_, err := e.svc.GetCharacter(context.Background(), 3)
		require.ErrorIs(t, err, boom)
	})
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
