package charactersvc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ledgers answers both ledgers' totals of the characters.
type ledgers struct{ hp, psp map[int64]int }

func (l ledgers) HPTotal(_ context.Context, id int64) (int, error)  { return l.hp[id], nil }
func (l ledgers) PSPTotal(_ context.Context, id int64) (int, error) { return l.psp[id], nil }

func copyEnv(t *testing.T, l ledgers) *env {
	t.Helper()
	e := newEnv(t)
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Armour: e.armour, HP: l,
		Spells: e.spells, Book: e.book, FreeSpells: e.free,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// ADR 0029 step 12: the service checks the source is readable, reads the source's two ledger totals
// in the same transaction and hands them to the store as the copy's opening balances, with the
// name the caller sent, trimmed. It makes nothing current and forgets no session.
func TestCopy_Step12_ReadsTheSourcesTotalsAndStoresTheCopy(t *testing.T) {
	owner := uuid.New()
	e := copyEnv(t, ledgers{hp: map[int64]int{4: -11}, psp: map[int64]int{4: -3}})
	e.owners.EXPECT().ViewableCharacter(mock.Anything, int64(4)).Return(nil).Once()
	e.owners.EXPECT().CopyCharacter(mock.Anything, owner, int64(4), "Bram (copy)", charactersvc.Openings{HP: -11, PSP: -3}).Return(9, nil).Once()

	id, err := e.svc.Copy(context.Background(), owner, 4, "  Bram (copy) ")
	require.NoError(t, err)
	assert.Equal(t, int64(9), id)
	assert.Equal(t, 1, e.tx.n, "one transaction: the read of the totals and the write are one")
}

// Step 12.1: an unreadable source is the port's refusal, and no ledger is read or copy stored.
func TestCopy_Step12_UnreadableSourceIsNotFound(t *testing.T) {
	e := copyEnv(t, ledgers{})
	e.owners.EXPECT().ViewableCharacter(mock.Anything, int64(5)).Return(domain.ErrCharacterNotFound).Once()

	_, err := e.svc.Copy(context.Background(), uuid.New(), 5, "x")
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

// A blank or too long name is a field error before anything is read.
func TestCopy_Step12_NameIsRequiredAndBounded(t *testing.T) {
	for name, text := range map[string]string{"blank": "   ", "empty": "", "too long": strings.Repeat("é", charactersvc.MaxNameLen+1)} {
		e := copyEnv(t, ledgers{})
		_, err := e.svc.Copy(context.Background(), uuid.New(), 4, text)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, name)
		assert.Equal(t, "name", fe.Field, name)
	}
	e := copyEnv(t, ledgers{})
	e.owners.EXPECT().ViewableCharacter(mock.Anything, int64(4)).Return(nil).Once()
	e.owners.EXPECT().CopyCharacter(mock.Anything, mock.Anything, int64(4), strings.Repeat("é", charactersvc.MaxNameLen), mock.Anything).Return(2, nil).Once()
	_, err := e.svc.Copy(context.Background(), uuid.New(), 4, strings.Repeat("é", charactersvc.MaxNameLen))
	require.NoError(t, err, "the limit counts characters, not bytes")
}
